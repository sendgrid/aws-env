package awsenv

import (
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
)

// ParameterGetter is implemented by the ssm client
type ParameterGetter interface {
	GetParameter(*ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
}

// Replacer handles replacing existing environment variables with parameter values
type Replacer struct {
	ssm    ParameterGetter
	prefix string
}

// NewReplacer returns a Replacer that will operate on env vars with the
// given value prefix, using the given ParameterGetter.
func NewReplacer(envValuePrefix string, ssm ParameterGetter) *Replacer {
	return &Replacer{
		ssm:    ssm,
		prefix: envValuePrefix,
	}
}

// ReplaceAll looks at all environment variables and returns
// a set of new variables to apply.
func (r *Replacer) ReplaceAll() (map[string]string, error) {
	replacements := make(map[string]string)

	// Get all environment vars
	vars, err := getAllVars()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list environment")
	}

	// Check each once for prefix
	for name, val := range vars {
		if strings.HasPrefix(val, r.prefix) {
			// Get the replacement
			newVal, err := r.replaceOne(val)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to replace var: %s", name)
			}
			replacements[name] = newVal
		}
	}

	return replacements, nil
}

func (r *Replacer) replaceOne(oldVal string) (string, error) {
	// Trim off the prefix
	oldVal = strings.TrimPrefix(oldVal, r.prefix)

	// Look up the new value in parameter store
	out, err := r.ssm.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(oldVal),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to get parameter: %s", oldVal)
	}

	if out.Parameter.Value == nil {
		return "", errors.Errorf("parameter is empty: %s", oldVal)
	}

	return *out.Parameter.Value, nil
}

func getAllVars() (map[string]string, error) {
	rawVars := os.Environ()
	vars := make(map[string]string, len(rawVars))
	for _, rawVar := range rawVars {
		parts := strings.SplitN(rawVar, "=", 2)
		if len(parts) != 2 {
			return nil, errors.Errorf("got unexpected env var: %s", rawVar)
		}
		vars[parts[0]] = parts[1]
	}
	return vars, nil
}
