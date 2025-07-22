package shared

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AgentType string

const (
	AgentDaemon    AgentType = "daemon"
	AgentCLI       AgentType = "cli"
	AgentDashboard AgentType = "dashboard"
)

type MessageType string

const (
	TypeCommand         MessageType = "command" // Command to execute
	TypeCommandResponse MessageType = "command_response"
	TypeStatus          MessageType = "status"   // Status update
	TypeResponse        MessageType = "response" // Response to a command
	TypeEvent           MessageType = "event"    // Event notification
	TypeLog             MessageType = "log"      // Log message
	TypeError           MessageType = "error"    // Error message
	TypeAuth            MessageType = "auth"     // Authentication message
	TypeStatusAck       MessageType = "status_ack"
	TypeAuthResponse    MessageType = "auth_response"
)

type AgentMessage struct {
	Source    AgentType       `json:"source"`
	Target    AgentType       `json:"target"`
	Type      MessageType     `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
	AgentID   string          `json:"agent_id"`
	Signature string          `json:"signature,omitempty"` // ECC signature of the message
}

// CommandPayload represents a command sent to an agent
type CommandPayload struct {
	Name string      `json:"name"`           // Command name (e.g., "restart", "deploy")
	Args []string    `json:"args,omitempty"` // Command arguments
	ID   string      `json:"id"`             // Unique command ID for tracking
	Meta interface{} `json:"meta,omitempty"` // Additional metadata
}

// StatusPayload represents an agent status update
type StatusPayload struct {
	Status  string                 `json:"status"`            // Current status (e.g., "healthy", "degraded")
	Metrics map[string]interface{} `json:"metrics,omitempty"` // System metrics
	Load    SystemLoad             `json:"load,omitempty"`    // System load information
}

// SystemLoad contains system load information
type SystemLoad struct {
	CPU    float64 `json:"cpu"`    // CPU usage percentage
	Memory float64 `json:"memory"` // Memory usage percentage
	Disk   float64 `json:"disk"`   // Disk usage percentage
}

// AuthPayload represents an authentication request
type AuthPayload struct {
	Token    string `json:"token"`              // Authentication token
	Version  string `json:"version"`            // Agent version
	Hostname string `json:"hostname,omitempty"` // Agent hostname
}

// EventPayload represents an event notification
type EventPayload struct {
	Type string      `json:"type"` // Event type (e.g., "deployment_started")
	Data interface{} `json:"data"` // Event-specific data
}

// ErrorPayload represents an error response
type ErrorPayload struct {
	Message string `json:"message"`           // Error message
	Code    int    `json:"code,omitempty"`    // Optional error code
	Details string `json:"details,omitempty"` // Additional error details
}

// Helper functions

// GetCurrentTimestamp returns the current Unix timestamp
func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// NewCommandMessage creates a new command message
func NewCommandMessage(agentID string, command CommandPayload) (AgentMessage, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return AgentMessage{}, err
	}

	return AgentMessage{
		Type:      TypeCommand,
		Timestamp: GetCurrentTimestamp(),
		AgentID:   agentID,
		Payload:   payload,
	}, nil
}

// NewStatusMessage creates a new status message
func NewStatusMessage(agentID string, status StatusPayload) (AgentMessage, error) {
	payload, err := json.Marshal(status)
	if err != nil {
		return AgentMessage{}, err
	}

	return AgentMessage{
		Type:      TypeStatus,
		Timestamp: GetCurrentTimestamp(),
		AgentID:   agentID,
		Payload:   payload,
	}, nil
}

// EncryptedEnv represents the encrypted environment variables
type EncryptedEnv struct {
	KeyID        string            `json:"key_id"`         // Daemon's key ID used for encryption
	EnvBlob      string            `json:"env_blob"`       // Base64 encoded encrypted full .env content
	Variables    map[string]string `json:"variables"`      // Map of encrypted individual variables
	Nonce        string            `json:"nonce"`          // Base64 encoded nonce used for encryption
	Timestamp    time.Time         `json:"timestamp"`      // When the payload was created
	CLIPublicKey string            `json:"cli_public_key"` // Base64 encoded CLI's ECDH public key
}

// RBAC Roles
const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleDeployer = "deployer"
	RoleReader   = "reader"
)

type Identity struct {
	Fingerprint string    `json:"fingerprint"` // SHA-256 of public key
	PublicKey   string    `json:"public_key"`  // Base64 encoded public key
	SignPublic  string    `json:"sign_public"` // Base64 encoded Ed25519 public key
	Role        string    `json:"role"`        // RBAC role (owner, admin, deployer, etc.)
	Email       string    `json:"email"`       // User email/identifier
	AddedBy     string    `json:"added_by"`    // Who added this identity
	CreatedAt   time.Time `json:"created_at"`  // When this identity was added
}

type Envelope struct {
	Payload   []byte `json:"payload"`   // JSON string of EncryptedEnv
	Signature string `json:"signature"` // Base64 encoded signature of the payload
}

// PublicKeyResponse is the response from the daemon's /public-key endpoint
type PublicKeyResponse struct {
	KeyID      string `json:"key_id"`      // Identifier for the key
	PublicKey  string `json:"public_key"`  // Base64 encoded ECDH public key
	SignPublic string `json:"sign_public"` // Base64 encoded Ed25519 public key
}

// TrustedKey represents a trusted daemon public key stored by the CLI
type TrustedKey struct {
	KeyID       string `json:"key_id"`
	PublicKey   string `json:"public_key"`
	SignPublic  string `json:"sign_public"`
	Fingerprint string `json:"fingerprint"`
}

// TrustStore is a collection of trusted keys
type TrustStore struct {
	Keys       []TrustedKey `json:"keys"`
	Identities []Identity
}

type AuditLogEntry struct {
	Action    string    `json:"action"`       // What happened
	Actor     string    `json:"actor"`        // Who did it (fingerprint)
	Target    string    `json:"target"`       // What was affected
	Timestamp time.Time `json:"timestamp"`    // When it happened
	Signature string    `json:"signature"`    // Signature of the action
	IP        string    `json:"ip,omitempty"` // Optional IP address
	Message   string    `json:"message"`      // Optional message or details:
	Client    string    `json:"client_id"`    // Client identifier (if applicable)
}

// EnvFile represents a parsed .env file
type EnvFile struct {
	Variables map[string]string
	Raw       []byte
}

func ParseEnvFile(content []byte) (*EnvFile, error) {
	// Simplified parser
	variables := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and components
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			SharedLogger.Error("Invalid line in .env file: %s", line)
			return nil, fmt.Errorf("invalid line in .env file: %s", line)

		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			SharedLogger.Error("Empty key or value in .env file: %s", line)
			return nil, fmt.Errorf("empty key or value in .env file: %s", line)
		}
		variables[key] = value
	}
	return &EnvFile{
		Variables: variables,
		Raw:       content,
	}, nil
}
