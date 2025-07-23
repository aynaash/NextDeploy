package core

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

type ReverseTunnel struct {
	sshClient    *ssh.Client
	localPort    int
	remotePort   int
	sshHost      string
	sshUser      string
	sshKeyPath   string
	tunnelActive bool
}

func NewReverseTunnel(sshHost, sshUser, sshKeyPath string, localPort, remotePort int) *ReverseTunnel {
	return &ReverseTunnel{
		sshHost:    sshHost,
		sshUser:    sshUser,
		sshKeyPath: sshKeyPath,
		localPort:  localPort,
		remotePort: remotePort,
	}
}

func (rt *ReverseTunnel) Start() error {
	signer, err := ssh.ParsePrivateKey(readKeyFile(rt.sshKeyPath))
	if err != nil {
		return fmt.Errorf("unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: rt.sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Connect to SSH server
	rt.sshClient, err = ssh.Dial("tcp", rt.sshHost, config)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	// Start reverse tunnel
	listener, err := rt.sshClient.Listen("tcp", "0.0.0.0:"+strconv.Itoa(rt.remotePort))
	if err != nil {
		return fmt.Errorf("unable to register tcp forward: %v", err)
	}

	rt.tunnelActive = true

	go func() {
		for rt.tunnelActive {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept connection: %v", err)
				continue
			}

			go rt.handleConnection(conn)
		}
	}()

	return nil
}

func (rt *ReverseTunnel) handleConnection(conn net.Conn) {
	defer conn.Close()

	localConn, err := net.Dial("tcp", "localhost:"+strconv.Itoa(rt.localPort))
	if err != nil {
		log.Printf("Failed to connect to local service: %v", err)
		return
	}
	defer localConn.Close()

	go copyConn(localConn, conn)
	copyConn(conn, localConn)
}

func (rt *ReverseTunnel) Stop() {
	rt.tunnelActive = false
	if rt.sshClient != nil {
		rt.sshClient.Close()
	}
}

func copyConn(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

func readKeyFile(path string) []byte {
	// Implement key file reading

	return nil // Placeholder, replace with actual file reading logic
}
