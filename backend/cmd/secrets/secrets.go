package secrets

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxSecretNameLen  = 64
	maxSecretValueLen = 8 << 10 // 8 KiB
	maxSecretsPerUser = 100
)

// Placeholder form used in tool args: $secrets.NAME$
// JSON-safe, unambiguous, easy to substitute server-side only.
var secretPlaceholderRE = regexp.MustCompile(`\$secrets\.([A-Z][A-Z0-9_]*)\$`)

var (
	ErrInvalidName  = errors.New("invalid secret name")
	ErrInvalidValue = errors.New("invalid secret value")
	ErrNotFound     = errors.New("secret not found")
	ErrLimit        = errors.New("secret limit reached")
)

// Secret is the internal row (value never sent in list responses).
type Secret struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"-"`
	User  string `json:"-"`
}

// SecretResponse is metadata only — values are write-only after create.
type SecretResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SecretListResponse struct {
	Secrets []SecretResponse `json:"secrets"`
}

// SecretRequest creates or updates a secret.
// On update, empty Value keeps the existing value.
type SecretRequest struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NormalizeName uppercases and validates secret names.
// Allowed: A-Z, 0-9, underscore; must start with a letter; no spaces; max 64 chars.
func NormalizeName(name string) (string, error) {
	if strings.ContainsAny(name, " \t\n\r") {
		return "", ErrInvalidName
	}
	n := strings.ToUpper(strings.TrimSpace(name))
	if n == "" || len(n) > maxSecretNameLen {
		return "", ErrInvalidName
	}
	if n[0] < 'A' || n[0] > 'Z' {
		return "", ErrInvalidName
	}
	for i := 1; i < len(n); i++ {
		c := n[i]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return "", ErrInvalidName
	}
	return n, nil
}

func validateValue(value string) error {
	if value == "" {
		return ErrInvalidValue
	}
	if !utf8.ValidString(value) {
		return ErrInvalidValue
	}
	if len(value) > maxSecretValueLen {
		return ErrInvalidValue
	}
	return nil
}

// Expand replaces $secrets.NAME$ in s using the user's secret map (name -> value).
// Expansion is single-pass (values are not re-scanned). Unknown names error.
func Expand(s string, byName map[string]string) (string, error) {
	if s == "" || !strings.Contains(s, "$secrets.") {
		return s, nil
	}

	var firstErr error
	out := secretPlaceholderRE.ReplaceAllStringFunc(s, func(match string) string {
		if firstErr != nil {
			return match
		}
		sub := secretPlaceholderRE.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := sub[1]
		val, ok := byName[name]
		if !ok {
			firstErr = fmt.Errorf("unknown secret %q", name)
			return match
		}
		return val
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// ExpandForUser loads the user's secrets and expands placeholders in s.
func ExpandForUser(s, user string) (string, error) {
	if s == "" || !strings.Contains(s, "$secrets.") {
		return s, nil
	}
	return Expand(s, GetValueMap(user))
}
