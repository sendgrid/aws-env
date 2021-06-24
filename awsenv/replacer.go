package awsenv

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sync/errgroup"
)

var (
	setenv   = os.Setenv
	unsetenv = os.Unsetenv
	environ  = os.Environ
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
func NewReplacer(envValuePrefix string, unsetNotFound bool, ssm ParamsGetter) *Replacer {
	if envValuePrefix == "" {
		panic("awsenv: envValuePrefix must be non-empty")
	}

	return &Replacer{
		unsetNotFound: unsetNotFound,
		ssm:           ssm,
		prefix:        envValuePrefix,
	}
}

// Replacer handles replacing existing environment variables with values
// retrieved from AWS Parameter Store.
type Replacer struct {
	unsetNotFound bool
	ssm           ParamsGetter
	prefix        string
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

	unmapped := unmappedVars(r.prefix, environ())
	if len(unmapped) > 0 && !r.unsetNotFound {
		return fmt.Errorf("following variables not found in AWS: %v", unmapped)
	}
	for _, name := range unmapped {
		suberr := unsetenv(name)
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
	pathvals, err := fetch(ctx, r.ssm, keys(pathvars))
	if err != nil {
		return nil, err
	}

	// env name -> env value
	dest := make(map[string]string, len(pathvals))
	translate(dest, pathvars, pathvals)

	return dest, nil
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

	return dest, nil
}
