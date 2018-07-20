#!/bin/bash
set -e

# NOTE: you have to run the assume role tool first and put your profile name here
PROFILE=aai-preprod

make build

aws --profile $PROFILE --region us-east-1 ssm put-parameter --name /some/test/key --value "my-secret-value" --type SecureString --key-id "alias/aws/ssm" --overwrite

export AWS_ENV_TEST_KEY='awsenv:/some/test/key'
echo "before: AWS_ENV_TEST_KEY=$AWS_ENV_TEST_KEY"
eval $(./aws-env --profile $PROFILE)
echo "after: AWS_ENV_TEST_KEY=$AWS_ENV_TEST_KEY"



