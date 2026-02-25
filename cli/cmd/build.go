package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"nextdeploy/shared"
	"nextdeploy/shared/nextcore"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the Next.js app and prepare a deployable tarball",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("build", "🔨 BUILD")
		log.Info("Starting NextDeploy build process...")

		// 1. Generate Metadata (this also runs the Next.js build)
		payload, err := nextcore.GenerateMetadata()
		if err != nil {
			log.Error("Failed to generate metadata: %v", err)
			os.Exit(1)
		}

		log.Info("Build mode detected as: %s", payload.OutputMode)

		// 2. Prepare the release directory based on output mode
		releaseDir := ""
		if payload.OutputMode == nextcore.OutputModeStandalone {
			releaseDir = filepath.Join(".next", "standalone")
			log.Info("Copying public/ to standalone/public/...")
			copyDir("public", filepath.Join(releaseDir, "public"))
			log.Info("Copying .next/static/ to standalone/.next/static/...")
			copyDir(filepath.Join(".next", "static"), filepath.Join(releaseDir, ".next", "static"))

			log.Info("Copying deployment metadata...")
			copyFile(".nextdeploy/metadata.json", filepath.Join(releaseDir, "metadata.json"))

		} else if payload.OutputMode == nextcore.OutputModeExport {
			releaseDir = "out"
			copyFile(".nextdeploy/metadata.json", filepath.Join(releaseDir, "metadata.json"))
		} else {
			// Default mode
			releaseDir = "."
		}

		// 3. Create app.tar.gz
		tarballName := "app.tar.gz"
		log.Info("Creating deployment tarball: %s", tarballName)
		err = createTarball(releaseDir, tarballName, payload.OutputMode)
		if err != nil {
			log.Error("Failed to create tarball: %v", err)
			os.Exit(1)
		}

		log.Info("Build complete! Deployment artifact ready: %s", tarballName)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	os.MkdirAll(filepath.Dir(dst), 0755)
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func copyDir(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // directory doesn't exist, safely ignore
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return copyFile(path, dstPath)
	})
}

func createTarball(sourceDir, targetTar string, outputMode nextcore.OutputMode) error {
	tarfile, err := os.Create(targetTar)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	gzw := gzip.NewWriter(tarfile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the tarball itself
		if path == targetTar || filepath.Base(path) == targetTar {
			return nil
		}

		// Exclusions for default mode
		if outputMode == nextcore.OutputModeDefault {
			if strings.Contains(path, "node_modules") || strings.Contains(path, ".git") || strings.HasPrefix(path, ".nextdeploy") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// To prevent Windows path separators
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tw, file)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
