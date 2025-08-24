package main

import (
	"fmt"
)

func (d *NextDeployDaemon) executeCommand(cmd Command) Response {
	switch cmd.Type {
	case "swapcontainers":
		return d.swapContainers(cmd.Args)
	case "listcontainers":
		return d.listContainers(cmd.Args)
	case "deploy":
		return d.deployContainer(cmd.Args)
	case "status":
		return d.getStatus()
	case "restart":
		return d.restartContainer(cmd.Args)
	case "logs":
		return d.getContainerLogs(cmd.Args)
	case "stop":
		return d.stopContainer(cmd.Args)
	case "start":
		return d.startContainer(cmd.Args)
	case "remove":
		return d.removeContainer(cmd.Args)
	case "pull":
		return d.pullImage(cmd.Args)
	case "inspect":
		return d.inspectContainer(cmd.Args)
	case "health":
		return d.healthCheck(cmd.Args)
	case "rollback":
		return d.rollbackContainer(cmd.Args)
	default:
		return Response{
			Success: false,
			Message: fmt.Sprintf("Unknown command: %s", cmd.Type),
		}
	}
}
