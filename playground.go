//go:build ignore
// +build ignore

// internal/server/preparation/manager.go
package nextdeploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yourapp/git"
)

type Metadata struct {
	Routes     map[string][]string `json:"routes"`
	Env        map[string]string   `json:"env"`
	Middleware []string            `json:"middleware"`
}

type BuildLock struct {
	GitCommit     string    `json:"git_commit"`
	GitDirty      bool      `json:"git_dirty"`
	GeneratedAt   time.Time `json:"generated_at"`
	MetadataFile  string    `json:"metadata_file"`
}

const (
	LockFilePath        = ".nextdeploy/build.lock"
	MetadataFilePath    = ".nextdeploy/metadata.json"
	AssetsOutputDir     = ".nextdeploy/assets"
	PublicDir           = "public"
)

func GenerateMetadata() error {
	fmt.Println("Running `next build`...")
	if err := runNextBuild(); err != nil {
		return err
	}

	fmt.Println("Extracting metadata...")
	metadata, err := extractMetadata()
	if err != nil {
		return err
	}

	if err := saveMetadata(metadata); err != nil {
		return err
	}

	fmt.Println("Extracting static assets...")
	if err := extractAssets(); err != nil {
		return err
	}

	snapshot, err := git.GetGitSnapshot(MetadataFilePath)
	if err != nil {
		return err
	}
	return saveBuildLock(snapshot)
}

func ValidateMetadata() (bool, error) {
	lock, err := loadBuildLock()
	if err != nil {
		return false, err
	}

	currentCommit, err := git.CurrentCommit()
	if err != nil {
		return false, err
	}

	isDirty, err := git.IsDirty()
	if err != nil {
		return false, err
	}

	if currentCommit != lock.GitCommit || isDirty {
		fmt.Println("⚠️ Git state changed since last metadata snapshot.")
		fmt.Printf("Last commit: %s, Current commit: %s\n", lock.GitCommit, currentCommit)
		fmt.Printf("Dirty: %v\n", isDirty)
		return false, nil
	}

	return true, nil
}

func runNextBuild() error {
	// TODO: Run `next build` via exec.Command
	fmt.Println("(Simulated) next build complete.")
	return nil
}

func extractMetadata() (*Metadata, error) {
	// TODO: Parse .next/build-manifest.json, routes-manifest.json, etc.
	return &Metadata{
		Routes: map[string][]string{
			"static":  {"/about", "/contact"},
			"dynamic": {"/user/[id]"},
		},
		Env: map[string]string{
			"NEXT_PUBLIC_API_URL": "https://api.example.com",
		},
		Middleware: []string{"middleware.ts"},
	}, nil
}

func extractAssets() error {
	source := PublicDir
	destination := AssetsOutputDir

	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(destination, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
}

func saveMetadata(metadata *Metadata) error {
	if err := os.MkdirAll(filepath.Dir(MetadataFilePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MetadataFilePath, data, 0644)
}

func saveBuildLock(lock *git.GitSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(LockFilePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(LockFilePath, data, 0644)
}

func loadBuildLock() (*BuildLock, error) {
	data, err := os.ReadFile(LockFilePath)
	if err != nil {
		return nil, errors.New("build.lock not found")
	}
	var lock BuildLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}
