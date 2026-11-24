package controller

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"joincare_server/pkg/common/v3diagnostic"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestV3HTTPDiagnosticKeepsLegacyCodeAndRedactsCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/project", nil)
	e := v3diagnostic.New("CREATE_DEADLINE_EXPIRED", "signature_persist", errors.New("https://private.example/SECRET raw_tx password"))
	if !respondV3Diagnostic(c, 400, "PROJECT_CREATE_ONCHAIN_FAILED", e) {
		t.Fatal("not handled")
	}
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatal(w.Body.String())
	}
	if body["code"] != "PROJECT_CREATE_ONCHAIN_FAILED" || body["reason_code"] != "CREATE_DEADLINE_EXPIRED" || body["stage"] != "signature_persist" || len(body["request_id"].(string)) != 24 {
		t.Fatal(body)
	}
	if strings.Contains(w.Body.String(), "SECRET") || w.Header().Get("Cache-Control") != "no-store, private" {
		t.Fatal("sensitive response", w.Body.String())
	}
}
