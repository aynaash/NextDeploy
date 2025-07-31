Looking at your code and the error logs, I can identify several key issues that need to be addressed:

## Main Issues

1. **Docker Build Failure**:
   - The error `sh: next: not found` indicates Next.js isn't installed in your Docker container
   - Your dynamically generated Dockerfile has syntax issues in the `RUN` commands

2. **Image Tag Mismatch**:
   - You're building with tag `latest` but trying to run with tag `60ccdec` (Git commit hash)
   - This causes the "No such image" error when creating the container

3. **Dockerfile Generation Problems**:
   - The `createTarContext` function generates a malformed Dockerfile
   - The `RUN` commands are concatenated incorrectly

## Specific Fixes Needed

### 1. Fix the Dockerfile Generation

In `cmd/preview.go`, update the `createTarContext` function:

```go
func createTarContext(meta *nextcore.NextCorePayload) (io.Reader, error) {
    pr, pw := io.Pipe()
    tw := tar.NewWriter(pw)

    go func() {
        defer pw.Close()
        defer tw.Close()

        // Proper Dockerfile template
        dockerfile := fmt.Sprintf(`
FROM node:18-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --include=dev
COPY . .
RUN npm run build
ENV PORT=3000
CMD ["npm", "start"]
`)

        err := writeToTar(tw, "Dockerfile", []byte(dockerfile))
        if err != nil {
            PreviewLogger.Error("Error writing Dockerfile to tar: %v", err)
            pw.CloseWithError(err)
            return
        }

        // Rest of the file copying logic remains the same...
        err = filepath.Walk(meta.RootDir, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return err
            }
            if info.IsDir() {
                return nil
            }
            // Skip node_modules and other build artifacts
            if strings.Contains(path, "node_modules") || strings.Contains(path, ".next") {
                return nil
            }
            
            relPath := strings.TrimPrefix(path, meta.RootDir+string(filepath.Separator))
            fileData, err := os.ReadFile(path)
            if err != nil {
                return err
            }
            return writeToTar(tw, relPath, fileData)
        })

        if err != nil {
            pw.CloseWithError(err)
        }
    }()

    return pr, nil
}
```

### 2. Fix Image Tag Consistency

Update the build step to use the same tagging logic as the runtime:

```go
// In the previewCmd Run function:
imageName := fmt.Sprintf("%s:%s", strings.ToLower(payload.Config.App.Name), payload.GitCommit)
if payload.GitCommit == "" {
    imageName = fmt.Sprintf("%s:latest", strings.ToLower(payload.Config.App.Name))
}

buildOptions := build.ImageBuildOptions{
    Tags:       []string{imageName},
    Remove:     true,
    Dockerfile: "Dockerfile",
}
```

### 3. Add Build Verification

Add a check after building to verify the image exists before container creation:

```go
// After building the image:
_, _, err = cli.ImageInspectWithRaw(ctx, imageName)
if err != nil {
    PreviewLogger.Error("Failed to verify built image: %v", err)
    os.Exit(1)
}
```

### 4. Improve Error Handling

Wrap the build output processing to catch build failures:

```go
buildResp, err := cli.ImageBuild(ctx, tarBuf, buildOptions)
if err != nil {
    PreviewLogger.Error("Error building image: %v", err)
    os.Exit(1)
}
defer buildResp.Body.Close()

// Process build output and check for errors
var buildErr error
scanner := bufio.NewScanner(buildResp.Body)
for scanner.Scan() {
    line := scanner.Text()
    fmt.Println(line) // Print build output
    
    // Parse Docker build error messages
    var result struct {
        Error string `json:"error"`
    }
    if err := json.Unmarshal([]byte(line), &result); err == nil && result.Error != "" {
        buildErr = fmt.Errorf(result.Error)
    }
}

if buildErr != nil {
    PreviewLogger.Error("Build failed: %v", buildErr)
    os.Exit(1)
}
```

## Additional Recommendations

1. **Multi-stage Builds**: Consider using a multi-stage Dockerfile for production:
```dockerfile
# Build stage
FROM node:18-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --include=dev
COPY . .
RUN npm run build

# Production stage
FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
RUN npm install --production
CMD ["npm", "start"]
```

2. **Build Cache Optimization**: 
   - Separate `COPY package.json` and `RUN npm install` from source copy
   - Add `.dockerignore` to exclude unnecessary files

3. **Health Checks**:
   - Add proper health checks to your container config
   - Consider using `next start --port $PORT` for better process management

These changes should resolve both the "next: not found" error and the image tag mismatch issues you're experiencing. The key is ensuring:
1. Proper Next.js installation in the Docker image
2. Consistent tagging between build and run phases
3. Better error handling throughout the process
