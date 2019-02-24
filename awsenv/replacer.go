package awsenv

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	setenv  = os.Setenv
	environ = os.Environ
)

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
func NewReplacer(envValuePrefix string, ssm ParamsGetter) *Replacer {
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
// as many values as possible, after which it will return the first error
// that occurred.
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

	return dest, nil
}

func (r *Replacer) fetch(ctx context.Context, paths []string) (map[string]string, error) {
	eg, ctx := errgroup.WithContext(ctx)

	var limit int

	lpg, ok := r.ssm.(LimitedParamsGetter)
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
			results[i], err = r.ssm.GetParams(ctx, batch)
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
