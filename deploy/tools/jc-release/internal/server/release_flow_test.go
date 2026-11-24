package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

func uploadFixture(t *testing.T, api *API, component string, payload []byte) (int, shared.Release) {
	return uploadJobFixture(t, api, component, payload, "")
}

func uploadJobFixture(t *testing.T, api *API, component string, payload []byte, admission string) (int, shared.Release) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range map[string]string{"component": component, "version": "test-1"} {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if admission != "" {
		if err := mw.WriteField("job_admission", admission); err != nil {
			t.Fatal(err)
		}
	}
	f, err := mw.CreateFormFile("file", "artifact.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	api.handleUploadRelease(rec, req)
	var result struct {
		Data shared.Release `json:"data"`
	}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code, result.Data
}

func TestJobUploadRequiresDeclarationAndPreservesExactAgentIdentity(t *testing.T) {
	api, ag := newReleaseShutdownAPI(t)
	if code, _ := uploadFixture(t, api, "job", []byte("image")); code != http.StatusBadRequest {
		t.Fatalf("invalid image accepted: %d", code)
	}
	if code, _ := uploadJobFixture(t, api, "frontend", []byte("image"), `{}`); code != http.StatusBadRequest {
		t.Fatalf("job admission on frontend accepted: %d", code)
	}
	if code, _ := uploadJobFixture(t, api, "job", []byte("image"), "not json"); code != http.StatusBadRequest {
		t.Fatalf("invalid JSON accepted: %d", code)
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	config := []byte(`{"architecture":"amd64","os":"linux"}`)
	configDigest := sha256.Sum256(config)
	configName := hex.EncodeToString(configDigest[:]) + ".json"
	manifest, _ := json.Marshal([]map[string]any{{"Config": configName, "RepoTags": []string{"joincare/job:test"}, "Layers": []string{"layer/layer.tar"}}})
	for _, entry := range []struct {
		name string
		body []byte
	}{{configName, config}, {"layer/layer.tar", []byte("layer")}, {"manifest.json", manifest}} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	code, rel := uploadFixture(t, api, "job", archive.Bytes())
	if code != http.StatusOK {
		t.Fatalf("job upload %d", code)
	}
	get := httptest.NewRecorder()
	api.handleAgentRelease(get, httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+ag.ID+"/releases/"+rel.ID, nil), ag)
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"image_id":"sha256:`+hex.EncodeToString(configDigest[:])+`"`) {
		t.Fatalf("job declaration lost: %d %s", get.Code, get.Body.String())
	}
	// Server derives image identity; Agent checks loaded image and runtime
	// isolation before any Job mutation.
}

func TestJobImageIDAcceptsCurrentDockerOCISaveLayout(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	config := []byte(`{"architecture":"amd64","os":"linux"}`)
	digest := sha256.Sum256(config)
	name := "blobs/sha256/" + hex.EncodeToString(digest[:])
	layer := "blobs/sha256/" + strings.Repeat("a", 64)
	manifest, _ := json.Marshal([]map[string]any{{"Config": name, "RepoTags": []string{"joincare/job:test"}, "Layers": []string{layer}}})
	for _, entry := range []struct {
		name string
		body []byte
	}{{name, config}, {layer, []byte("layer")}, {"manifest.json", manifest}} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	file := t.TempDir() + "/job.tar.gz"
	if err := os.WriteFile(file, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := jobImageID(file)
	if err != nil || got != "sha256:"+hex.EncodeToString(digest[:]) {
		t.Fatalf("OCI save image id = %q, %v", got, err)
	}
}

// Opt-in validation against a freshly built CI-shaped docker-save gzip.
func TestJobImageIDRealPackage(t *testing.T) {
	archive := os.Getenv("JC_JOB_IMAGE_PACKAGE")
	if archive == "" { t.Skip("requires a reviewed local Job image package") }
	image, err := jobImageID(archive)
	if err != nil || image != os.Getenv("JC_JOB_IMAGE_ID") { t.Fatalf("real image id = %q, err=%v", image, err) }
}

func TestReleaseUploadDownloadAndExactHistoricalAgentLookup(t *testing.T) {
	api, ag := newReleaseShutdownAPI(t)
	for _, comp := range []shared.Component{shared.CompBackend, shared.CompFrontend, shared.CompAdmin} {
		payload := []byte("archive identity for " + comp)
		code, rel := uploadFixture(t, api, string(comp), payload)
		if code != http.StatusOK {
			t.Fatalf("%s upload code=%d", comp, code)
		}
		sum := sha256.Sum256(payload)
		if rel.Component != comp || rel.Size != int64(len(payload)) || rel.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("release=%+v", rel)
		}
		get := httptest.NewRecorder()
		api.handleAgentRelease(get, httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+ag.ID+"/releases/"+rel.ID, nil), ag)
		if get.Code != http.StatusOK {
			t.Fatalf("exact lookup %d %s", get.Code, get.Body.String())
		}
		get = httptest.NewRecorder()
		api.handleArtifact(get, httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/"+rel.ID, nil), ag)
		if get.Code != http.StatusOK || !bytes.Equal(get.Body.Bytes(), payload) {
			t.Fatalf("artifact %d %s", get.Code, get.Body.String())
		}
	}
	for _, comp := range []string{"job", "tgbot", "lang"} {
		if code, _ := uploadFixture(t, api, comp, []byte("not supported")); code != http.StatusBadRequest {
			t.Fatalf("%s admitted: %d", comp, code)
		}
	}
}

func TestStoredArtifactMutationAndCommandConflictsFailClosed(t *testing.T) {
	api, ag := newReleaseShutdownAPI(t)
	_, rel := uploadFixture(t, api, "frontend", []byte("original"))
	if err := os.WriteFile(api.Store.ArtifactPath(rel.ID), []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	get := httptest.NewRecorder()
	api.handleArtifact(get, httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/"+rel.ID, nil), ag)
	if get.Code != http.StatusConflict || responseCode(t, get) != "ARTIFACT_MISMATCH" {
		t.Fatalf("corrupt artifact %d %s", get.Code, get.Body.String())
	}
	cmd := shared.Command{ID: "one", AgentID: ag.ID, Kind: shared.CmdUpgrade, Component: shared.CompFrontend, ReleaseID: rel.ID, Status: shared.StatusPending, CreatedAt: time.Now()}
	if err := api.Store.AddCommand(cmd); err != nil {
		t.Fatal(err)
	}
	inUse, err := api.Store.ReleaseInUse(rel.ID)
	if err != nil || !inUse {
		t.Fatalf("pending release can be deleted: %v %v", inUse, err)
	}
	cmd.ID = "two"
	if err := api.Store.AddCommand(cmd); err == nil || !strings.Contains(err.Error(), "already_active") {
		t.Fatalf("concurrent command: %v", err)
	}
	cmd.Kind = shared.CmdRollback
	if err := api.Store.AddCommand(cmd); err == nil || err.Error() != "rollback_target_unverified" {
		t.Fatalf("rollback: %v", err)
	}
}
