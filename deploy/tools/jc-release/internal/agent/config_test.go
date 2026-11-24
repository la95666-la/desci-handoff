package agent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigRequiresAbsoluteHTTPOrHTTPS(t *testing.T) {
	bad := []string{
		"ftp://example.com", "https://", "https://user@example.com",
		"https://example.com/#fragment", "https://example.com/path", "https://example.com?x=1", "://bad",
	}
	for _, raw := range bad {
		t.Run(strings.ReplaceAll(raw, "/", "_"), func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, "server_url: \""+raw+"\"\n")); err == nil {
				t.Fatalf("accepted %q", raw)
			}
		})
	}
	if _, err := LoadConfig(writeConfig(t, "server_url: https://example.com\ninsecure_skip_verify: true\n")); err == nil {
		t.Fatal("insecure_skip_verify accepted")
	}
	legacy, err := LoadConfig(writeConfig(t, "server_url: https://example.com\ninsecure_skip_verify: false\n"))
	if err != nil || legacy.InsecureSkipVerify {
		t.Fatalf("legacy false compatibility: %#v %v", legacy, err)
	}
	cfg, err := LoadConfig(writeConfig(t, "server_url: https://example.com\n"))
	if err != nil || cfg.ServerURL != "https://example.com" {
		t.Fatalf("valid config: %#v %v", cfg, err)
	}
	for _, raw := range []string{"http://127.0.0.1:8080", "http://10.0.0.8", "http://172.26.10.135:8080", "http://192.168.1.9", "http://8.8.8.8", "http://release.example.com:8080"} {
		cfg, err := LoadConfig(writeConfig(t, "server_url: \""+raw+"\"\n"))
		if err != nil || cfg.ServerURL != raw {
			t.Fatalf("private HTTP %q: %#v %v", raw, cfg, err)
		}
	}
}

func TestClientRejectsUnsafeURLBeforeTokenRequest(t *testing.T) {
	var called atomic.Bool
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		called.Store(true)
		return nil, nil
	})
	for _, raw := range []string{"ftp://example.com", "https://user@example.com", "https://example.com/#x", "not-a-url"} {
		if _, err := newClient(raw, "secret-token", &http.Client{Transport: transport}); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if called.Load() {
		t.Fatal("transport called before URL validation")
	}
}

func TestHTTPClientIsAllowedWithoutDisablingTLSVerification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"data":[]}`))
	}))
	defer server.Close()
	client, err := NewTestClientAllowLoopbackHTTP(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.PollCommands("agent-1"); err != nil {
		t.Fatal(err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
