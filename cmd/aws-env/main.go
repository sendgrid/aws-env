package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	awsenv "github.com/sendgrid/aws-env"
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
			Usage:       "enable ECS mode, using the default credential provider to support ECS",
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
	// Create a bridge between v1 SSM client and v2 interface
	getter := &v1SSMGetter{client: ssmClient}
	r := awsenv.NewReplacer(prefix, getter)

	if c.NArg() == 0 {
		return dump(r)
	}

	args := c.Args()
	return invoke(r, args.First(), args.Tail())
}

func fileReplacement(ssmClient *ssm.SSM) error {
	// Create a bridge between v1 SSM client and v2 interface
	getter := &v1SSMGetter{client: ssmClient}
	r := awsenv.NewFileReplacer(prefix, fileName, getter)

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

	// in order to make sure that we catch and propagate signals correctly, we need
	// to decouple starting the command and waiting for it to complete, so we can
	// send signals as it runs
	err = cmd.Start()
	if err != nil {
		log.WithError(err).Error("failed to start child process")
		return err
	}

	// wait for the command to finish
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
		close(errCh)
	}()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigCh:
			// this errror case only seems possible if the OS has released the process
			// or if it isn't started. So we _should_ be able to break
			if err := cmd.Process.Signal(sig); err != nil {
				log.WithError(err).WithField("signal", sig).Error("error sending signal")
				return err
			}
		case err := <-errCh:
			// the command finished.
			if err != nil {
				log.WithError(err).Error("command failed")
				return err
			}
			return nil
		}
	}
}

// v1SSMGetter is a bridge that adapts AWS SDK v1 SSM client to work with the v2-based library interface
type v1SSMGetter struct {
	client *ssm.SSM
}

func (v *v1SSMGetter) GetParams(ctx context.Context, names []string) (map[string]string, error) {
	// Convert string slice to AWS v1 format
	ptrs := make([]*string, len(names))
	for i := range names {
		ptrs[i] = aws.String(names[i])
	}

	input := &ssm.GetParametersInput{
		Names:          ptrs,
		WithDecryption: aws.Bool(true),
	}

	resp, err := v.client.GetParametersWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	// Convert response to map
	result := make(map[string]string, len(resp.Parameters))
	for _, param := range resp.Parameters {
		if param.Name != nil && param.Value != nil {
			result[*param.Name] = *param.Value
		}
	}

	return result, nil
}

func (v *v1SSMGetter) GetParamsLimit() int {
	return 10 // AWS SSM limit
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Fatalf("%s failed to start", app.Name)
	}
}
