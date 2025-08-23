#!/bin/bash
# Real-world deployment example using the JSON configurations

set -e

# Configuration
DAEMON_CONFIG="/etc/nextdeployd/config.json"
TEMPLATES_CONFIG="/etc/nextdeployd/templates.json"
ENV_CONFIG="/etc/nextdeployd/environments/production.json"

# Environment variables (loaded from secrets)
export DB_PASSWORD="$(cat /run/secrets/db_password)"
export REDIS_PASSWORD="$(cat /run/secrets/redis_password)"
export JWT_SECRET="$(cat /run/secrets/jwt_secret)"
export API_KEY="$(cat /run/secrets/api_key)"
export VERSION="v2.1.0"

echo "🚀 Starting production deployment for version $VERSION"

# 1. Pre-deployment checks
echo "📋 Running pre-deployment checks..."

# Check daemon status
if ! nextdeployd status > /dev/null 2>&1; then
    echo "❌ Daemon not accessible"
    exit 1
fi

# Check Docker resources
MEMORY_USAGE=$(docker system df --format "table {{.Type}}\t{{.Size}}" | grep "Local Volumes" | awk '{print $3}' | sed 's/GB//')
if (( $(echo "$MEMORY_USAGE > 50" | bc -l) )); then
    echo "⚠️  High disk usage: ${MEMORY_USAGE}GB"
fi

# Pull new images
echo "📦 Pulling application images..."
nextdeployd pull --image="mycompany/app:$VERSION"
nextdeployd pull --image="nginx:1.21"
nextdeployd pull --image="postgres:15"
nextdeployd pull --image="redis:7-alpine"

# 2. Deploy infrastructure components first
echo "🗄️  Deploying infrastructure components..."

# Deploy database if not exists
if ! nextdeployd listcontainers | grep -q "database-primary"; then
    echo "🗄️  Deploying database..."
    nextdeployd deploy \
        --image="postgres:15" \
        --name="database-primary" \
        --ports="5432:5432" \
        --env="POSTGRES_DB=app_production" \
        --env="POSTGRES_USER=app_user" \
        --env="POSTGRES_PASSWORD=$DB_PASSWORD" \
        --env="POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256" \
        --volumes="/var/lib/postgresql/data:/var/lib/postgresql/data" \
        --volumes="/etc/postgresql/postgresql.conf:/etc/postgresql/postgresql.conf:ro" \
        --restart="always"
    
    # Wait for database to be ready
    echo "⏳ Waiting for database to be ready..."
    sleep 30
    
    # Health check
    nextdeployd health --container="database-primary"
fi

# Deploy Redis cache
if ! nextdeployd listcontainers | grep -q "redis-cache"; then
    echo "🗄️  Deploying Redis cache..."
    nextdeployd deploy \
        --image="redis:7-alpine" \
        --name="redis-cache" \
        --ports="6379:6379" \
        --env="REDIS_PASSWORD=$REDIS_PASSWORD" \
        --volumes="/var/lib/redis:/data" \
        --volumes="/etc/redis/redis.conf:/usr/local/etc/redis/redis.conf:ro" \
        --restart="unless-stopped"
    
    sleep 10
    nextdeployd health --container="redis-cache"
fi

# 3. Blue-Green deployment for application servers
echo "🔄 Starting blue-green deployment for application servers..."

# Deploy new version (green)
NEW_CONTAINER="app-server-green"
OLD_CONTAINER="app-server-blue"

echo "🚢 Deploying new application version..."
nextdeployd deploy \
    --image="mycompany/app:$VERSION" \
    --name="$NEW_CONTAINER" \
    --ports="3001:3000" \
    --env="NODE_ENV=production" \
    --env="DATABASE_URL=postgresql://app_user:$DB_PASSWORD@database-primary:5432/app_production" \
    --env="REDIS_URL=redis://:$REDIS_PASSWORD@redis-cache:6379" \
    --env="JWT_SECRET=$JWT_SECRET" \
    --env="API_KEY=$API_KEY" \
    --env="LOG_LEVEL=info" \
    --volumes="/var/log/app:/app/logs" \
    --volumes="/etc/app/config.json:/app/config.json:ro" \
    --restart="unless-stopped"

# Health check new version
echo "🏥 Performing health check on new version..."
sleep 15

MAX_RETRIES=6
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if nextdeployd health --container="$NEW_CONTAINER"; then
        echo "✅ Health check passed"
        break
    else
        echo "⏳ Health check failed, retrying... ($((RETRY_COUNT + 1))/$MAX_RETRIES)"
        sleep 10
        ((RETRY_COUNT++))
    fi
    
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo "❌ Health check failed after $MAX_RETRIES attempts"
        echo "🧹 Cleaning up failed deployment..."
        nextdeployd remove --container="$NEW_CONTAINER" --force=true
        exit 1
    fi
done

# Smoke tests
echo "🧪 Running smoke tests..."
# Test basic functionality
if curl -f -s "http://localhost:3001/api/health" > /dev/null; then
    echo "✅ Application endpoint responding"
else
    echo "❌ Application endpoint not responding"
    nextdeployd logs --container="$NEW_CONTAINER" --lines=50
    exit 1
fi

# 4. Swap containers (Blue-Green switch)
if nextdeployd listcontainers | grep -q "$OLD_CONTAINER"; then
    echo "🔄 Swapping containers (Blue-Green switch)..."
    
    # Update load balancer to point to new container
    # (In a real setup, you'd update your load balancer config here)
    
    # Swap the containers
    nextdeployd swapcontainers --from="$OLD_CONTAINER" --to="$NEW_CONTAINER"
    
    echo "✅ Container swap completed"
    
    # Keep old version for rollback (rename it)
    BACKUP_NAME="app-server-backup-$(date +%s)"
    nextdeployd stop --container="$OLD_CONTAINER"
    # In real implementation, you'd rename the container for potential rollback
    
else
    echo "ℹ️  No existing container to swap, this is initial deployment"
    # Update container name for consistency
    nextdeployd stop --container="$NEW_CONTAINER"
    # Rename to standard name (would need to implement rename command)
    nextdeployd start --container="$NEW_CONTAINER"
fi

# 5. Deploy/update web servers
echo "🌐 Deploying web servers..."

for i in 1 2; do
    WEB_CONTAINER="web-server-$i"
    
    # Stop old web server
    if nextdeployd listcontainers | grep -q "$WEB_CONTAINER"; then
        nextdeployd stop --container="$WEB_CONTAINER"
        nextdeployd remove --container="$WEB_CONTAINER"
    fi
    
    # Deploy new web server
    PORT=$((8080 + i - 1))
    nextdeployd deploy \
        --image="nginx:1.21" \
        --name="$WEB_CONTAINER" \
        --ports="$PORT:80" \
        --ports="$((PORT + 1000)):443" \
        --env="NGINX_HOST=app.company.com" \
        --env="NGINX_PORT=80" \
        --volumes="/etc/nginx/nginx.conf:/etc/nginx/nginx.conf:ro" \
        --volumes="/var/www/html:/usr/share/nginx/html:ro" \
        --volumes="/var/log/nginx:/var/log/nginx" \
        --restart="unless-stopped"
    
    # Health check
    nextdeployd health --container="$WEB_CONTAINER"
    echo "✅ Web server $i deployed"
done

# 6. Post-deployment verification
echo "🔍 Running post-deployment verification..."

# Check all containers are running
EXPECTED_CONTAINERS=("database-primary" "redis-cache" "app-server-green" "web-server-1" "web-server-2")

for container in "${EXPECTED_CONTAINERS[@]}"; do
    if nextdeployd health --container="$container"; then
        echo "✅ $container: healthy"
    else
        echo "❌ $container: unhealthy"
        nextdeployd logs --container="$container" --lines=20
    fi
done

# System resource check
echo "💾 System resource usage:"
nextdeployd status

# Application-level tests
echo "🧪 Running integration tests..."

# Test database connectivity
if curl -f -s "http://localhost:3001/api/db-health" > /dev/null; then
    echo "✅ Database connectivity: OK"
else
    echo "❌ Database connectivity: FAILED"
fi

# Test cache connectivity  
if curl -f -s "http://localhost:3001/api/cache-health" > /dev/null; then
    echo "✅ Cache connectivity: OK"
else
    echo "❌ Cache connectivity: FAILED"
fi

# Test end-to-end workflow
if curl -f -s "http://localhost:8080/api/status" > /dev/null; then
    echo "✅ End-to-end workflow: OK"
else
    echo "❌ End-to-end workflow: FAILED"
fi

# 7. Cleanup old resources
echo "🧹 Cleaning up old resources..."

# Remove old images (keep last 3 versions)
OLD_IMAGES=$(docker images "mycompany/app" --format "table {{.Repository}}:{{.Tag}}" | grep -v "$VERSION" | tail -n +4)
if [ ! -z "$OLD_IMAGES" ]; then
    echo "🗑️  Removing old images: $OLD_IMAGES"
    # docker rmi $OLD_IMAGES (would implement this)
fi

# Remove old backup containers (keep last 3)
OLD_BACKUPS=$(nextdeployd listcontainers --all=true | grep "backup-" | tail -n +4 | awk '{print $2}')
if [ ! -z "$OLD_BACKUPS" ]; then
    echo "🗑️  Removing old backups: $OLD_BACKUPS"
    for backup in $OLD_BACKUPS; do
        nextdeployd remove --container="$backup" --force=true
    done
fi

# 8. Update monitoring/alerting
echo "📊 Updating monitoring configuration..."

# Generate deployment report
cat > /var/log/nextdeploy/deployment-$(date +%Y%m%d-%H%M%S).json << EOF
{
  "deployment_id": "$(uuidgen)",
  "timestamp": "$(date -Iseconds)",
  "version": "$VERSION",
  "status": "success",
  "containers_deployed": [
    {
      "name": "app-server-green",
      "image": "mycompany/app:$VERSION",
      "status": "healthy"
    },
    {
      "name": "web-server-1",
      "image": "nginx:1.21",
      "status": "healthy"
    },
    {
      "name": "web-server-2", 
      "image": "nginx:1.21",
      "status": "healthy"
    }
  ],
  "rollback_available": true,
  "deployment_time": "$(date -d "$START_TIME" +%s)s"
}
EOF

echo "🎉 Deployment completed successfully!"
echo "📋 Summary:"
echo "   Version deployed: $VERSION"
echo "   Containers running: $(nextdeployd listcontainers | wc -l)"
echo "   System status: $(nextdeployd status | grep -o 'healthy\|unhealthy')"
echo "   Deployment log: /var/log/nextdeploy/deployment-$(date +%Y%m%d-%H%M%S).json"

# Send success notification (webhook, Slack, etc.)
curl -X POST "https://hooks.slack.com/webhook-url" \
  -H "Content-Type: application/json" \
  -d "{\"text\": \"✅ Deployment $VERSION completed successfully on $(hostname)\"}" \
  > /dev/null 2>&1 || true

echo "✅ All done!"
