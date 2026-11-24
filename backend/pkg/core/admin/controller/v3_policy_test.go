package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/service/project"
)

func TestV3PolicyPreviewRequiresCreatedProject(t *testing.T) {
	base := project.OnchainParamsCurrent{Version: constant.ContractVersionIDOV3Stablecoin,
		FundingMode: constant.FundingModeCappedPresale, Status: constant.ProjectStatusSet}
	if !v3PolicyProjectNotCreated(&base) {
		t.Fatal("uncreated approved Set was not identified")
	}
	base.Locked = true
	if v3PolicyProjectNotCreated(&base) {
		t.Fatal("locked project was called uncreated")
	}
	base.Locked = false
	base.Version = constant.ContractVersionV2Stablecoin
	if v3PolicyProjectNotCreated(&base) || v3PolicyProjectNotCreated(nil) {
		t.Fatal("unrelated identity was called uncreated V3")
	}
}

func TestV3PolicyDisplayStatusRejectsDisabledModeAndIsNotCached(t *testing.T) {
	old := viper.Get("idoV3Approved.enabled")
	viper.Set("idoV3Approved.enabled", false)
	t.Cleanup(func() { viper.Set("idoV3Approved.enabled", old) })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/projects/:id/ido-v3/policy/status", GetV3PolicyStatus)
	for _, id := range []string{"0", "43359438123302913"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/projects/"+id+"/ido-v3/policy/status", nil))
		if w.Header().Get("Cache-Control") != "no-store, private" {
			t.Fatal("display cached")
		}
		if id == "0" && w.Code != http.StatusBadRequest || id != "0" && w.Code != http.StatusConflict {
			t.Fatal("invalid display mode admitted", w.Code)
		}
	}
}

func TestV3PolicyPreviewRejectsMalformedRequestWithoutCandidateWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/projects/:id/ido-v3/policy/preview", PreviewV3InitialPolicy)
	for _, path := range []string{"/projects/0/ido-v3/policy/preview", "/projects/9/ido-v3/policy/preview"} {
		body := `{"wallets":[],"reason":"review"}`
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "V3_POLICY_") {
			t.Fatal("malformed V3 preview was not rejected before a candidate write")
		}
	}
}

func TestV3ReviewRejectsMissingImmutableBaseWithoutMFA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/projects/:id/ido-v3/policy/reviews", StartV3PolicyReview)
	for _, body := range []string{
		`{"wallets":["0x1111111111111111111111111111111111111111"],"enabled":true,"reason":"review","preview_digest":"0x1111"}`,
		`{"wallets":[],"enabled":false,"reason":"close","base_block":"90","base_hash":"","preview_digest":"0x1111"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/projects/9/ido-v3/policy/reviews", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("unbound review reached service: %d", response.Code)
		}
	}
}

func TestV3PolicyReviewAcceptsNoMFAFieldAtHTTPBoundary(t *testing.T) {
	// A well-formed request without MFA reaches the independent feature gate;
	// with it disabled neither a draft nor a decision can be written.
	old := viper.Get("idoV3PolicyReview.enabled")
	t.Cleanup(func() { viper.Set("idoV3PolicyReview.enabled", old) })
	viper.Set("idoV3PolicyReview.enabled", false)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/:id/ido-v3/policy/reviews", StartV3PolicyReview)
	router.POST("/projects/:id/ido-v3/policy/reviews/:reviewID/approve", ApproveV3PolicyReview)
	for _, test := range []struct{ path, body, code string }{
		{"/projects/9/ido-v3/policy/reviews", `{"wallets":["0x1111111111111111111111111111111111111111"],"enabled":true,"reason":"review","preview_digest":"0x1111","base_block":"90","base_hash":"0x1111"}`, "V3_POLICY_REVIEW_UNAVAILABLE"},
		{"/projects/9/ido-v3/policy/reviews/1/approve", `{"preview_digest":"0x1111"}`, "V3_POLICY_APPROVAL_UNAVAILABLE"},
	} {
		req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), test.code) {
			t.Fatalf("MFA-free review rejected at input boundary: %s %d %s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestV3PolicySafeHandoffRejectsInvalidScopeBeforeReadingDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/projects/:id/ido-v3/policy/reviews/:reviewID/safe-handoff", GetV3PolicySafeHandoff)
	for _, path := range []string{
		"/projects/0/ido-v3/policy/reviews/1/safe-handoff",
		"/projects/9/ido-v3/policy/reviews/0/safe-handoff",
		"/projects/not-an-id/ido-v3/policy/reviews/1/safe-handoff",
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "V3_POLICY_SCOPE_INVALID") {
			t.Fatalf("invalid scope %q reached Safe handoff: %d %s", path, response.Code, response.Body.String())
		}
	}
}

func TestV3PolicyObservationConflictHasFiniteModeStage(t *testing.T) {
	// Fail before parsing or touching any chain/database identity.
	old := viper.Get("idoV3Approved.enabled")
	t.Cleanup(func() { viper.Set("idoV3Approved.enabled", old) })
	viper.Set("idoV3Approved.enabled", false)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/:id/ido-v3/policy/reviews/:reviewID/observe", ObserveV3PolicyExecution)
	request := httptest.NewRequest(http.MethodPost, "/projects/19/ido-v3/policy/reviews/7/observe", strings.NewReader(`{"tx_hash":"unused"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"stage":"mode_ready"`) ||
		!strings.Contains(response.Body.String(), `"code":"V3_POLICY_EXECUTION_UNAVAILABLE"`) {
		t.Fatalf("unexpected observation rejection: %d %s", response.Code, response.Body.String())
	}
}

func TestV3PolicyReviewListRejectsInvalidProjectScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/projects/:id/ido-v3/policy/reviews", ListV3PolicyReviews)
	for _, id := range []string{"0", "not-an-id"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/projects/"+id+"/ido-v3/policy/reviews", nil))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "V3_POLICY_SCOPE_INVALID") {
			t.Fatalf("invalid list scope %s: %d %s", id, response.Code, response.Body.String())
		}
	}
}
