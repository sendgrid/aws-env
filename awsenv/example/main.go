// Package main demonstrates how to use awsenv with AWS SDK v2
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/sendgrid/aws-env/awsenv"
)

func main() {
	// Example 1: Simple usage - replace all awsenv: prefixed environment variables
	fmt.Println("=== Example 1: Simple Environment Replacement ===")
	
	// Set some example environment variables (in real usage, these would be set externally)
	os.Setenv("DATABASE_URL", "awsenv:/myapp/database/url")
	os.Setenv("API_KEY", "awsenv:/myapp/secrets/api-key")
	os.Setenv("DEBUG_MODE", "true") // This won't be replaced (no prefix)
	
	fmt.Printf("Before replacement:\n")
	fmt.Printf("  DATABASE_URL: %s\n", os.Getenv("DATABASE_URL"))
	fmt.Printf("  API_KEY: %s\n", os.Getenv("API_KEY"))
	fmt.Printf("  DEBUG_MODE: %s\n", os.Getenv("DEBUG_MODE"))
	
	// Note: This would normally work with real SSM parameters
	// awsenv.MustReplaceEnv()
	
	fmt.Printf("\n=== Example 2: Custom AWS Configuration ===")
	
	ctx := context.Background()
	
	// Load custom AWS config with specific region
	cfg, err := config.LoadDefaultConfig(ctx, 
		config.WithRegion("us-west-2"),
	)
	if err != nil {
		log.Printf("Failed to load AWS config: %v", err)
		return
	}
	
	// Use custom config for replacement
	// awsenv.MustReplaceEnvWithConfig(ctx, cfg)
	
	fmt.Printf("\n=== Example 3: Programmatic Parameter Fetching ===")
	
	// Create SSM client and parameter getter
	ssmClient := ssm.NewFromConfig(cfg)
	getter := awsenv.NewParamsGetter(ssmClient)
	
	// Example parameter names (these would need to exist in your AWS account)
	parameterNames := []string{
		"/myapp/database/url",
		"/myapp/secrets/api-key",
	}
	
	// Fetch parameters programmatically
	params, err := getter.GetParams(ctx, parameterNames)
	if err != nil {
		log.Printf("Failed to get parameters: %v", err)
		// In a real scenario with valid parameters, this would work
	} else {
		fmt.Printf("Fetched parameters:\n")
		for name, value := range params {
			fmt.Printf("  %s: %s\n", name, value)
		}
	}
	
	fmt.Printf("\n=== Example 4: Using with Custom Prefix ===")
	
	// Create a replacer with custom prefix
	customReplacer := awsenv.NewReplacer("myapp:", getter)
	
	// This would look for environment variables starting with "myapp:"
	err = customReplacer.ReplaceAll(ctx)
	if err != nil {
		log.Printf("Failed to replace with custom prefix: %v", err)
	}
	
	fmt.Printf("\n=== Example 5: Error Handling ===")
	
	// Non-panicking version for better error handling
	replacer := awsenv.NewReplacer(awsenv.DefaultPrefix, getter)
	err = replacer.ReplaceAll(ctx)
	if err != nil {
		log.Printf("Replacement failed (expected with demo data): %v", err)
	} else {
		fmt.Printf("Replacement successful!\n")
	}
	
	fmt.Printf("\n=== Example 6: Batch Size Information ===")
	
	// Show the parameter limit for batching
	if limitedGetter, ok := getter.(awsenv.LimitedParamsGetter); ok {
		fmt.Printf("Parameter batch limit: %d\n", limitedGetter.GetParamsLimit())
	}
	
	fmt.Printf("\nExamples completed! In a real environment with valid SSM parameters,\n")
	fmt.Printf("the replacements would actually occur.\n")
}
