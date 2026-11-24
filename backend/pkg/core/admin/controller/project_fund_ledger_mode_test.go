package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLegacyV2FundLedgerErrorClearlyRejectsFairLaunch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fundProjectReadError(c, errors.New("fund ledger is available only for V2 stablecoin projects"))
	if w.Code != http.StatusConflict || !containsAll(w.Body.String(), "FUND_LEDGER_NOT_V2", "V2 stablecoin projects") {
		t.Fatalf("unexpected V2-only rejection: %d %s", w.Code, w.Body.String())
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
