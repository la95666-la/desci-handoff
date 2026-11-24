package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProjectMutationRoutesAreNotAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/client"))

	for _, path := range []string{
		"/client/project/v1/create",
		"/client/project/v1/active",
		"/client/project/v1/agree",
		"/client/project/v1/reject",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s must not be exposed anonymously: got status %d", path, resp.Code)
		}
	}
}

func TestV3SingleWalletProofRouteRequiresUserAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/client"))
	req := httptest.NewRequest(http.MethodGet,
		"/client/project/v1/ido-v3/93/proof?wallet=0x1111111111111111111111111111111111111111", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous V3 proof route returned %d", resp.Code)
	}
}

func TestV3PortfolioRouteRequiresUserAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/client"))
	req := httptest.NewRequest(http.MethodGet,
		"/client/project/v1/ido-v3/portfolio?wallet=0x1111111111111111111111111111111111111111", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous V3 portfolio route returned %d", resp.Code)
	}
}

func TestProjectApplicationRequiresUserAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/client"))

	req := httptest.NewRequest(http.MethodPost, "/client/project/v1/new", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusPreconditionRequired && resp.Code != http.StatusUnauthorized {
		t.Fatalf("project application must be rejected before auth: got status %d", resp.Code)
	}
}

func TestPointFlowHistoryRequiresUserAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/client"))

	req := httptest.NewRequest(http.MethodGet, "/client/taskhall/v1/pointFlows?page=1&page_size=8", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusPreconditionRequired && resp.Code != http.StatusUnauthorized {
		t.Fatalf("point flow history must be rejected before auth: got status %d", resp.Code)
	}
}
