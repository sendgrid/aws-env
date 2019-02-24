package awsenv

import (
	"reflect"
	"sort"
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

func TestKeys(t *testing.T) {
	tests := []struct {
		input map[string]string
		want  []string
	}{
		{
			input: nil,
			want:  []string{},
		},
		{
			input: map[string]string{"k": "v"},
			want:  []string{"k"},
		},
		{
			input: map[string]string{"k1": "v1", "k2": "v2"},
			want:  []string{"k1", "k2"},
		},
	}

	for _, test := range tests {
		got := keys(test.input)
		sort.Strings(got)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("keys(%v) = %v, want %v", test.input, got, test.want)
		}
	}
}

func TestTranslate(t *testing.T) {
	tests := []struct {
		src, trans, want map[string]string
	}{
		{
			src:   nil,
			trans: nil,
			want:  map[string]string{},
		},
		{
			src:   nil,
			trans: map[string]string{"x": "a"},
			want:  map[string]string{},
		},
		{
			src:   map[string]string{"x": "1"},
			trans: nil,
			want:  map[string]string{},
		},
		{
			src:   map[string]string{"x": "1", "y": "2"},
			trans: map[string]string{"x": "a", "y": "b"},
			want:  map[string]string{"a": "1", "b": "2"},
		},
	}

	for _, test := range tests {
		got := make(map[string]string)

		translate(got, test.trans, test.src)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("translate(%v, %v) -> %v, want %v",
				test.trans, test.src, got, test.want)
		}
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

func TestPathmap(t *testing.T) {
	tests := []struct {
		prefix string
		input  []string
		want   map[string]string
	}{
		{
			prefix: "",
			input:  nil,
			want:   map[string]string{},
		},
		{
			prefix: "awsenv:",
			input:  []string{"BAD", "X=1", "Y=pre:/y", "Z=awsenv:/z"},
			want:   map[string]string{"/z": "Z"},
		},
		{
			prefix: "pre:",
			input:  []string{"BAD", "X=1", "Y=pre:/y", "Z=awsenv:/z"},
			want:   map[string]string{"/y": "Y"},
		},
	}

	for _, test := range tests {
		got := pathmap(test.prefix, test.input)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("pathmap(%q, %q) = %v, want %v",
				test.prefix, test.input, got, test.want)
		}
	}
}
