package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"nextdeploy/shared"
	"nextdeploy/shared/websocket"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type LogStreamer struct {
	dockerClient *client.Client
	wsClient     *websocket.WSClient
	streams      map[string]context.CancelFunc
}

func NewLogStreamer(dockerClient *client.Client, wsClient *websocket.WSClient) *LogStreamer {
	return &LogStreamer{
		dockerClient: dockerClient,
		wsClient:     wsClient,
		streams:      make(map[string]context.CancelFunc),
	}
}

func (ls *LogStreamer) StreamContainerLogs(containerID string) {
	ctx, cancel := context.WithCancel(context.Background())
	ls.streams[containerID] = cancel
	go func() {
		options := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
		}

		reader, err := ls.dockerClient.ContainerLogs(ctx, containerID, options)
		if err != nil {
			return
		}
		defer reader.Close()

		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			fmt.Printf("Read %d bytes from container %s\n", n, containerID)
			if err != nil {
				if err != io.EOF {
					break
				}
				continue
			}

			msg := shared.AgentMessage{
				Source: shared.AgentDaemon,
				Target: shared.AgentDashboard,
				Type:   shared.TypeLog,
				//Payload:   string(buf[:n]),
				Timestamp: time.Now().Unix(),
				//AgentID:   ls.wsClient.WSClient.AgentID,
				Context: map[string]string{"container_id": containerID},
			}

			if err := ls.wsClient.SendMessage(msg); err != nil {
				break
			}
		}
	}()
}

func (ls *LogStreamer) StopStreaming(containerID string) {
	if cancel, exists := ls.streams[containerID]; exists {
		cancel()
		delete(ls.streams, containerID)
	}
}
func SetupLogger(daemonize bool, debug bool, logFormat string, logFile string) (*slog.Logger, *os.File) {
	var logOutput *os.File
	var err error
	if daemonize {
		logOutput, err = os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("failed to open log file: %v\n", err)
			os.Exit(1)
		}
	} else {
		logOutput = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if debug {
		opts.Level = slog.LevelDebug
	}

	var handler slog.Handler
	if logFormat == "json" {
		handler = slog.NewJSONHandler(logOutput, opts)
	} else {
		handler = slog.NewTextHandler(logOutput, opts)
	}

	return slog.New(handler), logOutput
}
