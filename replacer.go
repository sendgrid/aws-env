package awsenv

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	setenv  = os.Setenv
	environ = os.Environ
)

// DefaultPrefix holds the standard environment value prefix.
var DefaultPrefix = "awsenv:"

// ParamsGetter represents a data source that can translate parameter names
// or paths into parameter values.
type ParamsGetter interface {
	GetParams(ctx context.Context, names []string) (map[string]string, error)
}

// LimitedParamsGetter represents a ParamsGetter that can describe its own
// request limit. A ParamsGetter implementing this interface will not be
// given more names per request than the number returned by GetParamsLimit,
// unless it returns a value <= 0, which will be interpreted as unlimited.
type LimitedParamsGetter interface {
	ParamsGetter
	GetParamsLimit() int
}

// NewReplacer returns a Replacer that will operate on env vars with the
// given value prefix, using the given ParamsGetter.
//
// NewReplacer will panic if envValuePrefix is the empty string.
func NewReplacer(envValuePrefix string, ssm ParamsGetter) *Replacer {
	if envValuePrefix == "" {
		panic("awsenv: envValuePrefix must be non-empty")
	}

	return &Replacer{
		ssm:    ssm,
		prefix: envValuePrefix,
	}
}

// Replacer handles replacing existing environment variables with values
// retrieved from AWS Parameter Store.
type Replacer struct {
	ssm    ParamsGetter
	prefix string
}

// ReplaceAll overwrites applicable environment variables with values
// retrieved from Parameter Store. ReplaceAll will attempt to replace
// as many values as possible.
func (r *Replacer) ReplaceAll(ctx context.Context) error {
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

// MustReplaceAll overwrites the applicable environment generating a panic if something goes wrong.
func (r *Replacer) MustReplaceAll(ctx context.Context) {
	err := r.ReplaceAll(ctx)
	if err != nil {
		panic(err)
	}
}

// Replacements returns a map of environment variable names to new values
// that have been fetched from Parameter Store.
func (r *Replacer) Replacements(ctx context.Context) (map[string]string, error) {
	// environment variables parsed
	envvars := parseEnvironment(environ())

	// param path
	pathvars := r.filterPaths(envvars)

	// param path -> env value
	pathvals, err := fetch(ctx, r.ssm, pathvars)
	if err != nil {
		return nil, err
	}

	envvars = r.applyParamPathValues(envvars, pathvals)
	return envvars, nil
}

// filterPaths filters out all the path.
func (r *Replacer) filterPaths(envvars map[string]string) []string {
	if len(envvars) == 0 {
		return []string{}
	}
	// param path
	values := make([]string, 0, len(envvars))

	for _, value := range envvars {
		if !strings.HasPrefix(value, r.prefix) {
			continue
		}

		values = append(values, strings.TrimPrefix(value, r.prefix))
	}

	return values
}

// applyParamPathValues takes applies values from src keys translated through
func (r *Replacer) applyParamPathValues(srcEnv map[string]string, replaceWithValues map[string]string) map[string]string {
	for name, value := range srcEnv {
		// If the value lacks a prefix we skip it.
		if !strings.HasPrefix(value, r.prefix) {
			continue
		}

		lookupValue := strings.TrimPrefix(value, r.prefix)
		if val, ok := replaceWithValues[lookupValue]; ok {
			srcEnv[name] = val
		}
	}
	return srcEnv
}

func fetch(ctx context.Context, ssm ParamsGetter, paths []string) (map[string]string, error) {
	eg, ctx := errgroup.WithContext(ctx)

	var limit int

	lpg, ok := ssm.(LimitedParamsGetter)
	if ok {
		limit = lpg.GetParamsLimit()
	}

	batches := chunk(limit, paths)
	results := make([]map[string]string, len(batches))

	for i := range batches {

		// copied to avoid race condition
		i := i
		batch := batches[i]

		eg.Go(func() error {
			var err error
			results[i], err = ssm.GetParams(ctx, batch)
			return err
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	// merge separate batch results into a single map
	dest := make(map[string]string, len(paths))
	merge(dest, results)

	for _, path := range paths {
		_, ok := dest[path]
		if !ok {
			return dest, errors.Errorf("awsenv: param not found: %q", path)
		}
	}

	return dest, nil
}

// SSMGetParametersAPI defines the interface for the GetParameters function.
// We use this interface to test the function using a mocked service.
type SSMGetParametersAPI interface {
	GetParameters(ctx context.Context,
		params *ssm.GetParametersInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
}

// NewParamsGetter implements ParamsGetter using AWS SDK v2 SSM client.
func NewParamsGetter(ssmClient SSMGetParametersAPI) LimitedParamsGetter {
	return &ssmFetcher{ssmClient, true}
}

// NewParamsGetterFromConfig creates a new ParamsGetter using AWS SDK v2 with the provided config.
func NewParamsGetterFromConfig(cfg aws.Config) LimitedParamsGetter {
	return NewParamsGetter(ssm.NewFromConfig(cfg))
}

type ssmFetcher struct {
	ssm     SSMGetParametersAPI
	decrypt bool
}

func (f *ssmFetcher) GetParamsLimit() int { return 10 }

func (f *ssmFetcher) GetParams(ctx context.Context, names []string) (map[string]string, error) {
	input := &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: &f.decrypt,
	}

	resp, err := f.ssm.GetParameters(ctx, input)
	if err != nil {
		return nil, err
	}

	m := make(map[string]string, len(resp.Parameters))
	for _, param := range resp.Parameters {
		m[*param.Name] = *param.Value
	}

	return m, nil
}

// MustReplaceEnv replaces the environment with values from SSM parameter store using AWS SDK v2.
func MustReplaceEnv() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	replacer := NewReplacer(DefaultPrefix, NewParamsGetterFromConfig(cfg))
	replacer.MustReplaceAll(ctx)
}

// MustReplaceEnvWithContext replaces the environment with values from SSM parameter store using AWS SDK v2.
// This function allows you to pass a custom context.
func MustReplaceEnvWithContext(ctx context.Context) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	replacer := NewReplacer(DefaultPrefix, NewParamsGetterFromConfig(cfg))
	replacer.MustReplaceAll(ctx)
}

// MustReplaceEnvWithConfig replaces the environment with values from SSM parameter store using the provided AWS config.
func MustReplaceEnvWithConfig(ctx context.Context, cfg aws.Config) {
	replacer := NewReplacer(DefaultPrefix, NewParamsGetterFromConfig(cfg))
	replacer.MustReplaceAll(ctx)
}
