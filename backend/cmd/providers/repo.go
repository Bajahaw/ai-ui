package providers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

var ErrUnauthorizedProviderReference = errors.New("unauthorized provider reference")

const ProviderTypeOpenAI = "openai"

type Provider struct {
	ID      string                    `json:"id"`
	Type    string                    `json:"type"`
	BaseURL string                    `json:"base_url"`
	APIKey  string                    `json:"api_key"`
	User    string                    `json:"-"`
	Headers map[string]string         `json:"headers"`
	OAuth   *chatgptoauth.Tokens      `json:"-"`
}

type Repository interface {
	GetAll(user string) []*Provider
	GetByID(id string, user string) (*Provider, error)
	Save(provider *Provider) error
	Upsert(provider *Provider) error
	DeleteByID(id string, user string) error
	SaveModels(models []*Model, user string) error
	GetAllModels(user string) []*Model
	GetModelsByProvider(providerID string) []*Model
	DeleteModelsNotIn(providerID string, modelIDs []string) error
}

type Repo struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &Repo{
		db: db,
	}
}

func scanProvider(id, baseURL, apiKey, headersJson, pType, oauthJson, user string) *Provider {
	var headers map[string]string
	if headersJson != "" {
		_ = json.Unmarshal([]byte(headersJson), &headers)
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	if pType == "" {
		pType = ProviderTypeOpenAI
	}
	var oauth *chatgptoauth.Tokens
	if oauthJson != "" {
		var t chatgptoauth.Tokens
		if err := json.Unmarshal([]byte(oauthJson), &t); err == nil && t.AccessToken != "" {
			oauth = &t
		}
	}
	return &Provider{
		ID:      id,
		Type:    pType,
		BaseURL: baseURL,
		APIKey:  apiKey,
		User:    user,
		Headers: headers,
		OAuth:   oauth,
	}
}

func (repo *Repo) GetAll(user string) []*Provider {
	var allProviders = make([]*Provider, 0)
	query := `SELECT id, url, api_key, headers_json, type, oauth_json FROM Providers WHERE user = ?`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying providers", "err", err)
		return allProviders
	}
	defer rows.Close()
	for rows.Next() {
		var id, baseURL, apiKey, headersJson, pType, oauthJson string
		if err = rows.Scan(&id, &baseURL, &apiKey, &headersJson, &pType, &oauthJson); err != nil {
			log.Error("Error scanning provider", "err", err)
			continue
		}
		allProviders = append(allProviders, scanProvider(id, baseURL, apiKey, headersJson, pType, oauthJson, user))
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over provider rows", "err", err)
	}

	return allProviders
}

func (repo *Repo) GetByID(id string, user string) (*Provider, error) {
	var baseURL, apiKey, headersJson, pType, oauthJson string
	query := `SELECT id, url, api_key, headers_json, type, oauth_json FROM Providers WHERE id = ? AND user = ?`
	var scannedID string
	err := repo.db.QueryRow(query, id, user).Scan(&scannedID, &baseURL, &apiKey, &headersJson, &pType, &oauthJson)
	if err != nil {
		return nil, err
	}
	return scanProvider(scannedID, baseURL, apiKey, headersJson, pType, oauthJson, user), nil
}

func providerOAuthJSON(p *Provider) string {
	if p.OAuth == nil {
		return ""
	}
	b, err := json.Marshal(p.OAuth)
	if err != nil {
		return ""
	}
	return string(b)
}

func (repo *Repo) Save(provider *Provider) error {
	if provider.Headers == nil {
		provider.Headers = make(map[string]string)
	}
	if provider.Type == "" {
		provider.Type = ProviderTypeOpenAI
	}
	headersBytes, _ := json.Marshal(provider.Headers)
	headersJson := string(headersBytes)

	query := `INSERT INTO Providers (id, url, api_key, user, headers_json, type, oauth_json) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(query, provider.ID, provider.BaseURL, provider.APIKey, provider.User, headersJson, provider.Type, providerOAuthJSON(provider))
	return err
}

func (repo *Repo) Upsert(provider *Provider) error {
	if provider.Headers == nil {
		provider.Headers = make(map[string]string)
	}
	if provider.Type == "" {
		provider.Type = ProviderTypeOpenAI
	}
	headersBytes, _ := json.Marshal(provider.Headers)
	headersJson := string(headersBytes)

	query := `INSERT INTO Providers (id, url, api_key, user, headers_json, type, oauth_json) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			url=excluded.url,
			api_key=excluded.api_key,
			headers_json=excluded.headers_json,
			type=excluded.type,
			oauth_json=excluded.oauth_json
		WHERE Providers.user=excluded.user`
	res, err := repo.db.Exec(query, provider.ID, provider.BaseURL, provider.APIKey, provider.User, headersJson, provider.Type, providerOAuthJSON(provider))
	if err != nil {
		return err
	}
	// Conflict with a different user's id: UPDATE WHERE fails and 0 rows change.
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("provider id conflict or unauthorized update")
	}
	return nil
}

func (repo *Repo) DeleteByID(id string, user string) error {
	query := `DELETE FROM Providers WHERE id = ? AND user = ?`
	_, err := repo.db.Exec(query, id, user)
	return err
}

func (repo *Repo) SaveModels(models []*Model, user string) error {
	if len(models) == 0 {
		return nil
	}

	// Use one transaction so ownership validation and upsert happen atomically.
	tx, err := repo.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	providerIDsMap := make(map[string]struct{})
	var upsertSQL strings.Builder
	upsertSQL.WriteString("INSERT INTO Models (id, provider_id, name, is_enabled) VALUES ")
	upsertArgs := make([]any, 0, len(models)*4)

	for i, m := range models {
		if m.ProviderID == "" {
			return fmt.Errorf("%w: missing provider id", ErrUnauthorizedProviderReference)
		}
		providerIDsMap[m.ProviderID] = struct{}{}

		if i > 0 {
			upsertSQL.WriteString(",")
		}
		upsertSQL.WriteString("(?, ?, ?, ?)")
		upsertArgs = append(upsertArgs, m.ID, m.ProviderID, m.Name, m.IsEnabled)
	}

	// Validate all distinct provider IDs in one DB call.
	validateQuery := "SELECT COUNT(1) FROM Providers WHERE user = ? AND id IN (" + utils.SqlPlaceholders(len(providerIDsMap)) + ")"
	validateArgs := make([]any, 0, len(providerIDsMap)+1)
	validateArgs = append(validateArgs, user)
	for providerID := range providerIDsMap {
		validateArgs = append(validateArgs, providerID)
	}

	var foundCount int
	err = tx.QueryRow(validateQuery, validateArgs...).Scan(&foundCount)
	if err != nil {
		return err
	}
	if foundCount != len(providerIDsMap) {
		return fmt.Errorf("%w: at least one provider does not belong to user", ErrUnauthorizedProviderReference)
	}

	// on conflict, update only when provider_id matches to prevent cross-provider overwrites.
	upsertSQL.WriteString(" ON CONFLICT(id) DO UPDATE SET is_enabled=excluded.is_enabled WHERE Models.provider_id=excluded.provider_id")

	_, err = tx.Exec(upsertSQL.String(), upsertArgs...)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (repo *Repo) GetAllModels(user string) []*Model {
	var models = make([]*Model, 0)
	query := `
		SELECT m.id, m.provider_id, m.name, m.is_enabled 
		FROM Models m
		JOIN Providers p ON m.provider_id = p.id
		WHERE p.user = ?
	`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying models", "err", err)
		return models
	}
	defer rows.Close()
	for rows.Next() {
		var m Model
		if err = rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.IsEnabled); err != nil {
			log.Error("Error scanning model", "err", err)
			continue
		}
		models = append(models, &Model{
			ID:         m.ID,
			Name:       m.Name,
			ProviderID: m.ProviderID,
			IsEnabled:  m.IsEnabled,
		})
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over model rows", "err", err)
	}

	return models
}

func (repo *Repo) GetModelsByProvider(providerID string) []*Model {
	var models = make([]*Model, 0)
	query := `SELECT id, provider_id, name, is_enabled FROM Models WHERE provider_id = ?`
	rows, err := repo.db.Query(query, providerID)
	if err != nil {
		log.Error("Error querying models by provider", "err", err)
		return models
	}
	defer rows.Close()
	for rows.Next() {
		var m Model
		if err = rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.IsEnabled); err != nil {
			log.Error("Error scanning model", "err", err)
			continue
		}
		models = append(models, &Model{
			ID:         m.ID,
			Name:       m.Name,
			ProviderID: m.ProviderID,
			IsEnabled:  m.IsEnabled,
		})
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over model rows by provider", "err", err)
	}
	return models
}

// DeleteModelsNotIn deletes models for a provider that are NOT in the provided list of model IDs.
func (repo *Repo) DeleteModelsNotIn(providerID string, modelIDs []string) error {
	if len(modelIDs) == 0 {
		_, err := repo.db.Exec("DELETE FROM Models WHERE provider_id = ?", providerID)
		return err
	}

	var sb strings.Builder
	sb.WriteString("DELETE FROM Models WHERE provider_id = ? AND id NOT IN (")
	args := make([]any, 0, len(modelIDs)+1)
	args = append(args, providerID)
	for i, id := range modelIDs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		args = append(args, id)
	}
	sb.WriteString(")")

	_, err := repo.db.Exec(sb.String(), args...)
	return err
}
