// Package v1 implements awsenv.ParamsGetter using aws-sdk-go (v1).
package v1

import (
	"context"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/sendgrid/aws-env/awsenv"
)

// NewParamsGetter implements awsenv.ParamsGetter using a v1 ssm client.
func NewParamsGetter(ssm *ssm.SSM) awsenv.LimitedParamsGetter {
	return &fetcher{ssm, true}
}

type fetcher struct {
	ssm     *ssm.SSM
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
