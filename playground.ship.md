
Here’s your **brutally clean and surgical pseudocode** for the `nextdeploy ship` command with **step-by-step logic**, **Functional Options Pattern**, and **zero fluff**.

---

### 🧠 **Overview:**

You're building a `copra-cli` command `ship` that:

* Loads config from `nextdeploy.yml`
* Hashes & encrypts it
* SSHes into a remote server using config
* Decrypts the config on server
* Pulls Docker image from registry using creds
* Runs the Docker container

---

### ✅ **Command Signature**

```bash
nextdeploy ship
```

No flags. Everything from `nextdeploy.yml`.

---

### 🧱 Pseudocode with Functional Option Pattern

#### Step 0: Types and Functional Options Setup

```go
type ShipOptions struct {
	ConfigPath   string
	EncryptedOut string
	Logger       Logger
}

type Option func(*ShipOptions)

func WithLogger(l Logger) Option {
	return func(o *ShipOptions) {
		o.Logger = l
	}
}

func WithConfigPath(path string) Option {
	return func(o *ShipOptions) {
		o.ConfigPath = path
	}
}
```

---

#### Step 1: `Ship()` Entrypoint

```go
func Ship(opts ...Option) error {
	// Default options
	options := &ShipOptions{
		ConfigPath:   "./nextdeploy.yml",
		EncryptedOut: "./nextdeploy.enc.yml",
		Logger:       DefaultLogger(),
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	ctx := context.Background()

	// 1. Read Config
	cfg, err := LoadAndParseYML(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	options.Logger.Info("Config loaded")

	// 2. Encrypt Config
	hash := GenerateHash(cfg)
	encrypted := EncryptYML(cfg)
	if err := SaveEncryptedYML(options.EncryptedOut, encrypted); err != nil {
		return fmt.Errorf("failed to save encrypted config: %w", err)
	}
	options.Logger.Info("Config encrypted and saved")

	// 3. SSH into Server
	sshClient, err := ConnectSSH(cfg.Server.IP, cfg.Server.SSHKey)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()
	options.Logger.Info("SSH connection established")

	// 4. Transfer Encrypted YML
	if err := sshClient.UploadFile(options.EncryptedOut, "/tmp/nextdeploy.enc.yml"); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	options.Logger.Info("Encrypted file uploaded")

	// 5. Remote: Decrypt the Config
	if err := sshClient.Exec("decrypt /tmp/nextdeploy.enc.yml -o /tmp/nextdeploy.yml"); err != nil {
		return fmt.Errorf("failed to decrypt file: %w", err)
	}
	options.Logger.Info("Config decrypted on remote server")

	// 6. Remote: Read and Parse Config
	remoteCfg, err := sshClient.ReadYML("/tmp/nextdeploy.yml")
	if err != nil {
		return fmt.Errorf("failed to read remote config: %w", err)
	}

	// 7. Docker Auth
	authCmd := fmt.Sprintf(
		"echo %s | docker login -u %s --password-stdin",
		remoteCfg.Registry.Password, remoteCfg.Registry.Username,
	)
	if err := sshClient.Exec(authCmd); err != nil {
		return fmt.Errorf("docker login failed: %w", err)
	}
	options.Logger.Info("Docker authenticated on server")

	// 8. Pull Image with Tag
	tag := GetLatestTag(remoteCfg.Image.Name) // Your existing function
	fullImage := fmt.Sprintf("%s:%s", remoteCfg.Image.Name, tag)
	if err := sshClient.Exec("docker pull " + fullImage); err != nil {
		return fmt.Errorf("docker pull failed: %w", err)
	}
	options.Logger.Info("Docker image pulled")

	// 9. Docker Run Command
	runCmd := BuildDockerRunCommand(remoteCfg)
	if err := sshClient.Exec(runCmd); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}
	options.Logger.Info("Container deployed and running")

	return nil
}
```

---

### 🔩 Supporting Functions (Pseudocode Level)

```go
func LoadAndParseYML(path string) (*Config, error)
func EncryptYML(cfg *Config) []byte
func SaveEncryptedYML(path string, data []byte) error
func ConnectSSH(ip, key string) (*SSHClient, error)
func GetLatestTag(image string) string
func BuildDockerRunCommand(cfg *Config) string
```

---

### 🔥 Brutal Feedback

* **NO flags** means **zero runtime flexibility**. Is that acceptable long-term? You’re hardcoding behavior behind the scenes.
* Encrypting the config before shipping is smart — **but make sure your decryption tool exists on the target server** (GPG? Custom binary?).
* Don’t reuse plaintext SSH key logic — use **agent forwarding** or **temporary session tokens** if you can.
* Docker pull + run in production with no volume management = you’re begging for persistence issues. Think: mounted volumes or data-loss risks.

---

### 🧭 Final Advice

You’re on the right path. But **don’t just ship containers. Ship a lifecycle.**

* Think of teardown/redeploy/version rollback
* Add post-deploy hooks
* Ensure logs are streamed (NextDeploy Daemon #2)
* Capture failures and expose them to your dashboard

Want me to generate the full Go implementation for this with your module structure? Let’s go all in.

Here’s the **clean, modular boilerplate** for your `ship` command using **Cobra CLI**, organized for clarity and scalability. It includes:

* Cobra command setup
* Functional options placeholder
* Stubs for critical logic blocks
* Skeleton for SSH, YAML, Docker actions

---

### 📁 Project Structure (Minimal CLI Skeleton)

```
copra-cli/
├── cmd/
│   ├── root.go
│   └── ship.go     <-- your ship command here
├── internal/
│   ├── config/     <-- config loader, parser, encryptor
│   ├── ssh/        <-- SSH connection logic
│   └── docker/     <-- Docker logic (auth, pull, run)
├── main.go
```

---

### 📦 `main.go`

```go
package main

import (
	"copra-cli/cmd"
)

func main() {
	cmd.Execute()
}
```

---

### 📦 `cmd/root.go`

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nextdeploy",
	Short: "NextDeploy CLI",
	Long:  "NextDeploy CLI handles deployments, shipping, and container lifecycle for your apps.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
```

---

### 📦 `cmd/ship.go` (Boilerplate for `ship` Command)

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"copra-cli/internal/ship"
)

var shipCmd = &cobra.Command{
	Use:   "ship",
	Short: "Ships your app to a configured remote server",
	Long: `Reads nextdeploy.yml, encrypts it, connects to your server via SSH, 
pulls the Docker image, and runs the container.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := ship.Ship()
		if err != nil {
			fmt.Printf("❌ Ship failed: %v\n", err)
		} else {
			fmt.Println("✅ Ship complete!")
		}
	},
}

func init() {
	rootCmd.AddCommand(shipCmd)
}
```

---

### 🧠 `internal/ship/ship.go` (Logic Entry)

```go
package ship

import (
	"context"
	"fmt"
)

type Options struct {
	ConfigPath   string
	EncryptedOut string
}

func defaultOptions() *Options {
	return &Options{
		ConfigPath:   "./nextdeploy.yml",
		EncryptedOut: "./nextdeploy.enc.yml",
	}
}

type Option func(*Options)

func Ship(opts ...Option) error {
	ctx := context.Background()

	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Step 1: Load config
	fmt.Println("🔧 Loading configuration...")
	// config, err := config.Load(options.ConfigPath)
	// if err != nil {
	// 	return fmt.Errorf("config load failed: %w", err)
	// }

	// Step 2: Encrypt config
	fmt.Println("🔐 Encrypting config...")
	// encrypted, err := encryptor.Encrypt(config)
	// if err != nil {
	// 	return err
	// }

	// Step 3: Save encrypted file
	fmt.Println("💾 Saving encrypted config...")
	// err = encryptor.Save(encrypted, options.EncryptedOut)

	// Step 4: Connect to remote via SSH
	fmt.Println("🔌 Connecting to server...")
	// sshClient := ssh.NewClient(config.ServerIP, config.SSHKey)

	// Step 5: Upload, decrypt, docker login, pull image, run it
	fmt.Println("🐳 Authenticating and running Docker image...")
	// docker.Login()
	// docker.Pull()
	// docker.Run()

	fmt.Println("✅ Deployment successful!")
	return nil
}
```

---

### ⏭️ Next Steps (Your TODO List)

**Implement these inside `internal/`:**

| Module      | Responsibility                               |
| ----------- | -------------------------------------------- |
| `config`    | Load and validate `nextdeploy.yml`           |
| `encryptor` | Generate hash, encrypt, decrypt              |
| `ssh`       | Handle SSH login, file transfer, remote exec |
| `docker`    | Login, pull image, run container             |

---

### 🚀 Run It

```bash
go run main.go ship
```

---

### 💣 Brutal Truth

This is bare bones. You can "run it", but it's dumb until you plug in real logic. You’ve now got the **CLI frame** — don’t stop here.

Want me to plug in encryption and SSH logic next with sample YAML structure? Let’s level this up.

