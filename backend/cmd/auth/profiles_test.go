package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	logger "github.com/charmbracelet/log"
	_ "modernc.org/sqlite"
	"os"
)

// setupProfilesTest wires profiles mode with a real temp DB (DeleteProfile
// uses db directly for cascade + disk cleanup).
func setupProfilesTest(t *testing.T) {
	t.Helper()
	log = logger.New(os.Stderr)

	tmp := t.TempDir()
	dsn := tmp + "/test.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	var err error
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE Users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		pass_hash TEXT NOT NULL
	);`)
	if err != nil {
		t.Fatalf("create users table: %v", err)
	}
	users = NewUserRepository(db)

	JWT_SECRET = "test-secret-key"
	authMode = AuthModeProfiles
	t.Cleanup(func() { authMode = AuthModePassword })

	hooked := []string{}
	OnRegister = []PostRegisterHook{func(u string) { hooked = append(hooked, u) }}
	t.Cleanup(func() { OnRegister = nil })
}

func TestProfilesEmptyList(t *testing.T) {
	setupProfilesTest(t)

	rr := httptest.NewRecorder()
	Profiles().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/auth/profiles", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var list []ProfileInfo
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %v", list)
	}
}

func TestCreateSelectDeleteProfile(t *testing.T) {
	setupProfilesTest(t)

	// Invalid names rejected.
	for _, bad := range []string{"", "   ", "a/b", strings.Repeat("x", 33)} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/profiles/create", strings.NewReader(`{"username":`+quote(bad)+`}`))
		req.Header.Set("Content-Type", "application/json")
		CreateProfile().ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("name %q: expected 400, got %d", bad, rr.Code)
		}
	}

	// Create.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/profiles/create", strings.NewReader(`{"username":"Work"}`))
	req.Header.Set("Content-Type", "application/json")
	CreateProfile().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var created SelectProfileResponse
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Username != "Work" || created.AccessToken == "" {
		t.Fatalf("bad create response: %+v", created)
	}
	if len(OnRegister) != 1 {
		t.Fatalf("expected OnRegister hooks preserved")
	}

	// List shows it.
	rr = httptest.NewRecorder()
	Profiles().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/auth/profiles", nil))
	var list []ProfileInfo
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil || len(list) != 1 || list[0].Username != "Work" {
		t.Fatalf("bad list: %v %v", list, err)
	}

	// Select issues cookie + token.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/profiles/select", strings.NewReader(`{"username":"Work"}`))
	req.Header.Set("Content-Type", "application/json")
	SelectProfile().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("select: expected 200, got %d", rr.Code)
	}
	if cookie := rr.Header().Get("Set-Cookie"); !strings.Contains(cookie, AUTH_COOKIE+"=") {
		t.Fatalf("select did not set cookie: %q", cookie)
	}

	// access_token authorizes like the cookie.
	var sel SelectProfileResponse
	if err := json.NewDecoder(rr.Body).Decode(&sel); err != nil {
		t.Fatalf("decode select: %v", err)
	}
	gotUser := ""
	probe := Authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Context().Value("user").(string)
	}))
	rr2 := httptest.NewRecorder()
	probe.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/chat/?access_token="+sel.AccessToken, nil))
	if gotUser != "Work" {
		t.Fatalf("access_token did not authenticate, user=%q", gotUser)
	}

	// Select missing profile -> 404.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/profiles/select", strings.NewReader(`{"username":"Nobody"}`))
	req.Header.Set("Content-Type", "application/json")
	SelectProfile().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("select missing: expected 404, got %d", rr.Code)
	}

	// Delete (204), then gone.
	rr = httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/api/auth/profiles/Work", nil)
	delReq.SetPathValue("username", "Work")
	DeleteProfile().ServeHTTP(rr, delReq)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d (%s)", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	Profiles().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/auth/profiles", nil))
	list = nil
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil || len(list) != 0 {
		t.Fatalf("list after delete: %v %v", list, err)
	}
}

func TestPasswordEndpointsDisabledInProfilesMode(t *testing.T) {
	setupProfilesTest(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"username":"u","password":"12345678"}`))
	req.Header.Set("Content-Type", "application/json")
	Register().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("register: expected 403, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	form := strings.NewReader("username=u&password=12345678")
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Login().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("login: expected 403, got %d", rr.Code)
	}
}

func TestCookieAttributesAdaptToTransport(t *testing.T) {
	setupProfilesTest(t)

	// Plain HTTP LAN host: no Secure, Lax.
	rr := httptest.NewRecorder()
	setAuthCookie(rr, httptest.NewRequest(http.MethodGet, "http://192.168.1.10:8080/", nil), "tok")
	c := rr.Header().Get("Set-Cookie")
	if strings.Contains(c, "Secure") {
		t.Fatalf("plain http cookie must not be Secure: %q", c)
	}
	if !strings.Contains(c, "SameSite=Lax") {
		t.Fatalf("plain http cookie should be Lax: %q", c)
	}

	// HTTPS: Secure + None.
	rr = httptest.NewRecorder()
	setAuthCookie(rr, httptest.NewRequest(http.MethodGet, "https://example.com/", nil), "tok")
	c = rr.Header().Get("Set-Cookie")
	if !strings.Contains(c, "Secure") || !strings.Contains(c, "SameSite=None") {
		t.Fatalf("https cookie should be Secure+None: %q", c)
	}

	// Loopback HTTP (in-app API server): Secure + None so the
	// cross-site WebView can send it back.
	rr = httptest.NewRecorder()
	setAuthCookie(rr, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil), "tok")
	c = rr.Header().Get("Set-Cookie")
	if !strings.Contains(c, "Secure") || !strings.Contains(c, "SameSite=None") {
		t.Fatalf("loopback cookie should be Secure+None: %q", c)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
