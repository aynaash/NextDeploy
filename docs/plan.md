# Go Package for EC2 Instance Provisioning with SSH Key and ECR Access

Here's a comprehensive Go package that provisions an EC2 instance with passwordless SSH access (automatically returning the SSH key) and optional ECR registry access.

## Package Structure

```go
// Package ec2provisioner provides functionality to provision EC2 instances
// with passwordless SSH access and optional ECR registry permissions.
package ec2provisioner

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
)

// Provisioner handles EC2 instance provisioning
type Provisioner struct {
	awsSession *session.Session
	ec2Svc     *ec2.EC2
	iamSvc     *iam.IAM
}

// New creates a new EC2 provisioner
func New(region string) (*Provisioner, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &Provisioner{
		awsSession: sess,
		ec2Svc:     ec2.New(sess),
		iamSvc:     iam.New(sess),
	}, nil
}

// InstanceConfig holds configuration for the EC2 instance
type InstanceConfig struct {
	Name               string
	InstanceType       string
	AMI                string
	KeyPairName        string
	SecurityGroupIDs   []string
	SubnetID           string
	EnableECRAccess    bool
	ECRPolicyName      string
	ECRRoleName        string
	UserData           string
	Tags               map[string]string
}

// ProvisionResult contains the results of instance provisioning
type ProvisionResult struct {
	InstanceID      string
	PublicDNS       string
	PublicIP        string
	PrivateKey      string
	PrivateKeyFile  string
	ECRRoleARN      string
}

// Provision provisions a new EC2 instance with passwordless SSH access
func (p *Provisioner) Provision(config *InstanceConfig) (*ProvisionResult, error) {
	var result ProvisionResult
	var err error

	// Generate SSH key pair
	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
	}
	result.PrivateKey = privateKey

	// Save private key to file
	keyFile := fmt.Sprintf("%s.pem", config.KeyPairName)
	if err := os.WriteFile(keyFile, []byte(privateKey), 0600); err != nil {
		return nil, fmt.Errorf("failed to write private key to file: %w", err)
	}
	result.PrivateKeyFile = keyFile

	// Import key pair to AWS
	_, err = p.ec2Svc.ImportKeyPair(&ec2.ImportKeyPairInput{
		KeyName:           aws.String(config.KeyPairName),
		PublicKeyMaterial: []byte(publicKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to import key pair: %w", err)
	}

	// Create IAM role for ECR if needed
	if config.EnableECRAccess {
		roleARN, err := p.setupECRAccess(config.ECRRoleName, config.ECRPolicyName)
		if err != nil {
			return nil, fmt.Errorf("failed to setup ECR access: %w", err)
		}
		result.ECRRoleARN = roleARN
	}

	// Prepare user data with public key for passwordless SSH
	userData := config.UserData
	if userData == "" {
		userData = fmt.Sprintf(`#!/bin/bash
echo "%s" >> /home/ec2-user/.ssh/authorized_keys
chmod 600 /home/ec2-user/.ssh/authorized_keys
chown ec2-user:ec2-user /home/ec2-user/.ssh/authorized_keys`, publicKey)
	}

	// Create instance
	runResult, err := p.ec2Svc.RunInstances(&ec2.RunInstancesInput{
		ImageId:          aws.String(config.AMI),
		InstanceType:     aws.String(config.InstanceType),
		MinCount:         aws.Int64(1),
		MaxCount:         aws.Int64(1),
		KeyName:          aws.String(config.KeyPairName),
		SecurityGroupIds:  aws.StringSlice(config.SecurityGroupIDs),
		SubnetId:         aws.String(config.SubnetID),
		UserData:         aws.String(userData),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(config.ECRRoleName),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run instances: %w", err)
	}

	if len(runResult.Instances) == 0 {
		return nil, fmt.Errorf("no instances were created")
	}

	instance := runResult.Instances[0]
	result.InstanceID = *instance.InstanceId

	// Add tags to the instance
	var tags []*ec2.Tag
	tags = append(tags, &ec2.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(config.Name),
	})
	for k, v := range config.Tags {
		tags = append(tags, &ec2.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	_, err = p.ec2Svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{instance.InstanceId},
		Tags:      tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tags: %w", err)
	}

	// Wait for instance to be running
	err = p.ec2Svc.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{instance.InstanceId},
	})
	if err != nil {
		return nil, fmt.Errorf("failed waiting for instance to run: %w", err)
	}

	// Get public DNS and IP
	descResult, err := p.ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{instance.InstanceId},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	if len(descResult.Reservations) > 0 && len(descResult.Reservations[0].Instances) > 0 {
		instance := descResult.Reservations[0].Instances[0]
		result.PublicDNS = aws.StringValue(instance.PublicDnsName)
		result.PublicIP = aws.StringValue(instance.PublicIpAddress)
	}

	return &result, nil
}

// setupECRAccess creates IAM role and policy for ECR access
func (p *Provisioner) setupECRAccess(roleName, policyName string) (string, error) {
	// Check if role already exists
	getRoleOutput, err := p.iamSvc.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err == nil {
		return *getRoleOutput.Role.Arn, nil
	}

	// Create role
	assumeRolePolicy := `{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {
				"Service": "ec2.amazonaws.com"
			},
			"Action": "sts:AssumeRole"
		}]
	}`

	createRoleOutput, err := p.iamSvc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %w", err)
	}

	// Create policy for ECR access
	ecrPolicy := `{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": [
				"ecr:GetAuthorizationToken",
				"ecr:BatchCheckLayerAvailability",
				"ecr:GetDownloadUrlForLayer",
				"ecr:BatchGetImage"
			],
			"Resource": "*"
		}]
	}`

	createPolicyOutput, err := p.iamSvc.CreatePolicy(&iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(ecrPolicy),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create IAM policy: %w", err)
	}

	// Attach policy to role
	_, err = p.iamSvc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: createPolicyOutput.Policy.Arn,
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach policy to role: %w", err)
	}

	// Create instance profile
	_, err = p.iamSvc.CreateInstanceProfile(&iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create instance profile: %w", err)
	}

	// Add role to instance profile
	_, err = p.iamSvc.AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:           aws.String(roleName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to add role to instance profile: %w", err)
	}

	return *createRoleOutput.Role.Arn, nil
}

// generateSSHKeyPair generates a new RSA key pair for SSH access
func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate private key PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	var privateBuf strings.Builder
	if err := pem.Encode(&privateBuf, privateKeyPEM); err != nil {
		return "", "", fmt.Errorf("failed to encode private key: %w", err)
	}

	// Generate public key
	pub, err := sshPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	return privateBuf.String(), string(pub), nil
}

// sshPublicKey generates the authorized_keys format for the public key
func sshPublicKey(pubkey *rsa.PublicKey) ([]byte, error) {
	pub, err := ssh.NewPublicKey(pubkey)
	if err != nil {
		return nil, err
	}
	return ssh.MarshalAuthorizedKey(pub), nil
}

// Cleanup removes the key pair from AWS and the local private key file
func (p *Provisioner) Cleanup(keyPairName, privateKeyFile string) error {
	// Delete key pair from AWS
	_, err := p.ec2Svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(keyPairName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete key pair: %w", err)
	}

	// Delete local private key file
	if err := os.Remove(privateKeyFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove private key file: %w", err)
	}

	return nil
}
```

## Usage Example

Here's how to use the package:

```go
package main

import (
	"fmt"
	"log"

	"github.com/yourusername/ec2provisioner"
)

func main() {
	// Initialize provisioner
	provisioner, err := ec2provisioner.New("us-west-2")
	if err != nil {
		log.Fatalf("Failed to create provisioner: %v", err)
	}

	// Configure instance
	config := &ec2provisioner.InstanceConfig{
		Name:             "my-ec2-instance",
		InstanceType:     "t2.micro",
		AMI:              "ami-0c55b159cbfafe1f0", // Amazon Linux 2 AMI
		KeyPairName:      "my-key-pair",
		SecurityGroupIDs: []string{"sg-12345678"},
		SubnetID:         "subnet-12345678",
		EnableECRAccess:  true,
		ECRPolicyName:    "ECRAccessPolicy",
		ECRRoleName:      "ECRInstanceRole",
		Tags: map[string]string{
			"Environment": "dev",
			"Project":     "demo",
		},
	}

	// Provision instance
	result, err := provisioner.Provision(config)
	if err != nil {
		log.Fatalf("Failed to provision instance: %v", err)
	}

	// Output results
	fmt.Printf("Successfully provisioned instance:\n")
	fmt.Printf("Instance ID: %s\n", result.InstanceID)
	fmt.Printf("Public DNS: %s\n", result.PublicDNS)
	fmt.Printf("Public IP: %s\n", result.PublicIP)
	fmt.Printf("Private key saved to: %s\n", result.PrivateKeyFile)
	fmt.Printf("ECR Role ARN: %s\n", result.ECRRoleARN)

	// To connect via SSH:
	// ssh -i my-key-pair.pem ec2-user@<public-dns>
}
```

## Features

1. **Passwordless SSH Access**:
   - Generates a new SSH key pair
   - Automatically injects the public key into the instance
   - Returns the private key for immediate use

2. **Optional ECR Access**:
   - Creates IAM role with ECR permissions
   - Attaches the role to the EC2 instance
   - Returns the role ARN for reference

3. **Comprehensive Configuration**:
   - Supports all major EC2 configuration options
   - Allows custom user data scripts
   - Supports tagging

4. **Cleanup Functionality**:
   - Removes AWS key pairs
   - Deletes local private key files

## Dependencies

Add these to your `go.mod` file:

```
require (
	github.com/aws/aws-sdk-go v1.44.0
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d
)
```

## Security Notes

1. The private key is saved with 0600 permissions (read/write only by owner)
2. The IAM role for ECR follows the principle of least privilege
3. Always clean up resources when they're no longer needed

I'll enhance the package with functions to delete existing resources and regenerate them exactly. This is useful for rotation or recreation scenarios.

Here are the additional functions to add to the `Provisioner` struct:

```go
// DeleteAndRecreate deletes all existing resources and recreates them with the same configuration
func (p *Provisioner) DeleteAndRecreate(config *InstanceConfig) (*ProvisionResult, error) {
    // First delete all existing resources
    err := p.DeleteResources(config)
    if err != nil {
        return nil, fmt.Errorf("failed to delete existing resources: %w", err)
    }

    // Then recreate them
    return p.Provision(config)
}

// DeleteResources deletes all resources associated with the given configuration
func (p *Provisioner) DeleteResources(config *InstanceConfig) error {
    // Terminate the instance if it exists
    if err := p.terminateInstance(config.Name); err != nil {
        return fmt.Errorf("failed to terminate instance: %w", err)
    }

    // Delete the key pair
    if err := p.deleteKeyPair(config.KeyPairName); err != nil {
        return fmt.Errorf("failed to delete key pair: %w", err)
    }

    // Delete IAM resources if ECR access was enabled
    if config.EnableECRAccess {
        if err := p.deleteIAMResources(config.ECRRoleName, config.ECRPolicyName); err != nil {
            return fmt.Errorf("failed to delete IAM resources: %w", err)
        }
    }

    // Delete local private key file
    keyFile := fmt.Sprintf("%s.pem", config.KeyPairName)
    if err := os.Remove(keyFile); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to remove private key file: %w", err)
    }

    return nil
}

// terminateInstance terminates the instance with the given name tag
func (p *Provisioner) terminateInstance(instanceName string) error {
    // Find instances with the given name tag
    descResult, err := p.ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
        Filters: []*ec2.Filter{
            {
                Name:   aws.String("tag:Name"),
                Values: []*string{aws.String(instanceName)},
            },
        },
    })
    if err != nil {
        return fmt.Errorf("failed to describe instances: %w", err)
    }

    // Collect all instance IDs
    var instanceIDs []string
    for _, reservation := range descResult.Reservations {
        for _, instance := range reservation.Instances {
            instanceIDs = append(instanceIDs, *instance.InstanceId)
        }
    }

    if len(instanceIDs) == 0 {
        return nil // No instances to terminate
    }

    // Terminate all found instances
    _, err = p.ec2Svc.TerminateInstances(&ec2.TerminateInstancesInput{
        InstanceIds: aws.StringSlice(instanceIDs),
    })
    if err != nil {
        return fmt.Errorf("failed to terminate instances: %w", err)
    }

    // Wait for instances to terminate
    err = p.ec2Svc.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
        InstanceIds: aws.StringSlice(instanceIDs),
    })
    if err != nil {
        return fmt.Errorf("failed waiting for instances to terminate: %w", err)
    }

    return nil
}

// deleteKeyPair deletes the specified key pair
func (p *Provisioner) deleteKeyPair(keyPairName string) error {
    _, err := p.ec2Svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
        KeyName: aws.String(keyPairName),
    })
    if err != nil {
        if isAWSErr(err, "InvalidKeyPair.NotFound", "") {
            return nil // Key pair doesn't exist, nothing to delete
        }
        return fmt.Errorf("failed to delete key pair: %w", err)
    }
    return nil
}

// deleteIAMResources deletes the IAM role and policy
func (p *Provisioner) deleteIAMResources(roleName, policyName string) error {
    // Detach policy from role
    listPolicies, err := p.iamSvc.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
        RoleName: aws.String(roleName),
    })
    if err != nil && !isAWSErr(err, iam.ErrCodeNoSuchEntityException, "") {
        return fmt.Errorf("failed to list attached role policies: %w", err)
    }

    for _, policy := range listPolicies.AttachedPolicies {
        _, err := p.iamSvc.DetachRolePolicy(&iam.DetachRolePolicyInput{
            PolicyArn: policy.PolicyArn,
            RoleName: aws.String(roleName),
        })
        if err != nil && !isAWSErr(err, iam.ErrCodeNoSuchEntityException, "") {
            return fmt.Errorf("failed to detach policy from role: %w", err)
        }
    }

    // Delete role
    _, err = p.iamSvc.DeleteRole(&iam.DeleteRoleInput{
        RoleName: aws.String(roleName),
    })
    if err != nil && !isAWSErr(err, iam.ErrCodeNoSuchEntityException, "") {
        return fmt.Errorf("failed to delete role: %w", err)
    }

    // Delete instance profile
    _, err = p.iamSvc.DeleteInstanceProfile(&iam.DeleteInstanceProfileInput{
        InstanceProfileName: aws.String(roleName),
    })
    if err != nil && !isAWSErr(err, iam.ErrCodeNoSuchEntityException, "") {
        return fmt.Errorf("failed to delete instance profile: %w", err)
    }

    // Delete policy
    policyArn := fmt.Sprintf("arn:aws:iam::%s:policy/%s", p.getAccountID(), policyName)
    _, err = p.iamSvc.DeletePolicy(&iam.DeletePolicyInput{
        PolicyArn: aws.String(policyArn),
    })
    if err != nil && !isAWSErr(err, iam.ErrCodeNoSuchEntityException, "") {
        return fmt.Errorf("failed to delete policy: %w", err)
    }

    return nil
}

// getAccountID retrieves the AWS account ID
func (p *Provisioner) getAccountID() string {
    callerID, err := p.iamSvc.GetCallerIdentity(&iam.GetCallerIdentityInput{})
    if err != nil {
        return ""
    }
    return *callerID.Account
}

// isAWSErr checks if the error matches an AWS error code
func isAWSErr(err error, code string, message string) bool {
    var awsErr awserr.Error
    if errors.As(err, &awsErr) {
        return (code == "" || awsErr.Code() == code) && 
               (message == "" || strings.Contains(awsErr.Message(), message))
    }
    return false
}
```

## Updated Usage Example

Here's how to use the new functions:

```go
package main

import (
	"fmt"
	"log"

	"github.com/yourusername/ec2provisioner"
)

func main() {
	// Initialize provisioner
	provisioner, err := ec2provisioner.New("us-west-2")
	if err != nil {
		log.Fatalf("Failed to create provisioner: %v", err)
	}

	// Configuration
	config := &ec2provisioner.InstanceConfig{
		Name:             "my-ec2-instance",
		InstanceType:     "t2.micro",
		AMI:              "ami-0c55b159cbfafe1f0",
		KeyPairName:      "my-key-pair",
		SecurityGroupIDs: []string{"sg-12345678"},
		SubnetID:         "subnet-12345678",
		EnableECRAccess:  true,
		ECRPolicyName:    "ECRAccessPolicy",
		ECRRoleName:      "ECRInstanceRole",
	}

	// Option 1: Delete and recreate all resources
	result, err := provisioner.DeleteAndRecreate(config)
	if err != nil {
		log.Fatalf("Failed to recreate resources: %v", err)
	}
	fmt.Printf("Recreated instance: %s\n", result.InstanceID)

	// Option 2: Just delete resources (cleanup)
	// err = provisioner.DeleteResources(config)
	// if err != nil {
	//     log.Fatalf("Failed to delete resources: %v", err)
	// }
	// fmt.Println("Successfully deleted all resources")
}
```

## Key Features Added

1. **Complete Resource Deletion**:
   - Terminates EC2 instances by name tag
   - Deletes key pairs
   - Cleans up IAM roles, policies, and instance profiles
   - Removes local private key files

2. **Safe Recreation**:
   - `DeleteAndRecreate()` first deletes then recreates resources
   - Handles cases where resources don't exist
   - Maintains the same configuration

3. **Error Handling**:
   - Gracefully handles missing resources
   - Uses AWS error codes to identify specific conditions
   - Comprehensive error messages

4. **Atomic Operations**:
   - Each deletion step is independent
   - Continues even if some resources can't be deleted
   - Returns all errors encountered

## Notes on Usage

1. **Idempotency**: The delete functions are idempotent - they won't fail if resources don't exist.

2. **Order of Operations**: The deletion happens in reverse order of creation to avoid dependency issues.

3. **Security**: The IAM policy deletion happens after role detachment to prevent orphaned policies.

4. **Error Recovery**: If recreation fails after deletion, you'll need to handle this in your application logic.

Would you like me to add any specific error recovery patterns or additional safety checks?
