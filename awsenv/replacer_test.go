package awsenv

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplacer_panic(t *testing.T) {
	mockGetter := mockParamsGetter(func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("no implementation")
	})

	require.Panics(t, func() { NewReplacer("", mockGetter) })
}

func TestReplacer_ReplaceAll_noop(t *testing.T) {
	mockGetter := mockParamsGetter(func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("forced")
	})

	env := fakeEnv{}
	env.install()

	ctx := context.Background()
	r := NewReplacer("awsenv:", mockGetter)
	err := r.ReplaceAll(ctx)
	require.NoError(t, err, "expected no error")
	require.Empty(t, env)
}

func TestReplacerMultiple(t *testing.T) {
	env := fakeEnv{
		"DB_PASSWORD":       "test",                       // no matching prefix
		"SOME_SECRET":       "awsenv:/param/path/here",    // match
		"SOME_OTHER_SECRET": "awsenv:/param/path/here/v2", // match
	}
	env.install()

	params := mockParamStore{
		"/param/path/here":    "val1",
		"/param/path/here/v2": "val2",
	}

	r := NewReplacer(DefaultPrefix, params)

	ctx := context.Background()
	err := r.ReplaceAll(ctx)

	require.NoError(t, err, "expected no error")

	want := fakeEnv{
		"DB_PASSWORD":       "test", // unchanged
		"SOME_SECRET":       "val1", // replaced
		"SOME_OTHER_SECRET": "val2", // replaced
	}

	require.Equal(t, want, env)
}

func TestReplacerNotFound(t *testing.T) {
	env := fakeEnv{
		"DB_PASSWORD": "test",                    // no matching prefix
		"SOME_SECRET": "awsenv:/param/path/here", // match
	}
	env.install()

	var params mockParamStore

	r := NewReplacer("awsenv:", params)

	ctx := context.Background()
	err := r.ReplaceAll(ctx)

	require.Error(t, err, "expected an error")
}

func TestReplacerMissing(t *testing.T) {
	env := fakeEnv{
		"SOME_SECRET": "awsenv:/param/path/here/doesnt/exist", // match
	}
	env.install()

	getter := func(context.Context, []string) (map[string]string, error) {
		return nil, nil
	}

	r := NewReplacer("awsenv:", mockParamsGetter(getter))
	ctx := context.Background()

	err := r.ReplaceAll(ctx)
	require.Error(t, err, "expected an error")
}

type mockParamStore map[string]string

func (m mockParamStore) GetParams(ctx context.Context, paths []string) (map[string]string, error) {
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		fmt.Printf("looking for path %s", path)
		val, ok := m[path]
		if !ok {
			return nil, errors.New("not found")
		}
		result[path] = val
	}
	return result, nil
}

type fakeEnv map[string]string

func (e fakeEnv) install() {
	environ = e.environ
	setenv = e.setenv
}

func (e fakeEnv) environ() []string {
	vars := make([]string, 0, len(e))
	for name, val := range e {
		vars = append(vars, name+"="+val)
	}
	return vars
}

func (e fakeEnv) setenv(name, val string) error {
	e[name] = val
	return nil
}

type mockParamsGetter func(context.Context, []string) (map[string]string, error)

func (f mockParamsGetter) GetParams(ctx context.Context, paths []string) (map[string]string, error) {
	return f(ctx, paths)
}
