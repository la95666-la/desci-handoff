package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestV3DiagnosticDoesNotBypassApprovedSetSubmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/:id/onchain-params", SubmitProjectOnchainParams)
	request := `{"contract_version":"ido_v3_stablecoin","payment_token":"USDT",` +
		`"token_address":"0xf77c5b5faabb9945ac8701ad08ef09e12dfc768a",` +
		`"owner_address":"0xfd712061ee08E070A930eCD45E935Ae5d7f5445C",` +
		`"token_price":"1","hard_cap":"10","soft_cap":"5","max_per_user":"1",` +
		`"start_time":"2030-01-01T00:00:00Z","end_time":"2030-01-02T00:00:00Z",` +
		`"tge":[{"percent":100}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/42630852370235393/onchain-params", strings.NewReader(request))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "V3 approved Set prerequisites unavailable") {
		t.Fatalf("V3 diagnostic queued a Set write: %d %s", w.Code, w.Body.String())
	}
}

func TestV3ApprovedSetSubmissionFailsClosedWithPartialMode(t *testing.T) {
	old := viper.Get("idoV3Approved.enabled")
	t.Cleanup(func() { viper.Set("idoV3Approved.enabled", old) })
	viper.Set("idoV3Approved.enabled", true)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/:id/onchain-params", SubmitProjectOnchainParams)
	request := `{"contract_version":"ido_v3_stablecoin","payment_token":"USDT",` +
		`"token_address":"0xf77c5b5faabb9945ac8701ad08ef09e12dfc768a",` +
		`"owner_address":"0xfd712061ee08E070A930eCD45E935Ae5d7f5445C",` +
		`"token_price":"1","hard_cap":"10","soft_cap":"5","max_per_user":"1",` +
		`"start_time":"2030-01-01T00:00:00Z","end_time":"2030-01-02T00:00:00Z",` +
		`"tge":[{"percent":100}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/42630852370235393/onchain-params", strings.NewReader(request))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "V3 approved Set prerequisites unavailable") {
		t.Fatalf("partial mode queued approval: %d %s", w.Code, w.Body.String())
	}
}
