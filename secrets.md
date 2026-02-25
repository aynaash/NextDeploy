### How Doppler Auth Works Programmatically

Doppler has two token types you care about:

| Token Type | What It Is | Where It Lives |
|------------|------------|----------------|
| **Personal token** | Tied to your Doppler account. Full access. | Developer's machine only |
| **Service token** | Scoped to one project + config. Read-only secrets. | VPS, CI, containers |
| **CLI token** | What `doppler login` creates. | Developer's machine |

The CLI uses a personal or CLI token to let the developer set up the project. The daemon uses a service token to fetch secrets at deploy time. These are different flows with different tokens.

---

### The CLI Flow

When the developer runs `nextdeploy init` or `nextdeploy secrets link`, the CLI needs to talk to Doppler on their behalf. The flow is:

```
developer runs: nextdeploy secrets setup
CLI asks: what is your Doppler service token for this project?
developer pastes token from Doppler dashboard
CLI stores token in nextdeploy.yml under secrets block
CLI verifies token works by fetching secret names (not values)
CLI confirms: "Connected to Doppler project my-app / production"
```

You do not do OAuth or browser-based login at the CLI level. That is complexity you do not need. Service tokens are the right primitive — the developer creates one in the Doppler dashboard, scoped to the right project and config, and hands it to NextDeploy. This is the same pattern Doppler's own CI integrations use.

The CLI code for verifying a token:

```go
package doppler

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

const baseURL = "https://api.doppler.com/v3"

type Client struct {
    serviceToken string
    http         *http.Client
}

func NewClient(serviceToken string) *Client {
    return &Client{
        serviceToken: serviceToken,
        http: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// VerifyToken checks the token is valid and returns the project/config it has access to.
// Call this from the CLI during setup to give the developer immediate feedback.
func (c *Client) VerifyToken(ctx context.Context) (*TokenInfo, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet,
        baseURL+"/me",
        nil,
    )
    if err != nil {
        return nil, fmt.Errorf("build verify request: %w", err)
    }
    req.SetBasicAuth(c.serviceToken, "")

    resp, err := c.http.Do(req)
    if err != nil {
        return nil, fmt.Errorf("verify token request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return nil, fmt.Errorf("invalid service token — check the token in your Doppler dashboard")
    }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("doppler returned %d", resp.StatusCode)
    }

    var result struct {
        Token struct {
            Name        string `json:"name"`
            Project     string `json:"project"`
            Environment string `json:"environment"`
            Config      string `json:"config"`
        } `json:"token"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode verify response: %w", err)
    }

    return &TokenInfo{
        Name:        result.Token.Name,
        Project:     result.Token.Project,
        Environment: result.Token.Environment,
        Config:      result.Token.Config,
    }, nil
}

type TokenInfo struct {
    Name        string
    Project     string
    Environment string
    Config      string
}
```

The CLI calls this during setup and prints the result so the developer knows exactly which project and config the token has access to before they deploy anything.

---

### The Daemon Flow

The daemon fetches secrets at deploy time. The service token is stored in `nextdeploy.yml` — but only the token, never the secret values. The fetch happens in memory, secrets are injected into the container, and nothing is written to disk.

```go
// FetchSecrets fetches all secret values for the configured project and config.
// Returns a map of SECRET_NAME -> value.
// Call this immediately before starting the container. Hold in memory only.
func (c *Client) FetchSecrets(ctx context.Context) (map[string]string, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet,
        baseURL+"/configs/config/secrets/download",
        nil,
    )
    if err != nil {
        return nil, fmt.Errorf("build secrets request: %w", err)
    }

    req.SetBasicAuth(c.serviceToken, "")

    // format=json gives us key/value pairs directly
    // include_dynamic=false excludes computed secrets we cannot inject as env vars
    q := req.URL.Query()
    q.Set("format", "json")
    q.Set("include_dynamic", "false")
    req.URL.RawQuery = q.Encode()

    resp, err := c.http.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetch secrets request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return nil, fmt.Errorf("service token rejected — has it been revoked in Doppler?")
    }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("doppler returned %d when fetching secrets", resp.StatusCode)
    }

    var secrets map[string]string
    if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
        return nil, fmt.Errorf("decode secrets response: %w", err)
    }

    return secrets, nil
}
```

---

### Injecting Into Docker Without Touching Disk

This is the critical part. Most people write secrets to a temp file and pass `--env-file` to Docker. That file exists on disk, even briefly. The safer approach is passing each secret as an individual `--env` flag, built in memory:

```go
func buildDockerRunArgs(containerName string, secrets map[string]string, cfg *config.NextDeployConfig) []string {
    args := []string{
        "run", "-d",
        "--name", containerName,
        "--restart", "always",
    }

    // inject each secret as an individual env var
    // never written to disk — built in memory here
    for key, value := range secrets {
        args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
    }

    // non-secret env vars from config
    args = append(args, "--env", fmt.Sprintf("NODE_ENV=%s", cfg.App.Environment))
    args = append(args, "--env", fmt.Sprintf("PORT=%d", cfg.App.Port))

    // port binding
    args = append(args, "-p", fmt.Sprintf("127.0.0.1:%d:%d", cfg.App.Port, cfg.App.Port))

    args = append(args, cfg.Docker.Image)

    return args
}
```

Then the deploy call:

```go
func (d *DockerDeployer) Deploy(ctx context.Context, payload *Payload) (*DeployResult, error) {
    // fetch secrets immediately before container start
    // do not store on the payload, do not pass around
    secrets, err := d.secretsProvider.FetchSecrets(ctx)
    if err != nil {
        return nil, fmt.Errorf("fetch secrets for deploy: %w", err)
    }

    args := buildDockerRunArgs(payload.ContainerName, secrets, payload.Config)

    cmd := exec.CommandContext(ctx, "docker", args...)
    if out, err := cmd.CombinedOutput(); err != nil {
        return nil, fmt.Errorf("docker run failed: %w\noutput: %s", err, out)
    }

    // secrets map goes out of scope here and is garbage collected
    // it was never written to disk, never logged, never stored

    return &DeployResult{
        ContainerName: payload.ContainerName,
        DeployedAt:    time.Now(),
    }, nil
}
```

---

### Wiring It Into Your Config

The `nextdeploy.yml` structure for Doppler:

```yaml
secrets:
  provider: doppler
  token: dp.st.production.xxxxxxxxxxxx  # service token only
```

And the provider interface so Doppler is swappable:

```go
type SecretsProvider interface {
    FetchSecrets(ctx context.Context) (map[string]string, error)
    VerifyToken(ctx context.Context) (*TokenInfo, error)
}

// wire it up based on config
func NewSecretsProvider(cfg *config.SecretsConfig) (SecretsProvider, error) {
    switch cfg.Provider {
    case "doppler":
        if cfg.Token == "" {
            return nil, fmt.Errorf("doppler provider requires a service token — run: nextdeploy secrets setup")
        }
        return doppler.NewClient(cfg.Token), nil
    case "age":
        return age.NewProvider(cfg.KeyPath, cfg.EncryptedFile), nil
    case "":
        return nil, fmt.Errorf("no secrets provider configured — add a secrets block to nextdeploy.yml")
    default:
        return nil, fmt.Errorf("unknown secrets provider: %s", cfg.Provider)
    }
}
```

---

### One Thing to Be Careful About

Doppler service tokens appear in your `nextdeploy.yml`. That file should never be committed to git. Add this to your CLI's `init` command — automatically append `nextdeploy.yml` to `.gitignore` if it is not already there. Print a warning if the file is already tracked by git. This is the kind of guardrail that makes developers trust a tool.

```go
func ensureGitignored(projectDir string) error {
    gitignorePath := filepath.Join(projectDir, ".gitignore")

    content, err := os.ReadFile(gitignorePath)
    if err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("read .gitignore: %w", err)
    }

    if strings.Contains(string(content), "nextdeploy.yml") {
        return nil // already ignored
    }

    f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("open .gitignore: %w", err)
    }
    defer f.Close()

    _, err = f.WriteString("\n# NextDeploy\nnextdeploy.yml\n")
    return err
}
```

That function runs automatically at the end of `nextdeploy init`. The developer never has to think about it.
