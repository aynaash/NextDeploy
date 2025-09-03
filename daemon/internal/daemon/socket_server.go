package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"nextdeploy/daemon/internal/types"
	"os"
	"path/filepath"
	"time"
)

type SocketServer struct {
	socketPath     string
	listener       net.Listener
	commandHandler *CommandHandler
}

func NewSocketServer(socketPath string, commandHandler *CommandHandler) *SocketServer {
	return &SocketServer{
		socketPath:     socketPath,
		commandHandler: commandHandler,
	}
}

func (ss *SocketServer) Start() error {
	// remove existing scoket file if it exists
	ss.cleanupSocket()

	listener, err := net.Listen("unix", ss.socketPath)
	if err != nil {
		return err
	}
	ss.listener = listener

	return ss.setSocketPermissions()
}

func (ss *SocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd types.Command

	if err := decoder.Decode(&cmd); err != nil {
		return
	}
	if err := ss.commandHandler.ValidateCommand(cmd); err != nil {
		resp := types.Response{
			Success: false,
			Message: fmt.Sprintf("invalid command: %v", err),
		}
		encoder.Encode(resp)
		return
	}
	response := ss.commandHandler.HandleCommand(cmd)
	encoder.Encode(response)
}

func (ss *SocketServer) cleanupSocket() {
	if _, err := os.Stat(ss.socketPath); err == nil {
		os.Remove(ss.socketPath)
	}
}

func (ss *SocketServer) setSocketPermissions() error {
	if err := os.Chmod(ss.socketPath, 0660); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}
	// get the socket directory and ensure its secure
	socketDir := filepath.Dir(ss.socketPath)
	if err := os.Chmod(socketDir, 0700); err != nil {
		return fmt.Errorf("failed to set socket directory permissions: %w", err)
	}
	return nil
}

func (ss *SocketServer) AcceptConnections() {
	for {
		conn, err := ss.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}
		go ss.handleConnection(conn)
	}
}
func (ss *SocketServer) Close() error {
	if ss.listener != nil {
		return ss.listener.Close()
	}
	// clean up socket file
	os.Remove(ss.socketPath)
	return nil
}
