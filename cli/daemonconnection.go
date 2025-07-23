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
	"nextdeploy/shared/websocket"
	"os"
	"strings"
	"sync"
	"time"

	"nextdeploy/shared"
)

var (
	daemonAddr = flag.String("daemon", "http://localhost:8080", "Daemon address")
	envFile    = flag.String("env", ".env", "Path to .env file")
	trustStore = flag.String("trust-store", ".trusted_keys.json", "Path to trust store")
)

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
		wsClient:   nil, // Initialize websocket client if needed
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Set a timeout for HTTP requests
		},
		responses: make(map[string]chan shared.AgentMessage),
	}
}

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
func (c *DaemonClient) SendCommand(command string, args map[string]interface{}) (interface{}, error) {
	// Convert args to JSON string slice
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	// Create proper command payload
	cmdPayload := shared.CommandPayload{
		Name: command,
		Args: []string{string(argsBytes)}, // Or parse args into proper string slice
		ID:   shared.GenerateCommandID(),  // You'll need to implement this
	}

	payloadBytes, err := json.Marshal(cmdPayload)
	if err != nil {
		return nil, err
	}

	// Try WebSocket first if connected
	if c.wsClient != nil {
		msg := shared.AgentMessage{
			Source:    shared.AgentCLI,
			Target:    shared.AgentDaemon,
			Type:      shared.TypeCommand,
			Payload:   payloadBytes,
			Timestamp: shared.GetCurrentTimestamp(), // Use the shared package function
			AgentID:   c.agentID,
		}

		signedMsg, err := shared.SignMessage(msg, c.privateKey)
		if err != nil {
			return nil, err
		}

		if err := c.wsClient.SendMessage(signedMsg); err != nil {
			// Fallback to HTTP
			return c.sendHTTPCommand(command, args)
		}

		// Wait for response
		d, e := c.waitForResponse(msg.AgentID, 30*time.Second) // Wait for up to 30 seconds

		return d.Payload, e
	}

	// Fallback to HTTP
	return c.sendHTTPCommand(command, args)
}

func (c *DaemonClient) sendHTTPCommand(command string, args map[string]interface{}) (interface{}, error) {
	// Convert args to JSON string slice
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	// Create proper command payload
	cmdPayload := shared.CommandPayload{
		Name: command,
		Args: []string{string(argsBytes)}, // Or parse args into proper string slice
		ID:   shared.GenerateCommandID(),  // You'll need to implement this
	}

	payloadBytes, err := json.Marshal(cmdPayload)
	if err != nil {
		return nil, err
	}

	msg := shared.AgentMessage{
		Source:    shared.AgentCLI,
		Target:    shared.AgentDaemon,
		Type:      shared.TypeCommand,
		Payload:   payloadBytes,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   c.agentID,
	}

	signedMsg, err := shared.SignMessage(msg, c.privateKey)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(signedMsg)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.baseURL+"/exec", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

	// Unmarshal the payload to get the actual response
	var responsePayload map[string]interface{}
	if err := json.Unmarshal(response.Payload, &responsePayload); err != nil {
		return nil, err
	}

	return responsePayload, nil
}
func (c *DaemonClient) ConnectWebSocket() error {
	wsURL := "ws" + strings.TrimPrefix(c.baseURL, "http") + "/ws"
	fmt.Printf("Connecting to WebSocket at %s\n", wsURL)
	wsClient := websocket.NewWSClient(c.agentID, c.privateKey)
	c.wsClient = wsClient
	c.handleWebSocketMessages() // Start listening for responses
	return nil
}
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

func (c *DaemonClient) DisconnectWebSocket() error {
	if c.wsClient != nil {
		if err := c.wsClient.Close(); err != nil {
			return fmt.Errorf("failed to close websocket connection: %w", err)
		}
		c.wsClient = nil
	}
	return nil
}

func (c *DaemonClient) GetStatus() (*shared.StatusPayload, error) {
	msg := shared.AgentMessage{
		Source:    shared.AgentCLI,
		Target:    shared.AgentDaemon,
		Type:      shared.TypeStatus,
		Payload:   nil, // No payload needed for GetStatus()
		Timestamp: time.Now().Unix(),
		AgentID:   c.agentID,
	}

	signedMsg, err := shared.SignMessage(msg, c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	response, err := c.sendRequest(signedMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to send status request: %w", err)
	}
	var status shared.StatusPayload
	if err := json.Unmarshal(response.Payload, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status response: %w", err)
	}

	return &status, nil
}

// Execute command on the daemon
func (c *DaemonClient) ExecuteCommand(name string, args map[string]interface{}) (interface{}, error) {
	cmd := shared.CommandPayload{
		Name: name,
		ID:   shared.GenerateCommandID(),
		Meta: args,
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command payload: %w", err)
	}
	msg := shared.AgentMessage{
		Source:    shared.AgentCLI,
		Target:    shared.AgentDaemon,
		Type:      shared.TypeCommand,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
		AgentID:   c.agentID,
	}

	signedMsg, err := shared.SignMessage(msg, c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return c.sendRequest(signedMsg)
}

// Get Deployment Status
func (c *DaemonClient) GetDeploymentStatus(deploymentID string) (map[string]interface{}, error) {
	response, err := c.ExecuteCommand("deploy_status", map[string]interface{}{
		"deployment_id": deploymentID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	status, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", response)
	}
	return status, nil
}

// restart restarts a system service
func (c *DaemonClient) RestartService(serviceName string) error {
	_, err := c.ExecuteCommand("restart_service", map[string]interface{}{
		"service_name": serviceName,
	})
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}
	return nil
}
func (c *DaemonClient) GetLogs(service string, lines int) ([]string, error) {
	response, err := c.ExecuteCommand("get_logs", map[string]interface{}{
		"service": service,
		"lines":   lines,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for service %s: %w", service, err)
	}
	if logList, ok := response.([]interface{}); ok {
		var logs []string
		for _, entry := range logList {
			if logEntry, ok := entry.(string); ok {
				logs = append(logs, logEntry)
			} else {
				return nil, fmt.Errorf("unexpected log entry type: %T", entry)
			}
		}
		return logs, nil
	}
	return nil, fmt.Errorf("unexpected response type: %T", response)
}

func (c *DaemonClient) GetConfig() (map[string]interface{}, error) {
	response, err := c.ExecuteCommand("get_config", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	config, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", response)
	}
	return config, nil
}
func (c *DaemonClient) sendRequest(msg shared.AgentMessage) (shared.AgentMessage, error) {
	if c.wsClient != nil {
		if err := c.wsClient.SendMessage(msg); err != nil {
			// Implement proper response handling via channels
			d, err := c.waitForResponse(msg.AgentID, 30*time.Second) // Wait for up to 30 seconds
			if err != nil {
				return shared.AgentMessage{}, fmt.Errorf("failed to send message via websocket: %w", err)
			}
			return d, nil
		}
		// fall to http if websocket is not connected
	}
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return shared.AgentMessage{}, fmt.Errorf("failed to marshal message: %w", err)
	}
	resp, err := c.httpClient.Post(c.baseURL+"/exec", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return shared.AgentMessage{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return shared.AgentMessage{}, fmt.Errorf("failed to read response body: %w", err)
	}
	var response shared.AgentMessage
	if err := json.Unmarshal(body, &response); err != nil {
		return shared.AgentMessage{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if !shared.VerifyMessageSignature(response) {
		return shared.AgentMessage{}, fmt.Errorf("invalid response signature")
	}
	return response, nil
}

func DaemonConnection() {
	flag.Parse()

	// Load or initialize trust store
	store, err := LoadTrustStore(*trustStore)
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}

	// Fetch daemon's public key
	daemonKey, err := FetchDaemonPublicKey(*daemonAddr, store)
	if err != nil {
		log.Fatalf("Failed to get daemon public key: %v", err)
	}

	// Read and parse the .env file
	envContent, err := os.ReadFile(*envFile)
	if err != nil {
		log.Fatalf("Failed to read .env file: %v", err)
	}

	env, err := shared.ParseEnvFile(envContent)
	if err != nil {
		log.Fatalf("Failed to parse .env file: %v", err)
	}

	// Encrypt and send the environment
	bytes, err := EncryptAndSendEnv(env, daemonKey, *daemonAddr)
	fmt.Printf("The bytes are :%v\n", bytes)
	if err != nil {
		log.Fatalf("Failed to send environment: %v", err)
	}

	log.Println("Environment successfully sent to daemon")
}

// func main() {
//     client := NewDaemonClient("http://localhost:8080", "my-agent-id", privateKey)
//
//     // Connect WebSocket if available
//     if err := client.ConnectWebSocket(); err != nil {
//         log.Printf("WebSocket not available, using HTTP: %v", err)
//     }
//
//     // Get system status
//     status, err := client.GetStatus()
//     if err != nil {
//         log.Fatal(err)
//     }
//     fmt.Printf("CPU: %.2f%%, Memory: %.2f%%\n", status.Load.CPU, status.Load.Memory)
//
//     // Start a deployment
//     deploymentID, err := client.StartDeployment(
//         "https://github.com/your/repo",
//         "main",
//         map[string]string{"ENV": "production"},
//     )
//     if err != nil {
//         log.Fatal(err)
//     }
//     fmt.Println("Started deployment:", deploymentID)
// }
//
//func (c *DaemonClient) handleWebSocketMessages() {
// 	if c.wsClient == nil {
// 		return // No WebSocket client to handle messages
// 	}
// 	go func() {
// 		for {
// 			msg, err := c.wsClient.ReceiveMessage()
// 			if err != nil {
// 				log.Printf("Error receiving WebSocket message: %v", err)
// 				continue
// 			}
// 			// verify message signature
// 			if !shared.VerifyMessageSignature(msg) {
// 				log.Printf("Invalid message signature: %s", msg.ID)
// 				continue
// 			}
//
// 			// Handle command responses
// 			if msg.Type == shared.TypeCommandResponse {
// 				var cmdResp shared.CommandPayload
// 				if err := json.Unmarshal(msg.Payload, &cmdResp); err != nil {
// 					c.responseMux.Lock()
// 					if ch, ok := c.responses[cmdResp.ID]; ok {
// 						ch < msg
// 						delete(c.responses, cmdResp.ID)
//
// 					}
// 					c.responseMux.Unlock()
// 				}
// 			}
// 		}
// 	}()
// }
