/*
This initial version of the server package establishes a solid foundation for SSH and SFTP-based remote server operations. It's modular, readable, and functional—but there are architectural concerns that need to be addressed before scaling:

1. **Tight Coupling with Config** – `WithConfig` loads a config from disk rather than accepting an injected value, which limits testability and flexibility.
2. **No Retry or Timeout Logic** – SSH and SFTP operations lack retry mechanisms or adaptive backoff strategies. Failures will be brittle in real-world environments.
3. **No Resource Reaper** – Idle clients are held in memory indefinitely. Add TTL + eviction or LRU caching to prevent memory leaks.
4. **Error Aggregation is Weak** – Errors are collected in slices and returned as a `fmt.Errorf`, but there's no structured error reporting or logging context (e.g., stack trace, severity).
5. **Server Preparation Logic is Monolithic** – The shell command installation logic is procedural and embedded. Break it into a plugin-style abstraction per OS/package manager.
6. **Security** – No audit of allowed SSH commands or shell environments. Can this logic be exploited by malicious input? Review needed.

Overall: good bones, but needs defensive engineering, cleaner abstraction boundaries, and more thought toward long-term extensibility.
*/

package server

import (
	"context"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	serverlogger = logger.PackageLogger("Server", "🅱)SERVERLOGGER")
)

// ServerStruct manages multiple SSH connections and provides operations
type ServerStruct struct {
	config     *config.NextDeployConfig
	sshClients map[string]*SSHClient
	mu         sync.RWMutex // protects sshClients map
}

// SSHClient wraps SSH client and related configurations
type SSHClient struct {
	Client     *ssh.Client
	Config     *ssh.ClientConfig
	SFTPClient *sftp.Client
	LastUsed   time.Time
	mu         sync.Mutex // protects individual client operations
}

// ServerOption defines the functional option type
type ServerOption func(*ServerStruct) error

// New creates a new ServerStruct with provided options
func New(opts ...ServerOption) (*ServerStruct, error) {
	s := &ServerStruct{
		sshClients: make(map[string]*SSHClient),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return s, nil
}

// WithConfig loads and applies the configuration
func WithConfig(configPath string) ServerOption {
	return func(s *ServerStruct) error {
		cfg, err := config.Load()
		serverlogger.Debug("Loading configuration from %v", cfg)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		s.config = cfg
		serverlogger.Info("Configuration loaded successfully")
		return nil
	}
}

// WithSSH initializes SSH connections for all configured servers
func WithSSH() ServerOption {
	return func(s *ServerStruct) error {
		if s.config == nil {
			return fmt.Errorf("configuration not loaded")
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		var wg sync.WaitGroup
		errChan := make(chan error, len(s.config.Servers))

		for _, serverCfg := range s.config.Servers {
			wg.Add(1)
			go func(cfg config.ServerConfig) {
				defer wg.Done()
				client, err := connectSSH(cfg)
				if err != nil {
					errChan <- fmt.Errorf("server %s: %w", cfg.Name, err)
					return
				}
				s.sshClients[cfg.Name] = client
				serverlogger.Info("Successfully connected to server %s (%s)", cfg.Name, cfg.Host)
			}(serverCfg)
		}

		wg.Wait()
		close(errChan)

		var errs []error
		for err := range errChan {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			return fmt.Errorf("failed to connect to some servers: %v", errs)
		}
		return nil
	}
}

// connectSSH establishes an SSH connection and initializes SFTP client
func connectSSH(cfg config.ServerConfig) (*SSHClient, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}

	authMethods, err := getAuthMethods(cfg)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := getHostKeyCallback()
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return &SSHClient{
		Client:     client,
		Config:     sshConfig,
		SFTPClient: sftpClient,
		LastUsed:   time.Now(),
	}, nil
}

func getAuthMethods(cfg config.ServerConfig) ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	if cfg.KeyPath != "" {
		key, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key file %s: %w", cfg.KeyPath, err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			if _, ok := err.(*ssh.PassphraseMissingError); ok && cfg.KeyPassphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(cfg.KeyPassphrase))
				if err != nil {
					return nil, fmt.Errorf("failed to parse SSH private key with passphrase: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
			}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods provided for server %s", cfg.Name)
	}

	return authMethods, nil
}

func getHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create known hosts callback: %w", err)
	}

	return hostKeyCallback, nil
}

// ExecuteCommand runs a command on the specified server with context support
func (s *ServerStruct) ExecuteCommand(ctx context.Context, serverName, command string) (string, error) {
	client, err := s.getSSHClient(serverName)
	if err != nil {
		return "", err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	session, err := client.Client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up context cancellation
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			session.Signal(ssh.SIGKILL)
		case <-done:
		}
	}()
	defer close(done)

	output, err := session.CombinedOutput(command)
	client.LastUsed = time.Now()

	if err != nil {
		return string(output), fmt.Errorf("command failed: %w (output: %s)", err, output)
	}

	serverlogger.Debug("Executed command on %s: %s", serverName, command)
	return string(output), nil
}

// UploadFile uploads a file to the remote server using SFTP
func (s *ServerStruct) UploadFile(ctx context.Context, serverName, localPath, remotePath string) error {
	client, err := s.getSSHClient(serverName)
	if err != nil {
		return err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	remoteFile, err := client.SFTPClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	client.LastUsed = time.Now()
	serverlogger.Info("Uploaded %s to %s:%s", localPath, serverName, remotePath)
	return nil
}

// DownloadFile downloads a file from the remote server using SFTP
func (s *ServerStruct) DownloadFile(ctx context.Context, serverName, remotePath, localPath string) error {
	client, err := s.getSSHClient(serverName)
	if err != nil {
		return err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	remoteFile, err := client.SFTPClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	client.LastUsed = time.Now()
	serverlogger.Info("Downloaded %s:%s to %s", serverName, remotePath, localPath)
	return nil
}

// PingServer checks if the server is reachable
func (s *ServerStruct) PingServer(serverName string) error {
	client, err := s.getSSHClient(serverName)
	if err != nil {
		return err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	session, err := client.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	_, err = session.CombinedOutput("echo ping")
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	client.LastUsed = time.Now()
	return nil
}

// CloseSSHConnections closes all active SSH connections
func (s *ServerStruct) CloseSSHConnections() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for name, client := range s.sshClients {
		client.mu.Lock()
		if client.SFTPClient != nil {
			if err := client.SFTPClient.Close(); err != nil {
				errs = append(errs, fmt.Errorf("error closing SFTP client for %s: %w", name, err))
			}
		}
		if err := client.Client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing SSH client for %s: %w", name, err))
		}
		client.mu.Unlock()
		delete(s.sshClients, name)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors while closing connections: %v", errs)
	}
	return nil
}

// getSSHClient safely retrieves an SSH client from the map
func (s *ServerStruct) getSSHClient(serverName string) (*SSHClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.sshClients[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverName)
	}
	return client, nil
}

// Reconnect re-establishes connection to a server
func (s *ServerStruct) Reconnect(serverName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	var serverCfg *config.ServerConfig
	for _, cfg := range s.config.Servers {
		if cfg.Name == serverName {
			serverCfg = &cfg
			break
		}
	}

	if serverCfg == nil {
		return fmt.Errorf("server configuration not found for %s", serverName)
	}

	// Close existing connection if it exists
	if oldClient, ok := s.sshClients[serverName]; ok {
		oldClient.mu.Lock()
		if oldClient.SFTPClient != nil {
			oldClient.SFTPClient.Close()
		}
		oldClient.Client.Close()
		oldClient.mu.Unlock()
	}

	client, err := connectSSH(*serverCfg)
	if err != nil {
		return fmt.Errorf("failed to reconnect to %s: %w", serverName, err)
	}

	s.sshClients[serverName] = client
	serverlogger.Info("Reconnected to server %s", serverName)
	return nil
}

// ListServers returns a list of configured server names
func (s *ServerStruct) ListServers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var servers []string
	for name := range s.sshClients {
		servers = append(servers, name)
	}
	return servers
}

// GetServerStatus returns connection status of a server
func (s *ServerStruct) GetServerStatus(serverName string) (string, error) {
	client, err := s.getSSHClient(serverName)
	if err != nil {
		return "", err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	session, err := client.Client.NewSession()
	if err != nil {
		return "disconnected", nil
	}
	session.Close()

	uptime, err := s.ExecuteCommand(context.Background(), serverName, "uptime")
	if err != nil {
		return "connected but command failed", nil
	}

	return fmt.Sprintf("connected (uptime: %s)", uptime), nil
}

// PrepareServer installs required tools on the target server
func (s *ServerStruct) PrepareServer(ctx context.Context, serverName string) error {
	client, err := s.getSSHClient(serverName)
	if err != nil {
		return fmt.Errorf("failed to get SSH client: %w", err)
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	// Check if preparation is already done
	if _, err := s.ExecuteCommand(ctx, serverName, "which docker && which caddy && which go"); err == nil {
		serverlogger.Info("Server already has required tools installed")
		return nil
	}

	serverlogger.Info("Preparing server %s by installing required tools...", serverName)

	// Determine package manager (apt/yum/dnf)
	pkgManagerCmd := "command -v apt-get >/dev/null && echo apt || echo yum"
	pkgManager, err := s.ExecuteCommand(ctx, serverName, pkgManagerCmd)
	if err != nil {
		return fmt.Errorf("failed to detect package manager: %w", err)
	}

	// Install base dependencies
	baseDepsCmd := ""
	switch pkgManager {
	case "apt":
		baseDepsCmd = `sudo apt-get update && 
			sudo apt-get install -y curl git make gcc build-essential
			ca-certificates software-properties-common apt-transport-https`
	case "yum":
		baseDepsCmd = `sudo yum install -y curl git make gcc glibc-static
			ca-certificates yum-utils device-mapper-persistent-data lvm2`
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}

	if _, err := s.ExecuteCommand(ctx, serverName, baseDepsCmd); err != nil {
		return fmt.Errorf("failed to install base dependencies: %w", err)
	}

	// Install Docker
	dockerInstallCmd := `curl -fsSL https://get.docker.com | sudo sh && 
		sudo usermod -aG docker $USER && 
		sudo systemctl enable docker && 
		sudo systemctl start docker`

	if _, err := s.ExecuteCommand(ctx, serverName, dockerInstallCmd); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	// Install Caddy
	caddyInstallCmd := `sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https && 
		curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && 
		curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list && 
		sudo apt update && 
		sudo apt install -y caddy`

	if pkgManager == "yum" {
		caddyInstallCmd = `sudo yum install -y yum-plugin-copr && 
			sudo yum copr enable -y @caddy/caddy && 
			sudo yum install -y caddy`
	}

	if _, err := s.ExecuteCommand(ctx, serverName, caddyInstallCmd); err != nil {
		return fmt.Errorf("failed to install Caddy: %w", err)
	}

	// Install Go
	goInstallCmd := `curl -OL https://golang.org/dl/go1.21.0.linux-amd64.tar.gz && 
		sudo rm -rf /usr/local/go && 
		sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz && 
		echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && 
		source ~/.bashrc && 
		rm go1.21.0.linux-amd64.tar.gz`

	if _, err := s.ExecuteCommand(ctx, serverName, goInstallCmd); err != nil {
		return fmt.Errorf("failed to install Go: %w", err)
	}

	// Verify installations
	verifyCmd := `docker --version && caddy version && go version`
	if _, err := s.ExecuteCommand(ctx, serverName, verifyCmd); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	serverlogger.Info("Server preparation completed successfully")
	return nil
}
