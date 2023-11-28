package awsenv

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplacer_panic(t *testing.T) {
	mockGetter := mockParamsGetter(func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("no implementation")
	})

	require.Panics(t, func() { NewReplacer("", mockGetter) })
}

func TestReplacer_MustReplaceAll(t *testing.T) {
	env := fakeEnv{
		"DB_PASSWORD": "test",                    // no matching prefix
		"SOME_SECRET": "awsenv:/param/path/here", // match
	}
	env.install()

	var params mockParamStore

	r := NewReplacer("awsenv:", params)

	ctx := context.Background()

	require.Panics(t, func() { r.MustReplaceAll(ctx) })
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

func TestReplacerMultipleSameValue(t *testing.T) {
	env := fakeEnv{
		"DB_PASSWORD":             "test",                        // no matching prefix
		"SOME_SECRET":             "awsenv:/param/path/here",     // match
		"SOME_OTHER_SECRET":       "awsenv:/param/path/here/v2",  // match
		"SOME_OTHER_DUPLICATED":   "awsenv:/param/path/here/v2",  // match
		"SOME_OTHER_NOT_REPLACED": "pre:/param/path/ignore/here", //not a matching prefix
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
		"DB_PASSWORD":             "test",                        // unchanged
		"SOME_SECRET":             "val1",                        // replaced
		"SOME_OTHER_SECRET":       "val2",                        // replaced
		"SOME_OTHER_DUPLICATED":   "val2",                        // replaced
		"SOME_OTHER_NOT_REPLACED": "pre:/param/path/ignore/here", // unchanged
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

func TestFilterPaths(t *testing.T) {
	tests := []struct {
		prefix string
		input  map[string]string
		want   []string
	}{
		{
			prefix: "",
			input:  nil,
			want:   []string{},
		},
		{
			prefix: "awsenv:",
			input:  map[string]string{"X": "1", "Y": "pre:/y", "Z": "awsenv:/z"},
			want:   []string{"/z"},
		},
		{
			prefix: "pre:",
			input:  map[string]string{"X": "1", "Y": "pre:/y", "Z": "awsenv:/z"},
			want:   []string{"/y"},
		},
	}

	for _, test := range tests {
		r := Replacer{prefix: test.prefix}
		got, want := r.filterPaths(test.input), test.want
		if !reflect.DeepEqual(got, want) {
			t.Errorf("filterPaths(%q) = %v, want %v", test.input, got, want)
		}
	}
}

func TestApplyPaths(t *testing.T) {
	tests := []struct {
		prefix            string
		src               map[string]string
		replaceWithValues map[string]string
		want              map[string]string
	}{
		{
			prefix:            "awsenv:",
			src:               nil,
			replaceWithValues: nil,
			want:              nil,
		},
		{
			prefix:            "awsenv:",
			src:               map[string]string{},
			replaceWithValues: map[string]string{"x": "a"},
			want:              map[string]string{},
		},
		{
			prefix:            "pre:",
			src:               map[string]string{"x": "pre:/a", "y": "pre:/b", "z": "awsenv:/c"},
			replaceWithValues: map[string]string{"/a": "A", "/b": "B", "/c": "C"},
			want:              map[string]string{"x": "A", "y": "B", "z": "awsenv:/c"},
		},
		{
			prefix:            "awsenv:",
			src:               map[string]string{"x": "1"},
			replaceWithValues: nil,
			want:              map[string]string{"x": "1"},
		},
		{
			prefix:            "awsenv:",
			src:               map[string]string{"x": "awsenv:/a", "y": "awsenv:/b"},
			replaceWithValues: map[string]string{"/a": "A", "/b": "B"},
			want:              map[string]string{"x": "A", "y": "B"},
		},
	}

	for idx, test := range tests {
		test := test
		t.Run(fmt.Sprintf("test_%v", idx), func(t *testing.T) {
			r := &Replacer{prefix: test.prefix}
			got, want := r.applyParamPathValues(test.src, test.replaceWithValues), test.want
			if !reflect.DeepEqual(got, want) {
				t.Errorf("applyPaths(%v, %v) -> %v, want %v",
					test.src, test.replaceWithValues, got, test.want)
			}
		})
	}
}
