
package secrets

import (
	"errors"
	"encoding/json"
	"os/exec"
	"fmt"
	"io"
	"net/http"
	"time"
)

func Load(env, provider string) (map[string]string, error) {
	switch provider {
	case "doppler":
		return fromDoppler(env)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func fromDoppler(env string) (map[string]string, error) {
	cmd := exec.Command("doppler", "secrets", "download", "--env", env, "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var secrets map[string]string
	if err := json.Unmarshal(out, &secrets); err != nil {
		return nil, err
	}
	return secrets, nil
}
type DopplerResponse struct {
	Project struct {
		ID          string  `json:"id"`
		Slug        string  `json:"slug"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
		CreatedAt   string  `json:"created_at"`
	} `json:"project"`
	Success bool `json:"success"`
}

// ValidateDopplerToken calls the Doppler API to check if the token is valid for the given project/config
func ValidateDopplerToken(token, project, config string) (*DopplerResponse, error) {
	if len(token) < 10 {
		return nil, errors.New("token too short to be valid")
	}

	url := fmt.Sprintf("https://api.doppler.com/v3/projects/project?project=%s", project)
	if config != "" {
		url += "&config=" + config
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doppler API returned status: %s", res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var dopplerResp DopplerResponse
	err = json.Unmarshal(body, &dopplerResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if !dopplerResp.Success {
		return nil, errors.New("doppler API indicated failure")
	}

	return &dopplerResp, nil
}
