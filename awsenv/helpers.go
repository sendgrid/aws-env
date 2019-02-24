package awsenv

import (
	"strings"
)

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func ceildiv(n, d int) int {
	return min(n, (n-1)/d+1)
}

func keys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// merge copies values from srcs to dest.
func merge(dest map[string]string, srcs []map[string]string) {
	for _, src := range srcs {
		for key, val := range src {
			dest[key] = val
		}
	}
}

// translate moves values from src to dest using keys translated through
// trans.
func translate(dest, trans, src map[string]string) {
	for oldkey, val := range src {
		newkey, ok := trans[oldkey]
		if ok {
			dest[newkey] = val
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

func pathmap(prefix string, env []string) map[string]string {
	// param path -> env var name
	m := make(map[string]string, len(env))

	for _, rawVar := range env {
		idx := strings.Index(rawVar, "=")
		if idx < 0 {
			// impossible on real systems?
			continue
		}

		name, path := rawVar[:idx], rawVar[idx+1:]
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		path = strings.TrimPrefix(path, prefix)
		m[path] = name
	}

	return m
}
