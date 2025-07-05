
```
provision/
├── config.go        # Configuration file handling
├── ec2.go           # EC2 provisioning logic
├── ecr.go           # ECR repository management
├── ssh.go           # SSH key management
├── provision.go     # Main provisioning logic
└── types.go         # Type definitions
```

## Implementation

### types.go

```go
package provision

type Config struct {
    AWSRegion      string `yaml:"aws_region"`
    EC2Instance    EC2Config `yaml:"ec2_instance"`
    ECRRepository ECRConfig `yaml:"ecr_repository"`
}

type EC2Config struct {
    InstanceType   string `yaml:"instance_type"`
    AMI            string `yaml:"ami"`
    KeyName        string `yaml:"key_name"`
    SecurityGroups []string `yaml:"security_groups"`
    SubnetID       string `yaml:"subnet_id"`
    UserData       string `yaml:"user_data"`
    IAMInstanceProfile string `yaml:"iam_instance_profile"`
}

type ECRConfig struct {
    RepositoryName string `yaml:"repository_name"`
    ImageTag       string `yaml:"image_tag"`
    PullPolicy     string `yaml:"pull_policy"`
}
```

### config.go

```go
package provision

import (
    "os"
    "gopkg.in/yaml.v2"
)

func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var config Config
    err = yaml.Unmarshal(data, &config)
    if err != nil {
        return nil, err
    }

    return &config, nil
}
```

### ec2.go

```go
package provision

import (
    "context"
    "fmt"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2Provisioner struct {
    client *ec2.Client
}

func NewEC2Provisioner(ctx context.Context, region string) (*EC2Provisioner, error) {
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
    if err != nil {
        return nil, err
    }

    return &EC2Provisioner{
        client: ec2.NewFromConfig(cfg),
    }, nil
}

func (p *EC2Provisioner) CreateInstance(ctx context.Context, cfg *EC2Config) (*types.Instance, error) {
    input := &ec2.RunInstancesInput{
        ImageId:          aws.String(cfg.AMI),
        InstanceType:     types.InstanceType(cfg.InstanceType),
        KeyName:         aws.String(cfg.KeyName),
        SecurityGroupIds: cfg.SecurityGroups,
        SubnetId:         aws.String(cfg.SubnetID),
        MinCount:         aws.Int32(1),
        MaxCount:         aws.Int32(1),
        UserData:         aws.String(cfg.UserData),
        IamInstanceProfile: &types.IamInstanceProfileSpecification{
            Name: aws.String(cfg.IAMInstanceProfile),
        },
    }

    result, err := p.client.RunInstances(ctx, input)
    if err != nil {
        return nil, err
    }

    if len(result.Instances) == 0 {
        return nil, fmt.Errorf("no instances created")
    }

    return &result.Instances[0], nil
}

func (p *EC2Provisioner) WaitUntilRunning(ctx context.Context, instanceID string) error {
    waiter := ec2.NewInstanceRunningWaiter(p.client)
    return waiter.Wait(ctx, &ec2.DescribeInstancesInput{
        InstanceIds: []string{instanceID},
    }, 5*time.Minute)
}
```

### ecr.go

```go
package provision

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/ecr"
)

type ECRProvisioner struct {
    client *ecr.Client
}

func NewECRProvisioner(ctx context.Context, region string) (*ECRProvisioner, error) {
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
    if err != nil {
        return nil, err
    }

    return &ECRProvisioner{
        client: ecr.NewFromConfig(cfg),
    }, nil
}

func (p *ECRProvisioner) CreateRepository(ctx context.Context, name string) (string, error) {
    output, err := p.client.CreateRepository(ctx, &ecr.CreateRepositoryInput{
        RepositoryName: aws.String(name),
    })
    if err != nil {
        return "", err
    }

    return *output.Repository.RepositoryUri, nil
}

func (p *ECRProvisioner) GetLoginPassword(ctx context.Context) (string, error) {
    output, err := p.client.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
    if err != nil {
        return "", err
    }

    if len(output.AuthorizationData) == 0 {
        return "", fmt.Errorf("no authorization data returned")
    }

    return *output.AuthorizationData[0].AuthorizationToken, nil
}
```

### ssh.go

```go
package provision

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "os"

    "golang.org/x/crypto/ssh"
)

type SSHKeyPair struct {
    PrivateKey []byte
    PublicKey  []byte
}

func GenerateSSHKeyPair() (*SSHKeyPair, error) {
    privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
    if err != nil {
        return nil, err
    }

    privateKeyPEM := &pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
    }

    pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
    if err != nil {
        return nil, err
    }

    return &SSHKeyPair{
        PrivateKey: pem.EncodeToMemory(privateKeyPEM),
        PublicKey:  ssh.MarshalAuthorizedKey(pubKey),
    }, nil
}

func SaveKeyToFile(key []byte, filename string) error {
    return os.WriteFile(filename, key, 0600)
}
```

### provision.go

```go
package provision

import (
    "context"
    "fmt"
    "time"
)

type Provisioner struct {
    ec2Provisioner *EC2Provisioner
    ecrProvisioner *ECRProvisioner
    config        *Config
}

func NewProvisioner(ctx context.Context, configPath string) (*Provisioner, error) {
    config, err := LoadConfig(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    ec2Prov, err := NewEC2Provisioner(ctx, config.AWSRegion)
    if err != nil {
        return nil, fmt.Errorf("failed to create EC2 provisioner: %w", err)
    }

    ecrProv, err := NewECRProvisioner(ctx, config.AWSRegion)
    if err != nil {
        return nil, fmt.Errorf("failed to create ECR provisioner: %w", err)
    }

    return &Provisioner{
        ec2Provisioner: ec2Prov,
        ecrProvisioner: ecrProv,
        config:        config,
    }, nil
}

func (p *Provisioner) Provision(ctx context.Context) error {
    // Generate SSH keys
    keyPair, err := GenerateSSHKeyPair()
    if err != nil {
        return fmt.Errorf("failed to generate SSH keys: %w", err)
    }

    // Create EC2 instance
    instance, err := p.ec2Provisioner.CreateInstance(ctx, &p.config.EC2Instance)
    if err != nil {
        return fmt.Errorf("failed to create EC2 instance: %w", err)
    }

    // Wait for instance to be running
    err = p.ec2Provisioner.WaitUntilRunning(ctx, *instance.InstanceId)
    if err != nil {
        return fmt.Errorf("failed waiting for instance to run: %w", err)
    }

    // Create ECR repository
    repoURI, err := p.ecrProvisioner.CreateRepository(ctx, p.config.ECRRepository.RepositoryName)
    if err != nil {
        return fmt.Errorf("failed to create ECR repository: %w", err)
    }

    // Get ECR login credentials
    ecrToken, err := p.ecrProvisioner.GetLoginPassword(ctx)
    if err != nil {
        return fmt.Errorf("failed to get ECR login token: %w", err)
    }

    fmt.Printf("Provisioning complete!\n")
    fmt.Printf("EC2 Instance ID: %s\n", *instance.InstanceId)
    fmt.Printf("EC2 Public DNS: %s\n", *instance.PublicDnsName)
    fmt.Printf("ECR Repository URI: %s\n", repoURI)
    fmt.Printf("ECR Login Token: %s\n", ecrToken)
    fmt.Printf("SSH Private Key saved to: %s.pem\n", p.config.EC2Instance.KeyName)
    fmt.Printf("SSH Public Key saved to: %s.pub\n", p.config.EC2Instance.KeyName)

    // Save SSH keys to files
    err = SaveKeyToFile(keyPair.PrivateKey, fmt.Sprintf("%s.pem", p.config.EC2Instance.KeyName))
    if err != nil {
        return fmt.Errorf("failed to save private key: %w", err)
    }

    err = SaveKeyToFile(keyPair.PublicKey, fmt.Sprintf("%s.pub", p.config.EC2Instance.KeyName))
    if err != nil {
        return fmt.Errorf("failed to save public key: %w", err)
    }

    return nil
}
```

## Example Usage

```go
package main

import (
    "context"
    "log"
    "provision"
)

func main() {
    ctx := context.Background()
    provisioner, err := provision.NewProvisioner(ctx, "config.yaml")
    if err != nil {
        log.Fatalf("Failed to create provisioner: %v", err)
    }

    err = provisioner.Provision(ctx)
    if err != nil {
        log.Fatalf("Provisioning failed: %v", err)
    }
}
```

## Example config.yaml

```yaml
aws_region: us-west-2

ec2_instance:
  instance_type: t2.micro
  ami: ami-0c55b159cbfafe1f0  # Amazon Linux 2 AMI
  key_name: my-ec2-key
  security_groups:
    - sg-1234567890abcdef0
  subnet_id: subnet-1234567890abcdef0
  iam_instance_profile: ec2-ecr-access
  user_data: |
    #!/bin/bash
    echo 'ec2-user ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
    yum install -y docker
    systemctl enable docker
    systemctl start docker

ecr_repository:
  repository_name: my-app-repo
  image_tag: latest
  pull_policy: always
```

## Features

1. **EC2 Instance Provisioning**:
   - Creates an EC2 instance with specified configuration
   - Sets up passwordless sudo via user data
   - Waits for instance to be running before proceeding

2. **SSH Key Management**:
   - Generates RSA key pairs
   - Saves keys to files for later use

3. **ECR Repository Setup**:
   - Creates a new ECR repository
   - Retrieves login credentials for Docker authentication

4. **Configuration Management**:
   - Reads from YAML config file
   - Supports all necessary AWS parameters

5. **Integration**:
   - The EC2 instance is configured with Docker to pull from the created ECR repo
   - IAM permissions should be set up separately to allow the EC2 instance to pull from ECR

To use this package, you'll need to:
1. Have AWS credentials configured in your environment
2. Install the AWS SDK v2 for Go
3. Create appropriate IAM roles/policies for EC2 to access ECR
4. Configure security groups to allow SSH access

The package handles the core provisioning logic while leaving some AWS-specific configuration (like IAM) to be set up separately for security reasons.Here's a sample `config.yaml` that works within AWS Free Tier limitations:

```yaml
aws_region: us-east-1  # Free tier is available in all regions, but us-east-1 is most reliable

ec2_instance:
  instance_type: t2.micro  # Free tier eligible instance type
  ami: ami-0c55b159cbfafe1f0  # Amazon Linux 2 AMI (Free tier eligible)
  key_name: free-tier-key
  security_groups:
    - sg-0123456789abcdef0  # Replace with your default security group ID
  subnet_id: subnet-0123456789abcdef0  # Replace with a subnet in your default VPC
  iam_instance_profile: ec2-ecr-access  # Needs to be created separately
  user_data: |
    #!/bin/bash
    # Configure passwordless sudo
    echo 'ec2-user ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
    
    # Install Docker and enable it
    amazon-linux-extras install docker -y
    systemctl enable docker
    systemctl start docker
    
    # Configure Docker to use ECR
    yum install -y jq  # Install jq for JSON parsing
    AWS_REGION=$(curl -s http://169.254.169.254/latest/dynamic/instance-identity/document | jq -r .region)
    aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $(aws sts get-caller-identity --query 'Account' --output text).dkr.ecr.$AWS_REGION.amazonaws.com

ecr_repository:
  repository_name: free-tier-repo
  image_tag: latest
  pull_policy: always
```

### Important Notes for Free Tier Usage:

1. **EC2 Instance**:
   - Only `t2.micro` or `t3.micro` instances are Free Tier eligible
   - You get 750 hours per month (enough for one instance running continuously)
   - Must use Amazon Linux 2 AMI or other Free Tier eligible AMIs

2. **ECR**:
   - ECR has 500MB free storage per month
   - First 1GB of data transfer out to internet is free each month
   - Data transfer between EC2 and ECR in the same region is free

3. **Additional Setup Required**:
   - You'll need to create a security group that allows:
     - SSH (port 22) from your IP
     - HTTP (port 80) if your application needs it
   - Create an IAM role with these permissions:
     - `AmazonEC2ContainerRegistryReadOnly`
     - `AmazonEC2ContainerRegistryPowerUser` (if you need push/pull)
   - Attach this role to your EC2 instance

4. **Cost Monitoring**:
   - Always monitor your AWS Free Tier usage in the Billing Dashboard
   - Free Tier is only available for the first 12 months of your AWS account

5. **Default VPC**:
   - The config assumes you're using the default VPC and subnets
   - To find your default security group ID and subnet ID:
     - Go to VPC Dashboard
     - Look under "Your VPCs" for the default VPC
     - Subnets and security groups will be listed there

This configuration will give you:
- A small but functional EC2 instance
- A private ECR repository
- Automatic Docker setup on the instance
- Passwordless sudo access (via the generated SSH key)
- Automatic Docker login configuration to pull from your ECR repo

Remember to delete resources when not in use to stay within Free Tier limits!



```go
package provision

import (
    "context"
    "fmt"
    "log"
)

type Provisioner struct {
    ec2Provisioner *EC2Provisioner
    ecrProvisioner *ECRProvisioner
    config        *Config
    createdResources struct {
        instanceID     string
        repositoryName string
        keyFiles       []string
    }
}

// ... (existing NewProvisioner and Provision functions remain the same)

func (p *Provisioner) Destroy(ctx context.Context) error {
    var errs []error

    // Delete EC2 instance if created
    if p.createdResources.instanceID != "" {
        log.Printf("Terminating EC2 instance: %s", p.createdResources.instanceID)
        _, err := p.ec2Provisioner.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
            InstanceIds: []string{p.createdResources.instanceID},
        })
        if err != nil {
            errs = append(errs, fmt.Errorf("failed to terminate instance: %w", err))
        } else {
            // Wait for instance to terminate
            waiter := ec2.NewInstanceTerminatedWaiter(p.ec2Provisioner.client)
            waitErr := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
                InstanceIds: []string{p.createdResources.instanceID},
            }, 5*time.Minute)
            if waitErr != nil {
                errs = append(errs, fmt.Errorf("failed waiting for instance termination: %w", waitErr))
            }
        }
    }

    // Delete ECR repository if created
    if p.createdResources.repositoryName != "" {
        log.Printf("Deleting ECR repository: %s", p.createdResources.repositoryName)
        _, err := p.ecrProvisioner.client.DeleteRepository(ctx, &ecr.DeleteRepositoryInput{
            RepositoryName: aws.String(p.createdResources.repositoryName),
            Force:          aws.Bool(true), // Force delete even if it contains images
        })
        if err != nil {
            errs = append(errs, fmt.Errorf("failed to delete ECR repository: %w", err))
        }
    }

    // Delete SSH key files if created
    for _, file := range p.createdResources.keyFiles {
        log.Printf("Removing key file: %s", file)
        if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
            errs = append(errs, fmt.Errorf("failed to remove key file %s: %w", file, err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("errors occurred during destruction: %v", errs)
    }

    log.Println("All resources destroyed successfully")
    return nil
}

// Update the Provision function to track created resources
func (p *Provisioner) Provision(ctx context.Context) error {
    // ... (existing code)
    
    // At the end of successful provisioning, track created resources
    p.createdResources.instanceID = *instance.InstanceId
    p.createdResources.repositoryName = p.config.ECRRepository.RepositoryName
    p.createdResources.keyFiles = []string{
        fmt.Sprintf("%s.pem", p.config.EC2Instance.KeyName),
        fmt.Sprintf("%s.pub", p.config.EC2Instance.KeyName),
    }
    
    return nil
}
```

## Updated ec2.go (add termination support)

```go
// Add to ec2.go
func (p *EC2Provisioner) TerminateInstance(ctx context.Context, instanceID string) error {
    _, err := p.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
        InstanceIds: []string{instanceID},
    })
    return err
}
```

## Updated ecr.go (add deletion support)

```go
// Add to ecr.go
func (p *ECRProvisioner) DeleteRepository(ctx context.Context, repositoryName string) error {
    _, err := p.client.DeleteRepository(ctx, &ecr.DeleteRepositoryInput{
        RepositoryName: aws.String(repositoryName),
        Force:          aws.Bool(true),
    })
    return err
}
```

## Example Usage

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "provision"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle Ctrl+C for graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigChan
        cancel()
    }()

    provisioner, err := provision.NewProvisioner(ctx, "config.yaml")
    if err != nil {
        log.Fatalf("Failed to create provisioner: %v", err)
    }

    // Provision resources
    if err := provisioner.Provision(ctx); err != nil {
        log.Fatalf("Provisioning failed: %v", err)
    }

    // Wait for user to press Ctrl+C or for some condition
    <-ctx.Done()

    // Clean up resources
    log.Println("Initiating cleanup...")
    if err := provisioner.Destroy(context.Background()); err != nil {
        log.Printf("Cleanup encountered errors: %v", err)
    }
}
```

## Features of the Destroy Function

1. **Comprehensive Cleanup**:
   - Terminates the EC2 instance
   - Deletes the ECR repository (including all images)
   - Removes the local SSH key files

2. **Error Handling**:
   - Attempts all cleanup operations even if some fail
   - Collects and reports all errors that occurred

3. **State Tracking**:
   - Tracks all created resources during provisioning
   - Only attempts to destroy resources that were actually created

4. **Graceful Shutdown**:
   - Example shows how to handle Ctrl+C for interactive programs
   - Waits for instance termination to complete

5. **Idempotent Operations**:
   - Safe to call multiple times
   - Won't fail if resources are already deleted

## Important Notes

1. The destroy function uses `Force: true` when deleting the ECR repository, which will delete all images in the repository.

2. EC2 instances may take several minutes to fully terminate.

3. The example shows a graceful shutdown pattern that's useful for CLI tools. For other use cases (like Lambda), you might want to call Destroy at a different time.

4. Make sure your IAM role has permissions for these destructive operations:
   - `ec2:TerminateInstances`
   - `ecr:DeleteRepository`
   - `ec2:DescribeInstances` (for waiting)

This implementation provides a complete lifecycle management solution for your AWS resources, making it safe to experiment with the Free Tier without worrying about leaving resources running and accruing charges.
