package awsenv

import "regexp"

// ssmARNPrefix matches the fully qualified SSM parameter ARN prefix used for cross-account parameters.
// note: this will not cover AWS GovCloud ARNs
//
//	example: `arn:aws:ssm:<region>:<account_id>:parameter<parameter_path>`
var ssmARNPrefix = regexp.MustCompile(`arn:aws:ssm:[^:]+:[^:]+:parameter`)

// stripARNPrefix removes the SSM ARN prefix from a parameter path, returning the plain path.
// If the path does not contain an ARN prefix, it is returned unchanged.
func stripARNPrefix(path string) string {
	return ssmARNPrefix.ReplaceAllString(path, "")
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func ceildiv(n, d int) int {
	return min(n, (n-1)/d+1)
}

// merge copies values from srcs to dest.
func merge(dest map[string]string, srcs []map[string]string) {
	for _, src := range srcs {
		for key, val := range src {
			dest[key] = val
		}
	}
}

func chunk(size int, lst []string) [][]string {
	if len(lst) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(lst)
	}

	chunks := make([][]string, 0, ceildiv(len(lst), size))

	for len(lst) > 0 {
		k := min(size, len(lst))
		chunks = append(chunks, lst[:k])
		lst = lst[k:]
	}

	return chunks
}
