package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
	"golang.org/x/crypto/bcrypt"
)

func newPublicAPI(t *testing.T) *API {
	t.Helper()
	store, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("valid-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &API{Store: store, Cfg: &Config{
		AdminUsername: "admin", AdminPasswordBcrypt: string(hash), SessionSecret: "0123456789abcdef",
	}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func storageSnapshot(t *testing.T, api *API) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(api.Store.Root, agentsFile))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLoginJSONBoundaryAndExactRoute(t *testing.T) {
	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{"method", http.MethodGet, "/api/v1/auth/login", `{"username":"admin","password":"valid-password"}`, http.StatusMethodNotAllowed},
		{"path", http.MethodPost, "/api/v1/auth/login/extra", `{"username":"admin","password":"valid-password"}`, http.StatusNotFound},
		{"unknown", http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"valid-password","extra":1}`, http.StatusBadRequest},
		{"trailing", http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"valid-password"} {}`, http.StatusBadRequest},
		{"malformed", http.MethodPost, "/api/v1/auth/login", `{`, http.StatusBadRequest},
		{"oversized", http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"` + strings.Repeat("x", int(maxLoginBodyBytes)) + `"}`, http.StatusRequestEntityTooLarge},
		{"username", http.MethodPost, "/api/v1/auth/login", `{"username":" admin","password":"valid-password"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newPublicAPI(t)
			rec := httptest.NewRecorder()
			api.handleLogin(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("failure set a session cookie")
			}
		})
	}
}

func TestRegisterJSONBoundaryNoStateMutation(t *testing.T) {
	validToken := strings.Repeat("a", 48)
	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{"method", http.MethodGet, "/api/v1/register", `{"enroll_token":"` + validToken + `","hostname":"host-1"}`, http.StatusMethodNotAllowed},
		{"path", http.MethodPost, "/api/v1/register/extra", `{"enroll_token":"` + validToken + `","hostname":"host-1"}`, http.StatusNotFound},
		{"unknown", http.MethodPost, "/api/v1/register", `{"enroll_token":"` + validToken + `","hostname":"host-1","extra":1}`, http.StatusBadRequest},
		{"trailing", http.MethodPost, "/api/v1/register", `{"enroll_token":"` + validToken + `","hostname":"host-1"} {}`, http.StatusBadRequest},
		{"malformed", http.MethodPost, "/api/v1/register", `{`, http.StatusBadRequest},
		{"oversized", http.MethodPost, "/api/v1/register", `{"enroll_token":"` + validToken + `","hostname":"` + strings.Repeat("x", int(maxRegisterBodyBytes)) + `"}`, http.StatusRequestEntityTooLarge},
		{"token", http.MethodPost, "/api/v1/register", `{"enroll_token":"not token","hostname":"host-1"}`, http.StatusBadRequest},
		{"hostname", http.MethodPost, "/api/v1/register", `{"enroll_token":"` + validToken + `","hostname":"bad/host"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newPublicAPI(t)
			if err := api.Store.AddEnrollToken(shared.EnrollToken{Token: validToken, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
				t.Fatal(err)
			}
			before := storageSnapshot(t, api)
			rec := httptest.NewRecorder()
			api.handleRegister(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			after := storageSnapshot(t, api)
			if string(before) != string(after) {
				t.Fatal("invalid registration mutated agent/enrollment state")
			}
		})
	}
}

func TestStorageRejectsInvalidRegistrationBeforeMutation(t *testing.T) {
	api := newPublicAPI(t)
	token := strings.Repeat("b", 48)
	if err := api.Store.AddEnrollToken(shared.EnrollToken{Token: token, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	before := storageSnapshot(t, api)
	err := api.Store.RegisterAgent(token, shared.Agent{ID: "agent-1", Hostname: "bad/host", IP: "127.0.0.1", Token: strings.Repeat("c", 64), Deployed: map[shared.Component]string{}})
	if err == nil {
		t.Fatal("invalid registration accepted")
	}
	if after := storageSnapshot(t, api); string(before) != string(after) {
		t.Fatal("storage invalid registration mutated state")
	}
	var data agentsData
	if err := json.Unmarshal(before, &data); err != nil || data.EnrollTokens[0].UsedAt != nil {
		t.Fatal("invalid registration consumed token")
	}
}
