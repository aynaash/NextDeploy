package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"nextdeploy/shared/nextcore"
)

func HandleNextCoreIntake(w http.ResponseWriter, r *http.Request) {
	var payload nextcore.NextCorePayload

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid request payload: %v", err),
			Payload: nil,
		})

	}
	log.Printf("Received NextCore intake payload: %+v", payload)
	// TODO: save to disk map to request or auto-deploy
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "NextCore metadata  recieved",
		Payload: payload,
	})
}
