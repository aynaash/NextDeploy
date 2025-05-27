
package cmd

import (
	"nextdeploy/internal/docker"
	"nextdeploy/internal/secrets"
	"nextdeploy/internal/health"
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

var localTestCmd = &cobra.Command{
	Use:   "localtest",
	Short: "Run your container locally with prod-like env for validation",
	RunE: func(cmd *cobra.Command, args []string) error {
		image, _ := cmd.Flags().GetString("image")
		env, _ := cmd.Flags().GetString("env")
		secretsFrom, _ := cmd.Flags().GetString("secrets-from")
		healthURL, _ := cmd.Flags().GetString("health")
		port, _ := cmd.Flags().GetInt("port")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		envVars, err := secrets.Load(env, secretsFrom)
		if err != nil {
			return fmt.Errorf("secrets error: %w", err)
		}

		containerID, err := docker.Run(image, envVars, port)
		if err != nil {
			return fmt.Errorf("docker error: %w", err)
		}
		defer docker.Stop(containerID) // ensure cleanup

		ok := health.CheckWithTimeout(healthURL, timeout)
		if !ok {
			return fmt.Errorf("health check failed")
		}

		fmt.Println("✅ Container passed localtest successfully.")
		return nil
	},
}

func init() {
	localTestCmd.Flags().String("image", "", "Docker image to run")
	localTestCmd.Flags().String("env", "prod", "Environment")
	localTestCmd.Flags().String("secrets-from", "doppler", "Secrets provider")
	localTestCmd.Flags().String("health", "", "Health URL to probe")
	localTestCmd.Flags().Int("port", 3000, "Container port to expose")
	localTestCmd.Flags().Duration("timeout", 30*time.Second, "Timeout for health check")
	rootCmd.AddCommand(localTestCmd)
}
