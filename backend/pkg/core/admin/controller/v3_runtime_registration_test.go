package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestV3RuntimeRegistrationFailsBeforeApprovalWithoutReviewedRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := viper.Get("idoV3TokenAdmission.enabled")
	viper.Set("idoV3TokenAdmission.enabled", false)
	t.Cleanup(func() { viper.Set("idoV3TokenAdmission.enabled", old) })
	router := gin.New()
	router.POST("/ido-v3/runtime-registration", SubmitV3RuntimeRegistration)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ido-v3/runtime-registration", nil))
	if w.Code != http.StatusUnprocessableEntity || w.Body.String() != `{"code":"V3_RUNTIME_UNAVAILABLE","status":false}` {
		t.Fatalf("registration was not fail-closed: code=%d body=%s", w.Code, w.Body.String())
	}
}
