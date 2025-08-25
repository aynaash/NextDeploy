Found the issue! In your `buildCmd()` function, you're building individual files instead of packages. Look at this line:

```go
cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, t.source)
```

Where `t.source` is set to:
- `daemon/main.go` (for the daemon)
- `cli/main.go` (for the CLI)

This compiles only the `main.go` file without the other files in the package.

## Fix

Change the build command to compile the **package** instead of the individual file:

```go
// Replace this:
cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, t.source)

// With this:
pkgPath := strings.TrimSuffix(t.source, "/main.go")
cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, pkgPath)
```

## Complete Fix for buildCmd()

Update the build loop in `buildCmd()`:

```go
// Build each target
for _, t := range targets {
    if *target != "all" && !strings.Contains(strings.ToLower(t.name), *target) {
        continue
    }

    log.Printf("🔨 Building %s...", t.name)
    
    // Build the PACKAGE, not just the main.go file
    pkgPath := strings.TrimSuffix(t.source, "/main.go")
    if pkgPath == t.source {
        // Fallback if the path doesn't end with /main.go
        pkgPath = filepath.Dir(t.source)
    }
    
    cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, pkgPath)
    cmd.Env = append(os.Environ(), t.environment...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        log.Fatalf("❌ Build failed for %s: %v", t.name, err)
    }
    log.Printf("✅ Successfully built %s", t.output)
}
```

## Alternative: Update the target definitions

You could also change the target definitions to use package paths:

```go
targets := []struct {
    name        string
    source      string
    output      string
    environment []string
    ldflags     string
}{
    {
        name:        "nextdeployd (daemon)",
        source:      "./daemon",  // Package path instead of file path
        output:      filepath.Join(*outputDir, "nextdeployd"),
        environment: getDaemonEnv(),
        ldflags:     fmt.Sprintf("-s -w -X main.Version=%s -X main.Commit=%s -X main.BuildTime=%s", version, commit, buildTime),
    },
    {
        name:    "nextdeploy (CLI)",
        source:  "./cli",  // Package path instead of file path
        output:  filepath.Join(*outputDir, "nextdeploy"),
        ldflags: fmt.Sprintf("-X main.Version=%s -X main.Commit=%s", version, commit),
    },
}
```

Then the build command stays the same:
```go
cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, t.source)
```

This second approach is cleaner and more maintainable. Choose whichever you prefer!
