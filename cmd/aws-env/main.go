package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/sendgrid/aws-env/awsenv"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	builtAt string
	gitHash string
	version string

	app = initApp()

	prefix  string
	region  string
	profile string
)

func initApp() *cli.App {
	newApp := cli.NewApp()
	newApp.Name = "aws-env"
	newApp.Version = version
	newApp.Authors = []cli.Author{
		{Name: "Michael Robinson", Email: "michael.robinson@sendgrid.com"},
	}
	newApp.Usage = "set environment variables with values from parameter store"
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
	}
	newApp.Action = run

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

	awsCfg := aws.NewConfig().WithRegion(region).WithCredentials(creds)
	sess := session.Must(session.NewSession(awsCfg))
	ssmClient := ssm.New(sess)

	r := awsenv.NewReplacer(prefix, ssmClient)
	newVars, err := r.ReplaceAll()
	if err != nil {
		log.WithError(err).Error("failed to replace env vars")
		os.Exit(1)
	}

	if len(newVars) == 0 {
		log.Info("nothing to replace")
		return nil
	}

	for name, newVal := range newVars {
		log.WithField("envvar", name).Info("replacing")
		fmt.Printf("export %s=$'%s'\n", name, newVal)
	}

	return nil
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Errorf("%s failed to start", app.Name)
	}
}
