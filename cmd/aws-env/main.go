package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/sendgrid/aws-env/awsenv"
	v1 "github.com/sendgrid/aws-env/awsenv/v1"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
)

var (
	builtAt string
	gitHash string
	version string

	app = initApp()

	prefix     string
	region     string
	profile    string
	assumeRole string
	fileName   string
	ecs        bool
)

const description = `
aws-env behaves similarly to the posix env command: if passed a command (with
optional arguments), that command will be invoked with additional environment
variables set from parameter store. If no command is passed, aws-env will
output export statements suitable for use with eval or source shell builtins.
It does support a -f flag to do an in place replacement of the first occurrence
per line of each prefixed item, delimited by whitespace (or \" then whitespace)
`

func initApp() *cli.App {
	newApp := cli.NewApp()
	newApp.Name = "aws-env"
	newApp.Version = version
	newApp.Usage = "set environment variables with values from parameter store"
	newApp.ArgsUsage = "[program [arguments...]]"
	newApp.Description = description
	newApp.Action = run
	newApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "prefix",
			EnvVar:      "AWS_ENV_PREFIX",
			Usage:       "prefix shared by values that should be replaced (value, NOT name)",
			Value:       awsenv.DefaultPrefix,
			Destination: &prefix,
		},
		cli.StringFlag{
			Name:        "region",
			EnvVar:      "AWS_ENV_REGION",
			Usage:       "aws region parameter store is in",
			Destination: &region,
			Value:       "us-east-1",
		},
		cli.StringFlag{
			Name:        "profile",
			EnvVar:      "AWS_ENV_PROFILE",
			Usage:       "aws profile to use for auth",
			Destination: &profile,
		},
		cli.StringFlag{
			Name:        "assume-role",
			EnvVar:      "AWS_ENV_ASSUME_ROLE",
			Usage:       "aws role to assume after initial creds",
			Destination: &assumeRole,
		},
		cli.StringFlag{
			Name:        "file, f",
			Usage:       "file to be updated by aws-env with Parameter Store values",
			Destination: &fileName,
		},
		cli.BoolFlag{
			Name:        "ecs",
			EnvVar:      "AWS_ENV_ECS",
			Usage:       "Enable ECS mode, using the default credential provider to support ECS",
			Destination: &ecs,
		},
	}
	newApp.Commands = append(newApp.Commands, cli.Command{
		Name:   "licenses",
		Usage:  "print licenses of libraries used",
		Action: licenseCommand,
	})

	return newApp
}

func run(c *cli.Context) error {
	log.WithFields(log.Fields{
		"app_version": version,
		"git_hash":    gitHash,
		"built_at":    builtAt,
	}).Info("aws-env starting")

	// First try the ec2 metadata service (kube2iam)
	// Then try the environment variables
	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(session.Must(session.NewSession())),
			},
			&credentials.EnvProvider{},
		})
	// Unless profile is specified, then that wins priority
	if profile != "" {
		creds = credentials.NewSharedCredentials("", profile)
	}
	// Unless ECS is specified, then use the default credentials
	if ecs {
		creds = defaults.Get().Config.Credentials
	}

	// If assumeRole is specified, then call sts and get further assume creds
	if assumeRole != "" {
		awsCfg := aws.NewConfig().WithRegion(region).WithCredentials(creds)
		sess := session.Must(session.NewSession(awsCfg))
		stsClient := sts.New(sess)

		assumeRoleInput := &sts.AssumeRoleInput{
			RoleArn:         aws.String(assumeRole),
			RoleSessionName: aws.String("awsenv_assume_role_session"),
		}

		assumeRoleOutput, err := stsClient.AssumeRole(assumeRoleInput)
		if err != nil {
			log.WithFields(log.Fields{
				"assume_role": assumeRole,
			}).Error("unable to assume role")
			return err
		}

		creds = credentials.NewStaticCredentials(
			aws.StringValue(assumeRoleOutput.Credentials.AccessKeyId),
			aws.StringValue(assumeRoleOutput.Credentials.SecretAccessKey),
			aws.StringValue(assumeRoleOutput.Credentials.SessionToken),
		)
	}

	awsCfg := aws.NewConfig().WithRegion(region).WithCredentials(creds)
	sess := session.Must(session.NewSession(awsCfg))
	ssmClient := ssm.New(sess)

	if fileName != "" {
		return fileReplacement(ssmClient)
	}
	return envReplacement(c, ssmClient)
}

func envReplacement(c *cli.Context, ssmClient *ssm.SSM) error {
	r := awsenv.NewReplacer(prefix, v1.NewParamsGetter(ssmClient))

	if c.NArg() == 0 {
		return dump(r)
	}

	args := c.Args()
	return invoke(r, args.First(), args.Tail())
}

func fileReplacement(ssmClient *ssm.SSM) error {
	r := awsenv.NewFileReplacer(prefix, fileName, v1.NewParamsGetter(ssmClient))

	ctx := context.Background()
	return r.ReplaceAll(ctx)
}

func dump(r *awsenv.Replacer) error {
	ctx := context.Background()

	vars, err := r.Replacements(ctx)
	if err != nil {
		return err
	}

	if len(vars) == 0 {
		log.Info("nothing to replace")
		return nil
	}

	for name, newVal := range vars {
		log.WithField("envvar", name).Info("replacing")
		fmt.Printf("export %s=$'%s'\n", name, newVal)
	}

	return nil
}

func invoke(r *awsenv.Replacer, prog string, args []string) error {
	ctx := context.Background()

	err := r.ReplaceAll(ctx)
	if err != nil {
		log.WithError(err).Error("failed to replace env vars")
		return err
	}

	cmd := exec.Command(prog, args...) // nolint: gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Fatalf("%s failed to start", app.Name)
	}
}
