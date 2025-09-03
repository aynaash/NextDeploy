package client

import (
	"encoding/json"
	"fmt"
	"nextdeploy/daemon/internal/client"
	"nextdeploy/daemon/internal/types"
	"os"
	"strconv"
	"strings"
)

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("NextDeploy Daemon - Docker Container Management")
		fmt.Println("\nUsage:")
		fmt.Println("  Start daemon: nextdeployd daemon [--config=/path/to/config.json]")
		fmt.Println("\nContainer Commands:")
		fmt.Println("  nextdeployd listcontainers [--all=true]")
		fmt.Println("  nextdeployd deploy --image=nginx:latest --name=web-server --ports=80:8080")
		fmt.Println("  nextdeployd start --container=web-server")
		fmt.Println("  nextdeployd stop --container=web-server")
		fmt.Println("  nextdeployd restart --container=web-server")
		fmt.Println("  nextdeployd remove --container=web-server [--force=true]")
		fmt.Println("  nextdeployd logs --container=web-server [--lines=50]")
		fmt.Println("  nextdeployd inspect --container=web-server")
		fmt.Println("\nDeployment Commands:")
		fmt.Println("  nextdeployd swapcontainers --from=app-v1 --to=app-v2")
		fmt.Println("  nextdeployd rollback --container=web-server")
		fmt.Println("  nextdeployd pull --image=nginx:latest")
		fmt.Println("\nHealth & Status:")
		fmt.Println("  nextdeployd status")
		fmt.Println("  nextdeployd health [--container=web-server]")
		os.Exit(1)
	}

	command := os.Args[1]
	socketPath := "/var/run/nextdeployd.sock"

	if command == "daemon" {
		fmt.Println("Use the 'nextdeployd' command to manage containers. This client does not start the daemon.")
		os.Exit(1)
	}

	// client mode - send command to running daemon

	var cmd types.Command
	args := make(map[string]interface{})

	// Parse command-line arguments
	// In the main function, update the argument parsing section:
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) == 2 {
				// Enhanced parsing for different types
				key := parts[0]
				value := parts[1]

				switch value {
				case "true":
					args[key] = true
				case "false":
					args[key] = false
				default:
					// Try to parse as number, otherwise keep as string
					if intVal, err := strconv.Atoi(value); err == nil {
						args[key] = intVal
					} else {
						args[key] = value
					}
				}
			} else if len(parts) == 1 {
				// Handle boolean flags without =value
				args[parts[0]] = true
			}
		}
	}

	cmd.Type = command
	cmd.Args = args

	// send command to daemon
	response, err := client.SendCommand(socketPath, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		// check if daemon is running
		if strings.Contains(err.Error(), "connect: no such file or directory") || strings.Contains(err.Error(), "connect: connection refused") {
			fmt.Println("Is the daemon running? Start it with 'nextdeployd daemon --config=/path/to/config.json'")
			fmt.Println("If the daemon is not running, this client cannot connect to it.")
		}
		os.Exit(1)
	}

	// Display response based on command type
	if response.Success {
		fmt.Printf("Success: %s\n", response.Message)
		if response.Data != nil {
			switch data := response.Data.(type) {
			case []interface{}:
				for _, item := range data {
					if itemMap, ok := item.(map[string]interface{}); ok {
						// format container list nicely
						for key, value := range itemMap {
							fmt.Printf("%s: %v\t", key, value)
						}
						fmt.Println()
					} else {
						fmt.Printf("%v\n", item)
					}
				}
			case map[string]interface{}:
				for key, value := range data {
					fmt.Printf("%s: %v\n", key, value)
				}
			case []map[string]string:
				// handle container list format
				if len(data) > 0 {
					fmt.Printf("\n%-12s %-20s %-30s %-15s %-20s\n", "ID", "NAME", "IMAGE", "STATUS", "PORTS")
					fmt.Println(strings.Repeat("-", 100))
					for _, container := range data {
						fmt.Printf("%-12s %-20s %-30s %-15s %-20s\n",
							truncate(container["id"], 12),
							truncate(container["name"], 20),
							truncate(container["image"], 30),
							container["status"],
							truncate(container["ports"], 20))
					}
				}

			case string:
				fmt.Printf(" %s\n", data)
			default:
				if jsonData, err := json.MarshalIndent(data, "", "  "); err == nil {
					fmt.Printf("Data: %s\n", string(jsonData))
				} else {
					fmt.Printf("Data: %v\n", data)
				}
			}
		}
	} else {
		fmt.Printf("Error: %s\n", response.Message)
		os.Exit(1)
	}
}
