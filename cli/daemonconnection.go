package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"nextdeploy/shared/websocket"
	"os"
	"time"

	"nextdeploy/shared"
)

var (
	daemonAddr = flag.String("daemon", "http://localhost:8080", "Daemon address")
	envFile    = flag.String("env", ".env", "Path to .env file")
	trustStore = flag.String("trust-store", ".trusted_keys.json", "Path to trust store")
)

type DaemonClient struct {
	baseURL    string
	agentID    string
	privateKey *ecdsa.PrivateKey
	wsClient   *websocket.WSClient
}

func NewDaemonClient(baseURL, agentID string, privateKey *ecdsa.PrivateKey) *DaemonClient {
	return &DaemonClient{
		baseURL:    baseURL,
		agentID:    agentID,
		privateKey: privateKey,
	}
}
func (c *DaemonClient) SendCommand(command string, args map[string]interface{}) (interface{}, error) {
	// Try WebSocket first if connected
	if c.wsClient != nil {
		msg := shared.AgentMessage{
			Source:    shared.AgentCLI,
			Target:    shared.AgentDaemon,
			Type:      shared.TypeCommand,
			Payload:   map[string]interface{}{"command": command, "args": args},
			Timestamp: time.Now(),
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
		// (would need response channel implementation)
		return waitForResponse()
	}

	// Fallback to HTTP
	return c.sendHTTPCommand(command, args)
}

func (c *DaemonClient) sendHTTPCommand(command string, args map[string]interface{}) (interface{}, error) {
	msg := shared.AgentMessage{
		Source:    shared.AgentCLI,
		Target:    shared.AgentDaemon,
		Type:      shared.TypeCommand,
		Payload:   map[string]interface{}{"command": command, "args": args},
		Timestamp: time.Now(),
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

	return response.Payload, nil
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
	if err := EncryptAndSendEnv(env, daemonKey, *daemonAddr); err != nil {
		log.Fatalf("Failed to send environment: %v", err)
	}

	log.Println("Environment successfully sent to daemon")
}
