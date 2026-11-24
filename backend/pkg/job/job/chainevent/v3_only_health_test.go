package chainevent

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestV3OnlyReadinessNeedsRealCurrentProgress(t *testing.T) {
	now := time.Now()
	s := v3StreamCursor{Mode: "stream", State: "running", Epoch: 3, CursorBlock: 100, ObservedHead: 114, LastSuccessAt: now.Add(-time.Second)}
	if !v3OnlyProgressReady(s, 3, "healthy", 100, 1, now) {
		t.Fatal("valid V3 rejected")
	}
	for _, mutate := range []func(*v3StreamCursor){func(x *v3StreamCursor) { x.State = "halted" }, func(x *v3StreamCursor) { x.Mode = "shadow" }, func(x *v3StreamCursor) { x.ObservedHead = 1000 }, func(x *v3StreamCursor) { x.LastSuccessAt = now.Add(-100 * time.Second) }, func(x *v3StreamCursor) { x.LastSuccessAt = now.Add(time.Second) }} {
		bad := s
		mutate(&bad)
		if v3OnlyProgressReady(bad, 3, "healthy", 100, 1, now) {
			t.Fatal("bad source became healthy")
		}
	}
	for _, tc := range []struct {
		epoch  uint64
		state  string
		target uint64
		age    int64
	}{{2, "healthy", 100, 1}, {3, "checking", 100, 1}, {3, "healthy", 100, 100}, {3, "healthy", 100, -1}, {3, "healthy", 101, 1}} {
		if v3OnlyProgressReady(s, tc.epoch, tc.state, tc.target, tc.age, now) {
			t.Fatal("invalid freshness accepted")
		}
	}
}
func TestV3OnlyWithoutAdmittedSourceIsNotHealthy(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("application.environment", "prod")
	viper.Set("application.jobMode", "ido-v3")
	r := gin.New()
	RegisterHealthRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/job-ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal("disabled source declared ready")
	}
}
