
Absolutely — here's a **refined, algorithmic step-by-step process** to **automate the Caddy + HTTPS setup** for a running Next.js app inside a Docker container on an EC2 instance.

This is designed to be **infrastructure-agnostic**, reusable, and DevOps-friendly.

---

# ✅ Algorithm: Automate Caddy Reverse Proxy Setup for Next.js Docker App on EC2

---

### **🧾 INPUTS**

* `domain_name`: e.g. `nextdeploy.one`
* `ec2_ip`: EC2 instance public IP
* `ssh_key_path`: local `.pem` file path
* `ssh_user`: usually `ubuntu`
* `app_port`: Docker container port on host (e.g. `3002`)
* `docker_container_name`: e.g. `production-green`

---

## 🧠 STEP 0: Validate Prerequisites

```text
If Docker container not running → abort
If domain A record does NOT point to EC2 IP → abort
If EC2 does not have ports 80/443 open → abort
```

---

## 🛠 STEP 1: Connect and Prepare EC2 Instance

```bash
ssh -i $ssh_key_path $ssh_user@$ec2_ip
```

Inside EC2:

```bash
# Ensure sudo is passwordless (do once)
echo "$ssh_user ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/90-cloud-init-users
sudo chmod 440 /etc/sudoers.d/90-cloud-init-users
```

---

## 📦 STEP 2: Install Caddy

```bash
sudo apt update
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy -y
```

---

## 📝 STEP 3: Write the `Caddyfile`

Overwrite `/etc/caddy/Caddyfile` with:

```caddy
{domain_name} {
    reverse_proxy localhost:{app_port}

    log {
        output file /var/log/caddy/{domain_name}.access.log
    }
}
```

Command:

```bash
sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
$domain_name {
    reverse_proxy localhost:$app_port

    log {
        output file /var/log/caddy/$domain_name.access.log
    }
}
EOF
```

---

## 🔓 STEP 4: Open Firewall and Confirm DNS

* Ensure EC2 Security Group allows ports `80` and `443`
* Run:

```bash
dig +short $domain_name | grep $ec2_ip || echo "❌ DNS mismatch"
```

Abort automation if it doesn't match.

---

## 🔁 STEP 5: Restart and Monitor Caddy

```bash
sudo systemctl restart caddy
sudo journalctl -u caddy -f --since "5s ago"
```

Wait for:

```
obtained certificate successfully
```

---

## 🧪 STEP 6: Test App Routing

```bash
curl -I https://$domain_name
# expect: HTTP/2 200
```

Also test in browser: `https://$domain_name`

---

## 🧹 OPTIONAL: Cleanup & Retry on Failure

```bash
sudo rm -rf /var/lib/caddy/.local/share/caddy/acme/*
sudo systemctl restart caddy
```

---

# 🧭 Final Output

✅ If successful, the domain will route traffic to your Docker container securely with HTTPS managed by Caddy.

---

### 💡 Want it as a Bash script or Go routine for your CLI?

I can generate that for you immediately — just tell me the format (bash, go, ansible, etc).
