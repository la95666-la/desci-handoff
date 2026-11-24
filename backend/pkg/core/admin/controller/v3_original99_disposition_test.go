package controller

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOriginal99RequiresExactConfirmationBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/recovery", V3Original99Disposition)
	for _, body := range []string{`{}`, `{"chain_id":56,"nonce":82,"target":"0xfd712061ee08e070a930ecd45e935ae5d7f5445c","gas":21000,"gas_price_raw":"100000000","value_raw":"0","reason":"authorized"}`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/recovery", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "ORIGINAL99_EXACT_CONFIRMATION_REQUIRED") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
