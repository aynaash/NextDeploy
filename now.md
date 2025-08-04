

The error indicates that Caddy is failing to start within the 5-second timeout period. Let's implement a more robust solution with better debugging capabilities:

## Comprehensive Solution

### 1. Enhanced Caddy Startup (runtime.go)

```go
func (nr *nextruntime) ensureCaddyRunning() error {
    // Check if admin API is already responsive
    if conn, err := net.DialTimeout("tcp", "localhost:2019", 500*time.Millisecond); err == nil {
        conn.Close()
        NextCoreLogger.Debug("Caddy admin API already running")
        return nil
    }

    // Format Caddyfile first
    fmtCmd := exec.Command("caddy", "fmt", "--overwrite", "/etc/caddy/Caddyfile")
    if output, err := fmtCmd.CombinedOutput(); err != nil {
        NextCoreLogger.Warn("Caddyfile formatting failed (non-critical): %s", string(output))
    }

    // Start Caddy with full output capture
    cmd := exec.Command("caddy", "run",
        "--config", "/etc/caddy/Caddyfile",
        "--adapter", "caddyfile",
        "--admin", "0.0.0.0:2019",
        "--resume")

    // Create pipe for real-time output
    stdoutPipe, _ := cmd.StdoutPipe()
    stderrPipe, _ := cmd.StderrPipe()
    
    if err := cmd.Start(); err != nil {
        NextCoreLogger.Error("Caddy failed to start: %v", err)
        return fmt.Errorf("Caddy failed to start: %w", err)
    }

    // Stream output in goroutines
    go func() {
        scanner := bufio.NewScanner(stdoutPipe)
        for scanner.Scan() {
            NextCoreLogger.Debug("Caddy stdout: %s", scanner.Text())
        }
    }()
    
    go func() {
        scanner := bufio.NewScanner(stderrPipe)
        for scanner.Scan() {
            NextCoreLogger.Debug("Caddy stderr: %s", scanner.Text())
        }
    }()

    // Wait for Caddy to be ready with longer timeout
    timeout := time.After(15 * time.Second)
    tick := time.Tick(500 * time.Millisecond)
    
    for {
        select {
        case <-timeout:
            if cmd.Process != nil {
                cmd.Process.Kill()
            }
            return fmt.Errorf("Caddy failed to start within 15 seconds")
        case <-tick:
            if conn, err := net.Dial("tcp", "localhost:2019"); err == nil {
                conn.Close()
                NextCoreLogger.Info("Caddy started successfully")
                return nil
            }
        }
    }
}
```

### 2. Caddyfile Validation

Add this helper function:

```go
func (nr *nextruntime) validateCaddyfile(config string) error {
    cmd := exec.Command("caddy", "validate", "--config", "-")
    cmd.Stdin = strings.NewReader(config)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("Caddyfile validation failed: %s\n%s", err, string(output))
    }
    return nil
}
```

### 3. Updated ConfigureReverseProxy

```go
func (nr *nextruntime) ConfigureReverseProxy() error {
    config := nr.GenerateCaddyfile()
    
    // Validate before writing
    if err := nr.validateCaddyfile(config); err != nil {
        return fmt.Errorf("invalid Caddyfile: %w", err)
    }

    // Ensure directory exists
    if err := os.MkdirAll("/etc/caddy", 0755); err != nil {
        return fmt.Errorf("failed to create Caddy directory: %w", err)
    }

    // Write config
    if err := os.WriteFile("/etc/caddy/Caddyfile", []byte(config), 0644); err != nil {
        return fmt.Errorf("failed to write Caddyfile: %w", err)
    }

    // Reload with retries
    var lastErr error
    for i := 0; i < 3; i++ {
        if err := nr.reloadCaddy(); err == nil {
            return nil
        }
        lastErr = err
        time.Sleep(time.Second * time.Duration(i+1))
    }
    
    return fmt.Errorf("failed to reload Caddy after 3 attempts: %w", lastErr)
}
```

## Debugging Steps

1. **Manual Caddy Start**:
```bash
sudo caddy run --config /etc/caddy/Caddyfile --adapter caddyfile --admin 0.0.0.0:2019
```

2. **Check Ports**:
```bash
sudo netstat -tulnp | grep 2019
```

3. **Verify Permissions**:
```bash
ls -la /etc/caddy/
sudo chown -R $USER:$USER /etc/caddy
```

## Key Improvements

1. **Extended Timeout**: Increased from 5 to 15 seconds
2. **Real-time Logging**: Streams Caddy's output during startup
3. **Pre-validation**: Checks Caddyfile before attempting to load
4. **Better Error Handling**: More detailed error messages
5. **Permission Handling**: Ensures proper file access

This solution should resolve the timeout issues by:
- Providing better visibility into Caddy's startup process
- Giving Caddy more time to initialize
- Validating configuration before attempting to load
- Ensuring proper file permissions

The logs will now show exactly what's happening during Caddy startup, making it much easier to diagnose any remaining issues.
