package middleware

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestPermissionDiagnosticDeniesBeforeBusinessHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// A nonempty CSV containing no usable roles cannot hit the DB or allow.
	router := gin.New()
	called := false
	router.POST("/preview", func(c *gin.Context) { c.Set(CtxAllRoles, ",") }, RequirePerm(ModProject, ActWrite), func(c *gin.Context) { called = true })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/preview", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 403 || called || body["code"] != "ADMIN_PERM_DENIED" || body["required_permission"] != "m2_project:write" {
		t.Fatal(w.Code, called, body)
	}
}
