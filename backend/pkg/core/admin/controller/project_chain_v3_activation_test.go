package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestV3ActivationInsufficientTokensResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondV3InsufficientTokens(c)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", w.Code)
	}
	var body struct {
		Status  bool   `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status || body.Code != "PROJECT_TOKEN_RESERVE_INSUFFICIENT" ||
		!strings.Contains(body.Message, "项目 Owner 钱包") ||
		!strings.Contains(body.Message, "depositProjectTokens") ||
		!strings.Contains(body.Message, "不要直接转账") {
		t.Fatalf("unexpected business response: %+v", body)
	}
}
