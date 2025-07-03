Here's a **robust, production-ready Caddyfile** for Next.js with API deployment capabilities:

### **Optimal Caddyfile for Next.js (with API support)**
```caddy
{
    # Global settings
    email your-email@example.com  # For Let's Encrypt
    acme_ca https://acme-v02.api.letsencrypt.org/directory
    log {
        output file /var/log/caddy/access.log {
            roll_size 100mb
            roll_keep 5
        }
    }
}

# Primary domain
nextdeploy.one {
    # Handle HTTP->HTTPS and www->non-www redirects
    redir https://nextdeploy.one{uri} permanent

    # WebSocket route matcher
    @ws {
        header Connection *Upgrade*
        header Upgrade websocket
    }

    # Reverse proxy configuration
    reverse_proxy @ws localhost:3001  # WebSocket traffic
    reverse_proxy localhost:3000 localhost:3001 localhost:3002 {  # Normal traffic
        # Next.js optimizations
        transport http {
            keepalive 30s
            tls_insecure_skip_verify  # For local Docker networks
        }

        # Load balancing
        lb_policy first
        lb_try_duration 5s
        health_uri /health
        health_interval 10s
    }

    # Security headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        -Server  # Remove server header
    }

    # Cache control (customize per route)
    @static {
        path *.css *.js *.jpg *.png *.svg *.ico *.woff2
    }
    header @static Cache-Control "public, max-age=31536000, immutable"
}
```

---

### **How to Update Config via Caddy API**

#### 1. First, enable the admin API:
Add to your global config (`/etc/caddy/Caddyfile`):
```caddy
{
    admin :2019
}
```

#### 2. Reload Caddy:
```bash
sudo systemctl reload caddy
```

#### 3. Update config via API:
```bash
# Get current config
curl http://localhost:2019/config/

# Post new config (example)
curl -X POST "http://localhost:2019/load" \
    -H "Content-Type: text/caddyfile" \
    --data-binary @/etc/caddy/Caddyfile.new
```

#### 4. For zero-downtime updates:
```bash
# Validate config first
curl -X POST "http://localhost:2019/config/validate" \
    -H "Content-Type: text/caddyfile" \
    --data-binary @new-config.caddy

# Then apply
curl -X POST "http://localhost:2019/config/" \
    -H "Content-Type: text/caddyfile" \
    --data-binary @new-config.caddy
```

---

### **Key Features of This Config:**
1. **Blue/Green Ready**: Handles port changes (3000/3001/3002) automatically
2. **WebSocket Support**: Explicit WebSocket routing for HMR
3. **Security Hardened**: HSTS, CSP-ready headers, and server hiding
4. **Performance Optimized**: 
   - Keepalive connections
   - Static asset caching
   - Health-check aware load balancing
5. **Observability**: Structured logging with rotation

---

### **Maintenance Commands:**
```bash
# Check active ports
curl http://localhost:2019/config/ | jq .apps.http.servers.srv0.routes

# Force certificate renewal
curl -X POST "http://localhost:2019/config/certificates/renew" \
    -H "Content-Type: application/json" \
    -d '{"domain":"nextdeploy.one"}'
```

Would you like me to add any specific features like:
- Rate limiting?
- Geo-blocking?
- A/B testing routes?
- Advanced caching rules?


Here's how to dynamically update your reverse proxy configuration via Caddy's admin API, specifically targeting the backend ports:

### 1. First, Get Your Current Config
```bash
curl http://localhost:2019/config/ | jq .
```

### 2. Update Just the Reverse Proxy Targets
```bash
curl -X PATCH "http://localhost:2019/config/apps/http/servers/srv0/routes/1/handle/0/routes/1/handle/0/upstreams" \
-H "Content-Type: application/json" \
-d '[
    {"dial": "localhost:3000"},
    {"dial": "localhost:3001"}, 
    {"dial": "localhost:3002"}
]'
```

### Alternative: Full Config Update Method
```bash
# Create a patch file (update-proxy.json)
cat <<EOF > update-proxy.json
{
    "@id": "proxy-upstreams",
    "upstreams": [
        {"dial": "localhost:3000"},
        {"dial": "localhost:3001"},
        {"dial": "localhost:3003"}  # Changed 3002→3003
    ]
}
EOF

# Apply the patch
curl -X PATCH "http://localhost:2019/config/" \
-H "Content-Type: application/json" \
--data-binary @update-proxy.json
```

### Key Notes:
1. **Zero Downtime**: The update happens instantly without dropping connections
2. **Health Checks**: New ports will be automatically verified (thanks to your `health_uri` config)
3. **WebSocket Preservation**: The separate `@ws` route remains unaffected

### To Verify the Update:
```bash
curl http://localhost:2019/config/ | jq '.apps.http.servers.srv0.routes[1].handle[0].routes[1].handle[0]'
```

### For Dynamic Environments (Docker Swarm/K8s):
Consider using Caddy's [dynamic upstreams](https://caddyserver.com/docs/json/apps/http/servers/routes/handle/reverse_proxy/upstreams/) with DNS discovery:
```json
"upstreams": [{
    "lookup_srv": "_http._tcp.backend.nextdeploy.svc.cluster.local"
}]
```

Would you like me to show how to:
1. Add/remove ports dynamically based on Docker events?
2. Implement weighted load balancing?
3. Set up canary deployments via the API?
