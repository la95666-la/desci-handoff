package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

func newReleaseShutdownAPI(t *testing.T) (*API, shared.Agent) {
	t.Helper()
	store, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ag := shared.Agent{ID: "agent-1", Token: "0123456789abcdef0123456789abcdef", Hostname: "host", IP: "127.0.0.1", RegisteredAt: time.Now(), LastSeenAt: time.Now(), Deployed: map[shared.Component]string{}}
	if err := store.AddAgent(ag); err != nil {
		t.Fatal(err)
	}
	return &API{Store: store, Cfg: &Config{SessionSecret: "0123456789abcdef"}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, ag
}
func responseCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var r shared.APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r.Code
}
func TestUnsupportedUploadFailsBeforeArtifactWrite(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	f, _ := mw.CreateFormFile("file", "x.tar.gz")
	f.Write([]byte("arbitrary"))
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	a.handleUploadRelease(rec, req)
	if rec.Code != http.StatusBadRequest || responseCode(t, rec) != "INVALID_COMPONENT" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	entries, _ := a.Store.ListReleases("", 10)
	if len(entries) != 0 {
		t.Fatal("release persisted")
	}
}
func TestMissingReleaseDeleteReturnsNotFound(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	rec := httptest.NewRecorder()
	a.handleReleaseDetail(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/releases/historical", nil))
	if rec.Code != http.StatusNotFound || responseCode(t, rec) != "NOT_FOUND" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestMissingArtifactReturnsNotFound(t *testing.T) {
	a, agent := newReleaseShutdownAPI(t)
	rec := httptest.NewRecorder()
	a.handleArtifact(rec, httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/historical", nil), agent)
	if rec.Code != http.StatusNotFound || responseCode(t, rec) != "NOT_FOUND" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestUpgradeCommandsBindReleaseComponent(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	front := shared.Release{ID: "front", Component: shared.CompFrontend, UploadedAt: time.Now()}
	admin := shared.Release{ID: "admin", Component: shared.CompAdmin, UploadedAt: time.Now()}
	if err := a.Store.addReleaseFixture(front); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.addReleaseFixture(admin); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{shared.CmdUpgrade} {
		for _, tc := range []struct {
			release   string
			component shared.Component
		}{{"front", shared.CompAdmin}, {"admin", shared.CompFrontend}} {
			body := bytes.NewBufferString(`{"kind":"` + kind + `","component":"` + string(tc.component) + `","release_id":"` + tc.release + `"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/commands", body)
			rec := httptest.NewRecorder()
			a.handleQueueCommand(rec, req)
			if rec.Code != http.StatusConflict || responseCode(t, rec) != "RELEASE_COMPONENT_MISMATCH" {
				t.Fatalf("%s %s->%s: %d %s", kind, tc.release, tc.component, rec.Code, rec.Body.String())
			}
		}
	}
}
func TestStorageRejectsReleaseWithoutArtifact(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	if err := a.Store.AddRelease(shared.Release{ID: "blocked", Component: shared.CompFrontend}); err == nil {
		t.Fatalf("release storage boundary: %v", err)
	}
}

func TestStorageRejectsMalformedReleaseCommand(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	for _, kind := range []string{shared.CmdUpgrade, shared.CmdRollback} {
		err := a.Store.AddCommand(shared.Command{ID: kind, AgentID: "agent-1", Kind: kind, Component: shared.CompFrontend, Status: shared.StatusPending})
		if err == nil {
			t.Fatalf("%s storage boundary: %v", kind, err)
		}
	}
}

func TestImplementedReleaseCommandsQueue(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	front := shared.Release{ID: "front", Component: shared.CompFrontend, UploadedAt: time.Now()}
	if err := a.Store.addReleaseFixture(front); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{shared.CmdUpgrade, shared.CmdRestart} {
		if kind != shared.CmdUpgrade {
			cmds, err := a.Store.ListCommands("agent-1", 10)
			if err != nil {
				t.Fatal(err)
			}
			for _, cmd := range cmds {
				if err := a.Store.UpdateCommand(cmd.ID, func(c *shared.Command) error { c.Status = shared.StatusFailed; return nil }); err != nil {
					t.Fatal(err)
				}
			}
		}
		release := ""
		if kind != shared.CmdRestart {
			release = "front"
		}
		body := bytes.NewBufferString(`{"kind":"` + kind + `","component":"frontend","release_id":"` + release + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/commands", body)
		rec := httptest.NewRecorder()
		a.handleQueueCommand(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", kind, rec.Code, rec.Body.String())
		}
	}
}

func TestRollbackTargetReleaseIDCannotProveLocalBackup(t *testing.T) {
	a, _ := newReleaseShutdownAPI(t)
	if err := a.Store.addReleaseFixture(shared.Release{ID: "front", Component: shared.CompFrontend}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/commands", bytes.NewBufferString(`{"kind":"rollback","component":"frontend","release_id":"front"}`))
	rec := httptest.NewRecorder()
	a.handleQueueCommand(rec, req)
	if rec.Code != http.StatusBadRequest || responseCode(t, rec) != "ROLLBACK_TARGET_UNVERIFIED" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
