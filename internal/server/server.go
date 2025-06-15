package server

import (
	"nextdeploy/internal/config"
	"nextdeploy/internal/logger"
)

var (
	serverlogger = logger.PackageLogger("Server", "🅱)SERVERLOGGER")
)

type ServerStruct struct {
	config *config.NextDeployConfig
}

type Server func(*ServerStruct)

func WithConfig() Server {
	cfg, err := config.Load()
	if err != nil {
		serverlogger.Error("Failed to load configuration: %v", err)
		return nil
	}
	return func(s *ServerStruct) {
		s.config = cfg
		serverlogger.Info("Configuration loaded successfully")
		serverlogger.Debug("Config: %+v", s.config)

	}
}
