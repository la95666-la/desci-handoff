package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"joincare_server/pkg/common/constant"
)

func TestFairLaunchMutationControllersRejectBeforeBodyMFAOrDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handler := range map[string]gin.HandlerFunc{
		"set submission":   SubmitFairLaunchSet,
		"action execution": ExecuteFairLaunchAction,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "1"}}
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			handler(c)
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), constant.FairLaunchMaterializationUnavailable) {
				t.Fatalf("mutation controller escaped shutdown: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
