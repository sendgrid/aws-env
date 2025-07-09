package awsenv

import "strings"

// parseEnvironment takes the results of environ and converts it into a map of Environment Keys => Values
func parseEnvironment(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	envvars := make(map[string]string, len(env))

	for _, rawVar := range env {
		idx := strings.Index(rawVar, "=")
		if idx < 0 {
			// impossible on real systems?
			continue
		}

		key, value := rawVar[:idx], rawVar[idx+1:]
		envvars[key] = value
	}

	return envvars
}
