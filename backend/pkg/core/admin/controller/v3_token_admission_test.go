package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestV3TokenAdmissionFailsClosedWithoutReviewedRPCConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := viper.Get("idoV3TokenAdmission.enabled")
	viper.Set("idoV3TokenAdmission.enabled", false)
	t.Cleanup(func() { viper.Set("idoV3TokenAdmission.enabled", previous) })
	router := gin.New()
	router.GET("/token", V3TokenAdmissionStatus)
	router.GET("/token/:asset/receipt", V3TokenAdmissionReceipt)
	router.GET("/project-tokens/:address", V3ProjectTokenStatus)
	router.GET("/project-tokens/:address/receipt", V3ProjectTokenReceipt)
	router.POST("/project-tokens/:address/safe-proposal", V3ProjectTokenSafeProposal)
	for _, tc := range []struct {
		path string
		code int
	}{
		{"/token", http.StatusServiceUnavailable},
		{"/token/usdc/receipt?tx_hash=0x" + strings.Repeat("a", 64), http.StatusBadRequest},
		{"/token/usdt/receipt?tx_hash=not-a-hash", http.StatusUnprocessableEntity},
		{"/token/usdt/receipt?tx_hash=0x" + strings.Repeat("a", 64), http.StatusUnprocessableEntity},
		{"/project-tokens/garbage", http.StatusBadRequest},
		{"/project-tokens/garbage/receipt?tx_hash=0x" + strings.Repeat("a", 64), http.StatusBadRequest},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), `"status":false`) {
			t.Errorf("%s: status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
	// A new address alone is not authority to propose an Owner Safe transaction.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/project-tokens/0x1234567890123456789012345678901234567890/safe-proposal", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "SAFE_SIGNATURE_REQUIRED") {
		t.Fatalf("unsigned project token proposal accepted: %d %s", w.Code, w.Body.String())
	}
}
