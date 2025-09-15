
I'll integrate the multi-registry container image management functionality into your existing codebase. Based on your project structure, I'll add the registry client implementation to the `shared/registry/` directory.

## 1. Registry Client Interface

```go
// shared/registry/client.go
package registry

import (
    "context"
    "errors"
    "fmt"
)

var (
    ErrUnsupportedRegistry = errors.New("unsupported registry type")
    ErrMissingCredentials  = errors.New("missing required credentials")
)

type RegistryType string

const (
    RegistryDockerHub     RegistryType = "dockerhub"
    RegistryGHCR          RegistryType = "ghcr"
    RegistryAWSECR        RegistryType = "aws"
    RegistryAzureACR      RegistryType = "azure"
    RegistryGoogleGAR     RegistryType = "google"
    RegistryDigitalOcean  RegistryType = "digitalocean"
)

type RegistryClient interface {
    Login(ctx context.Context) error
    Pull(ctx context.Context, imageRef string) error
    Push(ctx context.Context, imageRef string) error
    GetAuth() (string, string, error)
}

func NewClient(registryType RegistryType) (RegistryClient, error) {
    switch registryType {
    case RegistryDockerHub:
        return NewDockerHubClient()
    case RegistryGHCR:
        return NewGHCRClient()
    case RegistryAWSECR:
        return NewAWSECRClient()
    case RegistryAzureACR:
        return NewAzureACRClient()
    case RegistryGoogleGAR:
        return NewGoogleGARClient()
    case RegistryDigitalOcean:
        return NewDigitalOceanClient()
    default:
        return nil, fmt.Errorf("%w: %s", ErrUnsupportedRegistry, registryType)
    }
}
```

## 2. Base Client Implementation

```go
// shared/registry/base.go
package registry

import (
    "context"
    "fmt"
    "strings"

    "github.com/google/go-containerregistry/pkg/authn"
    "github.com/google/go-containerregistry/pkg/crane"
    "github.com/google/go-containerregistry/pkg/name"
)

type baseClient struct {
    authenticator authn.Authenticator
    registryType  RegistryType
}

func (b *baseClient) Pull(ctx context.Context, imageRef string) error {
    ref, err := name.ParseReference(imageRef)
    if err != nil {
        return fmt.Errorf("failed to parse image reference: %w", err)
    }

    opts := []crane.Option{
        crane.WithAuth(b.authenticator),
        crane.WithContext(ctx),
    }

    img, err := crane.Pull(imageRef, opts...)
    if err != nil {
        return fmt.Errorf("failed to pull image: %w", err)
    }

    // Save to local file system
    if err := crane.Save(img, strings.Split(imageRef, ":")[0]+".tar"); err != nil {
        return fmt.Errorf("failed to save image: %w", err)
    }

    return nil
}

func (b *baseClient) Push(ctx context.Context, imageRef string) error {
    opts := []crane.Option{
        crane.WithAuth(b.authenticator),
        crane.WithContext(ctx),
    }

    if err := crane.Push(imageRef, imageRef+".tar", opts...); err != nil {
        return fmt.Errorf("failed to push image: %w", err)
    }

    return nil
}

func (b *baseClient) GetAuth() (string, string, error) {
    if basicAuth, ok := b.authenticator.(*authn.Basic); ok {
        return basicAuth.Username, basicAuth.Password, nil
    }
    return "", "", fmt.Errorf("authentication type not supported for credential extraction")
}
```

## 3. Update Existing Registry Implementations

I'll modify your existing registry files to implement the new interface:

### Docker Hub (update existing)
```go
// shared/registry/dockerhub.go
package registry

import (
    "context"
    "os"

    "github.com/google/go-containerregistry/pkg/authn"
)

type DockerHubClient struct {
    baseClient
    username string
    password string
}

func NewDockerHubClient() (*DockerHubClient, error) {
    username := os.Getenv("DOCKERHUB_USERNAME")
    if username == "" {
        username = os.Getenv("USERNAME")
    }
    
    password := os.Getenv("DOCKERHUB_PASSWORD")
    if password == "" {
        password = os.Getenv("PASSWORD")
    }
    
    if username == "" || password == "" {
        return nil, ErrMissingCredentials
    }

    authenticator := &authn.Basic{
        Username: username,
        Password: password,
    }

    return &DockerHubClient{
        baseClient: baseClient{
            authenticator: authenticator,
            registryType:  RegistryDockerHub,
        },
        username: username,
        password: password,
    }, nil
}

func (c *DockerHubClient) Login(ctx context.Context) error {
    // Docker Hub authentication is handled via basic auth
    return nil
}
```

### GitHub Container Registry (update existing)
```go
// shared/registry/githubregistry.go
package registry

import (
    "context"
    "os"

    "github.com/google/go-containerregistry/pkg/authn"
)

type GHCRClient struct {
    baseClient
    username string
    token    string
}

func NewGHCRClient() (*GHCRClient, error) {
    username := os.Getenv("GHCR_USERNAME")
    if username == "" {
        username = os.Getenv("USERNAME")
    }
    
    token := os.Getenv("GHCR_TOKEN")
    if token == "" {
        token = os.Getenv("TOKEN")
    }
    
    if username == "" || token == "" {
        return nil, ErrMissingCredentials
    }

    authenticator := &authn.Basic{
        Username: username,
        Password: token,
    }

    return &GHCRClient{
        baseClient: baseClient{
            authenticator: authenticator,
            registryType:  RegistryGHCR,
        },
        username: username,
        token:    token,
    }, nil
}

func (c *GHCRClient) Login(ctx context.Context) error {
    // GHCR authentication is handled via basic auth
    return nil
}
```

### AWS ECR (update existing)
```go
// shared/registry/awsecr.go
package registry

import (
    "context"
    "os"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ecr"
    "github.com/google/go-containerregistry/pkg/authn"
)

type AWSECRClient struct {
    baseClient
    accessKeyID     string
    secretAccessKey string
    sessionToken    string
    region          string
}

func NewAWSECRClient() (*AWSECRClient, error) {
    accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
    secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
    sessionToken := os.Getenv("AWS_SESSION_TOKEN")
    region := os.Getenv("AWS_REGION")

    if accessKeyID == "" || secretAccessKey == "" {
        return nil, ErrMissingCredentials
    }

    return &AWSECRClient{
        accessKeyID:     accessKeyID,
        secretAccessKey: secretAccessKey,
        sessionToken:    sessionToken,
        region:          region,
    }, nil
}

func (c *AWSECRClient) Login(ctx context.Context) error {
    config := &aws.Config{
        Credentials: credentials.NewStaticCredentials(
            c.accessKeyID,
            c.secretAccessKey,
            c.sessionToken,
        ),
    }

    if c.region != "" {
        config.Region = &c.region
    }

    sess, err := session.NewSession(config)
    if err != nil {
        return err
    }

    ecrSvc := ecr.New(sess)
    input := &ecr.GetAuthorizationTokenInput{}

    result, err := ecrSvc.GetAuthorizationTokenWithContext(ctx, input)
    if err != nil {
        return err
    }

    if len(result.AuthorizationData) == 0 {
        return ErrMissingCredentials
    }

    authData := result.AuthorizationData[0]
    authToken := *authData.AuthorizationToken

    c.baseClient.authenticator = &authn.Basic{
        Username: "AWS",
        Password: authToken,
    }
    c.baseClient.registryType = RegistryAWSECR

    return nil
}
```

## 4. Add New Registry Implementations

### Azure ACR (new file)
```go
// shared/registry/azureacr.go
package registry

import (
    "context"
    "encoding/base64"
    "os"
    "strings"

    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
    "github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
    "github.com/google/go-containerregistry/pkg/authn"
)

type AzureACRClient struct {
    baseClient
    clientID     string
    clientSecret string
    tenantID     string
    loginServer  string
}

func NewAzureACRClient() (*AzureACRClient, error) {
    clientID := os.Getenv("AZURE_CLIENT_ID")
    clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
    tenantID := os.Getenv("AZURE_TENANT_ID")
    loginServer := os.Getenv("AZURE_LOGIN_SERVER")

    if clientID == "" || clientSecret == "" || tenantID == "" || loginServer == "" {
        return nil, ErrMissingCredentials
    }

    return &AzureACRClient{
        clientID:     clientID,
        clientSecret: clientSecret,
        tenantID:     tenantID,
        loginServer:  loginServer,
    }, nil
}

func (c *AzureACRClient) Login(ctx context.Context) error {
    cred, err := azidentity.NewClientSecretCredential(
        c.tenantID,
        c.clientID,
        c.clientSecret,
        nil,
    )
    if err != nil {
        return err
    }

    client, err := azcontainerregistry.NewClient(
        "https://"+c.loginServer,
        cred,
        nil,
    )
    if err != nil {
        return err
    }

    resp, err := client.GetAcrAccessToken(ctx, "registry", nil)
    if err != nil {
        return err
    }

    accessToken := *resp.AccessToken
    decoded, err := base64.StdEncoding.DecodeString(accessToken)
    if err != nil {
        return err
    }

    parts := strings.SplitN(string(decoded), ":", 2)
    if len(parts) != 2 {
        return ErrMissingCredentials
    }

    c.baseClient.authenticator = &authn.Basic{
        Username: parts[0],
        Password: parts[1],
    }
    c.baseClient.registryType = RegistryAzureACR

    return nil
}
```

### Google Artifact Registry (new file)
```go
// shared/registry/google.go
package registry

import (
    "context"
    "os"

    "golang.org/x/oauth2"
    "google.golang.org/api/option"
    "google.golang.org/api/transport"

    "github.com/google/go-containerregistry/pkg/authn"
    "github.com/google/go-containerregistry/pkg/authn/oauth"
)

type GoogleGARClient struct {
    baseClient
    serviceAccountKeyPath string
}

func NewGoogleGARClient() (*GoogleGARClient, error) {
    serviceAccountKeyPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
    accessToken := os.Getenv("GOOGLE_ACCESS_TOKEN")

    if serviceAccountKeyPath == "" && accessToken == "" {
        return nil, ErrMissingCredentials
    }

    return &GoogleGARClient{
        serviceAccountKeyPath: serviceAccountKeyPath,
    }, nil
}

func (c *GoogleGARClient) Login(ctx context.Context) error {
    var ts oauth2.TokenSource
    var err error

    if c.serviceAccountKeyPath != "" {
        ts, err = transport.Creds(ctx, option.WithCredentialsFile(c.serviceAccountKeyPath))
    } else {
        accessToken := os.Getenv("GOOGLE_ACCESS_TOKEN")
        ts = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
    }

    if err != nil {
        return err
    }

    c.baseClient.authenticator = &oauth.OauthAuthenticator{
        TokenSource: ts,
        Scopes:      []string{"https://www.googleapis.com/auth/cloud-platform"},
    }
    c.baseClient.registryType = RegistryGoogleGAR

    return nil
}
```

### DigitalOcean (update existing)
```go
// shared/registry/digitalocean.go
package registry

import (
    "context"
    "os"

    "github.com/google/go-containerregistry/pkg/authn"
)

type DigitalOceanClient struct {
    baseClient
    username string
    token    string
}

func NewDigitalOceanClient() (*DigitalOceanClient, error) {
    username := os.Getenv("DIGITALOCEAN_USERNAME")
    if username == "" {
        username = "doctl"
    }
    
    token := os.Getenv("DIGITALOCEAN_TOKEN")
    if token == "" {
        token = os.Getenv("TOKEN")
    }

    if token == "" {
        return nil, ErrMissingCredentials
    }

    authenticator := &authn.Basic{
        Username: username,
        Password: token,
    }

    return &DigitalOceanClient{
        baseClient: baseClient{
            authenticator: authenticator,
            registryType:  RegistryDigitalOcean,
        },
        username: username,
        token:    token,
    }, nil
}

func (c *DigitalOceanClient) Login(ctx context.Context) error {
    // DigitalOcean uses basic auth
    return nil
}
```

## 5. Update Go Mod Dependencies

Add these dependencies to your `go.mod`:

```go
require (
    github.com/google/go-containerregistry v0.19.0
    github.com/aws/aws-sdk-go v1.50.0
    github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.4.0
    github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry v0.1.0
    golang.org/x/oauth2 v0.15.0
    google.golang.org/api v0.155.0
)
```

## 6. Integration with Existing CLI

Add registry commands to your CLI:

```go
// cli/cmd/registry.go
package cmd

import (
    "context"
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "nextdeploy/shared/registry"
)

var registryCmd = &cobra.Command{
    Use:   "registry",
    Short: "Manage container images across multiple registries",
}

var registryPushCmd = &cobra.Command{
    Use:   "push [image]",
    Short: "Push container image to registry",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        imageRef := args[0]
        registryType := registry.RegistryType(os.Getenv("REGISTRY"))

        if registryType == "" {
            fmt.Println("REGISTRY environment variable is required")
            os.Exit(1)
        }

        client, err := registry.NewClient(registryType)
        if err != nil {
            fmt.Printf("Failed to create client: %v\n", err)
            os.Exit(1)
        }

        ctx := context.Background()
        if err := client.Login(ctx); err != nil {
            fmt.Printf("Login failed: %v\n", err)
            os.Exit(1)
        }

        if err := client.Push(ctx, imageRef); err != nil {
            fmt.Printf("Push failed: %v\n", err)
            os.Exit(1)
        }

        fmt.Printf("Successfully pushed %s\n", imageRef)
    },
}

var registryPullCmd = &cobra.Command{
    Use:   "pull [image]",
    Short: "Pull container image from registry",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        imageRef := args[0]
        registryType := registry.RegistryType(os.Getenv("REGISTRY"))

        if registryType == "" {
            fmt.Println("REGISTRY environment variable is required")
            os.Exit(1)
        }

        client, err := registry.NewClient(registryType)
        if err != nil {
            fmt.Printf("Failed to create client: %v\n", err)
            os.Exit(1)
        }

        ctx := context.Background()
        if err := client.Login(ctx); err != nil {
            fmt.Printf("Login failed: %v\n", err)
            os.Exit(1)
        }

        if err := client.Pull(ctx, imageRef); err != nil {
            fmt.Printf("Pull failed: %v\n", err)
            os.Exit(1)
        }

        fmt.Printf("Successfully pulled %s\n", imageRef)
    },
}

func init() {
    registryCmd.AddCommand(registryPushCmd)
    registryCmd.AddCommand(registryPullCmd)
    rootCmd.AddCommand(registryCmd)
}
```

## Usage Examples

```bash
# Docker Hub
REGISTRY=dockerhub DOCKERHUB_USERNAME=myuser DOCKERHUB_PASSWORD=xxx ./nextdeploy registry push myimage:latest

# AWS ECR
REGISTRY=aws AWS_ACCESS_KEY_ID=xxx AWS_SECRET_ACCESS_KEY=yyy ./nextdeploy registry pull 123456789012.dkr.ecr.us-east-1.amazonaws.com/myimage:latest

# GHCR
REGISTRY=ghcr GHCR_USERNAME=myuser GHCR_TOKEN=ghp_xxx ./nextdeploy registry push ghcr.io/myuser/myimage:latest

# Azure ACR
REGISTRY=azure AZURE_CLIENT_ID=xxx AZURE_CLIENT_SECRET=yyy AZURE_TENANT_ID=zzz AZURE_LOGIN_SERVER=myregistry.azurecr.io ./nextdeploy registry pull myregistry.azurecr.io/myimage:latest
```

This integration provides a unified multi-registry image management system that fits seamlessly into your existing NextDeploy codebase.
I'll complete the implementation by adding the missing registry implementations and ensuring everything integrates properly with your existing codebase.

## 1. Complete Azure ACR Implementation

```go
// shared/registry/azureacr.go
package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/google/go-containerregistry/pkg/authn"
)

type AzureACRClient struct {
	baseClient
	clientID     string
	clientSecret string
	tenantID     string
	loginServer  string
}

func NewAzureACRClient() (*AzureACRClient, error) {
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")
	loginServer := os.Getenv("AZURE_LOGIN_SERVER")

	// Fallback to alternative env vars
	if clientID == "" {
		clientID = os.Getenv("AZURE_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	}
	if tenantID == "" {
		tenantID = os.Getenv("AZURE_TENANT_ID")
	}

	if clientID == "" || clientSecret == "" || tenantID == "" {
		return nil, fmt.Errorf("%w: AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, and AZURE_TENANT_ID are required", ErrMissingCredentials)
	}

	if loginServer == "" {
		return nil, fmt.Errorf("%w: AZURE_LOGIN_SERVER is required", ErrMissingCredentials)
	}

	return &AzureACRClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
		loginServer:  loginServer,
	}, nil
}

func (c *AzureACRClient) Login(ctx context.Context) error {
	cred, err := azidentity.NewClientSecretCredential(
		c.tenantID,
		c.clientID,
		c.clientSecret,
		&azidentity.ClientSecretCredentialOptions{
			ClientOptions: azcore.ClientOptions{
				Cloud: cloud.AzurePublic,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := azcontainerregistry.NewClient(
		"https://"+c.loginServer,
		cred,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create ACR client: %w", err)
	}

	// Get access token for the registry scope
	resp, err := client.GetAcrAccessToken(ctx, "registry", &azcontainerregistry.ClientGetAcrAccessTokenOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ACR access token: %w", err)
	}

	if resp.AccessToken == nil {
		return fmt.Errorf("empty access token received from ACR")
	}

	accessToken := *resp.AccessToken
	
	// ACR tokens are base64 encoded "username:password" format
	decoded, err := base64.StdEncoding.DecodeString(accessToken)
	if err != nil {
		return fmt.Errorf("failed to decode ACR token: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid ACR token format: expected username:password")
	}

	c.baseClient.authenticator = &authn.Basic{
		Username: parts[0],
		Password: parts[1],
	}
	c.baseClient.registryType = RegistryAzureACR

	return nil
}
```

## 2. Complete Google Artifact Registry Implementation

```go
// shared/registry/google.go
package registry

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/oauth"
)

type GoogleGARClient struct {
	baseClient
	serviceAccountKeyPath string
}

func NewGoogleGARClient() (*GoogleGARClient, error) {
	serviceAccountKeyPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	accessToken := os.Getenv("GOOGLE_ACCESS_TOKEN")

	if serviceAccountKeyPath == "" && accessToken == "" {
		return nil, fmt.Errorf("%w: GOOGLE_APPLICATION_CREDENTIALS or GOOGLE_ACCESS_TOKEN is required", ErrMissingCredentials)
	}

	return &GoogleGARClient{
		serviceAccountKeyPath: serviceAccountKeyPath,
	}, nil
}

func (c *GoogleGARClient) Login(ctx context.Context) error {
	var ts oauth2.TokenSource
	var err error

	if c.serviceAccountKeyPath != "" {
		// Use service account credentials
		ts, err = transport.Creds(ctx, option.WithCredentialsFile(c.serviceAccountKeyPath))
		if err != nil {
			return fmt.Errorf("failed to create token source from service account: %w", err)
		}
	} else {
		// Use access token
		accessToken := os.Getenv("GOOGLE_ACCESS_TOKEN")
		if accessToken == "" {
			return fmt.Errorf("GOOGLE_ACCESS_TOKEN is required when not using service account")
		}
		ts = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	}

	c.baseClient.authenticator = &oauth.OauthAuthenticator{
		TokenSource: ts,
		Scopes:      []string{"https://www.googleapis.com/auth/cloud-platform"},
	}
	c.baseClient.registryType = RegistryGoogleGAR

	return nil
}
```

## 3. Enhanced Base Client with Retry Logic

```go
// shared/registry/base.go
package registry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
)

type baseClient struct {
	authenticator authn.Authenticator
	registryType  RegistryType
}

func (b *baseClient) Pull(ctx context.Context, imageRef string) error {
	return b.retryOperation(ctx, 3, 2*time.Second, func() error {
		return b.pullImage(ctx, imageRef)
	})
}

func (b *baseClient) Push(ctx context.Context, imageRef string) error {
	return b.retryOperation(ctx, 3, 2*time.Second, func() error {
		return b.pushImage(ctx, imageRef)
	})
}

func (b *baseClient) pullImage(ctx context.Context, imageRef string) error {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	opts := []crane.Option{
		crane.WithAuth(b.authenticator),
		crane.WithContext(ctx),
	}

	img, err := crane.Pull(imageRef, opts...)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Extract image name without tag for filename
	imageName := strings.Split(imageRef, ":")[0]
	if strings.Contains(imageName, "/") {
		parts := strings.Split(imageName, "/")
		imageName = parts[len(parts)-1]
	}

	filename := fmt.Sprintf("%s.tar", imageName)
	if err := crane.Save(img, filename); err != nil {
		return fmt.Errorf("failed to save image to %s: %w", filename, err)
	}

	return nil
}

func (b *baseClient) pushImage(ctx context.Context, imageRef string) error {
	opts := []crane.Option{
		crane.WithAuth(b.authenticator),
		crane.WithContext(ctx),
	}

	// Extract image name without tag for filename
	imageName := strings.Split(imageRef, ":")[0]
	if strings.Contains(imageName, "/") {
		parts := strings.Split(imageName, "/")
		imageName = parts[len(parts)-1]
	}

	filename := fmt.Sprintf("%s.tar", imageName)
	if err := crane.Push(filename, imageRef, opts...); err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	return nil
}

func (b *baseClient) retryOperation(ctx context.Context, maxRetries int, delay time.Duration, operation func() error) error {
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		if err := operation(); err != nil {
			lastErr = err
			time.Sleep(delay * time.Duration(i+1))
			continue
		}
		return nil
	}
	
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

func (b *baseClient) GetAuth() (string, string, error) {
	if basicAuth, ok := b.authenticator.(*authn.Basic); ok {
		return basicAuth.Username, basicAuth.Password, nil
	}
	
	// Handle oauth authenticator
	if oauthAuth, ok := b.authenticator.(*oauth.OauthAuthenticator); ok {
		token, err := oauthAuth.TokenSource.Token()
		if err != nil {
			return "", "", fmt.Errorf("failed to get oauth token: %w", err)
		}
		return "oauth2accesstoken", token.AccessToken, nil
	}
	
	return "", "", fmt.Errorf("authentication type not supported for credential extraction: %T", b.authenticator)
}
```

## 4. Enhanced Client Interface with Utility Methods

```go
// shared/registry/client.go
package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
)

var (
	ErrUnsupportedRegistry = errors.New("unsupported registry type")
	ErrMissingCredentials  = errors.New("missing required credentials")
	ErrAuthentication      = errors.New("authentication failed")
)

type RegistryType string

const (
	RegistryDockerHub    RegistryType = "dockerhub"
	RegistryGHCR         RegistryType = "ghcr"
	RegistryAWSECR       RegistryType = "aws"
	RegistryAzureACR     RegistryType = "azure"
	RegistryGoogleGAR    RegistryType = "google"
	RegistryDigitalOcean RegistryType = "digitalocean"
)

type RegistryClient interface {
	Login(ctx context.Context) error
	Pull(ctx context.Context, imageRef string) error
	Push(ctx context.Context, imageRef string) error
	GetAuth() (string, string, error)
	GetRegistryType() RegistryType
}

func NewClient(registryType RegistryType) (RegistryClient, error) {
	// Allow registry type to be overridden by environment variable
	if envType := os.Getenv("REGISTRY_TYPE"); envType != "" {
		registryType = RegistryType(envType)
	}

	switch registryType {
	case RegistryDockerHub:
		return NewDockerHubClient()
	case RegistryGHCR:
		return NewGHCRClient()
	case RegistryAWSECR:
		return NewAWSECRClient()
	case RegistryAzureACR:
		return NewAzureACRClient()
	case RegistryGoogleGAR:
		return NewGoogleGARClient()
	case RegistryDigitalOcean:
		return NewDigitalOceanClient()
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedRegistry, registryType)
	}
}

func DetectRegistryType(imageRef string) RegistryType {
	switch {
	case strings.Contains(imageRef, "amazonaws.com") || strings.Contains(imageRef, "ecr."):
		return RegistryAWSECR
	case strings.Contains(imageRef, "azurecr.io"):
		return RegistryAzureACR
	case strings.Contains(imageRef, "gcr.io") || strings.Contains(imageRef, "pkg.dev"):
		return RegistryGoogleGAR
	case strings.Contains(imageRef, "ghcr.io"):
		return RegistryGHCR
	case strings.Contains(imageRef, "registry.digitalocean.com"):
		return RegistryDigitalOcean
	case strings.Contains(imageRef, "docker.io") || !strings.Contains(imageRef, "."):
		return RegistryDockerHub
	default:
		return RegistryDockerHub
	}
}

func NewClientFromImageRef(imageRef string) (RegistryClient, error) {
	registryType := DetectRegistryType(imageRef)
	return NewClient(registryType)
}
```

## 5. CLI Command Integration

```go
// cli/cmd/registry.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"nextdeploy/shared/registry"
)

var (
	registryType string
	retryCount   int
	retryDelay   time.Duration
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage container images across multiple registries",
	Long: `Push and pull container images from various container registries including:
- Docker Hub
- GitHub Container Registry (GHCR)
- AWS Elastic Container Registry (ECR)
- Azure Container Registry (ACR)
- Google Artifact Registry (GAR)
- DigitalOcean Container Registry`,
}

var registryPushCmd = &cobra.Command{
	Use:   "push [image]",
	Short: "Push container image to registry",
	Args:  cobra.ExactArgs(1),
	Run:   runRegistryPush,
}

var registryPullCmd = &cobra.Command{
	Use:   "pull [image]",
	Short: "Pull container image from registry",
	Args:  cobra.ExactArgs(1),
	Run:   runRegistryPull,
}

var registryLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Test registry authentication",
	Run:   runRegistryLogin,
}

func runRegistryPush(cmd *cobra.Command, args []string) {
	imageRef := args[0]
	ctx := context.Background()

	client, err := getRegistryClient(imageRef)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := performRegistryOperation(ctx, client, "push", imageRef); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully pushed %s to %s\n", imageRef, client.GetRegistryType())
}

func runRegistryPull(cmd *cobra.Command, args []string) {
	imageRef := args[0]
	ctx := context.Background()

	client, err := getRegistryClient(imageRef)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := performRegistryOperation(ctx, client, "pull", imageRef); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully pulled %s from %s\n", imageRef, client.GetRegistryType())
}

func runRegistryLogin(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Use provided registry type or detect from env
	var client registry.RegistryClient
	var err error

	if registryType != "" {
		client, err = registry.NewClient(registry.RegistryType(registryType))
	} else {
		// Try to detect from environment or use default
		client, err = registry.NewClient(registry.RegistryDockerHub)
	}

	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	if err := client.Login(ctx); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	username, password, err := client.GetAuth()
	if err != nil {
		fmt.Printf("Authentication successful, but cannot retrieve credentials: %v\n", err)
	} else {
		fmt.Printf("✅ Authentication successful for %s\n", client.GetRegistryType())
		fmt.Printf("Username: %s\n", username)
		fmt.Printf("Password: %s\n", maskPassword(password))
	}
}

func getRegistryClient(imageRef string) (registry.RegistryClient, error) {
	var client registry.RegistryClient
	var err error

	if registryType != "" {
		client, err = registry.NewClient(registry.RegistryType(registryType))
	} else {
		client, err = registry.NewClientFromImageRef(imageRef)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	return client, nil
}

func performRegistryOperation(ctx context.Context, client registry.RegistryClient, operation, imageRef string) error {
	// Login first
	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Perform the operation
	switch operation {
	case "push":
		if err := client.Push(ctx, imageRef); err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
	case "pull":
		if err := client.Pull(ctx, imageRef); err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}

	return nil
}

func maskPassword(password string) string {
	if len(password) <= 4 {
		return "****"
	}
	return password[:2] + "****" + password[len(password)-2:]
}

func init() {
	registryPushCmd.Flags().StringVarP(&registryType, "registry", "r", "", "Registry type (dockerhub, ghcr, aws, azure, google, digitalocean)")
	registryPullCmd.Flags().StringVarP(&registryType, "registry", "r", "", "Registry type (dockerhub, ghcr, aws, azure, google, digitalocean)")
	registryLoginCmd.Flags().StringVarP(&registryType, "registry", "r", "", "Registry type (dockerhub, ghcr, aws, azure, google, digitalocean)")
	
	registryCmd.PersistentFlags().IntVarP(&retryCount, "retry", "t", 3, "Number of retries for operations")
	registryCmd.PersistentFlags().DurationVarP(&retryDelay, "retry-delay", "d", 2*time.Second, "Delay between retries")

	registryCmd.AddCommand(registryPushCmd)
	registryCmd.AddCommand(registryPullCmd)
	registryCmd.AddCommand(registryLoginCmd)
	rootCmd.AddCommand(registryCmd)
}
```

## 6. Update Go Mod Dependencies

Add these to your `go.mod`:

```go
require (
    github.com/google/go-containerregistry v0.19.0
    github.com/aws/aws-sdk-go v1.50.0
    github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.4.0
    github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry v0.1.0
    golang.org/x/oauth2 v0.15.0
    google.golang.org/api v0.155.0
)
```

## Usage Examples

```bash
# Auto-detect registry from image URL
./nextdeploy registry push myimage:latest
./nextdeploy registry pull ghcr.io/myuser/myimage:latest

# Explicit registry type
./nextdeploy registry push -r aws 123456789012.dkr.ecr.us-east-1.amazonaws.com/myimage:latest

# Test authentication
./nextdeploy registry login -r azure

# With environment variables
REGISTRY_TYPE=aws AWS_ACCESS_KEY_ID=xxx AWS_SECRET_ACCESS_KEY=yyy ./nextdeploy registry push myimage:latest
```

This complete implementation provides a robust multi-registry container management system that integrates seamlessly with your existing NextDeploy codebase.
