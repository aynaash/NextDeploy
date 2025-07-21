package websocket

import (
	"crypto/ecdsa"
	"errors"
	"net/http"
	"nextdeploy/shared/logger"
	"time"

	"github.com/gorilla/websocket"
)

var (
	websockerlogger = logger.PackageLogger("WEBSOCKET", "websocket")
)

func NewWSClient(agentID string, privateKey *ecdsa.PrivateKey) *WSClient {
	return &WSClient{
		agentID:    agentID,
		privateKey: privateKey,
		pingPeriod: 15 * time.Second,
		writeWait:  10 * time.Second,
		pongWait:   20 * time.Second,
	}
}

func (c *WSClient) Connect(url string, headers http.Header) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		websockerlogger.Error("Failed to connect to WebSocket server", "error", err)
		return err
	}

	c.conn = conn
	c.connected = true

	// Start reader/writer goroutines
	go c.readPump()
	go c.writePump()

	return nil

}
func (c *WSClient) SendMessage(msg interface{}) error {

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		websockerlogger.Error("WebSocket client is not connected")
		return errors.New("websocket client is not connected")
	}

	return c.conn.WriteJSON(msg)

}

func (c *WSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		websockerlogger.Error("WebSocket client is not connected")
		return errors.New("websocket client is not connected")
	}

	err := c.conn.Close()
	if err != nil {
		websockerlogger.Error("Failed to close WebSocket connection", "error", err)
		return err
	}

	c.connected = false
	return nil
}

func (c *WSClient) readPump() {
	defer c.Close()

	c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				websockerlogger.Error("WebSocket read error", "error", err)
			}
			return
		}
		websockerlogger.Info("Received message from WebSocket server", "message", string(message))
	}

}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				websockerlogger.Error("Failed to send ping", "error", err)
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()

		default:
			time.Sleep(1 * time.Second) // Avoid busy waiting
		}
	}
}
