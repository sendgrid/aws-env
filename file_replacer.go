package awsenv

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"unicode"
)

var (
	readFile = ioutil.ReadFile
)

// FileReplacer handles replacing the first instance per line of a prefixed
// field  and updating it with a value retrieved from AWS Parameter Store.
type FileReplacer struct {
	ssm      ParamsGetter
	prefix   string
	fileName string
	perms    os.FileMode
}

// NewFileReplacer takes a prefix to look for, and a ParamGetter that it will
// use to fetch the values from Parameter Store
//
// If the prefix is an empty string then the constructor will panic
func NewFileReplacer(prefix, fileName string, ssm ParamsGetter) *FileReplacer {

	if prefix == "" {
		panic("awsenv: prefix must be non-empty")
	}

	if fileName == "" {
		panic("awsenv: fileName must be non-empty")
	}

	fInfo, err := os.Stat(fileName)
	if err != nil {
		panic(fmt.Sprintf("failed to stat the file %s", fileName))
	}

	perms := fInfo.Mode().Perm()
	return &FileReplacer{
		ssm:      ssm,
		prefix:   prefix,
		fileName: fileName,
		perms:    perms,
	}
}

// ReplaceAll overwrites the first instance of every prefix-matching field
// per line with values retrieved from Parameter Store. ReplaceAll will
// attempt to replace as many values as possible, after which it will
// return the first error that occurred.
func (r *FileReplacer) ReplaceAll(ctx context.Context) error {

	f, err := readFile(r.fileName)
	if err != nil {
		return err
	}

	lines := strings.Split(string(f), "\n")
	replacementIndices := make(map[string][]replacementIndex, 8)
	paths := make([]string, 0, 8)

	// find the paths that need replacing
	for i, line := range lines {

		idx := strings.Index(line, r.prefix)
		if idx < 0 {
			// no prefix found in line
			continue
		}

		path := strings.FieldsFunc(line[idx+len(r.prefix):], splitPath)[0]

		// if we haven't seen the path yet, init the slice
		if _, ok := replacementIndices[path]; !ok {
			replacementIndices[path] = make([]replacementIndex, 0, 4)
		}

		replacementIndices[path] = append(replacementIndices[path], replacementIndex{
			lineNumber: i,
			index:      idx,
		})
		paths = append(paths, path)
	}

	// fetch the values for the paths
	paramValues, err := fetch(ctx, r.ssm, paths)
	if err != nil {
		return err
	}

	// for each param we found, replace the corresponding line
	for path, value := range paramValues {
		replacements, ok := replacementIndices[path]

		// this shouldn't really happen
		if !ok {
			continue
		}

		for _, replacement := range replacements {

			ln := replacement.lineNumber
			idx := replacement.index
			lines[ln] = fmt.Sprintf("%s%s%s", lines[ln][:idx], value, lines[ln][idx+len(r.prefix)+len(path):])
		}
	}

	newContent := strings.Join(lines, "\n")
	err = ioutil.WriteFile(r.fileName, []byte(newContent), r.perms)
	if err != nil {
		return err
	}

	return nil
}

// MustReplaceAll overwrites the applicable environment and generates a panic if something goes wrong.
func (r *FileReplacer) MustReplaceAll(ctx context.Context) {
	err := r.ReplaceAll(ctx)
	if err != nil {
		panic(err)
	}
}

type replacementIndex struct {
	lineNumber int
	index      int
}

// return false if the given rune isn't an acceptable Parameter Store path
func splitPath(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
		r != '/' && r != '_' && r != '-' && r != '.'
}
