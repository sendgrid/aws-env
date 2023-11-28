// Package v1 implements awsenv.ParamsGetter using aws-sdk-go (v1).
package v1

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/sendgrid/aws-env/awsenv"
)

// SSMGetParametersAPI defines the interface for the GetParameters function.
// We use this interface to test the function using a mocked service.
type SSMGetParametersAPI interface {
	GetParametersWithContext(ctx aws.Context,
		input *ssm.GetParametersInput,
		opts ...request.Option) (*ssm.GetParametersOutput, error)
}

// NewParamsGetter implements awsenv.ParamsGetter using a v1 ssm client.
func NewParamsGetter(ssm SSMGetParametersAPI) awsenv.LimitedParamsGetter {
	return &fetcher{ssm, true}
}

type fetcher struct {
	ssm     SSMGetParametersAPI
	decrypt bool
}

func (f *fetcher) GetParamsLimit() int { return 10 }

func (f *fetcher) GetParams(ctx context.Context, names []string) (map[string]string, error) {
	ptrs := make([]*string, len(names))
	for i := range names {
		ptrs[i] = &names[i]
	}

	input := &ssm.GetParametersInput{
		Names:          ptrs,
		WithDecryption: &f.decrypt,
	}

	resp, err := f.ssm.GetParametersWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	m := make(map[string]string, len(resp.Parameters))
	for _, param := range resp.Parameters {
		m[*param.Name] = *param.Value
	}

	return m, nil
}

func MustReplaceEnv() {
	sess := session.Must(session.NewSession(
		&aws.Config{
			// Default to region us-east-2 because it requires a region be set for awsenv to work.
			Region: aws.String("us-east-2"),
		},
	))
	replacer := awsenv.NewReplacer(awsenv.DefaultPrefix, NewParamsGetter(ssm.New(sess)))

	replacer.MustReplaceAll(context.Background())
}
