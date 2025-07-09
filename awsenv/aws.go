package awsenv

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SSMGetParametersAPI defines the interface for the GetParameters function.
// We use this interface to test the function using a mocked service.
type SSMGetParametersAPI interface {
	GetParameters(ctx context.Context,
		params *ssm.GetParametersInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
}

// NewParamsGetter implements ParamsGetter using AWS SDK v2 SSM client.
func NewParamsGetter(ssm SSMGetParametersAPI) LimitedParamsGetter {
	return &fetcher{ssm, true}
}

// NewParamsGetterFromConfig creates a new ParamsGetter using AWS SDK v2 with the provided config.
func NewParamsGetterFromConfig(cfg aws.Config) LimitedParamsGetter {
	return NewParamsGetter(ssm.NewFromConfig(cfg))
}

type fetcher struct {
	ssm     SSMGetParametersAPI
	decrypt bool
}

func (f *fetcher) GetParamsLimit() int { return 10 }

func (f *fetcher) GetParams(ctx context.Context, names []string) (map[string]string, error) {
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
