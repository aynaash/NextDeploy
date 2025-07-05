# **Guide: Creating an EC2 Instance with a Passwordless sudo User (nextdeploy)**  

This guide walks you through launching an **Amazon EC2** instance and setting up a user (`nextdeploy`) with **passwordless sudo** access for secure and convenient administration.  

---

## **Prerequisites**  
✅ An **AWS account** with EC2 access.  
✅ Basic familiarity with **AWS Management Console**.  
✅ **SSH key pair** (or willingness to create one).  

---

## **Step 1: Launch an EC2 Instance**  

### **1.1 Log in to AWS Console**  
- Go to [AWS Console](https://aws.amazon.com/console/) and sign in.  
- Navigate to **EC2 Dashboard** → **Launch Instance**.  

### **1.2 Configure Instance**  
- **Name**: `nextdeploy-server` (or your preferred name).  
- **AMI**: Choose an OS (Ubuntu 22.04 LTS recommended).  
- **Instance Type**: `t2.micro` (Free Tier eligible).  
- **Key Pair**:  
  - Select an existing key pair **OR**  
  - **Create new key pair** → Download `.pem` file (keep it secure!).  

### **1.3 Network & Security Settings**  
- **Security Group**: Allow SSH (port 22) from your IP (for security).  
- **Storage**: Default (8GB SSD is fine for testing).  

### **1.4 Launch Instance**  
- Click **"Launch Instance"** and wait for initialization.  

---

## **Step 2: SSH into the EC2 Instance**  

### **2.1 Locate Public IP**  
- Go to **EC2 Dashboard** → **Instances** → Copy the **Public IPv4 address**.  

### **2.2 Connect via SSH**  
```bash
ssh -i "your-key.pem" ubuntu@<PUBLIC_IP>
```
*(Replace `your-key.pem` with your key file and `<PUBLIC_IP>` with the instance’s IP.)*  

---

## **Step 3: Create the `nextdeploy` User & Grant Passwordless sudo**  

### **3.1 Create the User**  
```bash
sudo adduser nextdeploy
sudo usermod -aG sudo nextdeploy  # Add to sudo group
```

### **3.2 Set Up Passwordless sudo**  
Edit the sudoers file securely with `visudo`:  
```bash
sudo visudo
```  
Add this line at the **end** of the file:  
```bash
nextdeploy ALL=(ALL) NOPASSWD:ALL
```  
Save & exit (`Ctrl+X`, then `Y`, then `Enter`).  

### **3.3 Test Passwordless sudo**  
Switch to `nextdeploy` and verify:  
```bash
su - nextdeploy
sudo -v  # Should NOT ask for a password
```
If successful, you can now run `sudo` commands without a password.  

---

## **Step 4 (Optional): Harden Security**  

### **4.1 Disable Password Authentication (SSH)**  
Edit `/etc/ssh/sshd_config`:  
```bash
sudo nano /etc/ssh/sshd_config
```  
Ensure these settings:  
```bash
PasswordAuthentication no
PermitRootLogin no
```  
Restart SSH:  
```bash
sudo systemctl restart sshd
```

### **4.2 Set Up SSH Key for `nextdeploy` (Recommended)**  
From your local machine:  
```bash
ssh-copy-id -i ~/.ssh/id_rsa.pub nextdeploy@<PUBLIC_IP>
```  
Now, `nextdeploy` can log in **without a password** (using SSH keys).  

---

## **Conclusion**  
You’ve successfully:  
✔ Launched an **EC2 instance** with Ubuntu.  
✔ Created a **`nextdeploy` user** with **passwordless sudo**.  
✔ (Optional) **Hardened SSH security**.  

Now, you can securely manage your server without constant password prompts while maintaining security best practices.  

**Next Steps:**  
- Install a firewall (`ufw`).  
- Set up automatic updates (`unattended-upgrades`).  
- Configure monitoring (e.g., CloudWatch).  

Need further customization? Let me know! 🚀
