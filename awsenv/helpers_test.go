package awsenv

import (
	"reflect"
	"testing"
)

func TestMin(t *testing.T) {
	tests := []struct {
		x, y, min int
	}{
		{-1, 0, -1},
		{5, 11, 5},
		{5, 5, 5},
	}

	run := func(x, y, want int) {
		t.Helper()
		got := min(x, y)
		if got != want {
			t.Errorf("min(%v, %v) = %v, want %v", x, y, got, want)
		}
	}

	for _, test := range tests {
		run(test.x, test.y, test.min)
		run(test.y, test.x, test.min)
	}
}

func TestCeildiv(t *testing.T) {
	tests := []struct {
		n, d, want int
	}{
		{0, 3, 0},
		{1, 1, 1},
		{2, 1, 2},
		{3, 2, 2},
		{9, 5, 2},
		{10, 5, 2},
		{11, 5, 3},
	}

	for _, test := range tests {
		got := ceildiv(test.n, test.d)
		if got != test.want {
			t.Errorf("ceildiv(%v, %v) = %v, want %v",
				test.n, test.d, got, test.want)
		}
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		input []map[string]string
		want  map[string]string
	}{
		{
			input: nil,
			want:  map[string]string{},
		},
		{
			input: []map[string]string{nil, nil, nil},
			want:  map[string]string{},
		},
		{
			input: []map[string]string{{"k": "v"}},
			want:  map[string]string{"k": "v"},
		},
		{
			input: []map[string]string{{"k1": "v1"}, {"k2": "v2"}, {"k2": "v3"}},
			want:  map[string]string{"k1": "v1", "k2": "v3"},
		},
	}

	for _, test := range tests {
		got := make(map[string]string)

		merge(got, test.input)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("merge(%v) -> %v, want %v", test.input, got, test.want)
		}
	}
}

func TestStripARNPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain_path",
			input: "/my/param/path",
			want:  "/my/param/path",
		},
		{
			name:  "full_arn",
			input: "arn:aws:ssm:us-east-1:123456789012:parameter/my/param/path",
			want:  "/my/param/path",
		},
		{
			name:  "empty_string",
			input: "",
			want:  "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := stripARNPrefix(test.input)
			if got != test.want {
				t.Errorf("stripARNPrefix(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestChunk(t *testing.T) {
	tests := []struct {
		size  int
		input []string
		want  [][]string
	}{
		{
			size:  0,
			input: nil,
			want:  nil,
		},
		{
			size:  -1,
			input: nil,
			want:  nil,
		},
		{
			size:  1,
			input: nil,
			want:  nil,
		},
		{
			size:  0,
			input: []string{"x", "y", "z"},
			want:  [][]string{{"x", "y", "z"}},
		},
		{
			size:  -1,
			input: []string{"x", "y", "z"},
			want:  [][]string{{"x", "y", "z"}},
		},
		{
			size:  1,
			input: []string{"x", "y", "z"},
			want:  [][]string{{"x"}, {"y"}, {"z"}},
		},
		{
			size:  2,
			input: []string{"x", "y", "z"},
			want:  [][]string{{"x", "y"}, {"z"}},
		},
		{
			size:  2,
			input: []string{"w", "x", "y", "z"},
			want:  [][]string{{"w", "x"}, {"y", "z"}},
		},
		{
			size:  2,
			input: []string{"v", "w", "x", "y", "z"},
			want:  [][]string{{"v", "w"}, {"x", "y"}, {"z"}},
		},
		{
			size:  3,
			input: []string{"x", "y", "z"},
			want:  [][]string{{"x", "y", "z"}},
		},
		{
			size:  4,
			input: []string{"x", "y", "z"},
			want:  [][]string{{"x", "y", "z"}},
		},
	}

	for _, test := range tests {
		got := chunk(test.size, test.input)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("chunk(%v, %v) = %v, want %v",
				test.size, test.input, got, test.want)
		}
	}
}
