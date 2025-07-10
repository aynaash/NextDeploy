package daemon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func nextCoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	fmt.Println("🧠 Received NextCore data:")
	pretty, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(pretty))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("NextCore data received"))
}
