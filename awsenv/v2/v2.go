// Package v2 implements awsenv.ParamsGetter using aws-sdk-go-v2.
package v2

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/sendgrid/aws-env/awsenv"
)

// SSMGetParametersAPI defines the interface for the GetParameters function.
// We use this interface to test the function using a mocked service.
type SSMGetParametersAPI interface {
	GetParameters(ctx context.Context,
		params *ssm.GetParametersInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
}

// NewParamsGetter implements awsenv.ParamsGetter using a v2 ssm client.
func NewParamsGetter(ssm SSMGetParametersAPI) awsenv.LimitedParamsGetter {
	return &fetcher{ssm, true}
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

// MustReplaceEnv replaces the environment with values from ssm parameter store.
func MustReplaceEnv() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	replacer := awsenv.NewReplacer(awsenv.DefaultPrefix, NewParamsGetter(ssm.NewFromConfig(cfg)))

	replacer.MustReplaceAll(ctx)
}
