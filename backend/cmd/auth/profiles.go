package auth

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/utils"
)

// ProfileInfo is a passwordless local profile. Profiles are rows in the
// Users table (like OAuth users) so all per-user data scoping keeps working.
type ProfileInfo struct {
	Username string `json:"username"`
}

// SelectProfileResponse is returned after selecting or creating a profile.
type SelectProfileResponse struct {
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
}

func profilesModeOnly(w http.ResponseWriter) bool {
	if authMode != AuthModeProfiles {
		http.Error(w, "Profiles are only available in profiles auth mode", http.StatusNotFound)
		return false
	}
	return true
}

// validateProfileName keeps names filesystem/URL friendly: 1-32 chars,
// letters, digits, space, underscore and dash.
func validateProfileName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 32 {
		return "", false
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == ' ' || r == '_' || r == '-' {
			continue
		}
		return "", false
	}
	return name, true
}

// Profiles lists local profiles. Unauthenticated on purpose: the picker needs
// it before any session exists. Profiles mode is for local/device use.
func Profiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !profilesModeOnly(w) {
			return
		}
		profiles := []ProfileInfo{}
		for _, u := range users.GetAll() {
			profiles = append(profiles, ProfileInfo{Username: u.Username})
		}
		utils.RespondWithJSON(w, profiles, http.StatusOK)
	}
}

// SelectProfile starts a session for an existing profile (no password).
func SelectProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !profilesModeOnly(w) {
			return
		}
		var req struct {
			Username string `json:"username"`
		}
		if err := utils.ExtractJSONBody(r, &req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		name, ok := validateProfileName(req.Username)
		if !ok {
			http.Error(w, "Invalid profile name", http.StatusBadRequest)
			return
		}
		if _, err := users.GetByUsername(name); err != nil {
			http.Error(w, "Profile not found", http.StatusNotFound)
			return
		}
		respondWithProfileSession(w, r, name)
	}
}

// CreateProfile creates a passwordless profile (running the same
// post-register hooks as signup so defaults + MCP servers are seeded) and
// selects it immediately.
func CreateProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !profilesModeOnly(w) {
			return
		}
		var req struct {
			Username string `json:"username"`
		}
		if err := utils.ExtractJSONBody(r, &req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		name, ok := validateProfileName(req.Username)
		if !ok {
			http.Error(w, "Invalid profile name (1-32 chars: letters, digits, space, _ -)", http.StatusBadRequest)
			return
		}
		username, created, err := EnsureOAuthUser(name)
		if err != nil {
			log.Error("Failed to create profile", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if created {
			for _, hook := range OnRegister {
				hook(username)
			}
		}
		respondWithProfileSession(w, r, username)
	}
}

// DeleteProfile removes a profile. DB rows cascade (conversations, files,
// providers, settings, ...); uploaded files are removed from disk too.
func DeleteProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !profilesModeOnly(w) {
			return
		}
		name, ok := validateProfileName(r.PathValue("username"))
		if !ok {
			http.Error(w, "Invalid profile name", http.StatusBadRequest)
			return
		}
		if _, err := users.GetByUsername(name); err != nil {
			http.Error(w, "Profile not found", http.StatusNotFound)
			return
		}
		removeProfileFiles(name)
		if _, err := db.Exec(`DELETE FROM Users WHERE username = ?`, name); err != nil {
			log.Error("Failed to delete profile", "username", name, "err", err)
			http.Error(w, "Failed to delete profile", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func respondWithProfileSession(w http.ResponseWriter, r *http.Request, username string) {
	if err := issueAuthCookie(w, r, username); err != nil {
		http.Error(w, "Failed to start session", http.StatusInternalServerError)
		return
	}
	token, err := generateAccessToken(username)
	if err != nil {
		http.Error(w, "Failed to start session", http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, SelectProfileResponse{Username: username, AccessToken: token}, http.StatusOK)
}

// removeProfileFiles best-effort deletes a profile's uploaded files and their
// thumbnails from disk. DB rows are removed by ON DELETE CASCADE.
func removeProfileFiles(username string) {
	rows, err := db.Query(`SELECT path FROM Files WHERE user = ?`, username)
	if err != nil {
		return
	}
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			paths = append(paths, p)
		}
	}
	_ = rows.Close()
	for _, p := range paths {
		_ = os.Remove(p)
		if base := path.Base(p); base != "" {
			if dot := strings.LastIndex(base, "."); dot > 0 {
				_ = os.Remove(path.Join("data", "resources", "thumbs", base[:dot]+".jpg"))
			}
		}
	}
}
