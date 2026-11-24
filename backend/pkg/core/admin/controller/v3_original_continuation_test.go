package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOriginalContinuationRejectsBeforeAnySend(t *testing.T) {
	for _, handler := range []gin.HandlerFunc{V3OriginalPreflight, V3OriginalBroadcast} {
		for _, tc := range []struct {
			id, body string
			want     int
		}{{"0", `{}`, 400}, {"19", `{"tx_hash":"0x1","reason":"x"}`, 400}, {"19", `{"tx_hash":"0x1","reason":"audit reason"}`, 409}} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: tc.id}}
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			handler(c)
			if w.Code != tc.want {
				t.Fatal(w.Code, w.Body.String())
			}
		}
	}
}
