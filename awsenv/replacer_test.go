package awsenv

import (
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/require"
)

func TestReplacerNoop(t *testing.T) {
	mockGetter := &mockParameterGetter{f: func(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
		return nil, errors.New("forced")
	}}

	withEnv(t, nil, func() {
		r := NewReplacer("awsenv:", mockGetter)
		newVars, err := r.ReplaceAll()
		require.NoError(t, err, "expected no error")
		require.Len(t, newVars, 0, "expected no replacements")
	})
}

func TestReplacerMultiple(t *testing.T) {
	withEnv(t, map[string]string{
		"DB_PASSWORD":       "test",                       // no matching prefix
		"SOME_SECRET":       "awsenv:/param/path/here",    // match
		"SOME_OTHER_SECRET": "awsenv:/param/path/here/v2", // match
	}, func() {
		r := NewReplacer("awsenv:", mockParamStore(map[string]string{
			"/param/path/here":    "val1",
			"/param/path/here/v2": "val2",
		}))
		newVars, err := r.ReplaceAll()
		require.NoError(t, err, "expected no error")
		require.Len(t, newVars, 2, "expected 2 replacements")
		require.Equal(t, "val1", newVars["SOME_SECRET"], "expected var to be set correctly")
		require.Equal(t, "val2", newVars["SOME_OTHER_SECRET"], "expected var to be set correctly")
	})
}

func TestReplacerNotFound(t *testing.T) {
	withEnv(t, map[string]string{
		"DB_PASSWORD": "test",                    // no matching prefix
		"SOME_SECRET": "awsenv:/param/path/here", // match
	}, func() {
		r := NewReplacer("awsenv:", mockParamStore(map[string]string{}))
		_, err := r.ReplaceAll()
		require.Error(t, err, "expected an error")
	})
}

func TestReplacerMissing(t *testing.T) {
	mockGetter := &mockParameterGetter{f: func(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
		return &ssm.GetParameterOutput{Parameter: &ssm.Parameter{}}, nil
	}}

	withEnv(t, map[string]string{
		"SOME_SECRET": "awsenv:/param/path/here/doesnt/exist", // match
	}, func() {
		r := NewReplacer("awsenv:", mockGetter)
		_, err := r.ReplaceAll()
		require.Error(t, err, "expected an error")
	})
}

func withEnv(t *testing.T, vars map[string]string, f func()) {
	for name, val := range vars {
		require.NoError(t, os.Setenv(name, val), "expected to set env var")
	}

	f()

	for name := range vars {
		require.NoError(t, os.Unsetenv(name), "expected to unset env var")
	}
}

func mockParamStore(vals map[string]string) ParameterGetter {
	return &mockParameterGetter{f: func(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
		val, ok := vals[*in.Name]
		if !ok {
			return nil, errors.New("not found")
		}
		return &ssm.GetParameterOutput{Parameter: &ssm.Parameter{Value: aws.String(val)}}, nil
	}}
}

type mockParameterGetter struct {
	f func(*ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
}

func (m *mockParameterGetter) GetParameter(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	return m.f(in)
}
