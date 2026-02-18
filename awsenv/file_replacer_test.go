package awsenv

import (
	"context"
	"errors"
	"fmt"
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
	sampleCnfFile3 = `
mysql_users:
 (
	{
		username = "awsenv:/path/to/the/username",
		password = "awsenv:/path/to/the/password",
		default_hostgroup = 0,
		max_connections=1000,
		default_schema="information_schema",
		active = 1,
	}
 )
`
	sampleCnfFile4 = `
mysql_users:
 (
	{
		username = "awsenv:/path/to/the/username",
		password = "awsenv:/path/to/the/password",
		default_hostgroup = 0,
		max_connections=1000,
		default_schema="information_schema",
		active = 1,
		admin_username = "awsenv:/path/to/the/username",
		admin_password = "awsenv:/path/to/the/password",
	}
 )
`
	sampleCnfFile5 = `
mysql_users:
 (
	{
		username = "awsenv:arn:aws:ssm:us-east-1:123456789012:parameter/remote/username",
		password = "awsenv:arn:aws:ssm:us-east-1:123456789012:parameter/remote/password",
	}
 )
`
	sampleCnfFile6 = `
mysql_users:
 (
	{
		username = "awsenv:/path/to/the/username",
		password = "awsenv:arn:aws:ssm:us-east-1:123456789012:parameter/remote/password",
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
	oldContent, err := ioutil.ReadFile(fileName) //nolint: gosec
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
	f, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)

	require.Equal(t, expectedContent, string(f))
}

func TestFileReplacer_ReplaceAll_with_commas(t *testing.T) {

	fileName, cleanup := writeTempFile(sampleCnfFile3)
	defer cleanup()

	// read content before the change
	oldContent, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)
	require.Equal(t, sampleCnfFile3, string(oldContent))

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
		username = "user",
		password = "password",
		default_hostgroup = 0,
		max_connections=1000,
		default_schema="information_schema",
		active = 1,
	}
 )
`
	f, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)

	require.Equal(t, expectedContent, string(f))
}

func TestFileReplacer_ReplaceAll_with_multiple_occurrences(t *testing.T) {

	fileName, cleanup := writeTempFile(sampleCnfFile4)
	defer cleanup()

	// read content before the change
	oldContent, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)
	require.Equal(t, sampleCnfFile4, string(oldContent))

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
		username = "user",
		password = "password",
		default_hostgroup = 0,
		max_connections=1000,
		default_schema="information_schema",
		active = 1,
		admin_username = "user",
		admin_password = "password",
	}
 )
`
	f, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)

	require.Equal(t, expectedContent, string(f))
}

func TestFileReplacer_ReplaceAll_CrossAccountARN(t *testing.T) {

	fileName, cleanup := writeTempFile(sampleCnfFile5)
	defer cleanup()

	oldContent, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)
	require.Equal(t, sampleCnfFile5, string(oldContent))

	// Use mockParamsGetter to simulate SSM's behavior of stripping ARN prefixes from result keys
	getter := mockParamsGetter(func(_ context.Context, paths []string) (map[string]string, error) {
		store := map[string]string{
			"/remote/username": "remote_user",
			"/remote/password": "remote_pass",
		}
		result := make(map[string]string, len(paths))
		for _, p := range paths {
			plain := stripARNPrefix(p)
			val, ok := store[plain]
			if !ok {
				return nil, fmt.Errorf("not found: %s", p)
			}
			result[plain] = val
		}
		return result, nil
	})

	r := NewFileReplacer(DefaultPrefix, fileName, getter)

	ctx := context.Background()
	err = r.ReplaceAll(ctx)
	require.NoError(t, err, "expected no error")

	expectedContent := `
mysql_users:
 (
	{
		username = "remote_user",
		password = "remote_pass",
	}
 )
`
	f, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)

	require.Equal(t, expectedContent, string(f))
}

func TestFileReplacer_ReplaceAll_MixedLocalAndCrossAccount(t *testing.T) {

	fileName, cleanup := writeTempFile(sampleCnfFile6)
	defer cleanup()

	oldContent, err := ioutil.ReadFile(fileName) //nolint: gosec
	require.NoError(t, err)
	require.Equal(t, sampleCnfFile6, string(oldContent))

	getter := mockParamsGetter(func(_ context.Context, paths []string) (map[string]string, error) {
		store := map[string]string{
			"/path/to/the/username": "local_user",
			"/remote/password":      "remote_pass",
		}
		result := make(map[string]string, len(paths))
		for _, p := range paths {
			plain := stripARNPrefix(p)
			val, ok := store[plain]
			if !ok {
				return nil, fmt.Errorf("not found: %s", p)
			}
			result[plain] = val
		}
		return result, nil
	})

	r := NewFileReplacer(DefaultPrefix, fileName, getter)

	ctx := context.Background()
	err = r.ReplaceAll(ctx)
	require.NoError(t, err, "expected no error")

	expectedContent := `
mysql_users:
 (
	{
		username = "local_user",
		password = "remote_pass",
	}
 )
`
	f, err := ioutil.ReadFile(fileName) //nolint: gosec
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

	return tmpfile.Name(), func() { os.Remove(fName) } //nolint: errcheck,gosec
}
