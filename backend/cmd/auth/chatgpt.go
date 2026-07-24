package auth

import (
	"net/http"

	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

// ChatGPTProviderSaver is set by main/providers to avoid import cycles.
// It saves or updates a ChatGPT OAuth provider for the given user and returns provider id.
var ChatGPTProviderSaver func(username string, tokens *chatgptoauth.Tokens) (providerID string, model string, err error)

type chatgptStartResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type chatgptStatusResponse struct {
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
	Username   string `json:"username,omitempty"`
	Model      string `json:"model,omitempty"`
	Created    bool   `json:"created,omitempty"`
}

// optionalUserFromCookie returns username if a valid auth cookie is present.
func optionalUserFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(AUTH_COOKIE)
	if err != nil {
		return ""
	}
	claims, err := extractClaims(cookie.Value)
	if err != nil {
		return ""
	}
	username, ok := claimUsername(claims)
	if !ok {
		return ""
	}
	return username
}

func startChatGPTLogin(w http.ResponseWriter, r *http.Request) {
	// If already logged in, attach provider to that user; otherwise full sign-in.
	username := optionalUserFromCookie(r)
	redirectURI := chatgptoauth.ResolveRedirectURI(r)
	authURL, state, err := chatgptoauth.DefaultLoginManager.Start(username, redirectURI)
	if err != nil {
		log.Error("Failed to start ChatGPT OAuth", "err", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	log.Debug("ChatGPT OAuth started", "redirect_uri", redirectURI, "state", state)
	utils.RespondWithJSON(w, chatgptStartResponse{AuthURL: authURL, State: state}, http.StatusOK)
}

// handleChatGPTCallback is the public OAuth redirect target on this app's domain.
// OpenAI redirects the browser here with ?code=&state= after consent.
func handleChatGPTCallback(w http.ResponseWriter, r *http.Request) {
	chatgptoauth.DefaultLoginManager.HandleCallback(w, r)
}

func pollChatGPTLogin(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}

	// Peek first so we don't consume while still pending
	peek := chatgptoauth.DefaultLoginManager.Peek(state)
	if peek.Status == chatgptoauth.StatusPending {
		utils.RespondWithJSON(w, chatgptStatusResponse{Status: "pending"}, http.StatusOK)
		return
	}

	pending := chatgptoauth.DefaultLoginManager.Consume(state)
	if pending.Status == chatgptoauth.StatusError {
		utils.RespondWithJSON(w, chatgptStatusResponse{
			Status: "error",
			Error:  pending.Error,
		}, http.StatusOK)
		return
	}

	if pending.Tokens == nil {
		utils.RespondWithJSON(w, chatgptStatusResponse{
			Status: "error",
			Error:  "missing tokens",
		}, http.StatusOK)
		return
	}

	username := pending.Username
	created := false
	if username != "" {
		// Connect mode: state was bound to an existing session at Start.
		// Re-require that same user so a leaked state cannot attach tokens
		// to someone else's account.
		caller := optionalUserFromCookie(r)
		if caller == "" || caller != username {
			utils.RespondWithJSON(w, chatgptStatusResponse{
				Status: "error",
				Error:  "session mismatch; sign in again and reconnect ChatGPT",
			}, http.StatusOK)
			return
		}
	} else {
		// Sign-in mode: map ChatGPT account to a local user (account-id based).
		username = chatgptoauth.UsernameFromAccount(pending.Tokens.AccountID, "")
		var err error
		username, created, err = EnsureOAuthUser(username)
		if err != nil {
			log.Error("Failed to ensure OAuth user", "err", err)
			utils.RespondWithJSON(w, chatgptStatusResponse{
				Status: "error",
				Error:  "failed to create user: " + err.Error(),
			}, http.StatusOK)
			return
		}
		if created {
			for _, hook := range OnRegister {
				hook(username)
			}
		}
		if err := IssueSession(w, username); err != nil {
			log.Error("Failed to issue session", "err", err)
			utils.RespondWithJSON(w, chatgptStatusResponse{
				Status: "error",
				Error:  "failed to create session",
			}, http.StatusOK)
			return
		}
	}

	if ChatGPTProviderSaver == nil {
		utils.RespondWithJSON(w, chatgptStatusResponse{
			Status: "error",
			Error:  "ChatGPT provider saver not configured",
		}, http.StatusOK)
		return
	}

	providerID, model, err := ChatGPTProviderSaver(username, pending.Tokens)
	if err != nil {
		log.Error("Failed to save ChatGPT provider", "err", err)
		utils.RespondWithJSON(w, chatgptStatusResponse{
			Status: "error",
			Error:  "failed to save provider: " + err.Error(),
		}, http.StatusOK)
		return
	}

	utils.RespondWithJSON(w, chatgptStatusResponse{
		Status:     "success",
		ProviderID: providerID,
		Username:   username,
		Model:      model,
		Created:    created,
	}, http.StatusOK)
}
