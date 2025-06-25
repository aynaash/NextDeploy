# Caddy Web Server: Architecture and Programmatic Configuration via Go

## Introduction to Caddy

Caddy is a modern, open-source web server written in Go that emphasizes simplicity, security, and automatic HTTPS. It's known for its:

- Automatic HTTPS with Let's Encrypt
- Simple configuration with Caddyfile
- Extensible architecture
- High performance
- No dependencies (single binary)

## Caddy Architecture

### Core Components

1. **Server Types**: Caddy supports HTTP, HTTPS, HTTP/2, and HTTP/3 protocols
2. **Listeners**: Handles incoming connections
3. **Routes**: Manages request routing and middleware
4. **Apps**: Modular components that extend functionality (e.g., reverse proxy, file server)
5. **Admin API**: REST/JSON API for dynamic configuration
6. **Storage**: For certificates, configurations, and other persistent data

### Configuration Flow

1. **Startup**: Loads initial configuration from Caddyfile, JSON, or API
2. **Adaptation**: Converts configuration to native Caddy objects
3. **Validation**: Ensures configuration is correct
4. **Execution**: Applies configuration to running instance

## Programmatic Configuration with Go

Caddy is designed to be embedded and controlled programmatically in Go applications. Here's how to work with it:

### 1. Basic Programmatic Setup

```go
package main

import (
	"fmt"
	"log"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func main() {
	// Initialize Caddy
	caddy.AppName = "MyCaddyApp"
	caddy.AppVersion = "1.0.0"

	// Create a basic HTTP app configuration
	route := caddyhttp.Route{
		HandlersRaw: []json.RawMessage{
			caddyconfig.JSONModuleObject(
				caddyhttp.StaticResponse{
					StatusCode: caddyhttp.WeakString("200"),
					Body:       "Hello, world!",
				},
				"handler",
				"static_response",
				nil,
			),
		},
	}

	httpApp := caddyhttp.App{
		Servers: map[string]*caddyhttp.Server{
			"myserver": {
				Listen: []string{":8080"},
				Routes: caddyhttp.RouteList{route},
			},
		},
	}

	// Load the configuration
	cfg := &caddy.Config{
		AppsRaw: caddy.ModuleMap{
			"http": caddyconfig.JSON(httpApp, nil),
		},
	}

	// Start Caddy
	err := caddy.Run(cfg)
	if err != nil {
		log.Fatal(err)
	}
}
```

### 2. Using the Admin API

Caddy exposes a REST API (default: localhost:2019) for dynamic configuration:

#### Common API Endpoints:
- `GET /config/` - Get current config
- `POST /load` - Load a new config
- `POST /stop` - Stop the server
- `POST /ping` - Health check

#### Example API Usage in Go:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func updateCaddyConfig() error {
	newConfig := map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"myserver": map[string]interface{}{
						"listen": []string{":8080"},
						"routes": []map[string]interface{}{
							{
								"handle": []map[string]interface{}{
									{
										"handler": "static_response",
										"body":    "Updated via API!",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	configJSON, err := json.Marshal(newConfig)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		"http://localhost:2019/load",
		"application/json",
		bytes.NewBuffer(configJSON),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("API Response: %s\n", body)
	return nil
}
```

### 3. Dynamic Configuration Changes

You can modify the running configuration without restarting:

```go
func addNewRoute() error {
	// Get current config
	resp, err := http.Get("http://localhost:2019/config/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var currentConfig map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&currentConfig); err != nil {
		return err
	}

	// Modify config
	httpApp := currentConfig["apps"].(map[string]interface{})["http"].(map[string]interface{})
	server := httpApp["servers"].(map[string]interface{})["myserver"].(map[string]interface{})
	routes := server["routes"].([]interface{})

	newRoute := map[string]interface{}{
		"match": []map[string]interface{}{{"path": []string{"/api"}}},
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]interface{}{
					{"dial": "localhost:3000"},
				},
			},
		},
	}

	routes = append(routes, newRoute)
	server["routes"] = routes

	// Apply updated config
	return updateCaddyConfig(currentConfig)
}
```

## Advanced Topics

### 1. Custom Modules

You can extend Caddy by writing your own modules:

```go
package custommodule

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(Greeter{})
	httpcaddyfile.RegisterHandlerDirective("greet", parseCaddyfile)
}

type Greeter struct {
	Greeting string `json:"greeting,omitempty"`
}

func (Greeter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.greeter",
		New: func() caddy.Module { return new(Greeter) },
	}
}

func (g *Greeter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	fmt.Fprintf(w, "%s, %s!", g.Greeting, r.URL.Path[1:])
	return next.ServeHTTP(w, r)
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	g := new(Greeter)
	g.Greeting = "Hello"
	return g, nil
}
```

### 2. Configuration Management Patterns

1. **Incremental Changes**: Use the `/config/` endpoint with PATCH requests
2. **Configuration Versioning**: Store configs in version control
3. **Blue-Green Deployment**: Maintain two configs and switch between them

### 3. Error Handling and Logging

```go
// Set up custom logging
caddy.Log().SetFilter(map[caddy.LogLevel]bool{
	caddy.LogLevelDebug: true,
	caddy.LogLevelInfo:  true,
	caddy.LogLevelWarn:  true,
	caddy.LogLevelError: true,
	caddy.LogLevelPanic: true,
	caddy.LogLevelFatal: true,
})

// Handle configuration errors
func safeUpdate(config map[string]interface{}) error {
	// Validate config first
	resp, err := http.Post(
		"http://localhost:2019/validate",
		"application/json",
		bytes.NewBuffer(configJSON),
	)
	if err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("invalid config: %s", body)
	}
	
	// If validation passes, apply the config
	return updateCaddyConfig(config)
}
```

## Best Practices

1. **Use Configuration Management**: For production, consider tools like Ansible, Terraform, or Kubernetes to manage Caddy
2. **Monitor Changes**: Track configuration changes through the API
3. **Secure the Admin API**: Use TLS and authentication for the admin endpoint
4. **Graceful Reloads**: Use the API for zero-downtime configuration changes
5. **Backup Configurations**: Regularly export configurations via the API

## Conclusion

Caddy's architecture makes it uniquely suited for programmatic control, especially in Go applications. Its Admin API and modular design allow for dynamic, runtime configuration changes without server restarts. Whether embedding Caddy directly in your Go application or controlling it via the API, you have fine-grained control over web server behavior.

For more advanced use cases, explore Caddy's extensive module system and consider contributing custom modules to extend its functionality.
