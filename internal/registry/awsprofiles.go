package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func AddAWSProfile(profileName, accessKeyId, secretAccessKey string) error {
	// get home dir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	credPath := filepath.Join(homeDir, ".aws", "credentials")
	// Create .aws directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(credPath), 0700); err != nil {
		return fmt.Errorf("failed to create .aws directory: %w", err)
	}

	var existingContent string
	if _, err := os.Stat(credPath); err == nil {
		content, err := os.ReadFile(credPath)
		if err != nil {
			return fmt.Errorf("failed to read credentials file: %w", err)
		}
		existingContent = string(content)
	}
	// parse existing profiles
	profiles := parseProfiles(existingContent)
	// add/update the profile
	profiles[profileName] = profile{
		AccessKeyID:     accessKeyId,
		SecretAccessKey: secretAccessKey,
	}
	// write back to file
	if err := writeProfiles(credPath, profiles); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}
	ECRLogger.Info("Added AWS profile %s to %s", profileName, credPath)
	return nil
}

type profile struct {
	AccessKeyID     string
	SecretAccessKey string
}

func parseProfiles(content string) map[string]profile {
	profiles := make(map[string]profile)
	currentProfile := ""

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasPrefix(line, "]") {
			// New profile section
			currentProfile = strings.Trim(line, "[]")
			if _, exists := profiles[currentProfile]; !exists {
				profiles[currentProfile] = profile{}
			}
		} else if currentProfile != "" {
			// Profile property
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				p := profiles[currentProfile]

				switch key {
				case "aws_access_key_id":
					p.AccessKeyID = value
				case "aws_secret_access_key":
					p.SecretAccessKey = value
				}

				profiles[currentProfile] = p
			}
		}
	}

	return profiles
}

func writeProfiles(path string, profiles map[string]profile) error {
	var builder strings.Builder

	for name, p := range profiles {
		builder.WriteString(fmt.Sprintf("[%s]\n", name))
		builder.WriteString(fmt.Sprintf("aws_access_key_id = %s\n", p.AccessKeyID))
		builder.WriteString(fmt.Sprintf("aws_secret_access_key = %s\n", p.SecretAccessKey))
		builder.WriteString("\n")
	}

	// Write to temp file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, []byte(builder.String()), 0600); err != nil {
		return err
	}

	// Then rename to replace original
	return os.Rename(tempPath, path)
}
