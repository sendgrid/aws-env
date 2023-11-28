package awsenv

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
