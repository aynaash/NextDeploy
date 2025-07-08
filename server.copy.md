// package server
//
// import (
// 	"bytes"
// 	"context"
// 	"fmt"
// 	"io"
// 	"net"
// 	"nextdeploy/internal/config"
// 	"nextdeploy/internal/logger"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
// 	"sync"
// 	"time"
//
// 	"github.com/pkg/sftp"
// 	"golang.org/x/crypto/ssh"
// 	"golang.org/x/crypto/ssh/knownhosts"
// 	"nextdeploy/internal/envstore"
// 	"nextdeploy/internal/git"
// 	"nextdeploy/internal/registry"
// )
//
// var (
// 	serverlogger = logger.PackageLogger("Server", "🅱)SERVERLOGGER")
// )
//
// // ServerStruct manages multiple SSH connections and provides operations
// type ServerStruct struct {
// 	config     *config.NextDeployConfig
// 	sshClients map[string]*SSHClient
// 	mu         sync.RWMutex // protects sshClients map
// }
//
// // SSHClient wraps SSH client and related configurations
// type SSHClient struct {
// 	Client     *ssh.Client
// 	Config     *ssh.ClientConfig
// 	SFTPClient *sftp.Client
// 	LastUsed   time.Time
// 	mu         sync.Mutex // protects individual client operations
// }
//
// // ServerOption defines the functional option type
// type ServerOption func(*ServerStruct) error
//
// // New creates a new ServerStruct with provided options
// func New(opts ...ServerOption) (*ServerStruct, error) {
// 	s := &ServerStruct{
// 		sshClients: make(map[string]*SSHClient),
// 	}
//
// 	for _, opt := range opts {
// 		if err := opt(s); err != nil {
// 			return nil, fmt.Errorf("failed to apply option: %w", err)
// 		}
// 	}
//
// 	return s, nil
// }
//
// // WithConfig loads and applies the configuration
// func WithConfig() ServerOption {
// 	return func(s *ServerStruct) error {
// 		cfg, err := config.Load()
// 		if err != nil {
// 			return fmt.Errorf("failed to load configuration: %w", err)
// 		}
// 		s.config = cfg
// 		serverlogger.Info("Configuration loaded successfully")
// 		return nil
// 	}
// }
// func WithDaemon() ServerOption {
// 	return func(s *ServerStruct) error {
// 		serverlogger.Debug("Writing NextDeploy Agent connection herer")
// 		return nil
// 	}
// }
//
// // WithSSH initializes SSH connections for all configured servers
// func WithSSH() ServerOption {
// 	return func(s *ServerStruct) error {
// 		if s.config == nil || len(s.config.Servers) == 0 {
// 			return fmt.Errorf("the server configuration is not loaded or no servers configured")
// 		}
//
// 		s.mu.Lock()
// 		defer s.mu.Unlock()
//
// 		var wg sync.WaitGroup
// 		errChan := make(chan error, len(s.config.Servers))
// 		serverlogger.Info("The config.Servers value at with withssh function is :%v", s.config.Servers)
//
// 		for _, serverCfg := range s.config.Servers {
// 			wg.Add(1)
// 			go func(cfg config.ServerConfig) {
// 				defer wg.Done()
// 				client, err := connectSSH(cfg)
// 				if err != nil {
// 					errChan <- fmt.Errorf("server %s: %w", cfg.Name, err)
// 					return
// 				}
// 				s.sshClients[cfg.Name] = client
// 				serverlogger.Info("Successfully connected to server %s (%s)", cfg.Name, cfg.Host)
// 			}(serverCfg)
// 		}
//
// 		wg.Wait()
// 		close(errChan)
//
// 		var errs []error
// 		for err := range errChan {
// 			errs = append(errs, err)
// 		}
//
// 		if len(errs) > 0 {
// 			serverlogger.Debug("errs look like this %s:", errs)
// 			return fmt.Errorf("failed to connect to some servers: %v", errs)
// 		}
// 		return nil
// 	}
// }
//
// func (s *ServerStruct) GetDeploymentServer() (string, error) {
// 	if s.config == nil || s.config.Deployment.Server.Host == "" {
// 		return "", fmt.Errorf("deployment server configuration is not set")
// 	}
// 	// find the matchin in servers list
// 	for _, server := range s.config.Servers {
// 		if server.Name == s.config.Deployment.Server.Host {
// 			_, err := connectSSH(server)
// 			if err != nil {
// 				return "", fmt.Errorf("failed to connect to deployment server %s: %w", server.Name, err)
// 			}
// 			return server.Name, nil
// 		}
// 	}
// 	return "", fmt.Errorf("deployment server %s not found in configuration", s.config.Deployment.Server.Host)
// }
// func AddHostToKnownHosts(ip string, knownHostsPath string) error {
// 	// Validate IP/hostname
// 	if net.ParseIP(ip) == nil && !isValidHostname(ip) {
// 		return fmt.Errorf("invalid IP address or hostname: %s", ip)
// 	}
//
// 	// Set default known_hosts path if not provided
// 	if knownHostsPath == "" {
// 		home, err := os.UserHomeDir()
// 		if err != nil {
// 			return fmt.Errorf("failed to get home directory: %v", err)
// 		}
// 		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
// 	}
//
// 	// Create .ssh directory if it doesn't exist
// 	sshDir := filepath.Dir(knownHostsPath)
// 	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
// 		if err := os.Mkdir(sshDir, 0700); err != nil {
// 			return fmt.Errorf("failed to create .ssh directory: %v", err)
// 		}
// 	}
//
// 	// Get the host key using ssh-keyscan
// 	cmd := exec.Command("ssh-keyscan", ip)
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
// 	err := cmd.Run()
// 	if err != nil {
// 		return fmt.Errorf("ssh-keyscan failed: %v", err)
// 	}
//
// 	hostKey := strings.TrimSpace(out.String())
// 	if hostKey == "" {
// 		return fmt.Errorf("no host key returned for %s", ip)
// 	}
//
// 	// Append to known_hosts file
// 	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
// 	if err != nil {
// 		return fmt.Errorf("failed to open known_hosts file: %v", err)
// 	}
// 	defer f.Close()
//
// 	if _, err := f.WriteString(hostKey + "\n"); err != nil {
// 		return fmt.Errorf("failed to write to known_hosts file: %v", err)
// 	}
//
// 	return nil
// }
//
// // Helper function to validate hostnames
// func isValidHostname(hostname string) bool {
// 	if len(hostname) > 253 {
// 		return false
// 	}
// 	for _, part := range strings.Split(hostname, ".") {
// 		if len(part) > 63 {
// 			return false
// 		}
// 	}
// 	return true
// }
//
// // connectSSH establishes an SSH connection and initializes SFTP client
// func connectSSH(cfg config.ServerConfig) (*SSHClient, error) {
// 	if cfg.Port == 0 {
// 		cfg.Port = 22
// 	}
// 	ip := cfg.Host
// 	err := AddHostToKnownHosts(ip, "")
// 	if err != nil {
// 		serverlogger.Error("Failed to add host %s to known_hosts: %v", ip, err)
// 		return nil, fmt.Errorf("failed to add host %s to known_hosts: %w", ip, err)
// 	}
// 	authMethods, err := getAuthMethods(cfg)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	hostKeyCallback, err := getHostKeyCallback()
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	sshConfig := &ssh.ClientConfig{
// 		User:            cfg.Username,
// 		Auth:            authMethods,
// 		HostKeyCallback: hostKeyCallback,
// 		Timeout:         15 * time.Second,
// 		Config: ssh.Config{
// 			KeyExchanges: []string{"curve25519-sha256@libssh.org"},
// 			Ciphers:      []string{"chacha20-poly1305@openssh.com"},
// 		},
// 	}
//
// 	if len(authMethods) == 0 {
// 		sshConfig.Auth = []ssh.AuthMethod{authMethods[0]}
// 	}
//
// 	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
// 	client, err := ssh.Dial("tcp", addr, sshConfig)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to dial: %w", err)
// 	}
//
// 	sftpClient, err := sftp.NewClient(client)
// 	if err != nil {
// 		client.Close()
// 		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
// 	}
//
// 	return &SSHClient{
// 		Client:     client,
// 		Config:     sshConfig,
// 		SFTPClient: sftpClient,
// 		LastUsed:   time.Now(),
// 	}, nil
// }
//
// func getAuthMethods(cfg config.ServerConfig) ([]ssh.AuthMethod, error) {
// 	var authMethods []ssh.AuthMethod
//
// 	if cfg.KeyPath == "" {
// 		return nil, fmt.Errorf("no SSH key path provided for server %s", cfg.Name)
// 	}
//
// 	// Handle path expansion
// 	expandedPath := cfg.KeyPath
// 	if strings.HasPrefix(expandedPath, "~") {
// 		home, err := os.UserHomeDir()
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get user home directory: %w", err)
// 		}
// 		expandedPath = filepath.Join(home, expandedPath[1:])
// 	}
// 	expandedPath = os.ExpandEnv(expandedPath)
//
// 	serverlogger.Debug("Key path resolution: %s -> %s", cfg.KeyPath, expandedPath)
//
// 	key, err := os.ReadFile(expandedPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read SSH key file %s (resolved to %s): %w",
// 			cfg.KeyPath, expandedPath, err)
// 	}
//
// 	signer, err := ssh.ParsePrivateKey(key)
// 	if err != nil {
// 		if _, ok := err.(*ssh.PassphraseMissingError); ok && cfg.KeyPassphrase != "" {
// 			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(cfg.KeyPassphrase))
// 			if err != nil {
// 				return nil, fmt.Errorf("failed to parse SSH private key with passphrase: %w", err)
// 			}
// 		} else {
// 			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
// 		}
// 	}
// 	authMethods = append(authMethods, ssh.PublicKeys(signer))
//
// 	if cfg.Password != "" {
// 		authMethods = append(authMethods, ssh.Password(cfg.Password))
// 	}
//
// 	if len(authMethods) == 0 {
// 		return nil, fmt.Errorf("no authentication methods provided for server %s", cfg.Name)
// 	}
//
// 	return authMethods, nil
// }
// func getHostKeyCallback() (ssh.HostKeyCallback, error) {
// 	home, err := os.UserHomeDir()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get user home directory: %w", err)
// 	}
//
// 	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
// 	hostKeyCallback, err := knownhosts.New(knownHostsPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create known hosts callback: %w", err)
// 	}
//
// 	return hostKeyCallback, nil
// }
//
// func (s *ServerStruct) BasicCaddySetup(ctx context.Context, serverName string, stream io.Writer) error {
//
// 	return nil
//
// }
//
// // ExecuteCommand runs a command on the specified server with context support
// // ExecuteCommand runs a command on the specified server with context support and streaming
// func (s *ServerStruct) ExecuteCommand(ctx context.Context, serverName, command string, stream io.Writer) (string, error) {
// 	client, err := s.getSSHClient(serverName)
// 	if err != nil {
// 		return "", err
// 	}
//
// 	client.mu.Lock()
// 	defer client.mu.Unlock()
//
// 	session, err := client.Client.NewSession()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create session: %w", err)
// 	}
// 	defer session.Close()
//
// 	// Set up output pipes
// 	stdoutPipe, err := session.StdoutPipe()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
// 	}
//
// 	stderrPipe, err := session.StderrPipe()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
// 	}
//
// 	// Create a multi-writer to both capture output and stream it
// 	output := &bytes.Buffer{}
// 	var writers []io.Writer
//
// 	if stream != nil {
// 		writers = append(writers, stream)
// 	}
// 	writers = append(writers, output)
//
// 	multiWriter := io.MultiWriter(writers...)
//
// 	// Start command
// 	err = session.Start(command)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to start command: %w", err)
// 	}
//
// 	// Stream output in goroutines
// 	var wg sync.WaitGroup
// 	wg.Add(2)
//
// 	go func() {
// 		defer wg.Done()
// 		io.Copy(multiWriter, stdoutPipe)
// 	}()
//
// 	go func() {
// 		defer wg.Done()
// 		io.Copy(multiWriter, stderrPipe)
// 	}()
//
// 	// Set up context cancellation
// 	done := make(chan struct{})
// 	go func() {
// 		select {
// 		case <-ctx.Done():
// 			session.Signal(ssh.SIGKILL)
// 		case <-done:
// 		}
// 	}()
//
// 	// Wait for command completion
// 	err = session.Wait()
// 	close(done)
// 	wg.Wait()
//
// 	client.LastUsed = time.Now()
//
// 	if err != nil {
// 		return output.String(), fmt.Errorf("command failed: %w", err)
// 	}
//
// 	serverlogger.Debug("Executed command on %s: %s", serverName, command)
// 	return output.String(), nil
// }
//
// // UploadFile uploads a file to the remote server using SFTP
// func (s *ServerStruct) UploadFile(ctx context.Context, serverName, localPath, remotePath string) error {
// 	client, err := s.getSSHClient(serverName)
// 	if err != nil {
// 		return err
// 	}
//
// 	client.mu.Lock()
// 	defer client.mu.Unlock()
//
// 	localFile, err := os.Open(localPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to open local file: %w", err)
// 	}
// 	defer localFile.Close()
//
// 	remoteFile, err := client.SFTPClient.Create(remotePath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create remote file: %w", err)
// 	}
// 	defer remoteFile.Close()
//
// 	_, err = io.Copy(remoteFile, localFile)
// 	if err != nil {
// 		return fmt.Errorf("failed to copy file: %w", err)
// 	}
//
// 	client.LastUsed = time.Now()
// 	serverlogger.Info("Uploaded %s to %s:%s", localPath, serverName, remotePath)
// 	return nil
// }
//
// // DownloadFile downloads a file from the remote server using SFTP
// func (s *ServerStruct) DownloadFile(ctx context.Context, serverName, remotePath, localPath string) error {
// 	client, err := s.getSSHClient(serverName)
// 	if err != nil {
// 		return err
// 	}
//
// 	client.mu.Lock()
// 	defer client.mu.Unlock()
//
// 	remoteFile, err := client.SFTPClient.Open(remotePath)
// 	if err != nil {
// 		return fmt.Errorf("failed to open remote file: %w", err)
// 	}
// 	defer remoteFile.Close()
//
// 	localFile, err := os.Create(localPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create local file: %w", err)
// 	}
// 	defer localFile.Close()
//
// 	_, err = io.Copy(localFile, remoteFile)
// 	if err != nil {
// 		return fmt.Errorf("failed to copy file: %w", err)
// 	}
//
// 	client.LastUsed = time.Now()
// 	serverlogger.Info("Downloaded %s:%s to %s", serverName, remotePath, localPath)
// 	return nil
// }
//
// // PingServer checks if the server is reachable
// func (s *ServerStruct) PingServer(serverName string) error {
// 	client, err := s.getSSHClient(serverName)
// 	if err != nil {
// 		return err
// 	}
//
// 	client.mu.Lock()
// 	defer client.mu.Unlock()
//
// 	session, err := client.Client.NewSession()
// 	if err != nil {
// 		return fmt.Errorf("failed to create session: %w", err)
// 	}
// 	defer session.Close()
//
// 	_, err = session.CombinedOutput("echo ping")
// 	if err != nil {
// 		return fmt.Errorf("ping failed: %w", err)
// 	}
//
// 	client.LastUsed = time.Now()
// 	return nil
// }
//
// // CloseSSHConnections closes all active SSH connections
// func (s *ServerStruct) CloseSSHConnections() error {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
//
// 	var errs []error
// 	for name, client := range s.sshClients {
// 		client.mu.Lock()
// 		if client.SFTPClient != nil {
// 			if err := client.SFTPClient.Close(); err != nil {
// 				errs = append(errs, fmt.Errorf("error closing SFTP client for %s: %w", name, err))
// 			}
// 		}
// 		if err := client.Client.Close(); err != nil {
// 			errs = append(errs, fmt.Errorf("error closing SSH client for %s: %w", name, err))
// 		}
// 		client.mu.Unlock()
// 		delete(s.sshClients, name)
// 	}
//
// 	if len(errs) > 0 {
// 		return fmt.Errorf("errors while closing connections: %v", errs)
// 	}
// 	return nil
// }
//
// // getSSHClient safely retrieves an SSH client from the map
// func (s *ServerStruct) getSSHClient(serverName string) (*SSHClient, error) {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()
//
// 	client, ok := s.sshClients[serverName]
// 	if !ok {
// 		return nil, fmt.Errorf("server %s not found", serverName)
// 	}
// 	return client, nil
// }
//
// // Reconnect re-establishes connection to a server
// func (s *ServerStruct) Reconnect(serverName string) error {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
//
// 	if s.config == nil {
// 		return fmt.Errorf("configuration not loaded")
// 	}
//
// 	var serverCfg *config.ServerConfig
// 	for _, cfg := range s.config.Servers {
// 		if cfg.Name == serverName {
// 			serverCfg = &cfg
// 			break
// 		}
// 	}
//
// 	if serverCfg == nil {
// 		return fmt.Errorf("server configuration not found for %s", serverName)
// 	}
//
// 	// Close existing connection if it exists
// 	if oldClient, ok := s.sshClients[serverName]; ok {
// 		oldClient.mu.Lock()
// 		if oldClient.SFTPClient != nil {
// 			oldClient.SFTPClient.Close()
// 		}
// 		oldClient.Client.Close()
// 		oldClient.mu.Unlock()
// 	}
//
// 	client, err := connectSSH(*serverCfg)
// 	if err != nil {
// 		return fmt.Errorf("failed to reconnect to %s: %w", serverName, err)
// 	}
//
// 	s.sshClients[serverName] = client
// 	serverlogger.Info("Reconnected to server %s", serverName)
// 	return nil
// }
//
// // ListServers returns a list of configured server names
// func (s *ServerStruct) ListServers() []string {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()
//
// 	var servers []string
// 	for name := range s.sshClients {
// 		servers = append(servers, name)
// 	}
// 	return servers
// }
//
// // GetServerStatus returns connection status of a server
// func (s *ServerStruct) GetServerStatus(serverName string, stream io.Writer) (string, error) {
// 	client, err := s.getSSHClient(serverName)
// 	if err != nil {
// 		return "", err
// 	}
//
// 	client.mu.Lock()
// 	defer client.mu.Unlock()
//
// 	session, err := client.Client.NewSession()
// 	if err != nil {
// 		return "disconnected", nil
// 	}
// 	session.Close()
//
// 	uptime, err := s.ExecuteCommand(context.Background(), serverName, "uptime", stream)
// 	if err != nil {
// 		return "connected but command failed", nil
// 	}
//
// 	return fmt.Sprintf("connected (uptime: %s)", uptime), nil
// }
//
// // PrepareServer installs required tools on the target server
// // PrepareServer installs required tools on the target server with streaming output
// func (s *ServerStruct) PrepareServer(ctx context.Context, serverName string, stream io.Writer) error {
// 	client, err := s.getSSHClient(serverName)
// 	if err != nil {
// 		return fmt.Errorf("failed to get SSH client: %w", err)
// 	}
//
// 	client.mu.Lock()
// 	defer client.mu.Unlock()
//
// 	// Check if preparation is already done
// 	if _, err := s.ExecuteCommand(ctx, serverName, "which docker && which caddy && which go", stream); err == nil {
// 		if stream != nil {
// 			fmt.Fprintf(stream, "Server already has required tools installed\n")
// 		}
// 		serverlogger.Info("Server already has required tools installed")
// 		return nil
// 	}
//
// 	if stream != nil {
// 		fmt.Fprintf(stream, "Preparing server %s by installing required tools...\n", serverName)
// 	}
// 	serverlogger.Info("Preparing server %s by installing required tools...", serverName)
//
// 	// Determine package manager (apt/yum/dnf)
// 	pkgManagerCmd := "command -v apt-get >/dev/null && echo apt || echo yum"
// 	pkgManager, err := s.ExecuteCommand(ctx, serverName, pkgManagerCmd, stream)
// 	if err != nil {
// 		return fmt.Errorf("failed to detect package manager: %w", err)
// 	}
//
// 	// Install base dependencies
// 	baseDepsCmd := ""
// 	switch strings.TrimSpace(pkgManager) {
// 	case "apt":
// 		baseDepsCmd = `sudo apt-get update &&
//             sudo apt-get install -y curl git make gcc build-essential
//             ca-certificates software-properties-common apt-transport-https`
// 	case "yum":
// 		baseDepsCmd = `sudo yum install -y curl git make gcc glibc-static
//             ca-certificates yum-utils device-mapper-persistent-data lvm2`
// 	default:
// 		return fmt.Errorf("unsupported package manager: %s", pkgManager)
// 	}
//
// 	if _, err := s.ExecuteCommand(ctx, serverName, baseDepsCmd, stream); err != nil {
// 		return fmt.Errorf("failed to install base dependencies: %w", err)
// 	}
//
// 	// Install Docker
// 	dockerInstallCmd := `curl -fsSL https://get.docker.com | sudo sh &&
//         sudo usermod -aG docker $USER &&
//         sudo systemctl enable docker &&
//         sudo systemctl start docker`
//
// 	if _, err := s.ExecuteCommand(ctx, serverName, dockerInstallCmd, stream); err != nil {
// 		return fmt.Errorf("failed to install Docker: %w", err)
// 	}
//
// 	// Install Caddy
// 	caddyInstallCmd := `sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https &&
//         curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg &&
//         curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list &&
//         sudo apt update &&
//         sudo apt install -y caddy`
//
// 	if strings.TrimSpace(pkgManager) == "yum" {
// 		caddyInstallCmd = `sudo yum install -y yum-plugin-copr &&
//             sudo yum copr enable -y @caddy/caddy &&
//             sudo yum install -y caddy`
// 	}
//
// 	if _, err := s.ExecuteCommand(ctx, serverName, caddyInstallCmd, stream); err != nil {
// 		return fmt.Errorf("failed to install Caddy: %w", err)
// 	}
//
// 	// Install Go
// 	goInstallCmd := `curl -OL https://golang.org/dl/go1.21.0.linux-amd64.tar.gz &&
//         sudo rm -rf /usr/local/go &&
//         sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz &&
//         echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc &&
//         source ~/.bashrc &&
//         rm go1.21.0.linux-amd64.tar.gz`
//
// 	if _, err := s.ExecuteCommand(ctx, serverName, goInstallCmd, stream); err != nil {
// 		return fmt.Errorf("failed to install Go: %w", err)
// 	}
//
// 	// Verify installations
// 	verifyCmd := `docker --version && caddy version && go version`
// 	if _, err := s.ExecuteCommand(ctx, serverName, verifyCmd, stream); err != nil {
// 		return fmt.Errorf("verification failed: %w", err)
// 	}
//
// 	if stream != nil {
// 		fmt.Fprintf(stream, "Server preparation completed successfully\n")
// 	}
// 	serverlogger.Info("Server preparation completed successfully")
// 	return nil
// }
//
// func (s *ServerStruct) PrepareEcrCredentials(stream io.Writer) error {
// 	serverlogger.Info("Preparing ECR credentials")
//
// 	// Load environment variables
// 	store, err := envstore.New(
// 		envstore.WithEnvFile[string](".env"),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("failed to create env store: %w", err)
// 	}
//
// 	// Retrieve AWS credentials securely
// 	accessKey, err := store.GetEnv("AWS_ACCESS_KEY_ID")
// 	if err != nil {
// 		serverlogger.Error("Failed to get AWS_ACCESS_KEY_ID: %v", err)
// 		return fmt.Errorf("AWS_ACCESS_KEY_ID not found: %w", err)
// 	}
//
// 	secretKey, err := store.GetEnv("AWS_SECRET_ACCESS_KEY")
// 	if err != nil {
// 		serverlogger.Error("Failed to get AWS_SECRET_ACCESS_KEY: %v", err)
// 		return fmt.Errorf("AWS_SECRET_ACCESS_KEY not found: %w", err)
// 	}
//
// 	// Generate AWS credentials file content
// 	credentialsContent := fmt.Sprintf(`[default]
// aws_access_key_id = %s
// aws_secret_access_key = %s
// `, accessKey, secretKey)
//
// 	// Write credentials securely to ~/.aws/credentials
// 	command := fmt.Sprintf(`
// 		mkdir -p ~/.aws && \
// 		cat > ~/.aws/credentials <<'EOF'
// %sEOF
// 		chmod 600 ~/.aws/credentials
// 	`, credentialsContent)
//
// 	output, err := s.ExecuteCommand(
// 		context.Background(),
// 		"production", // Replace with actual server name
// 		command,
// 		stream, // Add stream parameter here
// 	)
// 	if err != nil {
// 		serverlogger.Error("Failed to write AWS credentials: %v (Output: %s)", err, output)
// 		return fmt.Errorf("failed to write credentials: %w", err)
// 	}
//
// 	// Get repo details
// 	cfg, err := config.Load()
// 	if err != nil {
// 		serverlogger.Error("Failed to load configuration: %v", err)
// 		return fmt.Errorf("failed to load configuration: %w", err)
// 	}
//
// 	image := cfg.Docker.Image
// 	if image == "" {
// 		serverlogger.Error("Docker image not specified in configuration")
// 		return fmt.Errorf("docker image not specified in configuration")
// 	}
//
// 	accountID, region, reponame, err := registry.ExtractECRDetails(image)
// 	// log out repo details
// 	serverlogger.Debug("Extracted ECR details - Account ID: %s, Region: %s, Repository Name: %s", accountID, region, reponame)
// 	if err != nil {
// 		serverlogger.Error("Failed to extract ECR details from image %s: %v", image, err)
// 		return fmt.Errorf("failed to extract ECR details from image %s: %w", image, err)
// 	}
// 	tag, err := git.GetCommitHash()
// 	if err != nil {
// 		serverlogger.Error("Failed to get commit hash: %v", err)
// 		return fmt.Errorf("failed to get commit hash: %w", err)
// 	}
// 	if tag == "" {
// 		serverlogger.Error("No commit hash found, using 'latest' tag")
// 		tag = "latest"
// 	}
// 	imagename := fmt.Sprintf("%s:%s", reponame, tag)
// 	serverlogger.Info("Using image: %s", imagename)
//
// 	ecrRegistry := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region)
// 	pullCommand := fmt.Sprintf(`aws ecr get-login-password --region %s | docker login --username AWS --password-stdin %s`, region, ecrRegistry)
// 	if stream != nil {
// 		fmt.Fprintf(stream, "🔑 Executing ECR login command...\n")
// 	}
//
// 	serverlogger.Debug("Pull command for ECR: %s", pullCommand)
// 	output, err = s.ExecuteCommand(
// 		context.Background(),
// 		"production", // Replace with actual server name
// 		pullCommand,
// 		stream,
// 	)
// 	if err != nil {
// 		serverlogger.Error("Failed to login to ECR: %v (Output: %s)", err, output)
// 		return fmt.Errorf("failed to login to ECR: %w", err)
// 	}
//
// 	if stream != nil {
// 		fmt.Fprintf(stream, "✅ Successfully logged in to ECR\n")
// 	}
//
// 	serverlogger.Info("Successfully prepared ECR credentials")
// 	return nil
// }
