# aws-env

This is a small utility to help populate environment variables using secrets stored in AWS Parameter Store.

Based loosely on [Droplr/aws-env](https://github.com/Droplr/aws-env): a tool that exports a portion of Parameter Store as environment variables.

Instead, this tool allows you greater control over what will be exported, and doesn't force environment variable naming upon you (easier to adapt for exising applications).

Jump to:
 - [How it works](#how-it-works)
 - [Usage](#usage)
 - [Auth](#auth)
 - [Region](#region)
 - [Prefix](#prefix)

## How it works
 - aws-env looks through the environment for any variables whose value begins with a special prefix (`awsenv:` by default).
 - It expects that those variables have a parameter store key after the `:`.
 - It looks up each parameter in AWS Parameter Store, and then outputs commands to export the new values.
 - If there are no variables with that prefix, then it does nothing (exits cleanly). This allows for running locally without effect.


## Command Usage

1. Add some stuff to the Parameter Store (or use items already there):

```
$ aws ssm put-parameter --name /testing/my-app/dbpass --value "some-secret-password" --type SecureString --key-id "alias/aws/ssm" --region us-east-1
$ aws ssm put-parameter --name /testing/my-app/privatekey --value "some-private-key" --type SecureString --key-id "alias/aws/ssm" --region us-east-1
```

2. Install aws-env using static binary (amd64 only) (choose proper [version](https://github.com/sendgrid/aws-env/releases)). 

```
$ wget https://github.com/sendgrid/aws-env/releases/download/1.3.3/aws-env -O aws-env
```

OR build from source

```
$ go install github.com/sendgrid/aws-env/cmd/aws-env
```

3. Start your application with aws-env

```
$ aws-env ./my-app --app-flag app-args
```

4. Multiple commands can be run after updating the environment once by
   invoking a shell:

```
$ aws-env sh -c 'command1 | command2; command3'
```

5. Alternatively (but not recommended), use aws-env to output
   bash-compatible export statements.

```
$ eval $(./aws-env) && ./my-app
```

Under the hood, aws-env will scan existing environment variable values for
any that begin with the prefix `awsenv:`. It will then export new values for
those using Parameter Store.

For example, if you had:

```
$ export DB_PASSWORD=awsenv:/testing/my-app/dbpass
$ export PRIVATE_KEY=awsenv:/testing/my-app/privatekey
```

Running `aws-env` in the command invocation style has similar behavior to
the unix `env` command:

```
$ export DB_USERNAME=$'some-secret-password'
$ export PRIVATE_KEY=$'some-private-key'
```

### Use as a Go library (with aws-sdk-go v1)

If running a Go application with access to `AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, and related environment variables (such as when
running a Go binary as an AWS Lambda function), the following code is
sufficient to get the process' environment replaced as described above:

```
import (
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ssm"
  "github.com/sendgrid/aws-env/awsenv"
  v1 "github.com/sendgrid/aws-env/awsenv/v1"
)

func init() {
  cfg := &aws.Config{Region: aws.String(os.Getenv("AWS_REGION"))}
  sess := session.Must(session.NewSession(cfg))

  paramsGetter := v1.NewParamsGetter(ssm.New(sess))
  replacer := awsenv.NewReplacer(awsenv.DefaultPrefix, paramsGetter)

  ctx := context.Background()
  err := replacer.ReplaceAll(ctx)
  if err != nil {
    // handle error
  }

  // optionally further process environment, e.g. via envconfig
}
```

aws-sdk-go-v2 can be used instead by importing the awsenv/v2 subpackage, and
initializing and passing an aws-sdk-go-v2 SSM client.

### Use to update a file in-place

The `-f` flag can be used to pass in a file to update in-place rather than 
operating on the environment variables. It will only update the first 
occurrence per line. It stops parsing when a character is no longer a valid
Parameter Store path. 

Example usage
```
$ mrroboto upload -p /path/to/the/username -v localtestuser               
2019/03/13 14:15:04 parameter=/path/to/the/username regions=[us-east-1 us-east-2 us-west-1 us-west-2]

$ mrroboto upload -p /path/to/the/password -v localtestpass            
2019/03/13 14:15:14 parameter=/path/to/the/password regions=[us-east-1 us-east-2 us-west-1 us-west-2]

$ cat test.txt4                                                           
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

$ ./aws-env -f test.txt4                                                  
INFO[0000] aws-env starting                              app_version=0.0.1 built_at="Wed Mar 13 20:12:42 UTC 2019" git_hash=545515b6b6646f1bd2f95dea13478066782deb0c


$ cat test.txt4                                                          
mysql_users:
 (
    {
        username = "localtestuser"
        password = "localtestpass"
        default_hostgroup = 0
        max_connections=1000
        default_schema="information_schema"
        active = 1
    }
 )
```

### Example Dockerfile

```
FROM alpine

RUN apk update && apk upgrade && \
  apk add --no-cache openssl ca-certificates

RUN wget https://github.com/sendgrid/aws-env/releases/download/1.0.0/aws-env -O /bin/aws-env && \
  chmod +x /bin/aws-env

CMD aws-env --region us-east-2 /my-app
```

### Use as a Docker image
Starting with version `v1.2.1`, we publish a Docker image that contains the `aws-env` binary.

## Auth
aws-env exposes a `--profile` flag (or `AWS_ENV_PROFILE`) for use when
running locally. This allows you to use the assume role tool and then
specify the profile. Otherwise, it will first look for a local metadata
service (if running in EC2 or on-prem Kubernetes), and then fall back to
environment variable auth.

## Region
aws-env defaults to looking at parameter store in the `us-east-1` region.
You can override this with the `--region` flag (or `AWS_ENV_REGION`).

## Prefix
The default environment variable value prefix is `awsenv:`, this can be
changed using the `--prefix` flag (or `AWS_ENV_PREFIX` env var).

## Assume Role
aws-env exposes an `--assume-role` flag (or `AWS_ENV_ASSUME_ROLE`). This can
be used to further assume roles if you have to gain access using a chain of
roles.

### Example Assume Role
In Kubernetes, if you are using Annotations with a service role, `kube2iam`
will assume your service role using the metadata service. You can then use
the `--assume-role` flag to have your service role assume the ssm role to
retrieve securely stored parameters with aws-env.

## Considerations

* When used without a command, aws-env uses `$'string'` notation to support
  multi-line variables export. For this reason, to use aws-env in this way,
  it's required to switch shell to /bin/bash:

```
CMD ["/bin/bash", "-c", "eval $(aws-env) && ./my-app"]
```

This isn't necessary if your Docker image's default shell is already bash.

Using the command invocation style does not have this limitation.
