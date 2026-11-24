package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFairLaunchSuccessUsesAdminEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fairLaunchSuccess(c, gin.H{"kind": "capabilities-set-action-execute"})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var body struct {
		Status bool           `json:"status"`
		Data   map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Status || body.Data["kind"] != "capabilities-set-action-execute" {
		t.Fatalf("unexpected envelope %s", w.Body.String())
	}
}
