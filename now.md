
The issue with your `dev` command not working likely stems from a few key problems in your implementation. Let me fix and improve it:

### Main Issues Found:

1. **Argument Parsing**: Your `main()` expects 3 arguments (`len(os.Args) < 3`), but `dev` command should work with just 2
2. **Command Execution**: The daemon binary path might not be correct
3. **Error Handling**: Some paths might not be properly constructed

### Here's the Corrected Version:

```go
func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }

    switch os.Args[1] {
    case "build":
        buildCmd()
    case "run":
        runCmd()
    case "dev":
        devCmd()
    default:
        printUsage()
        os.Exit(1)
    }
}

func devCmd() {
    fmt.Println("🚀 Setting up NextDeploy development environment...")

    // 1. First build the components
    fmt.Println("\n🔨 Building components...")
    os.Args = []string{os.Args[0], "build"} // Simulate 'build' command
    buildCmd()

    // 2. Setup environment
    homeDir, err := os.UserHomeDir()
    if err != nil {
        fail("Failed to get home directory: %v", err)
    }

    baseDir := filepath.Join(homeDir, ".nextdeploy")
    dirs := []string{
        filepath.Join(baseDir, "keys"),
        filepath.Join(baseDir, "logs"),
        filepath.Join(baseDir, "cache"),
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            fail("Failed to create directory %s: %v", dir, err)
        }
    }
    
    // Secure keys directory
    if err := os.Chmod(filepath.Join(baseDir, "keys"), 0700); err != nil {
        fail("Failed to set permissions on keys directory: %v", err)
    }

    // 3. Start the daemon
    fmt.Println("\n🔄 Starting NextDeploy daemon in development mode...")
    
    binaryPath := filepath.Join("bin", "nextdeployd")
    if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
        fail("Daemon binary not found at %s - did the build succeed?", binaryPath)
    }

    cmd := exec.Command(
        binaryPath,
        "-key-dir", filepath.Join(baseDir, "keys"),
        "-log-file", filepath.Join(baseDir, "logs", "daemon.log"),
        "-pid-file", filepath.Join(baseDir, "nextdeploy.pid"),
        "-port", "8080",
        "-host", "localhost",
        "-debug", "true",
    )

    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    fmt.Println("✅ Development environment ready!")
    fmt.Printf("   - API: http://localhost:8080\n")
    fmt.Printf("   - Logs: %s\n", filepath.Join(baseDir, "logs", "daemon.log"))
    fmt.Printf("   - Press CTRL+C to stop\n\n")

    if err := cmd.Run(); err != nil {
        fail("Daemon failed: %v", err)
    }
}
```

### Key Improvements:

1. **Fixed Argument Handling**: Now works with just `go run start.go dev`
2. **Better Path Handling**: Uses proper filepath.Join() for cross-platform compatibility
3. **Pre-flight Checks**: Verifies the binary exists before trying to run it
4. **Clearer Output**: More descriptive status messages
5. **Proper Error Handling**: Fail-fast with clear error messages

### How to Use:

```bash
# Clean start (recommended for first run)
rm -rf ~/.nextdeploy

# Start development environment
go run start.go dev
```

### Expected Behavior:

1. Builds both daemon and CLI
2. Creates `~/.nextdeploy` directory structure
3. Starts the daemon with:
   - Debug mode enabled
   - Logs to `~/.nextdeploy/logs/daemon.log`
   - Listens on `localhost:8080`
   - Runs in foreground (CTRL+C to stop)

### Troubleshooting:

If it still doesn't work:

1. Check if `bin/nextdeployd` exists after build
2. Verify directory permissions in `~/.nextdeploy`
3. Look for errors in the console output
4. Try running with absolute paths:

```go
absBinary, _ := filepath.Abs(filepath.Join("bin", "nextdeployd"))
cmd := exec.Command(absBinary, ...)
```

Would you like me to add any additional development features like automatic reload or test data generation?
