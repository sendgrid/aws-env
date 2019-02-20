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

CMD aws-env --region us-east-2 /my-app
```

## Auth
aws-env exposes a `--profile` flag (or `AWS_ENV_PROFILE`) for use when running locally. This allows you to use the assume role tool and then specify the profile.
Otherwise, it will first look for a local metadata service (if running in EC2 or on-prem Kubernetes), and then fall back to environment variable auth.

## Region
aws-env defaults to looking at parameter store in the `us-east-1` region. You can override this with the `--region` flag (or `AWS_ENV_REGION`).

## Prefix
The default environment variable value prefix is `awsenv:`, this can be changed using the `--prefix` flag (or `AWS_ENV_PREFIX` env var).

## Assume Role
aws-env exposes an `--assume-role` flag (or `AWS_ENV_ASSUME_ROLE`). This can be used to further assume roles if you have to gain access using a chain of roles.

### Example Assume Role
In Kubernetes, if you are using Annotations with a service role, `kube2iam` will assume your service role using the metadata service. You can then use the `--assume-role` flag to have your service role assume the ssm role to retrieve securely stored parameters with aws-env.

## Considerations

* When used without a command, aws-env uses `$'string'` notation to support multi-line variables export. For this reason, to use aws-env in this way, it's required to switch shell to /bin/bash:
```
CMD ["/bin/bash", "-c", "eval $(aws-env) && ./my-app"]
```
This isn't necessary if your Docker image's default shell is already bash.

Using the command invocation style does not have this limitation.
