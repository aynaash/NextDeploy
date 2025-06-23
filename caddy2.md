# Programmatically Manipulating Caddy Configs Using the Caddy API with Go

Caddy provides a powerful admin API that allows you to dynamically configure and manage your web server. Here's how to interact with the Caddy API using Go.

## Prerequisites

1. Caddy server running with admin API enabled (usually on `localhost:2019`)
2. Go installed on your system

## Basic Setup

First, create a new Go module and install the required Caddy packages:

```bash
go mod init caddyapi
go get github.com/caddyserver/caddy/v2
```

## Connecting to the Caddy API

Here's a basic client setup:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const caddyAdminAPI = "http://localhost:2019"

type CaddyClient struct {
	adminAPI string
	client   *http.Client
}

func NewCaddyClient(adminAPI string) *CaddyClient {
	return &CaddyClient{
		adminAPI: adminAPI,
		client:   &http.Client{},
	}
}

func (c *CaddyClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.adminAPI+path, body)
	if err != nil {
		return nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return c.client.Do(req)
}
```

## Common Operations

### 1. Loading a New Configuration

```go
func (c *CaddyClient) LoadConfig(ctx context.Context, config interface{}) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(ctx, "POST", "/load", bytes.NewReader(configBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to load config: %s", string(body))
	}

	return nil
}
```

### 2. Getting the Current Configuration

```go
func (c *CaddyClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/config/", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}
```

### 3. Adding a New Route

```go
func (c *CaddyClient) AddRoute(ctx context.Context, route interface{}) error {
	routeBytes, err := json.Marshal(route)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(ctx, "POST", "/config/apps/http/servers/srv0/routes", bytes.NewReader(routeBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add route: %s", string(body))
	}

	return nil
}
```

### 4. Updating a Route

```go
func (c *CaddyClient) UpdateRoute(ctx context.Context, routeID string, route interface{}) error {
	routeBytes, err := json.Marshal(route)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(ctx, "PATCH", "/config/apps/http/servers/srv0/routes/"+routeID, bytes.NewReader(routeBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update route: %s", string(body))
	}

	return nil
}
```

### 5. Deleting a Route

```go
func (c *CaddyClient) DeleteRoute(ctx context.Context, routeID string) error {
	resp, err := c.doRequest(ctx, "DELETE", "/config/apps/http/servers/srv0/routes/"+routeID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete route: %s", string(body))
	}

	return nil
}
```

## Example Usage

Here's how you might use these functions:

```go
func main() {
	ctx := context.Background()
	client := NewCaddyClient(caddyAdminAPI)

	// Example: Add a new route
	newRoute := map[string]interface{}{
		"@id": "my-route",
		"match": []map[string]interface{}{{
			"host": []string{"example.com"},
		}},
		"handle": []map[string]interface{}{{
			"handler": "static_response",
			"body":    "Hello from Caddy!",
		}},
	}

	err := client.AddRoute(ctx, newRoute)
	if err != nil {
		fmt.Printf("Error adding route: %v\n", err)
		return
	}
	fmt.Println("Route added successfully")

	// Example: Get current config
	config, err := client.GetConfig(ctx)
	if err != nil {
		fmt.Printf("Error getting config: %v\n", err)
		return
	}
	fmt.Printf("Current config: %+v\n", config)
}
```

## Advanced Configuration

For more complex configurations, you might want to use Caddy's native types:

```go
import "github.com/caddyserver/caddy/v2/modules/caddyhttp"

// Using Caddy's native types for better type safety
type Route struct {
	ID      string                   `json:"@id,omitempty"`
	Match   []caddyhttp.Match        `json:"match,omitempty"`
	Handler []map[string]interface{} `json:"handle,omitempty"`
}

func AddTypedRoute(ctx context.Context, client *CaddyClient) error {
	route := Route{
		ID: "api-route",
		Match: []caddyhttp.Match{{
			Host: []string{"api.example.com"},
			Path: []string{"/v1/*"},
		}},
		Handler: []map[string]interface{}{{
			"handler": "reverse_proxy",
			"upstreams": []map[string]interface{}{{
				"dial": "localhost:8080",
			}},
		}},
	}

	return client.AddRoute(ctx, route)
}
```

## Error Handling and Best Practices

1. Always check response status codes
2. Use proper context for request timeouts
3. Close response bodies to prevent resource leaks
4. Validate configurations before sending them to Caddy
5. Consider using Caddy's JSON schema for validation

## Conclusion

The Caddy API provides a powerful way to dynamically configure your web server. With Go, you can easily build automation tools, deployment pipelines, or dynamic configuration systems that interact with Caddy in real-time.

For more advanced use cases, refer to the [official Caddy documentation](https://caddyserver.com/docs/api) which provides comprehensive details about all available API endpoints and configuration options.

# Automated Caddy Reverse Proxy Tool for Containers

Here's a complete Go tool that automates exposing a container running on a VPS via Caddy's reverse proxy when given a domain name. This tool will:
1. Configure Caddy to serve the domain with HTTPS (automatic Let's Encrypt)
2. Set up a reverse proxy to your container
3. Handle basic error cases

## Complete Implementation

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	caddyAdminAPI = "http://localhost:2019"
	defaultEmail  = "admin@example.com" // For Let's Encrypt
)

type CaddyClient struct {
	adminAPI string
	client   *http.Client
}

func NewCaddyClient(adminAPI string) *CaddyClient {
	return &CaddyClient{
		adminAPI: adminAPI,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CaddyClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.adminAPI+path, body)
	if err != nil {
		return nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return c.client.Do(req)
}

// ConfigureDomain sets up a new domain with reverse proxy to container
func (c *CaddyClient) ConfigureDomain(ctx context.Context, domain, containerPort, email string) error {
	if domain == "" || containerPort == "" {
		return errors.New("domain and container port are required")
	}

	if email == "" {
		email = defaultEmail
	}

	// Caddy configuration structure
	config := map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"srv0": map[string]interface{}{
						"listen": []string{":80", ":443"},
						"routes": []map[string]interface{}{
							{
								"match": []map[string]interface{}{{
									"host": []string{domain},
								}},
								"handle": []map[string]interface{}{{
									"handler": "subroute",
									"routes": []map[string]interface{}{
										{
											"handle": []map[string]interface{}{{
												"handler": "reverse_proxy",
												"upstreams": []map[string]interface{}{{
													"dial": fmt.Sprintf("localhost:%s", containerPort),
												}},
											}},
										},
									},
								}},
								"terminal": true,
							},
						},
						"automatic_https": map[string]interface{}{
							"disable": false,
						},
					},
				},
			},
			"tls": map[string]interface{}{
				"automation": map[string]interface{}{
					"policies": []map[string]interface{}{{
						"subjects": []string{domain},
						"issuers": []map[string]interface{}{
							{
								"module": "acme",
								"email":  email,
							},
						},
					}},
				},
			},
		},
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/load", bytes.NewReader(configBytes))
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to configure domain (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// VerifyDomain checks if the domain is properly configured
func (c *CaddyClient) VerifyDomain(ctx context.Context, domain string) (bool, error) {
	resp, err := c.doRequest(ctx, "GET", "/config/apps/http/servers/srv0/routes", nil)
	if err != nil {
		return false, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var routes []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, route := range routes {
		if matches, ok := route["match"].([]interface{}); ok {
			for _, match := range matches {
				if hosts, ok := match.(map[string]interface{})["host"].([]interface{}); ok {
					for _, host := range hosts {
						if host == domain {
							return true, nil
						}
					}
				}
			}
		}
	}

	return false, nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: caddy-proxy <domain> <container-port> [email]")
		fmt.Println("Example: caddy-proxy example.com 8080 admin@example.com")
		os.Exit(1)
	}

	domain := os.Args[1]
	containerPort := os.Args[2]
	email := ""
	if len(os.Args) > 3 {
		email = os.Args[3]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := NewCaddyClient(caddyAdminAPI)

	fmt.Printf("Configuring domain %s to proxy to localhost:%s...\n", domain, containerPort)
	if err := client.ConfigureDomain(ctx, domain, containerPort, email); err != nil {
		fmt.Printf("Error configuring domain: %v\n", err)
		os.Exit(1)
	}

	// Give Caddy a moment to process the configuration
	time.Sleep(2 * time.Second)

	// Verify the configuration
	configured, err := client.VerifyDomain(ctx, domain)
	if err != nil {
		fmt.Printf("Error verifying configuration: %v\n", err)
		os.Exit(1)
	}

	if configured {
		fmt.Printf("Success! %s is now proxying to your container on port %s\n", domain, containerPort)
		fmt.Printf("HTTPS was automatically configured with Let's Encrypt\n")
	} else {
		fmt.Println("Configuration appears to have failed - please check Caddy logs")
		os.Exit(1)
	}
}
```

## How to Use This Tool

1. **Build the tool**:
   ```bash
   go build -o caddy-proxy
   ```

2. **Run it with your domain and container port**:
   ```bash
   sudo ./caddy-proxy example.com 8080 your-email@example.com
   ```

3. **Example output**:
   ```
   Configuring domain example.com to proxy to localhost:8080...
   Success! example.com is now proxying to your container on port 8080
   HTTPS was automatically configured with Let's Encrypt
   ```

## Requirements

1. Caddy v2+ installed and running with admin API enabled
2. Proper DNS records pointing to your VPS
3. Open ports 80 and 443 on your VPS firewall
4. The container must be running and accessible on the specified port

## Features

- Automatic HTTPS with Let's Encrypt
- Simple command-line interface
- Configuration verification
- Timeout handling
- Proper error reporting

## Extending the Tool

You could enhance this tool with:

1. Support for multiple domains
2. Basic authentication
3. Rate limiting configuration
4. Health checks for the container
5. Logging configuration
6. Support for Caddy clusters

This tool provides a solid foundation for automating Caddy configurations when deploying containers on your VPS.
