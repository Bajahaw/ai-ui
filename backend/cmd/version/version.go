package version

import (
	"encoding/json"
	"net/http"
)

var AppVersion = "0.1"

type VersionResponse struct {
	Version string `json:"version"`
}

func HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VersionResponse{Version: AppVersion})
}
