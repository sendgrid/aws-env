package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/sendgrid/aws-env/awsenv"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
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
)

const description = `
aws-env behaves similarly to the posix env command: if passed a command (with
optional arguments), that command will be invoked with additional environment
variables set from parameter store. If no command is passed, aws-env will
output export statements suitable for use with eval or source shell builtins.`

func initApp() *cli.App {
	newApp := cli.NewApp()
	newApp.Name = "aws-env"
	newApp.Version = version
	newApp.Authors = []cli.Author{
		{Name: "Michael Robinson", Email: "michael.robinson@sendgrid.com"},
	}
	newApp.Usage = "set environment variables with values from parameter store"
	newApp.ArgsUsage = "[program [arguments...]]"
	newApp.Description = description
	newApp.Action = run
	newApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "prefix",
			EnvVar:      "AWS_ENV_PREFIX",
			Usage:       "prefix shared by values that should be replaced (value, NOT name)",
			Value:       "awsenv:",
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
	}

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

	r := awsenv.NewReplacer(prefix, ssmClient)
	newVars, err := r.ReplaceAll()
	if err != nil {
		log.WithError(err).Error("failed to replace env vars")
		os.Exit(1)
	}

	if c.NArg() == 0 {
		return dump(newVars)
	}
	args := c.Args()
	return invoke(newVars, args.First(), args.Tail())
}

func dump(vars map[string]string) error {
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

func invoke(vars map[string]string, prog string, args []string) error {
	env := os.Environ()
	for k, v := range vars {
		log.WithField("envvar", k).Info("replacing")
		env = append(env, k+"="+v)
	}
	cmd := exec.Command(prog, args...) // nolint: gosec
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Errorf("%s failed to start", app.Name)
	}
}
