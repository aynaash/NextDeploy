package config

import (
	"fmt"
	"strings"
)

func ShowConfigSummary(cfg *NextDeployConfig) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎉 Configuration Summary")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("\n📱 Application: %s\n", cfg.App.Name)
	fmt.Printf("🌐 Port: %d | Env: %s\n", cfg.App.Port, cfg.App.Environment)
	if cfg.App.Domain != "" {
		fmt.Printf("🔗 Domain: %s\n", cfg.App.Domain)
	}

	fmt.Printf("\n📦 Repository: %s (%s)\n", cfg.Repository.URL, cfg.Repository.Branch)
	if cfg.Repository.AutoDeploy {
		fmt.Println("🤖 Auto-deploy: Enabled")
	}

	fmt.Printf("\n🐳 Docker Image: %s\n", cfg.Docker.Image)
	fmt.Printf("📦 Build Context: %s\n", cfg.Docker.Build.Context)

	fmt.Printf("\n🚀 Deployment Server: %s@%s\n", cfg.Deployment.Server.User, cfg.Deployment.Server.Host)
	fmt.Printf("📦 Container: %s (Restart: %s)\n", cfg.Deployment.Container.Name, cfg.Deployment.Container.Restart)

	if cfg.Database != nil {
		fmt.Printf("\n💾 Database: %s@%s:%s\n", cfg.Database.Username, cfg.Database.Host, cfg.Database.Port)
	}

	if cfg.Monitoring != nil {
		fmt.Printf("\n👀 Monitoring: %s (%s)\n", cfg.Monitoring.Type, cfg.Monitoring.Endpoint)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
}
