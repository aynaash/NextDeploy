//go:build ignore
// +build ignore

package playground

import (
	"context"
	"log"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

func main() {
	// Create custom credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "AKIA...", // Your access key
				SecretAccessKey: "...",     // Your secret key
			}, nil
		})),
	)
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	// Get ECR auth token
	ecrClient := ecr.NewFromConfig(cfg)
	output, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		log.Fatalf("Failed to get ECR token: %v", err)
	}

	// The token comes base64 encoded with "AWS:" prefix
	token := *output.AuthorizationData[0].AuthorizationToken
	decodedToken := strings.TrimPrefix(token, "AWS:")

	// Login to Docker
	cmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", "123456789.dkr.ecr.us-east-1.amazonaws.com")
	cmd.Stdin = strings.NewReader(decodedToken)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Docker login failed: %v", err)
	}

	log.Println("Successfully logged in to ECR")
}
