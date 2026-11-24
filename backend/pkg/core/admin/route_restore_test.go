package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestProjectRestoreRouteRegistered verifies the controlled restore endpoint is
// mounted and requires authentication: an anonymous POST must be rejected
// before any project state is touched.
func TestProjectRestoreRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/admin"))

	registered := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost && route.Path == "/admin/v1/projects/:id/restore" {
			registered = true
			break
		}
	}
	if !registered {
		t.Fatal("POST /admin/v1/projects/:id/restore is not registered")
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/projects/42/restore", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous restore must be rejected before auth: got status %d", resp.Code)
	}
	if resp.Code == http.StatusUnauthorized && !strings.Contains(resp.Body.String(), "ADMIN_AUTH_REQUIRED") {
		t.Fatalf("unexpected auth rejection body: %s", resp.Body.String())
	}
}
