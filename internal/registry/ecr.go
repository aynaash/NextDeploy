package registry

import (
	"bytes"
	"context"
	"nextdeploy/internal/config"
	"nextdeploy/internal/failfast"
	"nextdeploy/internal/git"
	"nextdeploy/internal/logger"
	"os/exec"
)

var (
	ECRLogger = logger.PackageLogger("ECR", "🅰️ ECR")
)

type ECRContext struct {
	ECRRepoName string
	ECRRegion   string
}

func (ctx ECRContext) ECRURL() string {
	cfg, err := config.Load()
	failfast.Failfast(err, failfast.Error, "Failed to config at ecr context preps")
	return cfg.Docker.Image
}
func (ctx ECRContext) FullImageName(image string) string {
	tag, err := git.GetCommitHash()
	failfast.Failfast(err, failfast.Error, "Failed to get commit hash")
	// faull image name
	fullImage := image + ":" + tag
	return fullImage
}
func PrepareECRPushContext(ctx context.Context, ecr ECRContext) error {
	ECRLogger.Info("Preparing ECR context for account %s in region %s", ecr.ECRRepoName, ecr.ECRRegion)
	// Get login password from aws CLI
	loginCommand := exec.CommandContext(ctx, "aws", "ecr", "get-login-password", "--region", ecr.ECRRegion)
	var stdout, stderr bytes.Buffer
	loginCommand.Stdout = &stdout
	loginCommand.Stderr = &stderr
	err := loginCommand.Run()
	failfast.Failfast(err, failfast.Error, "Failed to get ECR login password: %s")
	password := stdout.String()
	// pip token to docker login
	dockerLoginCommand := exec.CommandContext(ctx, "docker", "login", "--username", "AWS", "--password-stdin", ecr.ECRURL())
	dockerLoginCommand.Stdin = bytes.NewBufferString(password)
	err = dockerLoginCommand.Run()
	failfast.Failfast(err, failfast.Error, "Failed to login to ECR")

	ECRLogger.Info("Successfully logged in to ECR repository %s", ecr.ECRRepoName)
	ECRLogger.Success("ECR push context prepared successfully for account %s in region %s", ecr.ECRRepoName, ecr.ECRRegion)

	return nil
}

func PrepareECRPullContext(ctx context.Context, ecr ECRContext) (token string, error error) {
	ECRLogger.Info("Preparing ECR pull context")
	// Get login password from aws CLI
	loginCommand := exec.CommandContext(ctx, "aws", "ecr", "get-login-password", "--region", ecr.ECRRepoName)
	var stdout, stderr bytes.Buffer
	loginCommand.Stdout = &stdout
	loginCommand.Stderr = &stderr
	err := loginCommand.Run()
	failfast.Failfast(err, failfast.Error, "Failed to get ECR login password: %s")
	password := stdout.String()
	// pip token to docker login
	dockerLoginCommand := exec.CommandContext(ctx, "docker", "login", "--username", "AWS", "--password-stdin", ecr.ECRURL())
	dockerLoginCommand.Stdin = bytes.NewBufferString(password)
	err = dockerLoginCommand.Run()
	failfast.Failfast(err, failfast.Error, "Failed to login to ECR")

	ECRLogger.Info("Successfully logged in to ECR repository %s", ecr.ECRRepoName)
	ECRLogger.Success("ECR pull context prepared successfully")

	return password, nil
}
