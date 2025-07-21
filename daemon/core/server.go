package core

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"nextdeploy/shared"
	"nextdeploy/shared/logger"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	corelogs = logger.PackageLogger("CORE", "CORE")
)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	CheckOrigin: func(r *http.Request) bool {
		// TODO: Do context aware origins using config.yml data and db data
		return true
	},
}

type WSServer struct {
	clients    map[*websocket.Conn]bool
	mu         sync.Mutex
	privateKey *ecdsa.PrivateKey
	agentID    string
}
type KeyExchangeMessage struct {
	PublicKey []byte `json:"public_key"`
	SessionID string `json:"session_id"`
	ClientID  string `json:"client_id"`
	Timestamp int64  `json:"timestamp"`
	Signature []byte `json:"signature,omitempty"`
}

func (s *WSServer) performKeyExchange(conn *websocket.Conn, clientID string) ([]byte, error) {
	// 1. Generate ephemeral key pair for this session
	sessionKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session key: %w", err)
	}

	// 2. Receive client's public key
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to read key exchange message: %w", err)
	}

	var clientKeyMsg KeyExchangeMessage
	if err := json.Unmarshal(msg, &clientKeyMsg); err != nil {
		return nil, fmt.Errorf("invalid key exchange message: %w", err)
	}

	// 3. Verify message timestamp (prevent replay)
	if time.Since(time.Unix(clientKeyMsg.Timestamp, 0)) > 5*time.Second {
		return nil, errors.New("key exchange message too old")
	}

	// 4. Verify signature if present
	if len(clientKeyMsg.Signature) > 0 {
		// In production, you would verify against known client public key
		// For simplicity, we're skipping this in the example
	}

	// 5. Parse client's public key
	clientPublic, err := ecdh.X25519().NewPublicKey(clientKeyMsg.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid client public key: %w", err)
	}

	// 6. Derive shared secret
	sharedSecret, err := sessionKey.ECDH(clientPublic)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	// 7. Send server's public key to client
	serverKeyMsg := KeyExchangeMessage{
		PublicKey: sessionKey.PublicKey().Bytes(),
		SessionID: shared.GenerateSessionID(),
		Timestamp: time.Now().Unix(),
	}

	msgBytes, err := json.Marshal(serverKeyMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key message: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		return nil, fmt.Errorf("failed to send server public key: %w", err)
	}

	// 8. Return derived key (should be hashed for additional security)
	//return shared.DeriveSessionKey(sharedSecret), nil
	return sharedSecret, nil
}

//	func (s *WSServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
//		// 1. Authenticate with JWT
//		token := r.URL.Query().Get("token")
//		if token == "" {
//			http.Error(w, "Authorization token required", http.StatusUnauthorized)
//			return
//		}
//
//		claims, err := shared.VerifyWSJWT(token, &s.privateKey.PublicKey)
//		if err != nil {
//			http.Error(w, "Invalid token", http.StatusUnauthorized)
//			return
//		}
//
//		// 2. Upgrade to WebSocket
//		conn, err := s.upgrader.Upgrade(w, r, nil)
//		if err != nil {
//			s.logger.Error("WebSocket upgrade failed", "error", err)
//			return
//		}
//		defer conn.Close()
//
//		// 3. Perform key exchange
//		sessionKey, err := s.performKeyExchange(conn, claims.ClientID)
//		if err != nil {
//			s.logger.Error("Key exchange failed", "error", err)
//			return
//		}
//
//		// 4. Start secure message loop
//		s.handleSecureMessages(conn, sessionKey, claims.ClientID)
//	}
//
//	func (s *WSServer) handleSecureMessages(conn *websocket.Conn, sessionKey []byte, clientID string) {
//		var sequence uint64
//
//		for {
//			// Read encrypted message
//			_, msg, err := conn.ReadMessage()
//			if err != nil {
//				s.logger.Error("Failed to read message", "client", clientID, "error", err)
//				return
//			}
//
//			// Decrypt message
//			plaintext, msgSeq, err := shared.DecryptMessage(sessionKey, msg)
//			if err != nil {
//				s.logger.Error("Decryption failed", "client", clientID, "error", err)
//				return
//			}
//
//			// Verify sequence
//			if msgSeq <= sequence {
//				s.logger.Warn("Invalid sequence number", "client", clientID,
//					"received", msgSeq, "expected", sequence+1)
//				return
//			}
//			sequence = msgSeq
//
//			// Process message
//			if err := s.processMessage(clientID, plaintext); err != nil {
//				s.logger.Error("Message processing failed", "client", clientID, "error", err)
//				return
//			}
//
//			// Send response if needed
//			response := map[string]interface{}{"status": "ack", "sequence": sequence}
//			encryptedResp, err := shared.EncryptMessage(sessionKey, sequence, response)
//			if err != nil {
//				s.logger.Error("Failed to encrypt response", "client", clientID, "error", err)
//				return
//			}
//
//			if err := conn.WriteMessage(websocket.BinaryMessage, encryptedResp); err != nil {
//				s.logger.Error("Failed to send response", "client", clientID, "error", err)
//				return
//			}
//		}
//	}
func NewWSServer(privateKey *ecdsa.PrivateKey, agentID string) *WSServer {
	return &WSServer{
		clients:    make(map[*websocket.Conn]bool),
		privateKey: privateKey,
		agentID:    agentID,
	}
}

func (s *WSServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		corelogs.Error("Failed to upgrade connection", "error", err)
		return
	}
	s.mu.Lock()
	s.clients[ws] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, ws)
		s.mu.Unlock()
		ws.Close()
	}()

	for {
		var msg shared.AgentMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			corelogs.Error("Failed to read JSON message", "error", err)
			break
		}
		corelogs.Info("Received message", "message", msg)

		// verify message signature
		if !verifyMessageSignature(msg) {
			corelogs.Error("Invalid message signature", "message", msg)
			break
		}

		// Process the message based on type
		switch msg.Type {
		case shared.TypeCommand:
			go s.handleCommand(ws, msg)
		case shared.TypeStatus:
			go s.handleStatus(ws, msg)
		case shared.TypeAuth:
			go s.handleAuth(ws, msg)
		case shared.TypeEvent:
			go s.handleEvent(ws, msg)
		}

	}

}

func (s *WSServer) Broadcast(message shared.AgentMessage) error {
	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for client := range s.clients {
		if err := client.WriteMessage(websocket.TextMessage, jsonMsg); err != nil {
			corelogs.Error("Failed to broadcast message to client", "error", err)
			client.Close()
			delete(s.clients, client)
		}
	}

	return nil
}

func (s *WSServer) handleCommand(ws *websocket.Conn, msg shared.AgentMessage) {
	// Validate command structure
	var cmd shared.CommandPayload
	if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
		corelogs.Error("Invalid command payload", "error", err)
		s.sendError(ws, "invalid command payload")
		return
	}

	// Process command based on cmd.Name and cmd.Args
	// This is just a skeleton - actual implementation would depend on your command set
	response := shared.AgentMessage{
		Type:      shared.TypeCommandResponse,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   s.agentID,
	}

	switch cmd.Name {
	case "ping":
		response.Payload = []byte(`{"status": "pong"}`)
	default:
		response.Payload = []byte(`{"error": "unknown command"}`)
	}

	// Sign and send response
	signedMsg, err := s.signMessage(response)
	if err != nil {
		corelogs.Error("Failed to sign command response", "error", err)
		return
	}

	if err := ws.WriteJSON(signedMsg); err != nil {
		corelogs.Error("Failed to send command response", "error", err)
	}
}

func (s *WSServer) handleStatus(ws *websocket.Conn, msg shared.AgentMessage) {
	var status shared.StatusPayload
	if err := json.Unmarshal(msg.Payload, &status); err != nil {
		corelogs.Error("Invalid status payload", "error", err)
		s.sendError(ws, "invalid status payload")
		return
	}

	// Process status update (store in memory, forward to DB, etc.)
	corelogs.Info("Received status update", "status", status.Status, "metrics", status.Metrics)

	// Acknowledge status receipt
	ack := shared.AgentMessage{
		Type:      shared.TypeStatusAck,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   s.agentID,
		Payload:   []byte(`{"received": true}`),
	}

	signedAck, err := s.signMessage(ack)
	if err != nil {
		corelogs.Error("Failed to sign status ack", "error", err)
		return
	}

	if err := ws.WriteJSON(signedAck); err != nil {
		corelogs.Error("Failed to send status ack", "error", err)
	}
}

func (s *WSServer) handleAuth(ws *websocket.Conn, msg shared.AgentMessage) {
	var auth shared.AuthPayload
	if err := json.Unmarshal(msg.Payload, &auth); err != nil {
		corelogs.Error("Invalid auth payload", "error", err)
		s.sendError(ws, "invalid auth payload")
		return
	}

	// Verify authentication token or credentials
	// This is a simplified example - use proper authentication in production
	if auth.Token != "expected-token" { // Replace with real auth logic
		corelogs.Error("Authentication failed", "agent", msg.AgentID)
		s.sendError(ws, "authentication failed")
		return
	}

	// Authentication successful
	corelogs.Info("Authentication successful", "agent", msg.AgentID)
	response := shared.AgentMessage{
		Type:      shared.TypeAuthResponse,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   s.agentID,
		Payload:   []byte(`{"authenticated": true}`),
	}

	signedResponse, err := s.signMessage(response)
	if err != nil {
		corelogs.Error("Failed to sign auth response", "error", err)
		return
	}

	if err := ws.WriteJSON(signedResponse); err != nil {
		corelogs.Error("Failed to send auth response", "error", err)
	}
}

func (s *WSServer) handleEvent(ws *websocket.Conn, msg shared.AgentMessage) {
	var event shared.EventPayload
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		corelogs.Error("Invalid event payload", "error", err)
		s.sendError(ws, "invalid event payload")
		return
	}

	// Process event based on event.Type and event.Data
	corelogs.Info("Received event", "type", event.Type, "data", event.Data)

	// Acknowledge event receipt
}

func (s *WSServer) sendError(ws *websocket.Conn, errorMsg string) {
	errMsg := shared.AgentMessage{
		Type:      shared.TypeError,
		Timestamp: shared.GetCurrentTimestamp(),
		AgentID:   s.agentID,
		Payload:   []byte(`{"error": "` + errorMsg + `"}`),
	}

	signedErr, err := s.signMessage(errMsg)
	if err != nil {
		corelogs.Error("Failed to sign error message", "error", err)
		return
	}

	if err := ws.WriteJSON(signedErr); err != nil {
		corelogs.Error("Failed to send error message", "error", err)
	}
}

func (s *WSServer) signMessage(msg shared.AgentMessage) (shared.AgentMessage, error) {
	// Create a copy of the message without the signature for signing
	msgToSign := msg
	msgToSign.Signature = ""

	msgBytes, err := json.Marshal(msgToSign)
	if err != nil {
		return shared.AgentMessage{}, err
	}

	hash := sha256.Sum256(msgBytes)
	r, sig, err := ecdsa.Sign(nil, s.privateKey, hash[:])
	if err != nil {
		return shared.AgentMessage{}, err
	}

	signature := append(r.Bytes(), sig.Bytes()...)
	msg.Signature = hex.EncodeToString(signature)
	return msg, nil
}

func verifyMessageSignature(msg shared.AgentMessage) bool {
	// Extract public key from message or use a pre-shared key
	// This is a simplified example - implement proper verification
	if msg.Signature == "" {
		return false
	}

	// In a real implementation, you would:
	// 1. Get the public key associated with the AgentID
	// 2. Verify the signature against the message content
	// 3. Return true only if verification succeeds

	return true // Placeholder - replace with actual verification
}
