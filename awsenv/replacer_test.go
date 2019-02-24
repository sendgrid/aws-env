package awsenv

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var _ ParametersGetter = (*ssm.SSM)(nil)

func TestReplacer_Apply_noop(t *testing.T) {
	env := fakeEnv{}
	env.install()

	mockGetter := mockParameterGetter(func(string) (string, error) {
		return "", errors.New("forced")
	})

	ctx := context.Background()
	r := NewReplacer("awsenv:", mockGetter)
	err := r.Apply(ctx)
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

	r := NewReplacer("awsenv:", params)

	ctx := context.Background()
	err := r.Apply(ctx)

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
	err := r.Apply(ctx)

	require.Error(t, err, "expected an error")
}

func TestReplacerMissing(t *testing.T) {
	env := fakeEnv{
		"SOME_SECRET": "awsenv:/param/path/here/doesnt/exist", // match
	}
	env.install()

	getter := func(name string) (string, error) {
		return "", errSimulateMissing
	}

	r := NewReplacer("awsenv:", mockParameterGetter(getter))
	ctx := context.Background()

	err := r.Apply(ctx)
	require.Error(t, err, "expected an error")
}

type mockParamStore map[string]string

func (m mockParamStore) GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	return mockParameterGetter(m.fetch).GetParameter(input)
}

func (m mockParamStore) fetch(name string) (string, error) {
	val, ok := m[name]
	if !ok {
		return "", errors.Errorf("parameter %q not found", name)
	}
	return val, nil
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

// used to simulate a hypothetical ssm bug: forgetting about a param
var errSimulateMissing = errors.New("missing")

type mockParameterGetter func(name string) (string, error)

func (f mockParameterGetter) GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	val, err := f(*input.Name)

	switch err {
	case nil:
		// no-op
	case errSimulateMissing:
		return &ssm.GetParameterOutput{}, nil
	default:
		return nil, err
	}

	output := &ssm.GetParameterOutput{
		Parameter: &ssm.Parameter{
			Name:    input.Name,
			Type:    aws.String("String"),
			Value:   &val,
			Version: aws.Int64(1),
		},
	}

	return output, nil
}
