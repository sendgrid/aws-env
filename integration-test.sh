#!/bin/bash
set -e

export GO111MODULE=on
make build

aws --region us-east-1 ssm put-parameter --name /some/test/key --value "my-secret-value" --type SecureString --key-id "alias/aws/ssm" --overwrite

# Test eval invocation
echo "Eval Invocation:"
export AWS_ENV_TEST_KEY_EVAL='awsenv:/some/test/key'
export AWS_ENV_TEST_KEY_EVAL2='awsenv:/some/test/key'
echo "before: AWS_ENV_TEST_KEY_EVAL=$AWS_ENV_TEST_KEY_EVAL"
echo "before: AWS_ENV_TEST_KEY_EVAL2=$AWS_ENV_TEST_KEY_EVAL2"
eval $(./aws-env)
echo "after: AWS_ENV_TEST_KEY_EVAL=$AWS_ENV_TEST_KEY_EVAL"
echo "after: AWS_ENV_TEST_KEY_EVAL2=$AWS_ENV_TEST_KEY_EVAL2"

echo ""

# Test command invocation
echo "Command Invocation:"
export AWS_ENV_TEST_KEY_CMD='awsenv:/some/test/key'
echo "before: AWS_ENV_TEST_KEY_CMD=$AWS_ENV_TEST_KEY_CMD"
echo "after: $(./aws-env env | grep AWS_ENV_TEST_KEY_CMD)"

