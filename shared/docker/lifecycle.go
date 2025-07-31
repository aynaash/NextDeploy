package docker

// import (
// 	"archive/tar"
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"os"
// 	"path/filepath"
//
// 	"github.com/docker/docker/api/types"
// 	"github.com/docker/docker/api/types/container"
// 	"github.com/docker/docker/api/types/network"
// 	"github.com/docker/docker/api/types/strslice"
// 	"github.com/docker/docker/client"
// 	"github.com/docker/go-connections/nat"
// )
//
// type DockerClient struct {
// 	Raw *client.Client
// }
//
// func NewDockerClient() (*DockerClient, error) {
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &DockerClient{Raw: cli}, nil
// }
//
// func (d *DockerClient) BuildImageFromSpec(ctx context.Context, spec ImageSpec) (string, error) {
// 	tarBuf := new(bytes.Buffer)
// 	tw := tar.NewWriter(tarBuf)
//
// 	_ = filepath.Walk(spec.ContextPath, func(path string, info os.FileInfo, err error) error {
// 		if err != nil || info.IsDir() {
// 			return nil
// 		}
// 		relPath, _ := filepath.Rel(spec.ContextPath, path)
// 		file, err := os.Open(path)
// 		if err != nil {
// 			return nil
// 		}
// 		defer file.Close()
//
// 		hdr, err := tar.FileInfoHeader(info, "")
// 		if err != nil {
// 			return nil
// 		}
// 		hdr.Name = relPath
// 		_ = tw.WriteHeader(hdr)
// 		_, _ = io.Copy(tw, file)
// 		return nil
// 	})
// 	tw.Close()
// 	tag := fmt.Sprintf("%s:%s", spec.Name, spec.Tag)
// 	options := types.ImageBuildOptions{Tags: []string{tag}, Dockerfile: "Dockerfile", Remove: true}
// 	res, err := d.Raw.ImageBuild(ctx, bytes.NewReader(tarBuf.Bytes()), options)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer res.Body.Close()
// 	io.Copy(os.Stdout, res.Body)
// 	return tag, nil
// }
//
// func ToDockerContainerConfig(spec ContainerSpec) (*container.Config, *container.HostConfig) {
// 	envs := []string{}
// 	for k, v := range spec.Env {
// 		envs = append(envs, k+"="+v)
// 	}
//
// 	hostConfig := &container.HostConfig{
// 		Binds:        []string{},
// 		PortBindings: nat.PortMap{},
// 	}
//
// 	for host, container := range spec.Volumes {
// 		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", host, container))
// 	}
//
// 	for hostPort, containerPort := range spec.Ports {
// 		port := nat.Port(containerPort + "/tcp")
// 		hostConfig.PortBindings[port] = []nat.PortBinding{{HostPort: hostPort}}
// 	}
//
// 	return &container.Config{
// 		Image:      spec.Image,
// 		Env:        envs,
// 		Entrypoint: strslice.StrSlice(spec.Entrypoint),
// 		WorkingDir: spec.WorkingDir,
// 	}, hostConfig
// }
//
// func (d *DockerClient) CreateAndStartContainer(ctx context.Context, spec ContainerSpec) (string, error) {
// 	config, hostConfig := ToDockerContainerConfig(spec)
// 	resp, err := d.Raw.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, spec.Name)
// 	if err != nil {
// 		return "", err
// 	}
// 	err = d.Raw.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
// 	return resp.ID, err
// }
//
// func (d *DockerClient) StreamLogs(ctx context.Context, containerID string) error {
// 	reader, err := d.Raw.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
// 	if err != nil {
// 		return err
// 	}
// 	defer reader.Close()
// 	_, err = io.Copy(os.Stdout, reader)
// 	return err
// }
//
// func (d *DockerClient) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
// 	exec, err := d.Raw.ContainerExecCreate(ctx, containerID, types.ExecConfig{
// 		Cmd:          cmd,
// 		AttachStdout: true,
// 		AttachStderr: true,
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	resp, err := d.Raw.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Close()
// 	_, err = io.Copy(os.Stdout, resp.Reader)
// 	return err
// }
//
// func (d *DockerClient) GetContainerStats(ctx context.Context, containerID string) error {
// 	stats, err := d.Raw.ContainerStatsOneShot(ctx, containerID)
// 	if err != nil {
// 		return err
// 	}
// 	defer stats.Body.Close()
// 	_, err = io.Copy(os.Stdout, stats.Body)
// 	return err
// }
//
// func (d *DockerClient) InspectContainerHealth(ctx context.Context, containerID string) (string, error) {
// 	info, err := d.Raw.ContainerInspect(ctx, containerID)
// 	if err != nil {
// 		return "", err
// 	}
// 	if info.State != nil && info.State.Health != nil {
// 		return string(info.State.Health.Status), nil
// 	}
// 	return "unknown", nil
// }
//
// func (d *DockerClient) PushImage(ctx context.Context, image string) error {
// 	res, err := d.Raw.ImagePush(ctx, image, types.ImagePushOptions{RegistryAuth: ""})
// 	if err != nil {
// 		return err
// 	}
// 	defer res.Close()
// 	_, err = io.Copy(os.Stdout, res)
// 	return err
// }
//
// func (d *DockerClient) PullImage(ctx context.Context, image string) error {
// 	res, err := d.Raw.ImagePull(ctx, image, build.ImagePullOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	defer res.Close()
// 	_, err = io.Copy(os.Stdout, res)
// 	return err
// }
