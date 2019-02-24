package awsenv

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"golang.org/x/sync/errgroup"
)

const batchLimit = 10

var (
	setenv  = os.Setenv
	environ = os.Environ
)

// ParameterGetter is implemented by the ssm client.
type ParameterGetter interface {
	GetParameter(*ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
}

// ParametersGetter is more efficient than ParameterGetter and is also
// implemented by the ssm client.
type ParametersGetter interface {
	GetParametersWithContext(aws.Context, *ssm.GetParametersInput, ...request.Option) (*ssm.GetParametersOutput, error)
}

// NewReplacer returns a Replacer that will operate on env vars with the
// given value prefix, using the given ParamsGetter. If ssm also implements
// ParametersGetter, that interface will be used instead.
//
func NewReplacer(envValuePrefix string, ssm ParameterGetter) *Replacer {
	return &Replacer{
		ssm:    wrap(ssm),
		prefix: envValuePrefix,
	}
}

// Replacer handles replacing existing environment variables with values
// retrieved from AWS Parameter Store.
type Replacer struct {
	ssm    ParametersGetter
	prefix string
}

// Apply overwrites applicable environment variables with values retrieved
// from Parameter Store. ReplaceAll will attempt to replace as many values
// as possible, after which it will return the first error that occurred.
func (r *Replacer) Apply(ctx context.Context) error {
	vars, err := r.Replacements(ctx)
	if err != nil {
		return err
	}

	for name, val := range vars {
		suberr := setenv(name, val)
		if err == nil && suberr != nil {
			err = suberr
		}
	}

	return err
}

// Replacements returns a map of environment variable names to new values
// that have been fetched from Parameter Store.
func (r *Replacer) Replacements(ctx context.Context) (map[string]string, error) {
	// param path -> env name
	pathvars := pathmap(r.prefix, environ())

	// param path -> env value
	pathvals, err := r.fetch(ctx, keys(pathvars))
	if err != nil {
		return nil, err
	}

	// env name -> env value
	dest := make(map[string]string, len(pathvals))
	translate(dest, pathvars, pathvals)

	if len(dest) != len(pathvars) {
		err = errors.New("missing parameters")
	}

	return dest, err
}

// ReplaceAll behaves like Replacements, but does not accept a context. To
// directly overwrite values in the process environment, use Apply.
func (r *Replacer) ReplaceAll() (map[string]string, error) {
	return r.Replacements(context.Background())
}

func (r *Replacer) fetch(ctx context.Context, paths []string) (map[string]string, error) {

	// eg is not ctx-wrapped, since we want to fetch as many params as
	// possible, even if some fail. External cancellations on the passed ctx
	// will be respected.
	var eg errgroup.Group

	decryption := true
	batches := chunk(batchLimit, paths)
	results := make([][]*ssm.Parameter, len(batches))

	for i := range batches {

		// copied to avoid race condition
		i := i
		batch := batches[i]

		eg.Go(func() error {
			input := &ssm.GetParametersInput{
				Names:          aws.StringSlice(batch),
				WithDecryption: &decryption,
			}

			output, err := r.ssm.GetParametersWithContext(ctx, input)
			if err == nil {
				results[i] = output.Parameters
			}

			return err
		})
	}

	err := eg.Wait()

	// merge separate batch results into a single map
	dest := make(map[string]string, len(paths))

	for _, params := range results {
		for _, param := range params {
			dest[*param.Name] = *param.Value
		}
	}

	return dest, err
}

func wrap(ssm ParameterGetter) ParametersGetter {
	v, ok := ssm.(ParametersGetter)
	if ok {
		return v
	}
	return wrapper{ssm}
}

// wrapper implements ParametersGetter backed by a ParameterGetter.
type wrapper struct{ ParameterGetter }

func (w wrapper) GetParametersWithContext(_ aws.Context, input *ssm.GetParametersInput, _ ...request.Option) (*ssm.GetParametersOutput, error) {
	var firstErr error

	output := new(ssm.GetParametersOutput)

	for _, name := range input.Names {
		singleInput := &ssm.GetParameterInput{
			Name:           name,
			WithDecryption: input.WithDecryption,
		}

		singleOutput, err := w.GetParameter(singleInput)
		if err == nil && singleOutput.Parameter != nil {
			output.Parameters = append(output.Parameters, singleOutput.Parameter)
			continue
		}

		if firstErr == nil {
			firstErr = err
		}

		output.InvalidParameters = append(output.InvalidParameters, name)
	}

	return output, firstErr
}
