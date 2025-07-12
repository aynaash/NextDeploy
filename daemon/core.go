package daemon

import (
	"fmt"
	"nextdeploy/internal/nextcore"
)

// TODO: Later we can add: secrets, proxy, TLS, CDN setup here
func DeployApp(req DeployRequest) (DaemonResponse, error) {
	//TODO::
	//  -Pull image (docker.go)
	//  - Inject secrets (secrets.go)
	//  - Run container (docker.go)
	//  - Configure proxy (proxy.go)
	//  - Rotate certificates (certs.go) if needed
	//  - Track status
	//
	return DaemonResponse{
		Success: true,
		Message: "Deployment initiated",
		Payload: nil,
	}, nil
}

func StopApp(appName string) error {
	//TODO: stop container by name
	return nil
}

func RestartApp(appName string) error {
	//TODO: restart container by appName
	return nil
}

func DeployAppFromNextCore(data nextcore.NextCorePayload) error {
	app := data.AppName
	image := data.AppName + ":latest" // or from DockerHub
	port := data.Port

	// 1. Inject secrets
	env := ConvertSecretsToEnvVars(data.EnvVariables)
	if err := InjectSecrets(app, data.EnvVariables); err != nil {
		return err
	}

	// 2. Kill old container (if exists)
	existing, _ := FindContainerByAppName(app)
	if existing != "" {
		_ = KillContainer(existing)
	}

	// 3. Run container
	containerID, err := RunContainer(app, image, env, port)
	if err != nil {
		return err
	}

	// 4. Healthcheck
	status, _ := ProbeContainerHealth(containerID)
	if status != "healthy" {
		return fmt.Errorf("container unhealthy")
	}

	// 5. Reverse Proxy Config
	if err := ConfigureProxy(ProxyRoute{
		App:    app,
		Domain: data.Domain,
		Port:   port,
	}); err != nil {
		return err
	}

	// 6. TLS Cert
	if err := RotateCert(data.Domain); err != nil {
		return err
	}

	// 7. Register app
	RegisterApp(app, containerID, port)

	return nil
}
