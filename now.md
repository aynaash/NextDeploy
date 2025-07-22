

```go
package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"nextdeploy/shared"
	"nextdeploy/shared/websocket"
	"os"
	"time"
)

// ... [previous code remains the same until DaemonClient] ...

type DaemonClient struct {
	baseURL    string
	agentID    string
	privateKey *ecdsa.PrivateKey
	wsClient   *websocket.WSClient
	httpClient *http.Client
}

func NewDaemonClient(baseURL, agentID string, privateKey *ecdsa.PrivateKey) *DaemonClient {
	return &DaemonClient{
		baseURL:    baseURL,
		agentID:    agentID,
		privateKey: privateKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ConnectWebSocket establishes a WebSocket connection to the daemon
func (c *DaemonClient) ConnectWebSocket() error {
	wsURL := "ws" + strings.TrimPrefix(c.baseURL, "http") + "/ws"
	wsClient, err := websocket.NewWSClient(wsURL, c.agentID, c.privateKey)
	if err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}
	c.wsClient = wsClient
	return nil
}

// Common Commands ==============================================

// GetStatus retrieves the current status of the daemon
func (c *DaemonClient) GetStatus() (*shared.StatusPayload, error) {
	msg := shared.AgentMessage{
		Source:    shared.AgentCLI,
		Target:    shared.AgentDaemon,
		Type:      shared.TypeStatus,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   c.agentID,
	}

	signedMsg, err := shared.SignMessage(msg, c.privateKey)
	if err != nil {
		return nil, err
	}

	response, err := c.sendRequest(signedMsg)
	if err != nil {
		return nil, err
	}

	var status shared.StatusPayload
	if err := json.Unmarshal(response.Payload, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return &status, nil
}

// ExecuteCommand executes a command on the daemon
func (c *DaemonClient) ExecuteCommand(name string, args map[string]interface{}) (interface{}, error) {
	cmd := shared.CommandPayload{
		Name: name,
		ID:   shared.GenerateCommandID(),
		Meta: args, // Using Meta for complex args
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	msg := shared.AgentMessage{
		Source:    shared.AgentCLI,
		Target:    shared.AgentDaemon,
		Type:      shared.TypeCommand,
		Payload:   payload,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   c.agentID,
	}

	signedMsg, err := shared.SignMessage(msg, c.privateKey)
	if err != nil {
		return nil, err
	}

	return c.sendRequest(signedMsg)
}

// Deployment Commands ==========================================

// StartDeployment initiates a new deployment
func (c *DaemonClient) StartDeployment(repoURL, branch string, envVars map[string]string) (string, error) {
	args := map[string]interface{}{
		"repo_url": repoURL,
		"branch":   branch,
		"env_vars": envVars,
	}

	response, err := c.ExecuteCommand("deploy_start", args)
	if err != nil {
		return "", err
	}

	if respMap, ok := response.(map[string]interface{}); ok {
		if deploymentID, ok := respMap["deployment_id"].(string); ok {
			return deploymentID, nil
		}
	}

	return "", fmt.Errorf("invalid response format")
}

// GetDeploymentStatus checks the status of a deployment
func (c *DaemonClient) GetDeploymentStatus(deploymentID string) (map[string]interface{}, error) {
	response, err := c.ExecuteCommand("deploy_status", map[string]interface{}{
		"deployment_id": deploymentID,
	})
	if err != nil {
		return nil, err
	}

	if status, ok := response.(map[string]interface{}); ok {
		return status, nil
	}

	return nil, fmt.Errorf("invalid response format")
}

// System Commands =============================================

// RestartService restarts a system service
func (c *DaemonClient) RestartService(serviceName string) error {
	_, err := c.ExecuteCommand("service_restart", map[string]interface{}{
		"service": serviceName,
	})
	return err
}

// GetLogs retrieves logs from the daemon
func (c *DaemonClient) GetLogs(service string, lines int) ([]string, error) {
	response, err := c.ExecuteCommand("get_logs", map[string]interface{}{
		"service": service,
		"lines":   lines,
	})
	if err != nil {
		return nil, err
	}

	if logList, ok := response.([]interface{}); ok {
		var logs []string
		for _, entry := range logList {
			if logEntry, ok := entry.(string); ok {
				logs = append(logs, logEntry)
			}
		}
		return logs, nil
	}

	return nil, fmt.Errorf("invalid logs format")
}

// Configuration Commands =======================================

// UpdateConfig updates daemon configuration
func (c *DaemonClient) UpdateConfig(config map[string]interface{}) error {
	_, err := c.ExecuteCommand("config_update", config)
	return err
}

// GetConfig retrieves current configuration
func (c *DaemonClient) GetConfig() (map[string]interface{}, error) {
	response, err := c.ExecuteCommand("config_get", nil)
	if err != nil {
		return nil, err
	}

	if config, ok := response.(map[string]interface{}); ok {
		return config, nil
	}

	return nil, fmt.Errorf("invalid config format")
}

// Helper Methods ===============================================

func (c *DaemonClient) sendRequest(msg shared.AgentMessage) (shared.AgentMessage, error) {
	// Try WebSocket first if connected
	if c.wsClient != nil {
		if err := c.wsClient.SendMessage(msg); err == nil {
			// Implement proper response handling via channels
			return c.wsClient.WaitForResponse(msg)
		}
		// Fall through to HTTP on error
	}

	// Fallback to HTTP
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return shared.AgentMessage{}, err
	}

	resp, err := c.httpClient.Post(c.baseURL+"/command", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return shared.AgentMessage{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return shared.AgentMessage{}, err
	}

	var response shared.AgentMessage
	if err := json.Unmarshal(body, &response); err != nil {
		return shared.AgentMessage{}, err
	}

	if !shared.VerifyMessageSignature(response) {
		return shared.AgentMessage{}, fmt.Errorf("invalid response signature")
	}

	return response, nil
}

// ... [rest of your existing code] ...
```

### Added Commands Summary:

1. **Status Commands**:
   - `GetStatus()` - Gets system status and metrics

2. **Deployment Commands**:
   - `StartDeployment()` - Initiates a new deployment
   - `GetDeploymentStatus()` - Checks deployment progress

3. **System Commands**:
   - `RestartService()` - Restarts a service
   - `GetLogs()` - Retrieves service logs

4. **Configuration Commands**:
   - `UpdateConfig()` - Updates configuration
   - `GetConfig()` - Retrieves current config

5. **Core Improvements**:
   - Better WebSocket handling with connection management
   - Proper request/response pattern
   - Type-safe command execution
   - Better error handling

### Usage Example:

```go
func main() {
    client := NewDaemonClient("http://localhost:8080", "my-agent-id", privateKey)
    
    // Connect WebSocket if available
    if err := client.ConnectWebSocket(); err != nil {
        log.Printf("WebSocket not available, using HTTP: %v", err)
    }
    
    // Get system status
    status, err := client.GetStatus()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("CPU: %.2f%%, Memory: %.2f%%\n", status.Load.CPU, status.Load.Memory)
    
    // Start a deployment
    deploymentID, err := client.StartDeployment(
        "https://github.com/your/repo",
        "main",
        map[string]string{"ENV": "production"},
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Started deployment:", deploymentID)
}
```

This implementation provides a more compYou're right - I didn't implement the `waitForResponse` function in the previous example. Let me provide a complete implementation for handling WebSocket responses, including proper response waiting with timeout.

### Implementing Response Handling

First, let's enhance the `DaemonClient` struct to include response tracking:

```go
type DaemonClient struct {
    baseURL     string
    agentID     string
    privateKey  *ecdsa.PrivateKey
    wsClient    *websocket.WSClient
    httpClient  *http.Client
    responses   map[string]chan shared.AgentMessage // Track pending responses
    responseMux sync.Mutex                          // Protect concurrent access to responses
}

func NewDaemonClient(baseURL, agentID string, privateKey *ecdsa.PrivateKey) *DaemonClient {
    return &DaemonClient{
        baseURL:    baseURL,
        agentID:    agentID,
        privateKey: privateKey,
        httpClient: &http.Client{Timeout: 30 * time.Second},
        responses:  make(map[string]chan shared.AgentMessage),
    }
}
```

### WebSocket Response Handler

Add this method to handle incoming WebSocket messages:

```go
func (c *DaemonClient) handleWebSocketMessages() {
    if c.wsClient == nil {
        return
    }

    go func() {
        for {
            msg, err := c.wsClient.ReceiveMessage()
            if err != nil {
                log.Printf("WebSocket receive error: %v", err)
                continue
            }

            // Verify message signature
            if !shared.VerifyMessageSignature(msg) {
                log.Printf("Invalid message signature from %s", msg.AgentID)
                continue
            }

            // Handle command responses
            if msg.Type == shared.TypeCommandResponse {
                var cmdResp shared.CommandPayload
                if err := json.Unmarshal(msg.Payload, &cmdResp); err == nil {
                    c.responseMux.Lock()
                    if ch, ok := c.responses[cmdResp.ID]; ok {
                        ch <- msg
                        delete(c.responses, cmdResp.ID)
                    }
                    c.responseMux.Unlock()
                }
            }
        }
    }()
}
```

### Implement waitForResponse

Now implement the proper response waiting mechanism:

```go
func (c *DaemonClient) waitForResponse(commandID string, timeout time.Duration) (shared.AgentMessage, error) {
    // Create response channel
    respChan := make(chan shared.AgentMessage, 1)
    
    // Register the channel
    c.responseMux.Lock()
    c.responses[commandID] = respChan
    c.responseMux.Unlock()
    
    // Set up timeout
    timeoutChan := time.After(timeout)
    
    select {
    case response := <-respChan:
        return response, nil
    case <-timeoutChan:
        c.responseMux.Lock()
        delete(c.responses, commandID) // Clean up
        c.responseMux.Unlock()
        return shared.AgentMessage{}, fmt.Errorf("timeout waiting for response")
    }
}
```

### Updated SendCommand Method

Here's how to use it in your `SendCommand` method:

```go
func (c *DaemonClient) SendCommand(command string, args map[string]interface{}) (interface{}, error) {
    // Generate command ID
    commandID := shared.GenerateCommandID()
    
    // Create command payload
    cmdPayload := shared.CommandPayload{
        Name: command,
        Args: []string{}, // Can put string representations here if needed
        ID:   commandID,
        Meta: args,       // Complex args go in Meta
    }
    
    payload, err := json.Marshal(cmdPayload)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal command: %w", err)
    }
    
    msg := shared.AgentMessage{
        Source:    shared.AgentCLI,
        Target:    shared.AgentDaemon,
        Type:      shared.TypeCommand,
        Payload:   payload,
        Timestamp: shared.GetCurrentTimestamp(),
        AgentID:   c.agentID,
    }
    
    signedMsg, err := shared.SignMessage(msg, c.privateKey)
    if err != nil {
        return nil, err
    }
    
    // Try WebSocket first
    if c.wsClient != nil {
        if err := c.wsClient.SendMessage(signedMsg); err == nil {
            // Wait for response with 30 second timeout
            response, err := c.waitForResponse(commandID, 30*time.Second)
            if err != nil {
                return nil, fmt.Errorf("failed to get response: %w", err)
            }
            
            // Unmarshal the response payload
            var result interface{}
            if err := json.Unmarshal(response.Payload, &result); err != nil {
                return nil, fmt.Errorf("failed to unmarshal response: %w", err)
            }
            return result, nil
        }
        // Fall through to HTTP on error
    }
    
    // Fallback to HTTP
    return c.sendHTTPCommand(signedMsg)
}
```

### HTTP Fallback Method

And the HTTP version:

```go
func (c *DaemonClient) sendHTTPCommand(msg shared.AgentMessage) (interface{}, error) {
    jsonData, err := json.Marshal(msg)
    if err != nil {
        return nil, err
    }
    
    resp, err := c.httpClient.Post(c.baseURL+"/command", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("server returned %d", resp.StatusCode)
    }
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    var response shared.AgentMessage
    if err := json.Unmarshal(body, &response); err != nil {
        return nil, err
    }
    
    if !shared.VerifyMessageSignature(response) {
        return nil, fmt.Errorf("invalid response signature")
    }
    
    var result interface{}
    if err := json.Unmarshal(response.Payload, &result); err != nil {
        return nil, err
    }
    
    return result, nil
}
```

### Complete Initialization

Don't forget to start the WebSocket handler when connecting:

```go
func (c *DaemonClient) ConnectWebSocket() error {
    wsURL := "ws" + strings.TrimPrefix(c.baseURL, "http") + "/ws"
    wsClient, err := websocket.NewWSClient(wsURL, c.agentID, c.privateKey)
    if err != nil {
        return fmt.Errorf("failed to connect WebSocket: %w", err)
    }
    c.wsClient = wsClient
    c.handleWebSocketMessages() // Start listening for responses
    return nil
}
```

### Key Improvements:

1. **Proper Response Tracking**: Uses a map of channels to track pending requests
2. **Timeout Handling**: Prevents hanging forever on lost responses
3. **Thread Safety**: Uses mutex to protect concurrent access to the responses map
4. **Clean Error Handling**: Provides clear error messages at each failure point
5. **Consistent Interface**: Returns the same type of response whether using WebSocket or HTTP

This implementation provides a robust way to handle command responses with proper timeout and error handling, whether using WebSocket or falling back to HTTP.lete set of commands while maintaining type safety and consistency with your shared package definitions. You can easily extend it with more commands as needed.hese changes will align your implementation with the types defined in the shared package while maintaining the functionality you need.
You're right - I need to provide the implementation for `GenerateCommandID`. Here's how to properly generate unique command IDs for tracking requests and responses:

### Implementing GenerateCommandID

Add this to your `shared` package (or wherever you keep your utility functions):

```go
package shared

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateCommandID creates a unique ID for command tracking
func GenerateCommandID() string {
	b := make([]byte, 16) // 128-bit ID
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
```

### Alternative Implementations

If you want different behavior, here are some alternatives:

1. **UUID Version 4** (if you have access to a UUID library):
```go
import "github.com/google/uuid"

func GenerateCommandID() string {
	return uuid.New().String()
}
```

2. **Timestamp-based** (simple but less collision-resistant):
```go
func GenerateCommandID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
```

3. **ULID** (time-sortable unique IDs):
```go
import "github.com/oklog/ulid"

func GenerateCommandID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
```

### Updating the DaemonClient

Make sure your command-sending methods use this ID:

```go
func (c *DaemonClient) SendCommand(command string, args map[string]interface{}) (interface{}, error) {
    commandID := shared.GenerateCommandID() // Use the generated ID
    
    cmdPayload := shared.CommandPayload{
        Name: command,
        ID:   commandID,  // Include the ID
        Meta: args,       // Put complex args in Meta
    }
    
    // ... rest of the implementation ...
}
```

### Why This Matters

1. **Request Tracking**: Each command gets a unique ID for matching requests to responses
2. **Idempotency**: Helps prevent duplicate processing of the same command
3. **Debugging**: Makes it easier to trace command execution through logs
4. **WebSocket Correlation**: Essential for matching responses to the right request in async communication

### Complete Integration Example

Here's how it fits into the complete flow:

```go
// Client sends a command
func (c *DaemonClient) RestartService(service string) error {
    commandID := shared.GenerateCommandID()
    
    _, err := c.SendCommand("service_restart", map[string]interface{}{
        "service": service,
        "_cid":    commandID, // optional extra tracking
    })
    
    return err
}

// Server-side handling would then include this ID in the response
func handleCommand(cmd shared.CommandPayload) shared.AgentMessage {
    response := map[string]interface{}{
        "status":      "success",
        "command_id":  cmd.ID, // Echo back the ID
        "processed_at": time.Now(),
    }
    
    payload, _ := json.Marshal(response)
    
    return shared.AgentMessage{
        Type:      shared.TypeCommandResponse,
        Payload:   payload,
        Timestamp: shared.GetCurrentTimestamp(),
        // ... other fields ...
    }
}
```

This implementation gives you reliable command tracking whether you're using WebSockets or HTTP as the transport layer. The random-based generation ensures uniqueness even if multiple clients are sending commands simultaneously.
