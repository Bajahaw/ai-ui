package auth

import (
	"ai-client/cmd/utils"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	logger "github.com/charmbracelet/log"
	"golang.org/x/crypto/bcrypt"
)

// MockUserRepository implements UserRepository for testing
type MockUserRepository struct {
	users map[string]*User
}

func (m *MockUserRepository) GetAll() []*User {
	var u []*User
	for _, user := range m.users {
		u = append(u, user)
	}
	return u
}

func (m *MockUserRepository) GetByUsername(username string) (*User, error) {
	if u, ok := m.users[username]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("User not found")
}

func (m *MockUserRepository) Save(user *User) error {
	if _, ok := m.users[user.Username]; ok {
		return fmt.Errorf("Username already exists")
	}
	m.users[user.Username] = user
	return nil
}

func (m *MockUserRepository) Update(user *User) error {
	if _, ok := m.users[user.Username]; !ok {
		return fmt.Errorf("User not found")
	}
	m.users[user.Username] = user
	return nil
}

func setupTest() *MockUserRepository {
	log = logger.New(os.Stderr)

	repo := &MockUserRepository{
		users: make(map[string]*User),
	}
	users = repo

	JWT_SECRET = "test-secret-key"

	return repo
}

func TestRegister(t *testing.T) {
	setupTest()

	tests := []struct {
		name           string
		payload        RegisterRequest
		expectedStatus int
	}{
		{
			name: "Valid Registration",
			payload: RegisterRequest{
				Username: "testuser",
				Password: "password123",
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "Missing Username",
			payload: RegisterRequest{
				Username: "",
				Password: "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Short Password",
			payload: RegisterRequest{
				Username: "testuser2",
				Password: "short",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			Register().ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	repo := setupTest()

	// Create test user
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	repo.users["testuser"] = &User{
		Username: "testuser",
		passHash: string(hash),
	}

	tests := []struct {
		name           string
		username       string
		password       string
		expectedStatus int
		expectCookie   bool
	}{
		{
			name:           "Valid Login",
			username:       "testuser",
			password:       "password123",
			expectedStatus: http.StatusOK,
			expectCookie:   true,
		},
		{
			name:           "Invalid Password",
			username:       "testuser",
			password:       "wrongpass",
			expectedStatus: http.StatusUnauthorized,
			expectCookie:   false,
		},
		{
			name:           "Non-existent User",
			username:       "unknown",
			password:       "password123",
			expectedStatus: http.StatusUnauthorized,
			expectCookie:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/login", nil)
			q := req.URL.Query()
			q.Add("username", tc.username)
			q.Add("password", tc.password)
			req.URL.RawQuery = q.Encode()

			// Handle form values
			req.ParseForm()
			req.Form.Add("username", tc.username)
			req.Form.Add("password", tc.password)

			w := httptest.NewRecorder()
			Login().ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectCookie {
				cookies := w.Result().Cookies()
				found := false
				for _, c := range cookies {
					if c.Name == AUTH_COOKIE {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected auth cookie not found")
				}
			}
		})
	}
}

func TestChangePassword(t *testing.T) {
	repo := setupTest()

	// Create test user
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.DefaultCost)
	repo.users["testuser"] = &User{
		Username: "testuser",
		passHash: string(oldHash),
	}

	tests := []struct {
		name           string
		username       string
		payload        map[string]string
		expectedStatus int
		checkHash      bool
	}{
		{
			name:     "Valid Password Change",
			username: "testuser",
			payload: map[string]string{
				"password": "newpassword123",
			},
			expectedStatus: http.StatusNoContent,
			checkHash:      true,
		},
		{
			name:     "Short Password",
			username: "testuser",
			payload: map[string]string{
				"password": "short",
			},
			expectedStatus: http.StatusInternalServerError, // hashPassword returns error which we map to 500
			checkHash:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/change-pass", bytes.NewBuffer(body))

			// Inject user context
			ctx := context.WithValue(req.Context(), "user", tc.username)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			http.HandlerFunc(UpdateUser).ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.checkHash {
				user, _ := repo.GetByUsername(tc.username)
				err := bcrypt.CompareHashAndPassword([]byte(user.passHash), []byte(tc.payload["password"]))
				if err != nil {
					t.Errorf("Password was not updated correctly")
				}
			}
		})
	}
}

func TestAuthStatus(t *testing.T) {
	setupTest()

	// Create a valid token
	token, _ := generateJWT("testuser")

	tests := []struct {
		name           string
		cookie         *http.Cookie
		expectedStatus int
		authenticated  bool
	}{
		{
			name: "Authenticated",
			cookie: &http.Cookie{
				Name:  AUTH_COOKIE,
				Value: token,
			},
			expectedStatus: http.StatusOK,
			authenticated:  true,
		},
		{
			name:           "No Cookie",
			cookie:         nil,
			expectedStatus: http.StatusOK,
			authenticated:  false,
		},
		{
			name: "Invalid Token",
			cookie: &http.Cookie{
				Name:  AUTH_COOKIE,
				Value: "invalid-token",
			},
			expectedStatus: http.StatusOK,
			authenticated:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/status", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			w := httptest.NewRecorder()
			GetAuthStatus().ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			var status AuthStatus
			json.NewDecoder(w.Body).Decode(&status)

			if status.Authenticated != tc.authenticated {
				t.Errorf("Expected authenticated=%v, got %v", tc.authenticated, status.Authenticated)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	setupTest()

	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()

	Logout().ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == AUTH_COOKIE {
			found = true
			if c.Value != "" {
				t.Error("Expected empty cookie value for logout")
			}
			if !c.Expires.Before(time.Now()) {
				t.Error("Expected cookie to be expired")
			}
		}
	}
	if !found {
		t.Error("Auth cookie not found in logout response")
	}
}

func TestAuthenticatedMiddleware(t *testing.T) {
	setupTest()
	token, _ := generateJWT("testuser")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := utils.ExtractContextUser(r)
		if username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", username)
		}
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		cookie         *http.Cookie
		expectedStatus int
	}{
		{
			name: "Valid Token",
			cookie: &http.Cookie{
				Name:  AUTH_COOKIE,
				Value: token,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "No Cookie",
			cookie:         nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid Token",
			cookie: &http.Cookie{
				Name:  AUTH_COOKIE,
				Value: "invalid",
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			w := httptest.NewRecorder()
			Authenticated(nextHandler).ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}
