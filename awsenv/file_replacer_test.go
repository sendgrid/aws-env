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
 		username = "/path/to/the/username"
 		password = "/path/to/the/password"
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

	env := fakeEnv{}
	env.install()

	ctx := context.Background()
	r := NewFileReplacer("awsenv:", "", mockGetter)
	err := r.ReplaceAll(ctx)
	require.NoError(t, err, "expected no error")
	require.Empty(t, env)
}

func TestFileReplacer_ReplaceAll_multiple(t *testing.T) {

	fileName, cleanup := writeTempFile(sampleCnfFile1)
	defer cleanup()

	params := mockParamStore{
		"/param/to/the/username": "user",
		"/param/to/the/password": "password",
	}
	r := NewFileReplacer(DefaultPrefix, fileName, params)

	ctx := context.Background()
	err := r.ReplaceAll(ctx)

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
	require.Equal(t, expectedContent, "")
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

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write([]byte(contents)); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}
	return fName, func() { os.Remove(fName) }
}
