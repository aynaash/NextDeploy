package main

import (
	"fmt"
	"os"
	"os/exec"
)

func SetupCaddyReverseProxy() error {
	caddyfilePath := "/etc/caddy/Caddyfile"
	caddyConfigFilePath := "~/app/.nextdeploy/caddy/Caddyfile"
	if _, err := os.Stat(caddyConfigFilePath); os.IsNotExist(err) {
		fmt.Printf("Caddy config file does not exist at path: %s\n", caddyConfigFilePath)
		return nil
	}
	if _, err := os.Stat(caddyfilePath); os.IsNotExist(err) {
		fmt.Printf("Caddyfile does not exist at path: %s\n", caddyfilePath)
		return nil
	} else if err != nil {
		fmt.Printf("Failed to check Caddyfile: %v\n", err)
		return err
	}
	// read config from caddyConfigFilePath and write to caddyfilePath
	data, err := os.ReadFile(caddyConfigFilePath)
	if err != nil {
		fmt.Printf("Failed to read Caddy config file: %v\n", err)
		return err
	}
	err = os.WriteFile(caddyfilePath, data, 0644)
	if err != nil {
		fmt.Printf("Failed to write Caddyfile: %v\n", err)
		return err
	}
	// reload caddy with the new config

	cmd := exec.Command("caddy", "reload", "--config", caddyfilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to reload Caddy configuration: %v\nOutput: %s\n", err, string(output))
		return err
	}

	fmt.Println("Caddy configuration reloaded successfully")
	return nil
}
