// Package v2 implements awsenv.ParamsGetter using aws-sdk-go-v2.
package v2

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/sendgrid/aws-env/awsenv"
)

func NewParamsGetter(ssm *ssm.SSM) awsenv.LimitedParamsGetter {
	return &fetcher{ssm, true}
}

type fetcher struct {
	ssm     *ssm.SSM
	decrypt bool
}

func (f *fetcher) GetParamsLimit() int { return 10 }

func (f *fetcher) GetParams(ctx context.Context, names []string) (map[string]string, error) {
	input := &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: &f.decrypt,
	}

	req := f.ssm.GetParametersRequest(input)
	req.SetContext(ctx)

	resp, err := req.Send()
	if err != nil {
		return nil, err
	}

	m := make(map[string]string, len(resp.Parameters))
	for _, param := range resp.Parameters {
		m[*param.Name] = *param.Value
	}

	return m, nil
}
