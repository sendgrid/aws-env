package awsenv

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			want:   []string{},
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

// mockSSMClient implements SSMGetParametersAPI for testing
type mockSSMClient struct {
	getParametersFunc func(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
}

func (m *mockSSMClient) GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
	return m.getParametersFunc(ctx, params, optFns...)
}

func TestNewParamsGetter(t *testing.T) {
	t.Parallel()
	mockClient := &mockSSMClient{
		getParametersFunc: func(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
			param1 := types.Parameter{Name: stringPtr("/test/param1"), Value: stringPtr("value1")}
			param2 := types.Parameter{Name: stringPtr("/test/param2"), Value: stringPtr("value2")}

			return &ssm.GetParametersOutput{
				Parameters: []types.Parameter{param1, param2},
			}, nil
		},
	}

	getter := NewParamsGetter(mockClient)
	require.NotNil(t, getter)
	require.Equal(t, 10, getter.GetParamsLimit())

	ctx := context.Background()
	result, err := getter.GetParams(ctx, []string{"/test/param1", "/test/param2"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"/test/param1": "value1",
		"/test/param2": "value2",
	}, result)
}

func TestNewParamsGetter_Error(t *testing.T) {
	t.Parallel()
	mockClient := &mockSSMClient{
		getParametersFunc: func(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
			return nil, errors.New("SSM error")
		},
	}

	getter := NewParamsGetter(mockClient)
	ctx := context.Background()
	_, err := getter.GetParams(ctx, []string{"/test/param1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "SSM error")
}

func TestSSMFetcher_GetParamsLimit(t *testing.T) {
	t.Parallel()
	mockClient := &mockSSMClient{}
	getter := NewParamsGetter(mockClient)
	require.Equal(t, 10, getter.GetParamsLimit())
}

// stringPtr returns a pointer to the string value
func stringPtr(s string) *string {
	return &s
}
