package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOnChainProjectRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/admin"))

	for _, path := range []string{
		"/admin/v1/projects/123/ido-v3/policy/preview",
		"/admin/v1/projects/123/ido-v3/policy/reviews",
		"/admin/v1/projects/123/ido-v3/policy/reviews/4/approve",
		"/admin/v1/projects/123/ido-v3/policy/reviews/4/observe",
		"/admin/v1/projects/123/create-onchain",
		"/admin/v1/projects/123/activate-onchain",
		"/admin/v1/chain-events/failures/123/replay",
		"/admin/v1/chain-transactions/123/cancel",
		"/admin/v1/projects/123/onchain-reconciliations",
		"/admin/v1/onchain-reconciliations/456/cancel",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s must require admin auth: got status %d", path, resp.Code)
		}
	}
}

func TestChainEventReadRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/admin"))
	for _, path := range []string{
		"/admin/v1/projects/123/ido-v3/policy/status",
		"/admin/v1/projects/123/ido-v3/policy/reviews/4",
		"/admin/v1/chain-events/health",
		"/admin/v1/chain-events/failures",
		"/admin/v1/chain-events/failures/123",
		"/admin/v1/chain-transactions/summary",
		"/admin/v1/chain-transactions",
		"/admin/v1/chain-transactions/123",
		"/admin/v1/projects/123/fund-ledger",
		"/admin/v1/projects/123/purchase-entitlements",
		"/admin/v1/projects/123/fund-reconciliation",
		"/admin/v1/projects/123/onchain-reconciliation",
		"/admin/v1/projects/123/onchain-reconciliations",
		"/admin/v1/onchain-reconciliations/456",
		"/admin/v1/onchain-reconciliations/456/wallets",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s must require admin auth: got status %d", path, resp.Code)
		}
	}
}
