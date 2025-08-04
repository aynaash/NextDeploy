
# Docker Mounts & Volumes Reference

This document consolidates practical Docker mount/volume knowledge and analysis of `/proc/self/mountinfo` for deep infrastructure understanding.

---

## 🔧 Practical CLI Experiments to Understand Mounts

### 1. **Basic Container Filesystem**

```bash
docker run --rm -it alpine sh
# Inside container:
ls /
mount
```

➡ **Takeaway**: Isolated root filesystem.

---

### 2. **Bind Mount Host Directory**

```bash
mkdir -p /tmp/hostdata
echo "hello from host" > /tmp/hostdata/hello.txt
docker run --rm -it -v /tmp/hostdata:/data alpine sh
# Inside container:
cat /data/hello.txt
```

➡ **Takeaway**: External data mounted like USB drive.

---

### 3. **Compare Mount Info (Inside vs Host)**

```bash
# Inside container:
cat /proc/self/mountinfo
# On host:
cat /proc/self/mountinfo | grep hostdata
```

➡ **Takeaway**: Mount namespace isolation.

---

### 4. **Inspect Docker Volume**

```bash
docker volume create mydata
docker run --rm -it -v mydata:/data alpine sh
# Inside container:
touch /data/test.txt
# On host:
docker volume inspect mydata
ls -l /var/lib/docker/volumes/mydata/_data
```

➡ **Takeaway**: Persistent, Docker-managed storage.

---

### 5. **Validate Volume Persistence**

```bash
docker run -it --name testbox -v mydata:/data alpine sh
# Inside container:
echo "I persist" > /data/persist.txt
exit
docker rm -f testbox
docker run -it -v mydata:/data alpine sh
cat /data/persist.txt
```

➡ **Takeaway**: Volumes outlive containers.

---

### 6. **Read-Only Mount**

```bash
docker run --rm -it -v /tmp/hostdata:/data:ro alpine sh
# Inside container:
echo "fail" > /data/test.txt
```

➡ **Takeaway**: Mount flags matter inside containers.

---

### 7. **Named Volume vs Bind Mount**

➡ Rename your project directory.

* Bind mounts break.
* Named volumes stay intact.

➡ **Takeaway**: Use bind mounts for dev, named volumes for portability.

---

## 🔍 Analyzing `/proc/self/mountinfo` Inside Containers

Each line follows this structure:

```
<mount-id> <parent-id> <major:minor> <root> <mount-point> <mount-options> - <filesystem-type> <source> <superblock-options>
```

### Key Mounts:

#### 1. **Root (`/`)**

```
4425 4063 0:90 / / rw,relatime master:1021 - overlay overlay rw,lowerdir=...,upperdir=...,workdir=...,nouserxattr
```

* **OverlayFS** for Docker's layered filesystem
* Combines image + container layer

#### 2. **`/proc`**

```
4426 4425 0:93 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
```

* Virtual process filesystem
* Security: `nosuid`, `nodev`, `noexec`

#### 3. **`/dev`**

```
4427 4425 0:94 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
```

* In-memory `tmpfs` for device files

#### 4. **`/sys`**

```
4429 4425 0:107 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
```

* Kernel/system virtual files
* Mounted read-only

#### 5. **Bind Mount Volume (`/data`)**

```
4433 4425 0:39 /hostdata /data rw,nosuid,nodev - tmpfs tmpfs rw,nr_inodes=1048576,inode64
```

* Bind mount from host `/tmp/hostdata`

#### 6. **Docker Managed Files**

```
4434 4425 259:2 /var/lib/docker/containers/.../resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
4435 4425 259:2 /var/lib/docker/containers/.../hostname /etc/hostname rw,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
4436 4425 259:2 /var/lib/docker/containers/.../hosts /etc/hosts rw,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
```

* DNS, hostname, hosts file provided by Docker
* Backed by host ext4 filesystem

#### 7. **Other Mounts**

* `/dev/shm`: shared memory (tmpfs)
* `/dev/mqueue`: POSIX message queues
* `/sys/fs/cgroup`: cgroupv2

---

## 📌 Key Takeaways

1. Docker uses **OverlayFS** for root filesystem isolation.
2. Bind mounts behave like **USB drives** into the container.
3. Named volumes are **decoupled and persistent** across runs.
4. Docker injects critical system files via **bind mounts**.
5. Containers live in **their own mount namespace** — isolated from the host.
6. Mount flags (`ro`, `nosuid`, etc.) are used for **security**.
7. This knowledge scales into Kubernetes volumes and CI/CD pipelines.

---

## 🚀 Final Thought

> Containers are just Linux processes with boundaries. Mounts are how we carefully punch holes in those boundaries to let them touch external data.

-# Docker Networking, Firewalls, and Packet Filtering: A Deep Dive

Docker's networking system relies heavily on Linux's packet filtering capabilities, particularly `iptables` (for IPv4) and `ip6tables` (for IPv6). Understanding this interaction is crucial for securing your Docker deployments while maintaining proper network functionality.

## Core Concepts

### 1. Docker's Default Networking Behavior

When you install Docker, it:
- Creates several custom `iptables` chains
- Sets default policies for network isolation
- Implements NAT for container networking
- Manages port publishing rules

**Key Chains Created:**
- `DOCKER-USER` - For user-defined rules that process before Docker's rules
- `DOCKER` - Handles port forwarding to containers
- `DOCKER-ISOLATION-STAGE-1/2` - Network isolation between Docker networks
- `DOCKER-INGRESS` - For Swarm services

### 2. Viewing Docker's iptables Rules

To inspect current rules:

```bash
# View filter table (firewall rules)
sudo iptables -L -n -v

# View NAT table (port forwarding)
sudo iptables -t nat -L -n -v

# View rules in DOCKER-USER chain
sudo iptables -L DOCKER-USER -n -v
```

### 3. Network Traffic Flow

Understanding the packet flow is critical for diagnosis:

1. **Incoming Traffic:**
   - Hits `PREROUTING` (nat table)
   - Processed by Docker's NAT rules
   - Forwarded to `DOCKER` chain
   - Finally reaches container

2. **Outgoing Traffic:**
   - Goes through `POSTROUTING` (nat table)
   - Masqueraded (SNAT) by Docker
   - Leaves host interface

## Practical Diagnosis Techniques

### 1. Checking Published Ports

```bash
# List all port mappings
docker ps --format 'table {{.Names}}\t{{.Ports}}'

# Detailed port info for a container
docker port <container_name>
```

### 2. Tracing Network Path

```bash
# Check routing
ip route

# Check interface configuration
ip addr

# Trace NAT transformations
sudo iptables -t nat -L -n -v
```

### 3. Packet Capture

```bash
# Capture on docker0 bridge
sudo tcpdump -i docker0

# Capture on specific container interface
sudo tcpdump -i any -n host <container_ip>
```

## Firewall Management Best Practices

### 1. Custom Rules in DOCKER-USER

Always place custom rules in `DOCKER-USER` chain as they:
- Process before Docker's rules
- Persist across Docker restarts
- Don't interfere with Docker operations

Example - Restrict access to published ports:
```bash
# Allow only specific IP to access containers
sudo iptables -I DOCKER-USER -i eth0 ! -s 192.168.1.100 -j DROP

# Allow established connections
sudo iptables -I DOCKER-USER -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
```

### 2. Direct Routing Configuration

For advanced networking without NAT:
```bash
# Create network with direct routing
docker network create \
  --subnet 192.168.100.0/24 \
  -o com.docker.network.bridge.gateway_mode_ipv4=routed \
  my-routed-net
```

### 3. Integration with Host Firewalls

**With firewalld:**
```bash
# View docker zone
sudo firewall-cmd --zone=docker --list-all

# Add custom services
sudo firewall-cmd --zone=docker --add-service=custom-service --permanent
```

**With UFW:**
```bash
# Allow Docker traffic while maintaining UFW
sudo ufw allow in on docker0
sudo ufw allow out on docker0
```

## Common Issues and Solutions

### 1. Port Publishing Not Working

Diagnosis steps:
1. Check if container is running: `docker ps`
2. Verify port mapping: `docker inspect <container> | grep HostPort`
3. Check iptables NAT rules: `sudo iptables -t nat -L DOCKER -n -v`
4. Test connectivity: `telnet <host_ip> <published_port>`

### 2. Containers Can't Reach Internet

Check:
1. IP forwarding enabled: `sysctl net.ipv4.ip_forward`
2. NAT rules: `sudo iptables -t nat -L POSTROUTING -n -v`
3. DNS configuration: `docker run --rm alpine ping -c 1 google.com`

### 3. Firewall Blocking Container Traffic

Solution:
```bash
# Allow related/established connections
sudo iptables -I DOCKER-USER -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# Allow specific container traffic
sudo iptables -I DOCKER-USER -s <container_ip> -j ACCEPT
```

## Advanced Configuration

### 1. Customizing Default Binding Address

In `/etc/docker/daemon.json`:
```json
{
  "ip": "127.0.0.1",
  "iptables": true,
  "ip6tables": true
}
```

### 2. Disabling Docker's iptables Management

(Not recommended for most users)
```json
{
  "iptables": false,
  "ip6tables": false
}
```

### 3. Network Performance Tuning

```bash
# Increase connection tracking table size
echo 2000000 | sudo tee /proc/sys/net/netfilter/nf_conntrack_max

# Adjust TCP timeouts
echo 120 | sudo tee /proc/sys/net/netfilter/nf_conntrack_tcp_timeout_established
```

## Monitoring and Maintenance

### 1. Regular Rule Auditing

```bash
# Save current iptables rules
sudo iptables-save > docker-iptables-backup-$(date +%F).rules

# Compare with previous rules
diff iptables-backup-old.rules iptables-backup-new.rules
```

### 2. Logging Suspicious Traffic

```bash
# Log dropped packets
sudo iptables -I DOCKER-USER -j LOG --log-prefix "DOCKER-FW: "

# View logs
sudo tail -f /var/log/kern.log | grep DOCKER-FW
```

## Conclusion

Docker's network isolation relies on sophisticated iptables rules that:
1. Implement NAT for container networking
2. Manage port publishing
3. Isolate networks
4. Control traffic flow

For secure deployments:
- Place custom rules in `DOCKER-USER` chain
- Regularly audit your iptables configuration
- Monitor network traffic patterns
- Consider direct routing for performance-critical applications

Remember that Docker manages these rules automatically, and manual modifications outside the recommended practices can break container networking. Always test changes in a development environment before applying them to production systems.--
Let me explain this networking information in simple terms, like we're learning together step by step.

## Understanding the Basic Components

### 1. The Firewall Rules (iptables)
Think of iptables like a security guard for your computer's network traffic. It decides what gets in and out.

The output shows several "chains" (groups of rules):
- **INPUT**: Rules for incoming traffic to your machine
- **FORWARD**: Rules for traffic passing through your machine (like to containers)
- **OUTPUT**: Rules for outgoing traffic from your machine
- **DOCKER***: Special chains Docker uses to manage container traffic

### 2. Network Interfaces (ip addr)
These are like different doors into your computer:
- **lo**: The "loopback" interface (traffic that stays inside your computer)
- **wlp108s0**: Your WiFi connection (IP 10.29.6.173)
- **docker0**: Docker's default bridge network (not currently active)
- **br-7c291cfa78a6**: Another Docker network (active, IP 172.18.0.1)
- **mpqemubr0**: A network for Multipass (like Docker but for virtual machines)

### 3. Network Routes (ip route)
These are like road signs telling traffic where to go:
- Default traffic goes through your WiFi (10.29.6.10)
- Docker networks have their own routes (172.17.0.0/16, 172.18.0.0/16)
- Multipass network has its route (10.195.1.0/24)

## Breaking Down What's Happening

### Docker Networking
1. **docker0** (172.17.0.1) is Docker's default network, but it's currently down
2. **br-7c291cfa78a6** (172.18.0.1) is another Docker network that is active
3. Docker has set up rules to:
   - Isolate these networks from each other
   - Allow containers to talk to the internet (MASQUERADE rules)
   - Forward traffic properly between networks

### Multipass Networking
1. **mpqemubr0** (10.195.1.1) is for Multipass virtual machines
2. It has rules to:
   - Allow DNS traffic (ports 53, 67)
   - Allow communication within its network (10.195.1.0/24)
   - Block other unwanted traffic (REJECT rules)

### Current Traffic
Notice most packet counts are 0 (like `pkts bytes` showing `0 0`). This means:
- Not much network traffic is happening right now
- The MASQUERADE rule for Multipass shows 14 packets (938 bytes) - some traffic went through

## Key Things to Notice

1. **Docker Isolation**:
   - Docker has special rules (DOCKER-ISOLATION-STAGE-1/2) to keep different Docker networks separate
   - This prevents containers on different networks from talking directly to each other

2. **Port Forwarding**:
   - The NAT (Network Address Translation) table shows how Docker makes container ports available
   - The MASQUERADE rules hide container IPs behind your host IP when they go online

3. **Security**:
   - By default, Docker sets up reasonable security
   - The DOCKER-USER chain is empty - this is where you could add your own firewall rules

## Simple Analogies

1. Think of your computer like an apartment building:
   - Each network interface is a different entrance (front door, back door, garage)
   - iptables are the doormen at each entrance checking IDs
   - The routes are like hallway directions to different apartments

2. Docker networks are like separate wings of the building:
   - They have their own security (isolation rules)
   - Guests (packets) need special permission to move between wings

## Common Questions Answered

**Q: Why can't I see any Docker container traffic?**
A: The docker0 interface is down (state DOWN), and there's no active traffic on the other Docker network (br-7c291cfa78a6 shows 0 packets in rules)

**Q: Where would I add my own firewall rules?**
A: In the DOCKER-USER chain, which is currently empty. This is the safe place to add rules without breaking Docker's networking.

**Q: What does MASQUERADE mean?**
A: It's like putting a return address from your building (host IP) on packages instead of the apartment number (container IP) when they go out to the internet.

Would you like me to explain any specific part in more detail?
Here’s a suite of commands to test your understanding of Docker and `iptables` based on the concepts above:

---

### **1. Inspect Docker’s iptables Rules**
```sh
# View all iptables rules (filter table)
sudo iptables -L -v -n

# View NAT rules (for port publishing)
sudo iptables -t nat -L -v -n

# Check Docker-specific chains
sudo iptables -L DOCKER-USER -v -n
sudo iptables -L DOCKER -v -n
```

**Test Knowledge:**  
- Can you identify which chains Docker creates?  
- How does Docker handle NAT for published ports?  

---

### **2. Restrict Access to Published Ports**
```sh
# Allow only 192.168.1.100 to access published ports
sudo iptables -I DOCKER-USER -i eth0 ! -s 192.168.1.100 -j DROP

# Allow only a subnet (192.168.1.0/24)
sudo iptables -I DOCKER-USER -i eth0 ! -s 192.168.1.0/24 -j DROP

# Allow ESTABLISHED/RELATED traffic (e.g., responses)
sudo iptables -I DOCKER-USER -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
```

**Test Knowledge:**  
- Why use `DOCKER-USER` instead of `FORWARD`?  
- How does `conntrack` help match original traffic?  

---

### **3. Direct Routing (Advanced)**
```sh
# Create a bridge network with trusted interfaces
docker network create --subnet 10.10.0.0/24 -o com.docker.network.bridge.trusted_host_interfaces="eth1" mynet

# Run a container with direct routing (no NAT)
docker run -d --network mynet --ip 10.10.0.100 -p 8080:80 nginx
```

**Test Knowledge:**  
- How does `trusted_host_interfaces` affect routing?  
- When would you use `routed` mode instead of `nat`?  

---

### **4. Secure Default Binding**
```sh
# Change default bind address to localhost (daemon.json)
echo '{"ip": "127.0.0.1"}' | sudo tee /etc/docker/daemon.json
sudo systemctl restart docker

# Verify binding (should now bind to 127.0.0.1)
docker run -d -p 8080:80 nginx
ss -tulnp | grep 8080  # Should show 127.0.0.1:8080
```

**Test Knowledge:**  
- Why restrict binding to `127.0.0.1`?  
- How does this impact external access?  

---

### **5. Docker as a Router**
```sh
# Prevent Docker from setting FORWARD policy to DROP
echo '{"ip-forward-no-drop": true}' | sudo tee -a /etc/docker/daemon.json
sudo systemctl restart docker

# Manually allow forwarding between interfaces
sudo iptables -I DOCKER-USER -i eth1 -o eth2 -j ACCEPT
```

**Test Knowledge:**  
- When would you disable `FORWARD DROP`?  
- How does this interact with Docker’s networking?  

---

### **6. Disable Docker’s iptables (Not Recommended)**
```sh
# Warning: Breaks Docker networking!
echo '{"iptables": false}' | sudo tee -a /etc/docker/daemon.json
sudo systemctl restart docker
```

**Test Knowledge:**  
- Why is disabling `iptables` management dangerous?  
- What breaks when you do this?  

---

### **7. Firewalld Integration**
```sh
# Check firewalld zones (if using firewalld)
sudo firewall-cmd --get-active-zones
sudo firewall-cmd --zone=docker --list-all
```

**Test Knowledge:**  
- How does Docker interact with `firewalld`?  
- What is the purpose of the `docker` zone?  

---

### **Verification Commands**
```sh
# Check container IP and ports
docker inspect <container_id> | grep -i "ipaddress\|ports"

# Test connectivity (from another host)
curl http://<docker_host_ip>:8080  # Should fail if restricted by DOCKER-USER
```

---

### **Scenarios to Test Understanding**
1. **Port Publishing**:  
   - Run `docker run -p 8080:80 nginx`. Can you access it externally? Why/why not?  

2. **Network Isolation**:  
   - Create two bridge networks. Can containers in different networks communicate?  

3. **UFW Conflict**:  
   - Enable UFW (`sudo ufw enable`). Does Docker still work? Why not?  

4. **Direct Routing**:  
   - Set `gateway_mode_ipv4=routed`. How does traffic flow now?  

---

