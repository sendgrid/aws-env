# aws-env

This is a small utility to help populate environment variables using secrets stored in AWS Parameter Store.

Loosely based on [https://github.com/Droplr/aws-env] which exports a portion of Parameter Store as environment variables, this tool allows you greater control over what will be exported.

## How it works
 - aws-env looks through the environment for any variables whose value begins with a special prefix (`awsenv:` by default).
 - It expects that those variables have a parameter store key after the `:`.
 - It looks up each parameter in AWS Parameter Store, and then outputs commands to export the new values.
 - If there are no variables with that prefix, then it does nothing (exits cleanly). This allows for running locally without effect.


## Usage

1. Add some stuff to the Parameter Store (or use items already there):
```
$ aws ssm put-parameter --name /testing/my-app/dbpass --value "some-secret-password" --type SecureString --key-id "alias/aws/ssm" --region us-east-1
$ aws ssm put-parameter --name /testing/my-app/privatekey --value "some-private-key" --type SecureString --key-id "alias/aws/ssm" --region us-east-1
```

2. Install aws-env using static binary (amd64 only) (choose proper [version](https://github.com/sendgrid/aws-env/releases)). 
```
$ wget https://github.com/sendgrid/aws-env/releases/download/1.0.0/aws-env -O aws-env
```
OR build from source
```
$ go install github.com/sendgrid/aws-env/cmd/aws-env
```

3. Start your application with aws-env
```
$ eval $(./aws-env) && ./my-app
```

Under the hood, aws-env will scan existing environment variable values for any that begin with the prefix `awsenv:`. It will then export new values for those using Parameter Store.

For example, if you had:
```
$ export DB_PASSWORD=awsenv:/testing/my-app/dbpass
$ export PRIVATE_KEY=awsenv:/testing/my-app/privatekey
```

Running `aws-env` would output:

```
$ export DB_USERNAME=$'some-secret-password'
$ export PRIVATE_KEY=$'some-private-key'
```

### Example Dockerfile
```
FROM alpine

RUN apk update && apk upgrade && \
  apk add --no-cache openssl ca-certificates

RUN wget https://github.com/sendgrid/aws-env/releases/download/1.0.0/aws-env -O /bin/aws-env && \
  chmod +x /bin/aws-env

CMD eval $(aws-env --region us-east-2) && /my-app
```

## Auth
aws-env exposes a `--profile` flag (or `AWS_ENV_PROFILE`) for use when running locally. This allows you to use the assume role tool and then specify the profile.
Otherwise, it will first look for a local metadata service (if running in EC2 or on-prem Kubernetes), and then fall back to environment variable auth.

## Region
aws-env defaults to looking at parameter store in the `us-east-1` region. You can override this with the `--region` flag (or `AWS_ENV_REGION`).

## Prefix
The default environment variable value prefix is `awsenv:`, this can be changed using the `--prefix` flag (or `AWS_ENV_PREFIX` env var).

## Considerations

* aws-env uses `$'string'` notation to support multi-line variables export. For this reason, to use aws-env, it's required to switch shell to /bin/bash:
```
CMD ["/bin/bash", "-c", "eval $(aws-env) && ./my-app"]
```
This isn't necessary if your Docker image's default shell is already bash.
