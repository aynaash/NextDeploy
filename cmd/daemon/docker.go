package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func PullImage(imageName string) error {
	//TODO: Implement logic to pull Docker image
	return nil
}

func RunContainer(appName, image string, env []string, port int) (string, error) {
	// Placeholder for actual implementation
	// This should run the Docker container and return the container ID
	return "container-id-placeholder", nil
}

func KillContainer(containerName string) error {
	return nil
}

var dockerCli *client.Client

func init() {
	var err error
	dockerCli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
}

func ListRunningContainers() ([]types.Container, error) {
	containers, err := dockerCli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	return containers, nil
}

// func ProbeContainerHealth(containerID string) (string, error) {
// 	inspect, err := dockerCli.ContainerInspect(context.Background(), containerID)
// 	if err != nil {
// 		return "unknown", err
// 	}
//
// 	if inspect.State == nil || inspect.State.Health == nil {
// 		return "no-healthcheck", nil
// 	}
//
// 	return inspect.State.Health.Status, nil
// }
