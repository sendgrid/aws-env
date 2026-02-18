package awsenv

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplacer_panic(t *testing.T) {
	t.Parallel()
	mockGetter := mockParamsGetter(func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("no implementation")
	})

	require.Panics(t, func() { NewReplacer("", mockGetter) })
}

func TestReplacer_MustReplaceAll(t *testing.T) {
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

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
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

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

func TestReplacer_ReplaceAll_MultipleSameValue(t *testing.T) {
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

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

func TestReplacer_ReplaceAll_MultipleSameValueNotMatchingValue(t *testing.T) {
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

	env := fakeEnv{
		"DB_PASSWORD":             "test",                        // no matching prefix
		"SOME_SECRET":             "awsenv:/param/path/here",     // match
		"SOME_OTHER_SECRET":       "awsenv:/param/path/here/v2",  // match
		"SOME_OTHER_DUPLICATED":   "awsenv:/param/path/here/v2",  // match
		"SOME_OTHER_NOT_REPLACED": "pre:/param/path/ignore/here", // not a matching prefix
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

func TestReplacer_ReplaceAll_NotFound(t *testing.T) {
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

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

func TestReplacer_ReplaceAll_Missing(t *testing.T) {
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

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

func (m mockParamStore) GetParams(_ context.Context, paths []string) (map[string]string, error) {
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

func TestReplacer_filterPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		prefix string
		input  map[string]string
		want   []string
	}{
		{
			name:   "empty",
			prefix: "",
			input:  nil,
			want:   nil,
		},
		{
			name:   "default",
			prefix: "awsenv:",
			input:  map[string]string{"X": "1", "Y": "pre:/y", "Z": "awsenv:/z"},
			want:   []string{"/z"},
		},
		{
			name:   "using_a_nondefault",
			prefix: "pre:",
			input:  map[string]string{"X": "1", "Y": "pre:/y", "Z": "awsenv:/z"},
			want:   []string{"/y"},
		},
		{
			name:   "fully_qualified_arn",
			prefix: "awsenv:",
			input: map[string]string{
				"X": "1",
				"Y": "awsenv:arn:aws:ssm:us-east-1:123456789012:parameter/cross/account/secret",
			},
			want: []string{"arn:aws:ssm:us-east-1:123456789012:parameter/cross/account/secret"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := Replacer{prefix: test.prefix}
			got, want := r.filterPaths(test.input), test.want
			require.Equal(t, want, got, "filterPaths(%q) = %v, want %v", test.input, got, want)
		})
	}
}

func TestReplacer_applyParamPathValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		prefix            string
		src               map[string]string
		replaceWithValues map[string]string
		want              map[string]string
	}{
		{
			name:              "empty",
			prefix:            "awsenv:",
			src:               nil,
			replaceWithValues: nil,
			want:              nil,
		},
		{
			name:              "replace_with_empty",
			prefix:            "awsenv:",
			src:               map[string]string{},
			replaceWithValues: map[string]string{"x": "a"},
			want:              map[string]string{},
		},
		{
			name:              "replace_with_multiple_prefix",
			prefix:            "pre:",
			src:               map[string]string{"x": "pre:/a", "y": "pre:/b", "z": "awsenv:/c"},
			replaceWithValues: map[string]string{"/a": "A", "/b": "B", "/c": "C"},
			want:              map[string]string{"x": "A", "y": "B", "z": "awsenv:/c"},
		},
		{
			name:              "replace_empty_with_default",
			prefix:            "awsenv:",
			src:               map[string]string{"x": "1"},
			replaceWithValues: nil,
			want:              map[string]string{"x": "1"},
		},
		{
			name:              "replace_with_default",
			prefix:            "awsenv:",
			src:               map[string]string{"x": "awsenv:/a", "y": "awsenv:/b"},
			replaceWithValues: map[string]string{"/a": "A", "/b": "B"},
			want:              map[string]string{"x": "A", "y": "B"},
		},
		{
			name:   "replace_with_fully_qualified_arn",
			prefix: "awsenv:",
			src: map[string]string{
				"CROSS_ACCOUNT_SECRET": "awsenv:arn:aws:ssm:us-east-1:123456789012:parameter/cross/account/secret",
			},
			replaceWithValues: map[string]string{"/cross/account/secret": "secret_value"},
			want:              map[string]string{"CROSS_ACCOUNT_SECRET": "secret_value"},
		},
		{
			name:   "replace_mixed_arn_and_plain",
			prefix: "awsenv:",
			src: map[string]string{
				"LOCAL_SECRET":         "awsenv:/local/secret",
				"CROSS_ACCOUNT_SECRET": "awsenv:arn:aws:ssm:us-west-2:999999999999:parameter/remote/secret",
			},
			replaceWithValues: map[string]string{
				"/local/secret":  "local_val",
				"/remote/secret": "remote_val",
			},
			want: map[string]string{
				"LOCAL_SECRET":         "local_val",
				"CROSS_ACCOUNT_SECRET": "remote_val",
			},
		},
	}

	for idx, test := range tests {
		test := test
		t.Run(fmt.Sprintf(test.name, idx), func(t *testing.T) {
			t.Parallel()
			r := &Replacer{prefix: test.prefix}
			got, want := r.applyParamPathValues(test.src, test.replaceWithValues), test.want
			require.Equal(t, want, got, "applyParamPathValues(%v, %v) = %v, want %v", test.src, test.replaceWithValues, got, want)
		})
	}
}

func TestReplacer_ReplaceAll_FullyQualifiedARN(t *testing.T) {
	// Not parallel: this test mutates global environ/setenv via fakeEnv.install()
	origEnviron := environ
	origSetenv := setenv
	t.Cleanup(func() {
		environ = origEnviron
		setenv = origSetenv
	})

	env := fakeEnv{
		"DB_PASSWORD":          "test",
		"LOCAL_SECRET":         "awsenv:/param/path/here",
		"CROSS_ACCOUNT_SECRET": "awsenv:arn:aws:ssm:us-east-1:123456789012:parameter/remote/secret",
	}
	env.install()

	getter := mockParamsGetter(func(_ context.Context, paths []string) (map[string]string, error) {
		store := map[string]string{
			"/param/path/here": "local_val",
			"/remote/secret":   "remote_val",
		}
		// SSM API returns results keyed by plain parameter path even when queried with full ARN
		result := make(map[string]string, len(paths))
		for _, p := range paths {
			plain := stripARNPrefix(p)
			val, ok := store[plain]
			if !ok {
				return nil, fmt.Errorf("not found: %s", p)
			}
			result[plain] = val
		}
		return result, nil
	})

	r := NewReplacer(DefaultPrefix, getter)
	ctx := context.Background()
	err := r.ReplaceAll(ctx)

	require.NoError(t, err)

	want := fakeEnv{
		"DB_PASSWORD":          "test",
		"LOCAL_SECRET":         "local_val",
		"CROSS_ACCOUNT_SECRET": "remote_val",
	}
	require.Equal(t, want, env)
}
