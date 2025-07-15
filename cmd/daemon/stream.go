package main

import (
	"net/http"
)

func HandleLogStream(w http.ResponseWriter, r *http.Request) {
	// TODO: Upgrade connection
	// Attach to container logs
	// Stream log lines to WS client
}

func HandleMetricsStream(w http.ResponseWriter, r *http.Request) {
	// TODO: Periodic system metrics -> JSON -> WS
}

// package daemon
//
// import (
// 	"context"
// 	"fmt"
// 	"io"
// 	"net/http"
//
// 	"github.com/docker/docker/api/types"
// 	"github.com/gorilla/websocket"
// )
//
// var upgrader = websocket.Upgrader{}
//
// func HandleLogStream(w http.ResponseWriter, r *http.Request) {
// 	app := r.URL.Query().Get("app")
// 	if app == "" {
// 		http.Error(w, "App is required", http.StatusBadRequest)
// 		return
// 	}
//
// 	container, err := FindContainerByAppName(app)
// 	if err != nil {
// 		http.Error(w, "Container not found", http.StatusNotFound)
// 		return
// 	}
//
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		http.Error(w, "Failed to upgrade WS", 500)
// 		return
// 	}
// 	defer conn.Close()
//
// 	reader, err := dockerCli.ContainerLogs(context.Background(), container.ID, types.ContainerLogsOptions{
// 		ShowStdout: true,
// 		ShowStderr: true,
// 		Follow:     true,
// 		Timestamps: false,
// 	})
// 	if err != nil {
// 		conn.WriteMessage(websocket.TextMessage, []byte("Failed to stream logs"))
// 		return
// 	}
// 	defer reader.Close()
//
// 	buf := make([]byte, 1024)
// 	for {
// 		n, err := reader.Read(buf)
// 		if err != nil {
// 			if err != io.EOF {
// 				conn.WriteMessage(websocket.TextMessage, []byte("Log stream error"))
// 			}
// 			break
// 		}
// 		conn.WriteMessage(websocket.TextMessage, buf[:n])
// 	}
// }
