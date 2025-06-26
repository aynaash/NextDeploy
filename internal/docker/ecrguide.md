# **Guide: Using Amazon ECR Credential Helper for Docker Operations in Go**

This guide demonstrates how to:
1. **Push Docker images to ECR** using a Go CLI tool with the ECR Credential Helper
2. **Pull images on EC2** using the same credential-free approach

## **Prerequisites**
- AWS account with ECR access
- Go installed (≥1.16)
- Docker installed
- EC2 instance with:
  - Docker installed
  - IAM role having `AmazonEC2ContainerRegistryReadOnly` policy

---

## **Part 1: Setup ECR Credential Helper**

### **1. Install the Helper**
```bash
# Linux/macOS
ARCH=$(uname -m)  # "x86_64" or "arm64"
VERSION="0.7.1"
sudo curl -Lo /usr/local/bin/docker-credential-ecr-login \
  https://amazon-ecr-credential-helper-releases.s3.amazonaws.com/${VERSION}/linux-${ARCH}/docker-credential-ecr-login
sudo chmod +x /usr/local/bin/docker-credential-ecr-login

# Windows (PowerShell)
choco install docker-credential-helper-ecr
```

### **2. Configure Docker**
Create/update `~/.docker/config.json`:
```json
{
  "credsStore": "ecr-login"
}
```

---

## **Part 2: Go CLI Tool to Push to ECR**

### **1. Create `push-to-ecr.go`**
```go
package main

import (
	"fmt"
	"log"
	"os/exec"
)

func main() {
	// Build Docker image
	buildCmd := exec.Command("docker", "build", "-t", "my-app", ".")
	if err := buildCmd.Run(); err != nil {
		log.Fatal("Docker build failed:", err)
	}

	// Tag image
	ecrRepo := "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"
	tagCmd := exec.Command("docker", "tag", "my-app:latest", ecrRepo+":latest")
	if err := tagCmd.Run(); err != nil {
		log.Fatal("Docker tag failed:", err)
	}

	// Push to ECR (credential helper handles auth)
	pushCmd := exec.Command("docker", "push", ecrRepo+":latest")
	if err := pushCmd.Run(); err != nil {
		log.Fatal("Docker push failed:", err)
	}

	fmt.Println("✅ Successfully pushed to ECR using credential helper!")
}
```

### **2. Run the Tool**
```bash
go run push-to-ecr.go
```

---

## **Part 3: Pull Image on EC2 Instance**

### **1. Configure EC2 Instance**
- Attach IAM role with `AmazonEC2ContainerRegistryReadOnly` policy
- Install Docker and credential helper (same as Part 1)

### **2. Pull the Image**
```bash
docker pull 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:latest
```
*No manual authentication needed!*

---

## **Key Benefits**
- **Zero credential management** in your code
- **Automatic IAM role integration** on EC2
- **Secure, temporary tokens** (12-hour validity)
- **Works across all AWS services** (ECS, EKS, Lambda)

## **Troubleshooting**
```bash
# Verify helper is working
docker-credential-ecr-login -v

# Check IAM permissions
aws ecr describe-repositories
```

## **Next Steps**
- Implement in CI/CD pipelines
- Set up cross-account ECR access
- Explore image scanning features

This approach eliminates all manual credential handling while maintaining security through AWS IAM roles.
