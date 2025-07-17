package nextcore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func SendFakeData() error {
	data := map[string]interface{}{
		"app_name":     "contextbytes",
		"framework":    "Next.js",
		"build_target": "static",
		"env": []string{
			"NODE_ENV=production",
			"PORT=3000",
		},
		"domains": []string{
			"app.contextbytes.com",
		},
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	resp, err := http.Post("http://127.0.0.1:8371/nextcore/intake", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("post failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("📤 Sent fake NextCore data to daemon. Status:", resp.Status)
	return nil
}
