### **Step-by-Step Guide: Create an IAM User (`nextdeploy`) with ECR-Only Access**  
*(Save these instructions for future use.)*  

---

### **1. Create the IAM User (`nextdeploy`)**
```bash
aws iam create-user --user-name nextdeploy
```

---

### **2. Create a Custom Policy for ECR Access**  
*(Limits the user to only ECR actions.)*  

#### **A. Define the Policy (Save as `nextdeploy-ecr-policy.json`)**
```bash
cat <<EOF > nextdeploy-ecr-policy.json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecr:GetAuthorizationToken",
        "ecr:BatchCheckLayerAvailability",
        "ecr:GetDownloadUrlForLayer",
        "ecr:GetRepositoryPolicy",
        "ecr:DescribeRepositories",
        "ecr:ListImages",
        "ecr:DescribeImages",
        "ecr:BatchGetImage",
        "ecr:InitiateLayerUpload",
        "ecr:UploadLayerPart",
        "ecr:CompleteLayerUpload",
        "ecr:PutImage"
      ],
      "Resource": "*"
    }
  ]
}
EOF
```

#### **B. Create the Policy**
```bash
aws iam create-policy \
  --policy-name NextDeployECRAccess \
  --policy-document file://nextdeploy-ecr-policy.json
```
**Note the `PolicyArn` from the output (e.g., `arn:aws:iam::123456789012:policy/NextDeployECRAccess`).**  

---

### **3. Attach the Policy to the `nextdeploy` User**
*(Replace `123456789012` with your AWS account ID.)*  
```bash
aws iam attach-user-policy \
  --user-name nextdeploy \
  --policy-arn "arn:aws:iam::123456789012:policy/NextDeployECRAccess"
```

**Verify the attachment:**  
```bash
aws iam list-attached-user-policies --user-name nextdeploy
```
*(Should list `NextDeployECRAccess`.)*  

---

### **4. Generate an Access Key for `nextdeploy`**
```bash
aws iam create-access-key --user-name nextdeploy
```
**Output Example:**  
```json
{
  "AccessKey": {
    "UserName": "nextdeploy",
    "AccessKeyId": "AKIAXXXXXXXXXXXXXXXX",
    "Status": "Active",
    "SecretAccessKey": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
    "CreateDate": "2025-06-29T00:00:00Z"
  }
}
```
**⚠️ Save `AccessKeyId` and `SecretAccessKey` securely (they are shown only once).**  

---

### **5. Test the Access Key**
#### **A. Configure AWS CLI with the Key**
```bash
export AWS_ACCESS_KEY_ID="AKIAXXXXXXXXXXXXXXXX"
export AWS_SECRET_ACCESS_KEY="XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
```

#### **B. Verify ECR Access (Should Work)**
```bash
aws ecr describe-repositories
```

#### **C. Verify No Other AWS Access (Should Fail)**
```bash
aws s3 ls  # Expected: "AccessDenied"
```

---

### **6. Use the Key with Docker**
```bash
aws ecr get-login-password | docker login \
  --username AWS \
  --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

---

### **Security Best Practices**
1. **Rotate Keys Periodically**  
   ```bash
   # Create a new key
   aws iam create-access-key --user-name nextdeploy
   # Delete the old key
   aws iam delete-access-key --user-name nextdeploy --access-key-id "OLD_KEY_ID"
   ```

2. **Restrict Access to Specific ECR Repos**  
   Edit `nextdeploy-ecr-policy.json` and replace `"Resource": "*"` with:  
   ```json
   "Resource": "arn:aws:ecr:us-east-1:123456789012:repository/my-repo"
   ```

3. **Monitor Usage**  
   Check API activity in **AWS CloudTrail**.

---

### **Troubleshooting**
- **Error: "Policy does not exist"**  
  Double-check the `--policy-arn` matches the `Arn` from `create-policy` output.  
- **Error: "AccessDenied" for ECR**  
  Ensure the policy is attached to the user and the actions are correct.  

---

### **Summary**
- Created a **least-privilege IAM user** (`nextdeploy`).  
- Granted **ECR-only access** via a custom policy.  
- Generated **secure access keys** for programmatic use.  

Save these steps for future reference! Let me know if you need adjustments. 🚀

Here's a **Go function** that automates the entire process of creating an IAM user (`nextdeploy`) with ECR-only access, assuming the machine already has AWS credentials configured (via `~/.aws/credentials`, environment variables, or IAM instance profile):

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ecr"
)

const (
	userName         = "nextdeploy"
	policyName       = "NextDeployECRAccess"
	policyDocument   = `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"ecr:GetAuthorizationToken",
					"ecr:BatchCheckLayerAvailability",
					"ecr:GetDownloadUrlForLayer",
					"ecr:GetRepositoryPolicy",
					"ecr:DescribeRepositories",
					"ecr:ListImages",
					"ecr:DescribeImages",
					"ecr:BatchGetImage",
					"ecr:InitiateLayerUpload",
					"ecr:UploadLayerPart",
					"ecr:CompleteLayerUpload",
					"ecr:PutImage"
				],
				"Resource": "*"
			}
		]
	}`
)

func main() {
	// Initialize AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Change to your region
	})
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	// Create IAM and ECR clients
	iamSvc := iam.New(sess)
	ecrSvc := ecr.New(sess)

	// Step 1: Create IAM user
	if err := createUser(iamSvc); err != nil {
		log.Fatal(err)
	}

	// Step 2: Create policy
	policyArn, err := createPolicy(iamSvc)
	if err != nil {
		log.Fatal(err)
	}

	// Step 3: Attach policy to user
	if err := attachPolicy(iamSvc, policyArn); err != nil {
		log.Fatal(err)
	}

	// Step 4: Create access key
	accessKey, secretKey, err := createAccessKey(iamSvc)
	if err != nil {
		log.Fatal(err)
	}

	// Step 5: Verify ECR access
	if err := verifyECRAccess(ecrSvc, accessKey, secretKey); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nSuccess! User '%s' created with ECR-only access.\n", userName)
	fmt.Printf("Access Key: %s\n", accessKey)
	fmt.Printf("Secret Key: %s\n", secretKey)
	fmt.Println("\n⚠️ Save these credentials securely - they won't be shown again!")
}

func createUser(svc *iam.IAM) error {
	_, err := svc.CreateUser(&iam.CreateUserInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return fmt.Errorf("failed to create IAM user: %v", err)
	}
	fmt.Printf("Created IAM user: %s\n", userName)
	return nil
}

func createPolicy(svc *iam.IAM) (string, error) {
	result, err := svc.CreatePolicy(&iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create policy: %v", err)
	}
	fmt.Printf("Created policy: %s\n", *result.Policy.Arn)
	return *result.Policy.Arn, nil
}

func attachPolicy(svc *iam.IAM, policyArn string) error {
	_, err := svc.AttachUserPolicy(&iam.AttachUserPolicyInput{
		UserName:  aws.String(userName),
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		return fmt.Errorf("failed to attach policy: %v", err)
	}
	fmt.Printf("Attached policy to user %s\n", userName)
	return nil
}

func createAccessKey(svc *iam.IAM) (string, string, error) {
	result, err := svc.CreateAccessKey(&iam.CreateAccessKeyInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create access key: %v", err)
	}
	return *result.AccessKey.AccessKeyId, *result.AccessKey.SecretAccessKey, nil
}

func verifyECRAccess(svc *ecr.ECR, accessKey, secretKey string) error {
	// Create a new session with the temporary credentials
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
	})
	if err != nil {
		return fmt.Errorf("failed to create verification session: %v", err)
	}

	ecrSvc := ecr.New(sess)
	_, err = ecrSvc.DescribeRepositories(&ecr.DescribeRepositoriesInput{})
	if err != nil {
		return fmt.Errorf("ECR access verification failed: %v", err)
	}
	fmt.Println("Verified ECR access successfully")
	return nil
}
```

---

### **Key Features**
1. **Automated Workflow**:
   - Creates IAM user (`nextdeploy`)
   - Creates ECR-specific policy
   - Attaches policy to user
   - Generates access keys
   - Verifies ECR access

2. **Safety Checks**:
   - Uses AWS SDK's built-in error handling
   - Verifies ECR permissions before returning credentials

3. **Prerequisites**:
   ```bash
   go get github.com/aws/aws-sdk-go
   ```

4. **AWS Permissions Required**:
   - `iam:CreateUser`
   - `iam:CreatePolicy`
   - `iam:AttachUserPolicy`
   - `iam:CreateAccessKey`
   - `ecr:DescribeRepositories` (for verification)

---

### **Usage**
1. Save as `ecr-user.go`
2. Run:
   ```bash
   go run ecr-user.go
   ```
3. **Output**:
   ```
   Created IAM user: nextdeploy
   Created policy: arn:aws:iam::123456789012:policy/NextDeployECRAccess
   Attached policy to user nextdeploy
   Verified ECR access successfully

   Success! User 'nextdeploy' created with ECR-only access.
   Access Key: AKIAXXXXXXXXXXXXXXXX
   Secret Key: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

   ⚠️ Save these credentials securely - they won't be shown again!
   ```

---

### **Customization Options**
1. **Restrict to Specific ECR Repos**:
   Modify `policyDocument` to replace `"Resource": "*"` with specific ARNs:
   ```go
   "Resource": "arn:aws:ecr:us-east-1:123456789012:repository/my-repo"
   ```

2. **Add Tags**:
   ```go
   _, err = svc.TagUser(&iam.TagUserInput{
       UserName: aws.String(userName),
       Tags: []*iam.Tag{
           {Key: aws.String("Purpose"), Value: aws.String("ECR Access")},
       },
   })
   ```

3. **Error Recovery**:
   Add cleanup logic (delete user/policy) if intermediate steps fail.

Let me know if you'd like to extend this further!
