//go:build ignore
# NextDeploy Docker Daemon - Setup & Usage Guide

## Installation & Setup

### 1. Build the Daemon

```bash
# Build the daemon
go build -o nextdeployd main.go

# Make it executable
chmod +x nextdeployd

# Install system-wide
sudo mv nextdeployd /usr/local/bin/

# Create config directory
sudo mkdir -p /etc/nextdeployd
```

### 2. Configuration File

Create `/etc/nextdeployd/config.json`:

```json
{
  "socket_path": "/var/run/nextdeployd.sock",
  "socket_mode": "0660",
  "docker_socket": "/var/run/docker.sock",
  "container_prefix": "nextdeploy-",
  "log_level": "info",
  "allowed_users": ["root", "deploy", "docker"]
}
```

### 3. System Service Setup

Create `/etc/systemd/system/nextdeployd.service`:

```ini
[Unit]
Description=NextDeploy Docker Daemon
Documentation=https://github.com/yourorg/nextdeployd
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=root
Group=docker
ExecStart=/usr/local/bin/nextdeployd daemon --config=/etc/nextdeployd/config.json
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/run /var/lib/docker

[Install]
WantedBy=multi-user.target
```

### 4. Start and Enable Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable and start the service
sudo systemctl enable nextdeployd
sudo systemctl start nextdeployd

# Check status
sudo systemctl status nextdeployd

# View logs
sudo journalctl -u nextdeployd -f
```

## Security Configuration

### 1. Socket Permissions

The daemon creates a Unix socket with restricted permissions:

```bash
# Socket file: /var/run/nextdeployd.sock
# Permissions: 0660 (owner: root, group: docker)
# Only root and docker group members can access
```

### 2. User Access Control

Add users to the docker group to allow daemon access:

```bash
# Add user to docker group
sudo usermod -aG docker username

# Verify group membership
groups username
```

### 3. Firewall Configuration

Since the daemon only uses Unix sockets, no network ports are exposed. However, ensure proper system security:

```bash
# Ensure Docker daemon is not exposed to network
sudo netstat -tlnp | grep docker

# Should only show local Docker API socket, not network ports
```

## Basic Usage Examples

### Start the Daemon

```bash
# Start daemon manually (for testing)
sudo nextdeployd daemon

# Or use systemd
sudo systemctl start nextdeployd
```

### Container Management

```bash
# List running containers
nextdeployd listcontainers

# List all containers (including stopped)
nextdeployd listcontainers --all=true

# Deploy a new container
nextdeployd deploy --image=nginx:latest --name=web-server --ports=80:8080

# Deploy with environment variables and volumes
nextdeployd deploy \
  --image=mysql:8.0 \
  --name=database \
  --env=MYSQL_ROOT_PASSWORD=secret \
  --env=MYSQL_DATABASE=app \
  --volumes=/var/lib/mysql:/var/lib/mysql \
  --ports=3306:3306

# Start/stop/restart containers
nextdeployd start --container=web-server
nextdeployd stop --container=web-server
nextdeployd restart --container=web-server

# Remove container
nextdeployd remove --container=web-server
nextdeployd remove --container=web-server --force=true
```

### Advanced Operations

```bash
# Swap containers (blue-green deployment)
nextdeployd swapcontainers --from=app-v1 --to=app-v2

# Pull latest image
nextdeployd pull --image=nginx:latest

# View container logs
nextdeployd logs --container=web-server --lines=100

# Inspect container details
nextdeployd inspect --container=web-server

# Health checks
nextdeployd health --container=web-server
nextdeployd status

# Rollback to previous version
nextdeployd rollback --container=web-server
```

## Deployment Workflows

### 1. Blue-Green Deployment

```bash
#!/bin/bash
# blue-green-deploy.sh

APP_NAME="myapp"
NEW_VERSION="$1"
OLD_CONTAINER="${APP_NAME}-blue"
NEW_CONTAINER="${APP_NAME}-green"

if [ -z "$NEW_VERSION" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

echo "🚀 Starting blue-green deployment for $APP_NAME:$NEW_VERSION"

# Pull new image
echo "📦 Pulling new image..."
nextdeployd pull --image="$APP_NAME:$NEW_VERSION"

# Deploy new version
echo "🚢 Deploying new version..."
nextdeployd deploy \
  --image="$APP_NAME:$NEW_VERSION" \
  --name="$NEW_CONTAINER" \
  --ports=8080:8080 \
  --env="ENVIRONMENT=production"

# Health check
echo "🏥 Performing health check..."
sleep 10
if nextdeployd health --container="$NEW_CONTAINER"; then
    echo "✅ Health check passed"
    
    # Swap containers
    echo "🔄 Swapping containers..."
    nextdeployd swapcontainers --from="$OLD_CONTAINER" --to="$NEW_CONTAINER"
    
    echo "✅ Deployment completed successfully"
else
    echo "❌ Health check failed, cleaning up..."
    nextdeployd remove --container="$NEW_CONTAINER" --force=true
    exit 1
fi
```

### 2. Rolling Update

```bash
#!/bin/bash
# rolling-update.sh

APP_NAME="$1"
NEW_VERSION="$2"
REPLICAS=3

if [ -z "$APP_NAME" ] || [ -z "$NEW_VERSION" ]; then
    echo "Usage: $0 <app_name> <version>"
    exit 1
fi

echo "🔄 Starting rolling update for $APP_NAME:$NEW_VERSION"

# Pull new image
nextdeployd pull --image="$APP_NAME:$NEW_VERSION"

# Update each replica
for i in $(seq 1 $REPLICAS); do
    CONTAINER_NAME="${APP_NAME}-${i}"
    echo "📦 Updating replica $i..."
    
    # Stop old container
    nextdeployd stop --container="$CONTAINER_NAME"
    
    # Remove old container
    nextdeployd remove --container="$CONTAINER_NAME"
    
    # Deploy new version
    nextdeployd deploy \
      --image="$APP_NAME:$NEW_VERSION" \
      --name="$CONTAINER_NAME" \
      --ports="808${i}:8080"
    
    # Wait and health check
    sleep 5
    if ! nextdeployd health --container="$CONTAINER_NAME"; then
        echo "❌ Health check failed for replica $i"
        exit 1
    fi
    
    echo "✅ Replica $i updated successfully"
done

echo "✅ Rolling update completed"
```

### 3. Maintenance Mode

```bash
#!/bin/bash
# maintenance.sh

ACTION="$1"
APP_NAME="${2:-myapp}"

case "$ACTION" in
    "enable")
        echo "🚧 Enabling maintenance mode..."
        nextdeployd deploy \
          --image=nginx:alpine \
          --name="$APP_NAME-maintenance" \
          --ports=80:80
        
        nextdeployd stop --container="$APP_NAME"
        echo "✅ Maintenance mode enabled"
        ;;
    "disable")
        echo "🔄 Disabling maintenance mode..."
        nextdeployd start --container="$APP_NAME"
        nextdeployd remove --container="$APP_NAME-maintenance" --force=true
        echo "✅ Maintenance mode disabled"
        ;;
    *)
        echo "Usage: $0 {enable|disable} [app_name]"
        exit 1
        ;;
esac
```

## Monitoring & Logging

### 1. Container Health Monitoring

```bash
#!/bin/bash
# health-monitor.sh

CONTAINERS=("web-server" "database" "cache")

for container in "${CONTAINERS[@]}"; do
    echo -n "Checking $container: "
    if nextdeployd health --container="$container" > /dev/null 2>&1; then
        echo "✅ Healthy"
    else
        echo "❌ Unhealthy"
        # Send alert or restart container
        nextdeployd restart --container="$container"
    fi
done
```

### 2. Log Aggregation

```bash
#!/bin/bash
# collect-logs.sh

LOG_DIR="/var/log/nextdeploy"
DATE=$(date +%Y%m%d)

mkdir -p "$LOG_DIR"

# Get list of containers
CONTAINERS=$(nextdeployd listcontainers | grep -E "^\s*[a-f0-9]+" | awk '{print $2}')

for container in $CONTAINERS; do
    echo "📋 Collecting logs for $container..."
    nextdeployd logs --container="$container" --lines=1000 \
        > "$LOG_DIR/${container}_${DATE}.log"
done

echo "✅ Logs collected in $LOG_DIR"
```

### 3. System Status Dashboard

```bash
#!/bin/bash
# status-dashboard.sh

clear
echo "==========================================="
echo "    NextDeploy Docker Daemon Status"
echo "==========================================="
echo

# Daemon status
echo "🔧 Daemon Status:"
nextdeployd status
echo

# Container overview
echo "📦 Container Overview:"
nextdeployd listcontainers
echo

# Resource usage
echo "💾 Resource Usage:"
docker system df
echo

# Recent logs
echo "📋 Recent Activity:"
sudo journalctl -u nextdeployd --no-pager -n 5
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   # Check socket permissions
   ls -la /var/run/nextdeployd.sock
   
   # Add user to docker group
   sudo usermod -aG docker $USER
   ```

2. **Docker Not Accessible**
   ```bash
   # Check Docker service
   sudo systemctl status docker
   
   # Test Docker access
   docker ps
   ```

3. **Socket Connection Failed**
   ```bash
   # Check if daemon is running
   sudo systemctl status nextdeployd
   
   # Check socket file exists
   ls -la /var/run/nextdeployd.sock
   ```

4. **Container Deploy Failures**
   ```bash
   # Check Docker logs
   docker logs <container_name>
   
   # Check available resources
   docker system df
   df -h
   ```

### Log Analysis

```bash
# View daemon logs
sudo journalctl -u nextdeployd -f

# Filter by error level
sudo journalctl -u nextdeployd -p err

# View logs for specific time period
sudo journalctl -u nextdeployd --since "1 hour ago"
```

### Recovery Procedures

```bash
#!/bin/bash
# emergency-recovery.sh

echo "🚨 Emergency Recovery Procedure"

# Stop all containers gracefully
echo "⏹️  Stopping all containers..."
docker stop $(docker ps -q)

# Clean up daemon
echo "🧹 Cleaning up daemon..."
sudo systemctl stop nextdeployd
sudo rm -f /var/run/nextdeployd.sock

# Restart services
echo "🔄 Restarting services..."
sudo systemctl start docker
sudo systemctl start nextdeployd

echo "✅ Recovery completed"
```

This setup provides a secure, production-ready Docker daemon that only accepts local connections through Unix sockets, with proper permission controls and comprehensive management capabilities.
