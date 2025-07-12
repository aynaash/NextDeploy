package communication

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type DaemonClient struct {
	BaseURL string
}

func NewDaemonClient() *DaemonClient {
	return &DaemonClient{
		BaseURL: "http://127.0.0.1:8371",
	}
}

func (c *DaemonClient) DeployApp(req DeployRequest) (DaemonResponse, error) {
	data, _ := json.Marshal(req)
	resp, err := http.Post(c.BaseURL+"/deploy", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return DaemonResponse{}, err
	}
	defer resp.Body.Close()

	var out DaemonResponse
	json.NewDecoder(resp.Body).Decode(&out)
	return out, nil
}
