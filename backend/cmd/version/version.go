package version

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var AppVersion = "0.9.0"
var BuildID = ""

type VersionResponse struct {
	Version       string `json:"version"`
	Build         string `json:"build,omitempty"`
	BuildID       string `json:"build_id,omitempty"`
	SandboxOrigin string `json:"sandbox_origin,omitempty"`
}

func ParseSandboxOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host == "" || u.User != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func SandboxOrigin() string {
	return ParseSandboxOrigin(os.Getenv("SANDBOX_ORIGIN"))
}

func HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VersionResponse{
		Version:       AppVersion,
		BuildID:       BuildID,
		SandboxOrigin: SandboxOrigin(),
	})
}
