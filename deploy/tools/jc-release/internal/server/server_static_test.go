package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestServeStaticDisablesBrowserCache(t *testing.T) {
	previous := webAssets
	webAssets = fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("Service Logs")}}
	t.Cleanup(func() { webAssets = previous })
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	serveStatic(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestProductionUIRestoresReleaseWorkflowWithoutUnsafeActions(t *testing.T) {
	root := filepath.Join("..", "..", "cmd", "jc-release-server", "web")
	index, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile(filepath.Join(root, "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	combined := string(index) + string(app)
	for _, required := range []string{"doUpload", "upgradeAgent", "renderAgentCard", "genEnrollToken", "rel-comp", "log-agent", "toggleLog", "modal-content", "service-log-target", "up-comp", `value="backend"`, `value="frontend"`, `value="admin"`, `value="job"`} {
		if !strings.Contains(combined, required) {
			t.Fatalf("UI missing release control %q", required)
		}
	}
	start := strings.Index(string(index), `<select id="service-log-target">`)
	if start < 0 {
		t.Fatal("service log target selector missing")
	}
	end := strings.Index(string(index)[start:], "</select>")
	if end < 0 {
		t.Fatal("service log target selector unclosed")
	}
	logTargets := string(index)[start : start+end]
	for _, target := range []string{`value="backend"`, `value="job"`, `value="tgbot"`, `value="mysql"`, `value="hardhat"`} {
		if strings.Contains(logTargets, target) {
			t.Fatalf("UI retains protected log target %q", target)
		}
	}
	if strings.Contains(string(index), "static_release_admission_unavailable") || strings.Contains(string(app), "kind: 'rollback'") || !strings.Contains(string(app), "回滚不可用") || !strings.Contains(string(index), "IDO Job 镜像身份由上传制品自动提取") {
		t.Fatal("UI advertises an incorrect release boundary")
	}
}
