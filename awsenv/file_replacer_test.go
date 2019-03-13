package awsenv

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

var (
	sampleCnfFile1 = `
mysql_users:
 (
 	{
 		username = "username"
 		password = "password"
 		default_hostgroup = 0
 		max_connections=1000
 		default_schema="information_schema"
 		active = 1
 	}
 )
`
	sampleCnfFile2 = `
mysql_users:
 (
	{
		username = "awsenv:/path/to/the/username"
		password = "awsenv:/path/to/the/password"
		default_hostgroup = 0
		max_connections=1000
		default_schema="information_schema"
		active = 1
	}
 )
`
)

func TestFileReplacer_panic(t *testing.T) {
	mockGetter := mockParamsGetter(func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("no implementation")
	})

	require.Panics(t, func() { NewFileReplacer("", "", mockGetter) })
}

func TestFileReplacer_ReplaceAll_noop(t *testing.T) {
	mockGetter := mockParamsGetter(func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("forced")
	})

	fileName, cleanup := writeTempFile(sampleCnfFile1)
	defer cleanup()

	ctx := context.Background()
	r := NewFileReplacer("awsenv:", fileName, mockGetter)
	err := r.ReplaceAll(ctx)

	// since the error is forced, the only way for there to be no error is
	// if it didn't try to do any lookups
	require.NoError(t, err, "expected no error")
	require.Equal(t, sampleCnfFile1, sampleCnfFile1)
}

func TestFileReplacer_ReplaceAll_multiple(t *testing.T) {

	fileName, cleanup := writeTempFile(sampleCnfFile2)
	defer cleanup()

	// read content before the change
	oldContent, err := ioutil.ReadFile(fileName)
	require.NoError(t, err)
	require.Equal(t, sampleCnfFile2, string(oldContent))

	params := mockParamStore{
		"/path/to/the/username": "user",
		"/path/to/the/password": "password",
	}
	r := NewFileReplacer(DefaultPrefix, fileName, params)

	ctx := context.Background()
	err = r.ReplaceAll(ctx)
	require.NoError(t, err, "expected no error")

	expectedContent := `
mysql_users:
 (
	{
		username = "user"
		password = "password"
		default_hostgroup = 0
		max_connections=1000
		default_schema="information_schema"
		active = 1
	}
 )
`
	f, err := ioutil.ReadFile(fileName)
	require.NoError(t, err)

	require.Equal(t, expectedContent, string(f))
}

func writeTempFile(contents string) (string, func()) {

	uid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	fName := uid.String()

	tmpfile, err := ioutil.TempFile("", fName)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.Write([]byte(contents)); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	return tmpfile.Name(), func() { os.Remove(fName) }
}
