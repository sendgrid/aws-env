package awsenv

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"
)

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

func TestFetcher_GetParamsLimit(t *testing.T) {
	t.Parallel()
	mockClient := &mockSSMClient{}
	getter := NewParamsGetter(mockClient)
	require.Equal(t, 10, getter.GetParamsLimit())
}

// stringPtr returns a pointer to the string value
func stringPtr(s string) *string {
	return &s
}
