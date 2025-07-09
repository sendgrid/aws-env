# AWS Environment Variable Replacer

A Go library for replacing environment variables and configuration files with values from AWS Systems Manager Parameter Store, using **AWS SDK v2**.

## Features

- Replace environment variables with AWS SSM Parameter Store values
- Replace configuration file values with SSM parameters
- Concurrent parameter fetching with configurable batch sizes
- Support for encrypted parameters
- AWS SDK v2 support with improved performance and features

## Installation

```bash
go get github.com/sendgrid/aws-env
```

## Quick Start

### Environment Variable Replacement

The simplest way to use awsenv is to call `MustReplaceEnv()` which will:
1. Scan all environment variables for the `awsenv:` prefix
2. Fetch corresponding values from SSM Parameter Store
3. Replace the environment variables with the fetched values

```go
package main

import "github.com/sendgrid/aws-env"

func main() {
    // This will replace any environment variables that start with "awsenv:"
    // with their corresponding values from SSM Parameter Store
    awsenv.MustReplaceEnv()
    
    // Your application code here...
}
```

#### Example Environment Variables

```bash
export DATABASE_URL="awsenv:/myapp/database/url"
export API_KEY="awsenv:/myapp/secrets/api-key"
export DEBUG_MODE="true"  # This won't be replaced (no prefix)
```

After calling `MustReplaceEnv()`, the environment variables will contain the actual values from Parameter Store.

### Custom Configuration

For more control over the AWS configuration:

```go
package main

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/sendgrid/aws-env"
)

func main() {
    ctx := context.Background()
    
    // Load custom AWS config
    cfg, err := config.LoadDefaultConfig(ctx, 
        config.WithRegion("us-west-2"),
    )
    if err != nil {
        panic(err)
    }
    
    // Use custom config
    awsenv.MustReplaceEnvWithConfig(ctx, cfg)
}
```

### Programmatic Parameter Fetching

You can also use the library programmatically to fetch specific parameters:

```go
package main

import (
    "context"
    "fmt"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/ssm"
    "github.com/sendgrid/aws-env"
)

func main() {
    ctx := context.Background()
    
    // Load AWS config
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        panic(err)
    }
    
    // Create SSM client and parameter getter
    ssmClient := ssm.NewFromConfig(cfg)
    getter := awsenv.NewParamsGetter(ssmClient)
    
    // Fetch specific parameters
    params, err := getter.GetParams(ctx, []string{
        "/myapp/database/url",
        "/myapp/secrets/api-key",
    })
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Database URL: %s\n", params["/myapp/database/url"])
    fmt.Printf("API Key: %s\n", params["/myapp/secrets/api-key"])
}
```

### File Replacement

You can also replace values in configuration files:

```go
package main

import (
    "context"
    "github.com/sendgrid/aws-env"
)

func main() {
    ctx := context.Background()
    
    // Create a file replacer
    getter := awsenv.NewParamsGetterFromConfig(cfg)
    fileReplacer := awsenv.NewFileReplacer("awsenv:", "/path/to/config.conf", getter)
    
    // Replace all awsenv: prefixed values in the file
    err := fileReplacer.ReplaceAll(ctx)
    if err != nil {
        panic(err)
    }
}
```

## Configuration

### Environment Variable Prefix

By default, awsenv looks for environment variables with the `awsenv:` prefix. You can use a custom prefix:

```go
replacer := awsenv.NewReplacer("myprefix:", getter)
```

### AWS Credentials

awsenv uses the standard AWS credential chain:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM instance profile (when running on EC2)
4. IAM roles for service accounts (when running on EKS)

### Required IAM Permissions

Your AWS credentials need the following IAM permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ssm:GetParameters"
            ],
            "Resource": "arn:aws:ssm:*:*:parameter/*"
        }
    ]
}
```

## Migration from AWS SDK v1

This library has been updated to use AWS SDK v2. If you were using a previous version with SDK v1, here are the key changes:

### What's New in v2

- **Better Performance**: Improved connection pooling and request handling
- **Smaller Binary Size**: Modular architecture reduces dependency bloat  
- **Context Support**: Built-in context support for better cancellation handling
- **Improved Error Handling**: More structured and actionable error responses

### Breaking Changes

- The `v1/` package has been removed
- The `v2/` package has been merged into the main package
- AWS SDK v1 dependencies have been removed

### Migration Path

**Before (v1):**
```go
import "github.com/sendgrid/aws-env/v1"

v1.MustReplaceEnv()
```

**After (v2):**
```go
import "github.com/sendgrid/aws-env"

awsenv.MustReplaceEnv()
```

The core functionality remains the same, but you now get all the benefits of AWS SDK v2.

## Error Handling

### Panicking Functions

- `MustReplaceEnv()` - Panics on any error
- `MustReplaceEnvWithContext()` - Panics on any error
- `MustReplaceEnvWithConfig()` - Panics on any error

### Non-Panicking Functions

- `replacer.ReplaceAll()` - Returns error
- `getter.GetParams()` - Returns error

```go
replacer := awsenv.NewReplacer(awsenv.DefaultPrefix, getter)
err := replacer.ReplaceAll(ctx)
if err != nil {
    // Handle error appropriately
    log.Printf("Failed to replace environment variables: %v", err)
}
```

## Performance

### Concurrent Fetching

awsenv fetches parameters concurrently in batches. The default batch size is 10 (AWS SSM limit), but this is handled automatically.

### Caching

For applications that need to fetch parameters multiple times, consider implementing a caching layer:

```go
type CachedGetter struct {
    getter awsenv.ParamsGetter
    cache  map[string]string
    mu     sync.RWMutex
}

// Implement your own caching logic as needed
```

## Examples

See the test files for more usage examples:
- `aws_test.go` - Basic parameter fetching tests
- `replacer_test.go` - Environment replacement tests  
- `file_replacer_test.go` - File replacement tests

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Run `go test ./...` to ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the same terms as the original aws-env project.
