package providers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	logger "github.com/charmbracelet/log"
	_ "modernc.org/sqlite"
)

func setupProvidersTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := data.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"alice", "bob"} {
		if _, err := db.Exec(`INSERT INTO Users (username, pass_hash) VALUES (?, 'x')`, u); err != nil {
			t.Fatal(err)
		}
	}
	SetupProviderClient(logger.New(io.Discard), db)
	return db
}

// mockModelsServer serves an OpenAI-compatible /models list and records request headers.
func mockModelsServer(t *testing.T) (*httptest.Server, func() http.Header) {
	t.Helper()
	var mu sync.Mutex
	var last http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		last = r.Header.Clone()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"m1","object":"model","created":0,"owned_by":"x"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() http.Header {
		mu.Lock()
		defer mu.Unlock()
		return last
	}
}

func doSave(t *testing.T, user string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(raw))
	req = req.WithContext(context.WithValue(req.Context(), "user", user))
	rr := httptest.NewRecorder()
	saveProvider(rr, req)
	return rr
}

func TestSaveProvider_RedactsAndUpdatesInPlace(t *testing.T) {
	db := setupProvidersTest(t)
	srv, lastHeaders := mockModelsServer(t)

	rr := doSave(t, "alice", map[string]any{
		"base_url": srv.URL,
		"api_key":  "sk-original",
		"headers":  map[string]string{"x-api-key": "hdr-secret", "X-Org": "acme"},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body)
	}
	var created Response
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created.Headers["x-api-key"] != "" || created.Headers["X-Org"] != "" || len(created.Headers) != 2 {
		t.Fatalf("create response leaked header values: %v", created.Headers)
	}

	// List never exposes credentials.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", "alice"))
	list := httptest.NewRecorder()
	getProvidersList(list, req)
	if body := list.Body.String(); strings.Contains(body, "hdr-secret") || strings.Contains(body, "sk-original") || strings.Contains(body, "acme") {
		t.Fatalf("list leaked credentials: %s", body)
	}

	// Edit: blank key and blank header keep stored values; changed header is applied.
	rr = doSave(t, "alice", map[string]any{
		"id":       created.ID,
		"base_url": srv.URL,
		"api_key":  "",
		"headers":  map[string]string{"x-api-key": "", "X-Org": "new-org"},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rr.Code, rr.Body)
	}

	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM Providers WHERE user = 'alice'`).Scan(&count)
	if count != 1 {
		t.Fatalf("edit should update in place, got %d providers", count)
	}
	p, err := providers.GetByID(created.ID, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if p.APIKey != "sk-original" || p.Headers["x-api-key"] != "hdr-secret" || p.Headers["X-Org"] != "new-org" {
		t.Fatalf("stored after edit: key=%q headers=%v", p.APIKey, p.Headers)
	}
	h := lastHeaders()
	if h.Get("Authorization") != "Bearer sk-original" || h.Get("X-Api-Key") != "hdr-secret" {
		t.Fatalf("upstream request used wrong credentials: %v", h)
	}
	if models := providers.GetModelsByProvider(created.ID); len(models) != 1 {
		t.Fatalf("expected models to be synced, got %d", len(models))
	}

	// Another user cannot update this provider.
	rr = doSave(t, "bob", map[string]any{"id": created.ID, "base_url": srv.URL, "api_key": "x"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("cross-user update: %d", rr.Code)
	}
}

func TestToResponse_ChatGPTUsesLabel(t *testing.T) {
	r := toResponse(&Provider{ID: "cgpt-1", Type: "chatgpt-oauth", Headers: map[string]string{"label": "me@example.com"}})
	if r.Label != "me@example.com" || r.Headers != nil {
		t.Fatalf("got %+v", r)
	}
}
