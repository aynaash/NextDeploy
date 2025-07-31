package cmd

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/build"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
	"io"
	"nextdeploy/shared"
	"nextdeploy/shared/nextcore"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	PreviewLogger = shared.PackageLogger("preview", "PREVIEW")
)
var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "preview the application in production mode",
	Run: func(cmd *cobra.Command, args []string) {
		// Load metadata
		file, err := os.ReadFile(".nextdeploy/metadata.json")
		if err != nil {
			PreviewLogger.Error("Error loading metadata: %v", err)
			fmt.Printf("Error loading metadata: %v\n", err)
			os.Exit(1)
		}
		var payload nextcore.NextCorePayload
		if err := json.Unmarshal(file, &payload); err != nil {
			PreviewLogger.Error("Error parsing metadata: %v", err)
			fmt.Printf("Error parsing metadata: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			PreviewLogger.Error("Error creating Docker client: %v", err)
			panic(err)
		}

		// Step 1: Create tar stream
		tarBuf, err := createTarContext(&payload)
		if err != nil {
			PreviewLogger.Error("Error creating tar context: %v", err)
			panic(err)
		}

		imageName := fmt.Sprintf("%s:latest", payload.Config.App.Name)

		// Step 2: Build image
		buildOptions := build.ImageBuildOptions{
			Tags:       []string{imageName},
			Remove:     true,
			Dockerfile: "Dockerfile",
		}

		buildResp, err := cli.ImageBuild(ctx, tarBuf, buildOptions)
		if err != nil {
			PreviewLogger.Error("Error building image: %v", err)
			panic(err)
		}
		defer buildResp.Body.Close()

		// Output the build logs
		io.Copy(os.Stdout, buildResp.Body)
		// Create runtime
		runtime, err := nextcore.NewNextRuntime(&payload)
		if err != nil {
			PreviewLogger.Error("Error creating runtime: %v", err)
			fmt.Printf("Error creating runtime: %v\n", err)
			os.Exit(1)
		}

		// Create and start container
		containerID, err := runtime.CreateContainer(context.Background())
		PreviewLogger.Debug("Creating container with ID: %s", containerID)
		if err != nil {
			PreviewLogger.Error("Error creating container: %v", err)
			fmt.Printf("Error starting container: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Container started successfully: %s\n", containerID)

		// Configure reverse proxy if needed
		if err := runtime.ConfigureReverseProxy(); err != nil {
			PreviewLogger.Error("Error configuring reverse proxy: %v", err)
			fmt.Printf("Error configuring reverse proxy: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(previewCmd)
}
func createTarContext(meta *nextcore.NextCorePayload) (io.Reader, error) {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer tw.Close()

		// Add Dockerfile
		dockerfile := fmt.Sprintf(`
FROM node:%s-alpine
WORKDIR /app
COPY . .
RUN %s
RUN %s
ENV PORT=%d
CMD ["node", "server.js"]
`, meta.PackageManager+" install", meta.BuildCommand, meta.Config.Database.Port)

		err := writeToTar(tw, "Dockerfile", []byte(dockerfile))
		if err != nil {
			PreviewLogger.Error("Error writing Dockerfile to tar: %v", err)
			pw.CloseWithError(err)
			return
		}

		// Copy app source
		PreviewLogger.Debug("Writing app files to tar from root directory: %s", meta.RootDir)
		err = filepath.Walk(meta.RootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				PreviewLogger.Error("Error walking path %s: %v", path, err)
				return err
			}
			if info.IsDir() {
				return nil
			}

			relPath := strings.TrimPrefix(path, meta.RootDir+"/")
			fileData, err := os.ReadFile(path)
			if err != nil {
				PreviewLogger.Error("Error reading file %s: %v", path, err)
				return err
			}
			return writeToTar(tw, relPath, fileData)
		})

		if err != nil {
			PreviewLogger.Error("Error writing app files to tar: %v", err)
			pw.CloseWithError(err)
			return
		}
	}()

	return pr, nil
}

func writeToTar(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     0644,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		PreviewLogger.Error("Error writing header for %s: %v", name, err)
		return err
	}
	_, err := tw.Write(data)
	return err
}
