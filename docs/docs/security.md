# Comprehensive Security Architecture with BetterAuth Integration

Here's a full-scale security implementation for your daemon system using BetterAuth for centralized authentication across Next.js frontend, Go CLI, and Go daemon services:

## 1. System-Wide Authentication Flow

```
┌─────────────┐       ┌─────────────┐       ┌─────────────┐
│ Next.js     │───1──▶│ BetterAuth  │◀──2───│ Go CLI      │
│ Frontend    │◀──4───│ (Auth       │───3──▶│             │
└─────────────┘       │ Provider)   │       └─────────────┘
        │             └─────────────┘              │
        │                     ▲                    │
        │ 5                   │ 6                  │ 7
        ▼                     │                    ▼
┌─────────────────┐           │           ┌─────────────────┐
│ Go Daemon       │───────────┘           │ Other Services  │
│ (Central Hub)   │◀──────────────────────▶│ (if any)        │
└─────────────────┘      JWT Validation    └─────────────────┘
```

## 2. BetterAuth Integration Implementation

### Daemon Auth Middleware (Go)
```go
package auth

import (
    "context"
    "net/http"
    "strings"

    "github.com/betterauth/sdk-go"
)

type contextKey string

const (
    userContextKey contextKey = "user"
)

// BetterAuthMiddleware validates tokens and injects user context
func BetterAuthMiddleware(authClient *betterauth.Client) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract token from Authorization header
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }

            token := strings.TrimPrefix(authHeader, "Bearer ")
            if token == authHeader { // No Bearer prefix found
                http.Error(w, "Bearer token required", http.StatusUnauthorized)
                return
            }

            // Validate token with BetterAuth
            user, err := authClient.ValidateToken(r.Context(), token)
            if err != nil {
                http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
                return
            }

            // Add user to context
            ctx := context.WithValue(r.Context(), userContextKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// GetUserFromContext retrieves the authenticated user
func GetUserFromContext(ctx context.Context) (*betterauth.User, bool) {
    user, ok := ctx.Value(userContextKey).(*betterauth.User)
    return user, ok
}
```

## 3. CLI Authentication Flow (Go)

```go
package auth

import (
    "context"
    "fmt"
    "os"

    "github.com/betterauth/sdk-go"
    "golang.org/x/term"
)

func CLIInteractiveLogin(authClient *betterauth.Client) (*betterauth.Session, error) {
    fmt.Print("Enter BetterAuth username: ")
    var username string
    fmt.Scanln(&username)

    fmt.Print("Enter password: ")
    password, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        return nil, fmt.Errorf("reading password: %w", err)
    }
    fmt.Println() // Newline after password input

    // Initiate device authorization flow
    session, err := authClient.DeviceAuthorizationFlow(context.Background(), 
        betterauth.DeviceAuthRequest{
            Username: username,
            Password: string(password),
            ClientID: "your-cli-client-id",
            Scope:    "openid profile email offline_access",
        })
    if err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    return session, nil
}

func CLIValidateSession(authClient *betterauth.Client, session *betterauth.Session) error {
    // Refresh token if needed
    if session.IsExpired() {
        newSession, err := authClient.RefreshToken(context.Background(), 
            betterauth.RefreshRequest{
                RefreshToken: session.RefreshToken,
                ClientID:     "your-cli-client-id",
            })
        if err != nil {
            return fmt.Errorf("token refresh failed: %w", err)
        }
        *session = *newSession
    }
    return nil
}
```

## 4. Frontend Authentication (Next.js)

```javascript
// lib/auth.js
import { BetterAuth } from '@betterauth/sdk-js';

const betterAuth = new BetterAuth({
  clientId: process.env.NEXT_PUBLIC_BETTERAUTH_CLIENT_ID,
  domain: process.env.NEXT_PUBLIC_BETTERAUTH_DOMAIN,
  redirectUri: process.env.NEXT_PUBLIC_BETTERAUTH_CALLBACK_URL,
});

// Login redirect
export const login = async () => {
  await betterAuth.loginWithRedirect({
    authorizationParams: {
      scope: 'openid profile email offline_access',
    },
  });
};

// Handle callback
export const handleAuthCallback = async () => {
  await betterAuth.handleRedirectCallback();
};

// Get access token
export const getAccessToken = async () => {
  try {
    const token = await betterAuth.getTokenSilently();
    return token;
  } catch (err) {
    if (err.error === 'login_required') {
      await login();
    }
    throw err;
  }
};

// API route wrapper
export const withApiAuth = (handler) => async (req, res) => {
  try {
    const token = await getAccessToken();
    req.accessToken = token;
    return handler(req, res);
  } catch (err) {
    res.status(401).json({ error: 'Unauthorized' });
  }
};
```

## 5. Daemon-to-Daemon Authentication

```go
package auth

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "os"

    "github.com/betterauth/sdk-go"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

// MTLSConfig contains mTLS configuration
type MTLSConfig struct {
    CertFile string
    KeyFile  string
    CAFile   string
}

// NewServerTLSCredentials creates TLS credentials for servers
func NewServerTLSCredentials(cfg MTLSConfig) (grpc.ServerOption, error) {
    cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("loading key pair: %w", err)
    }

    caCert, err := os.ReadFile(cfg.CAFile)
    if err != nil {
        return nil, fmt.Errorf("reading CA cert: %w", err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to add CA cert to pool")
    }

    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientAuth:   tls.RequireAndVerifyClientCert,
        ClientCAs:    caCertPool,
        MinVersion:   tls.VersionTLS13,
    }

    return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

// NewClientTLSCredentials creates TLS credentials for clients
func NewClientTLSCredentials(cfg MTLSConfig) (grpc.DialOption, error) {
    cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("loading key pair: %w", err)
    }

    caCert, err := os.ReadFile(cfg.CAFile)
    if err != nil {
        return nil, fmt.Errorf("reading CA cert: %w", err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to add CA cert to pool")
    }

    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caCertPool,
        MinVersion:   tls.VersionTLS13,
    }

    return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}
```

## 6. Role-Based Access Control (RBAC)

```go
package auth

import (
    "context"
    "net/http"

    "github.com/betterauth/sdk-go"
)

// RBACMiddleware checks for required permissions
func RBACMiddleware(requiredPermission string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, ok := GetUserFromContext(r.Context())
            if !ok {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            hasPermission := false
            for _, role := range user.Roles {
                for _, perm := range role.Permissions {
                    if perm == requiredPermission {
                        hasPermission = true
                        break
                    }
                }
                if hasPermission {
                    break
                }
            }

            if !hasPermission {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Example usage:
// r.Use(auth.RBACMiddleware("daemon:write"))
```

## 7. Secure Communication Between Services

### gRPC Interceptor Chain
```go
package auth

import (
    "context"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// AuthUnaryInterceptor authenticates gRPC requests
func AuthUnaryInterceptor(authClient *betterauth.Client) grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        // Skip auth for certain methods
        if info.FullMethod == "/service.Auth/Login" {
            return handler(ctx, req)
        }

        // Extract token from metadata
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            return nil, status.Error(codes.Unauthenticated, "metadata missing")
        }

        authHeaders := md.Get("authorization")
        if len(authHeaders) == 0 {
            return nil, status.Error(codes.Unauthenticated, "authorization token missing")
        }

        token := strings.TrimPrefix(authHeaders[0], "Bearer ")
        if token == authHeaders[0] { // No Bearer prefix found
            return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
        }

        // Validate token
        user, err := authClient.ValidateToken(ctx, token)
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "invalid token")
        }

        // Add user to context
        newCtx := context.WithValue(ctx, userContextKey, user)
        return handler(newCtx, req)
    }
}
```

## 8. Token Management and Rotation

```go
package auth

import (
    "context"
    "sync"
    "time"

    "github.com/betterauth/sdk-go"
)

type TokenManager struct {
    authClient    *betterauth.Client
    currentToken  string
    refreshToken  string
    expiry        time.Time
    mutex         sync.RWMutex
    refreshBefore time.Duration
}

func NewTokenManager(authClient *betterauth.Client, initialToken, refreshToken string, expiry time.Time) *TokenManager {
    return &TokenManager{
        authClient:    authClient,
        currentToken:  initialToken,
        refreshToken:  refreshToken,
        expiry:        expiry,
        refreshBefore: 5 * time.Minute, // Refresh 5 minutes before expiry
    }
}

func (tm *TokenManager) GetToken(ctx context.Context) (string, error) {
    tm.mutex.RLock()
    if time.Now().Before(tm.expiry.Add(-tm.refreshBefore)) {
        defer tm.mutex.RUnlock()
        return tm.currentToken, nil
    }
    tm.mutex.RUnlock()

    return tm.refreshTokenIfNeeded(ctx)
}

func (tm *TokenManager) refreshTokenIfNeeded(ctx context.Context) (string, error) {
    tm.mutex.Lock()
    defer tm.mutex.Unlock()

    // Check again in case another goroutine refreshed
    if time.Now().Before(tm.expiry.Add(-tm.refreshBefore)) {
        return tm.currentToken, nil
    }

    // Perform refresh
    newToken, err := tm.authClient.RefreshToken(ctx, betterauth.RefreshRequest{
        RefreshToken: tm.refreshToken,
        ClientID:     tm.authClient.ClientID,
    })
    if err != nil {
        return "", fmt.Errorf("token refresh failed: %w", err)
    }

    tm.currentToken = newToken.AccessToken
    tm.refreshToken = newToken.RefreshToken
    tm.expiry = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)

    return tm.currentToken, nil
}
```

## 9. Security Headers for Frontend Communication

```go
package security

import "net/http"

func SecureHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Set security headers
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", 
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline' cdn.betterauth.com; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data:; "+
            "connect-src 'self' api.betterauth.com; "+
            "frame-ancestors 'none'; "+
            "form-action 'self'")
        
        // Remove server header
        w.Header().Del("Server")
        
        next.ServeHTTP(w, r)
    })
}
```

## 10. Audit Logging

```go
package audit

import (
    "context"
    "encoding/json"
    "time"

    "github.com/betterauth/sdk-go"
)

type AuditLogEntry struct {
    Timestamp   time.Time              `json:"timestamp"`
    Action      string                 `json:"action"`
    Principal   *betterauth.User       `json:"principal"`
    Target      string                 `json:"target"`
    Metadata    map[string]interface{} `json:"metadata"`
    Status      string                 `json:"status"`
    ClientIP    string                 `json:"client_ip"`
    UserAgent   string                 `json:"user_agent"`
}

type AuditLogger interface {
    Log(ctx context.Context, entry AuditLogEntry) error
}

func AuditMiddleware(logger AuditLogger, action string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Create a response writer that captures the status code
            rw := &responseWriter{ResponseWriter: w}
            
            // Get user from context
            user, _ := auth.GetUserFromContext(r.Context())
            
            // Process request
            next.ServeHTTP(rw, r)
            
            // Log the audit entry
            entry := AuditLogEntry{
                Timestamp: start,
                Action:    action,
                Principal: user,
                Target:    r.URL.Path,
                Metadata: map[string]interface{}{
                    "method": r.Method,
                    "params": r.URL.Query(),
                },
                Status:    http.StatusText(rw.status),
                ClientIP:  r.RemoteAddr,
                UserAgent: r.UserAgent(),
            }
            
            go logger.Log(r.Context(), entry) // Async logging
        })
    }
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(status int) {
    rw.status = status
    rw.ResponseWriter.WriteHeader(status)
}
```

## 11. Rate Limiting

```go
package security

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type IPRateLimiter struct {
    ips map[string]*rate.Limiter
    mu  sync.RWMutex
    r   rate.Limit
    b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
    return &IPRateLimiter{
        ips: make(map[string]*rate.Limiter),
        r:   r,
        b:   b,
    }
}

func (i *IPRateLimiter) AddIP(ip string) *rate.Limiter {
    i.mu.Lock()
    defer i.mu.Unlock()

    limiter := rate.NewLimiter(i.r, i.b)
    i.ips[ip] = limiter

    return limiter
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
    i.mu.RLock()
    limiter, exists := i.ips[ip]
    i.mu.RUnlock()

    if !exists {
        return i.AddIP(ip)
    }

    return limiter
}

func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := r.RemoteAddr
            
            limiter := limiter.GetLimiter(ip)
            if !limiter.Allow() {
                http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

## 12. Secrets Management

```go
package security

import (
    "context"
    "os"

    "github.com/hashicorp/vault/api"
)

type VaultSecretManager struct {
    client *api.Client
    mountPath string
}

func NewVaultSecretManager(addr, token, mountPath string) (*VaultSecretManager, error) {
    config := &api.Config{
        Address: addr,
    }
    
    client, err := api.NewClient(config)
    if err != nil {
        return nil, fmt.Errorf("creating vault client: %w", err)
    }
    
    client.SetToken(token)
    
    return &VaultSecretManager{
        client:    client,
        mountPath: mountPath,
    }, nil
}

func (v *VaultSecretManager) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
    secret, err := v.client.KVv2(v.mountPath).Get(ctx, path)
    if err != nil {
        return nil, fmt.Errorf("getting secret: %w", err)
    }
    
    return secret.Data, nil
}

// Fallback to environment variables if Vault is not available
func GetSecret(key string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    
    // Try Vault if configured
    if vaultClient != nil {
        secret, err := vaultClient.GetSecret(context.Background(), key)
        if err == nil {
            if val, ok := secret[key].(string); ok {
                return val
            }
        }
    }
    
    return ""
}
```

This comprehensive security implementation provides:
1. End-to-end authentication via BetterAuth
2. Secure service-to-service communication
3. Fine-grained access control
4. Comprehensive audit logging
5. Protection against common attacks
6. Proper secrets management
7. Rate limiting to prevent abuse

Each component can be customized based on your specific requirements while maintaining a high security standard across all parts of your system.
