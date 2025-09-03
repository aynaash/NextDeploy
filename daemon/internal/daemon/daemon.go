package daemon

import (
	"context"
	"fmt"
	"log"
	"nextdeploy/daemon/internal/config"
	"nextdeploy/daemon/internal/logging"
	"nextdeploy/daemon/internal/types"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type NextDeployDaemon struct {
	ctx            context.Context
	cancel         context.CancelFunc
	socketPath     string
	config         *types.DaemonConfig
	socketServer   *SocketServer
	dockerClient   *DockerClient
	commandHandler *CommandHandler
	logger         *log.Logger
}

func NewNextDeployDaemon(socketPath string) (*NextDeployDaemon, error) {
	cfg, err := config.LoadConfig(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	logConfig := types.LoggerConfig{
		LogDir:      cfg.LogDir,
		LogFileName: "nextdeployd.log",
		MaxFileSize: 10 * 1024 * 1024, // 10 MB
		MaxBackups:  5,
	}

	logger := logging.SetupLogger(logConfig)

	dockerClient := NewDockerClient(cfg)

	commandHandler := NewCommandHandler(dockerClient, cfg)

	socketServer := NewSocketServer(cfg.SocketPath, commandHandler)

	return &NextDeployDaemon{
		ctx:            ctx,
		cancel:         cancel,
		socketPath:     socketPath,
		config:         cfg,
		socketServer:   socketServer,
		dockerClient:   dockerClient,
		commandHandler: commandHandler,
		logger:         logger,
	}, nil

}

func (d *NextDeployDaemon) Start() error {
	if err := d.dockerClient.CheckDockerAccess(); err != nil {
		return fmt.Errorf("docker access check failed: %w", err)
	}

	if err := d.socketServer.Start(); err != nil {
		return fmt.Errorf("failed to start socket server: %w", err)
	}
	go d.socketServer.AcceptConnections()
	d.logger.Println("NextDeploy Daemon started successfully")
	go d.observerLoop()

	return d.handleSignals()
}

func (d *NextDeployDaemon) observerLoop() {
	ticker := time.NewTimer(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.checkDockerHealth()
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *NextDeployDaemon) checkDockerHealth() {
	if err := d.dockerClient.CheckDockerAccess(); err != nil {
		d.logger.Printf("Docker access check failed: %v", err)
	} else {
		d.logger.Println("Docker is accessible")
	}
}

func (d *NextDeployDaemon) handleSignals() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				d.logger.Println("Received SIGINT, shutting down...")
			case syscall.SIGTERM, syscall.SIGINT:
				d.logger.Println("Received SIGTERM, shutting down...")
				d.Shutdown()
				return nil
			}
		case <-d.ctx.Done():
			return nil
		}
	}
}

func (d *NextDeployDaemon) Shutdown() {
	d.logger.Println("Shutting down NextDeploy Daemon...")
	d.cancel()
	d.socketServer.Close()
	d.logger.Println("NextDeploy Daemon shut down gracefully")
}
