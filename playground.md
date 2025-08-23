# Integrating Logging into Your NextDeploy Daemon

I'll integrate a comprehensive logging system into your NextDeploy daemon using `init()` functions and the existing structure. Here's how to implement it:

## 1. First, add the logging package and configuration

Add this to your imports:
```go
import (
    // ... your existing imports
   )
```

## 2. Add logging configuration to your DaemonConfig

```go
type DaemonConfig struct {
    SocketPath      string   `json:"socket_path"`
    SocketMode      string   `json:"socket_mode"`
    AllowedUsers    []string `json:"allowed_users"`
    DockerSocket    string   `json:"docker_socket"`
    ContainerPrefix string   `json:"container_prefix"`
    LogLevel        string   `json:"log_level"`
    LogDir          string   `json:"log_dir"`          // New
    LogMaxSize      int      `json:"log_max_size"`     // New - in MB
    LogMaxBackups   int      `json:"log_max_backups"`  // New
}
```

## 3. Create global logger variables

Add these near your other package-level variables:
```go
var (
    logger      *log.Logger
    logFile     *os.File
    logFilePath string
    logConfig   LoggerConfig
)

// Logger configuration
type LoggerConfig struct {
    LogDir      string
    LogFileName string
    MaxFileSize int64 // in bytes
    MaxBackups  int
    LogLevel    string
}
```

## 4. Implement init() functions for logging setup

Add these init functions to your code:

```go
// init function for basic logging setup
func init() {
    fmt.Println("Initializing logging system...")
    
    // Default configuration
    logConfig = LoggerConfig{
        LogDir:      "/var/log/nextdeployd",
        LogFileName: "nextdeployd.log",
        MaxFileSize: 10 * 1024 * 1024, // 10MB
        MaxBackups:  5,
        LogLevel:    "info",
    }
    
    // Create log directory if it doesn't exist
    if err := os.MkdirAll(logConfig.LogDir, 0755); err != nil {
        log.Fatalf("Failed to create log directory: %v", err)
    }
}

// Second init function for file handling
func init() {
    logFilePath = filepath.Join(logConfig.LogDir, logConfig.LogFileName)
    
    // Open log file (append mode, create if not exists)
    var err error
    logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatalf("Failed to open log file: %v", err)
    }

    // Create multi-writer for both file and stdout
    multiWriter := io.MultiWriter(os.Stdout, logFile)
    
    // Initialize logger with timestamp and file/line information
    logger = log.New(multiWriter, "NEXTDEPLOY: ", log.LstdFlags|log.Lshortfile)
    
    logger.Println("Logging system initialized")
}
```

## 5. Update NewNextDeployDaemon to handle logging config

```go
func NewNextDeployDaemon(configPath string) (*NextDeployDaemon, error) {
    config := &DaemonConfig{
        SocketPath:      "/var/run/nextdeployd.sock",
        SocketMode:      "0660",
        DockerSocket:    "/var/run/docker.sock",
        ContainerPrefix: "nextdeploy-",
        LogLevel:        "info",
        LogDir:          "/var/log/nextdeployd",  // Default
        LogMaxSize:      10,                      // Default 10MB
        LogMaxBackups:   5,                       // Default 5 backups
    }

    // Load config if exists
    if configPath != "" {
        if err := loadConfig(configPath, config); err != nil {
            logger.Printf("Warning: Could not load config file: %v", err)
        }
    }
    
    // Update log configuration based on loaded config
    logConfig.LogDir = config.LogDir
    logConfig.MaxFileSize = int64(config.LogMaxSize) * 1024 * 1024
    logConfig.MaxBackups = config.LogMaxBackups
    logConfig.LogLevel = config.LogLevel

    ctx, cancel := context.WithCancel(context.Background())
    return &NextDeployDaemon{
        ctx:        ctx,
        cancel:     cancel,
        socketPath: config.SocketPath,
        config:     config,
    }, nil
}
```

## 6. Add log rotation function

```go
// RotateLog checks if log rotation is needed and performs it
func RotateLog() error {
    // Check if file exists and its size
    info, err := os.Stat(logFilePath)
    if os.IsNotExist(err) {
        return nil // No file to rotate
    }
    if err != nil {
        return fmt.Errorf("failed to stat log file: %v", err)
    }

    // Check if rotation is needed
    if info.Size() < logConfig.MaxFileSize {
        return nil
    }

    // Close current log file
    if logFile != nil {
        logFile.Close()
    }

    // Rotate logs
    for i := logConfig.MaxBackups - 1; i >= 0; i-- {
        oldLog := fmt.Sprintf("%s.%d", logFilePath, i)
        newLog := fmt.Sprintf("%s.%d", logFilePath, i+1)
        
        if _, err := os.Stat(oldLog); err == nil {
            if i == logConfig.MaxBackups-1 {
                // Remove the oldest backup
                os.Remove(oldLog)
            } else {
                os.Rename(oldLog, newLog)
            }
        }
    }

    // Rename current log to .0
    if err := os.Rename(logFilePath, logFilePath+".0"); err != nil {
        return fmt.Errorf("failed to rotate log: %v", err)
    }

    // Reopen log file
    logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to reopen log file: %v", err)
    }
    
    // Update logger output
    multiWriter := io.MultiWriter(os.Stdout, logFile)
    logger.SetOutput(multiWriter)
    
    logger.Println("Log file rotated successfully")
    return nil
}
```

## 7. Create logging helper functions

```go
// LogMessage logs a message with rotation check and level filtering
func LogMessage(level, message string) {
    // Check and perform log rotation if needed
    if err := RotateLog(); err != nil {
        log.Printf("ERROR: Log rotation failed: %v", err)
    }

    // Level filtering
    levelPriority := map[string]int{
        "debug":   1,
        "info":    2,
        "warning": 3,
        "error":   4,
        "fatal":   5,
    }
    
    configPriority := levelPriority[logConfig.LogLevel]
    msgPriority := levelPriority[strings.ToLower(level)]
    
    if msgPriority < configPriority {
        return // Skip if message level is below configured level
    }

    // Log the message with timestamp and level
    logger.Printf("[%s] %s", strings.ToUpper(level), message)
}

// Simple logging functions
func LogInfo(message string) {
    LogMessage("info", message)
}

func LogError(message string) {
    LogMessage("error", message)
}

func LogWarning(message string) {
    LogMessage("warning", message)
}

func LogDebug(message string) {
    LogMessage("debug", message)
}
```

## 8. Integrate logging throughout your daemon

Replace all existing `log.Printf` and `log.Println` calls with the appropriate logging functions:

```go
// Example replacements throughout your code:
func (d *NextDeployDaemon) Start() error {
    // Check if we can access Docker
    if err := d.checkDockerAccess(); err != nil {
        LogError(fmt.Sprintf("Docker access check failed: %v", err))
        return fmt.Errorf("docker access check failed: %w", err)
    }
    // ... rest of the function
}

func (d *NextDeployDaemon) handleConnection(conn net.Conn) {
    defer conn.Close()

    // Set connection timeout
    conn.SetDeadline(time.Now().Add(30 * time.Second))

    decoder := json.NewDecoder(conn)
    encoder := json.NewEncoder(conn)

    var cmd Command
    if err := decoder.Decode(&cmd); err != nil {
        LogError(fmt.Sprintf("Error decoding command: %v", err))
        return
    }

    LogInfo(fmt.Sprintf("Received command: %s with args: %v", cmd.Type, cmd.Args))
    // ... rest of the function
}
```

## 9. Update main function to use the new logging

```go
func main() {
    // No need for manual logger setup here since init() functions handle it
    
    socketPath := "/var/run/nextdeployd.sock"
    configPath := "/etc/nextdeployd/config.json"

    if len(os.Args) < 2 {
        LogInfo("Starting NextDeploy daemon without arguments, showing help")
        // ... rest of your help text
        os.Exit(1)
    }

    command := os.Args[1]

    if command == "daemon" {
        // Check for config flag
        for _, arg := range os.Args[2:] {
            if strings.HasPrefix(arg, "--config=") {
                configPath = strings.TrimPrefix(arg, "--config=")
                break
            }
        }

        LogInfo("Starting NextDeploy daemon...")
        // Run as daemon
        daemon, err := NewNextDeployDaemon(configPath)
        if err != nil {
            LogError(fmt.Sprintf("Failed to create daemon: %v", err))
            os.Exit(1)
        }

        if err := daemon.Start(); err != nil {
            LogError(fmt.Sprintf("Daemon failed: %v", err))
            os.Exit(1)
        }
        return
    }
    
    // ... rest of main function
}
```

## 10. Add cleanup on shutdown

Update your Shutdown method to properly close the log file:

```go
func (d *NextDeployDaemon) Shutdown() {
    LogInfo("Shutting down daemon...")
    d.cancel()

    if d.listener != nil {
        d.listener.Close()
    }

    // Clean up socket file
    os.Remove(d.socketPath)
    
    // Close log file
    if logFile != nil {
        logFile.Close()
    }
    
    LogInfo("Daemon stopped")
}
```

## 11. Create a function to handle registry authentication logging

```go
func handleRegistryAuth(image string) error {
    // Check if image is from a private registry
    if strings.Contains(image, ".azurecr.io/") || 
       strings.Contains(image, ".amazonaws.com/") ||
       strings.Contains(image, "gcr.io/") ||
       strings.Contains(image, "registry.gitlab.com/") {
        
        LogInfo(fmt.Sprintf("Private registry detected for image: %s", image))
        // Add your registry authentication logic here
        // This is where you'd implement token retrieval, etc.
    }
    return nil
}
```

This implementation provides:

1. **Automatic initialization** through `init()` functions
2. **Log rotation** based on file size
3. **Configurable log levels** (debug, info, warning, error)
4. **Multiple output targets** (both file and stdout)
5. **Structured logging** with timestamps and log levels
6. **Integration** with your existing daemon configuration

The logging system will automatically initialize before your main function runs, and will handle log rotation based on the configured file size limits.
