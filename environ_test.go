package awsenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  []string
		output map[string]string
	}{
		{
			name:   "empty",
			input:  nil,
			output: nil,
		},
		{
			name: "invalid_environment",
			input: []string{
				"NAME0=VALUE0",
				"InvalidTest",
				"NAME1=VALUE1",
			},
			output: map[string]string{
				"NAME0": "VALUE0",
				"NAME1": "VALUE1",
			},
		},
		{
			name: "valid_environment",
			input: []string{
				"NAME1=VALUE1",
				"NAME2=VALUE2",
			},
			output: map[string]string{
				"NAME1": "VALUE1",
				"NAME2": "VALUE2",
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, want := parseEnvironment(tc.input), tc.output
			assert.Equal(t, want, got)
		})
	}
}
