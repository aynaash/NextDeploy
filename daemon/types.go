package main

import (
	"context"
	"net"
)

type Command struct {
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ContainerInfo struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	Ports   map[string]string `json:"ports"`
	Created string            `json:"created"`
	Labels  map[string]string `json:"labels"`
}

// Daemon struct
type NextDeployDaemon struct {
	ctx        context.Context
	cancel     context.CancelFunc
	listener   net.Listener
	socketPath string
	config     *DaemonConfig
}

type DaemonConfig struct {
	SocketPath      string   `json:"socket_path"`
	SocketMode      string   `json:"socket_mode"`
	AllowedUsers    []string `json:"allowed_users"`
	DockerSocket    string   `json:"docker_socket"`
	ContainerPrefix string   `json:"container_prefix"`
	LogLevel        string   `json:"log_level"`
	LogDir          string   `json:"log_dir"`         // New
	LogMaxSize      int      `json:"log_max_size"`    // New - in MB
	LogMaxBackups   int      `json:"log_max_backups"` // New

}
