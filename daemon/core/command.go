package core

import (
	"nextdeploy/shared/docker"
	"nextdeploy/shared"
	"nextdeploy/shared/websocket"
)

type CommandProcessor struct {
	queue       *CommandQueue
	wsClient    *websocket.WSClient
	docker      *docker.DockerManager
	logStreamer *LogStreamer
}

func NewCommandProcessor(wsClient *websocket.WSClient, docker *docker.DockerManager) *CommandProcessor {
	cp := &CommandProcessor{
		queue:       NewCommandQueue("/var/lib/nextdeploy/queue.json"),
		wsClient:    wsClient,
		docker:      docker,
		//logStreamer: NewLogStreamer(docker.Client(), wsClient),
	}

	go cp.queue.ProcessQueue(cp.processCommand)
	return cp
}

func (cp *CommandProcessor) processCommand(cmd shared.AgentMessage) error {
	//		switch cmd.Payload.(type) {
	//		case map[string]interface{}:
	//			payload := cmd.Payload.(map[string]interface{})
	//			command := payload["command"].(string)
	//
	//			switch command {
	//			case "start_container":
	//				return cp.docker.StartContainer(payload["container_id"].(string))
	//			case "stop_container":
	//				return cp.docker.StopContainer(payload["container_id"].(string))
	//			case "stream_logs":
	//				cp.logStreamer.StreamContainerLogs(payload["container_id"].(string))
	//				return nil
	//			default:
	//				return cp.handleCustomCommand(command, payload)
	//			}
	//		default:
	//			return nil
	//		}
	//	}
	return nil
}
func (cp *CommandProcessor) handleCustomCommand(command string, payload map[string]interface{}) error {
	// Implement custom command handling
	return nil
}
