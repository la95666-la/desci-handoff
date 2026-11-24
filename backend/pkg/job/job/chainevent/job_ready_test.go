package chainevent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/entity"
)

func TestIDOV2ReadinessRequiresRegisteredExactSource(t *testing.T) {
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	implementation := common.HexToAddress("0x2222222222222222222222222222222222222222")
	hash := common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333")
	good := entity.ChainEventSyncState{Initialized: true, V2Registered: true, V2ContractAddress: address.Hex(), V2StartBlock: 10, V2ImplementationAddress: implementation.Hex(), V2ImplementationCodeHash: hash.Hex(), CursorBlock: 9}
	if !idoV2ReadinessSourceMatches(good, address, 10, implementation, hash) {
		t.Fatal("exact registered V2 source should pass identity gate")
	}
	for _, tc := range []struct {
		name string
		edit func(*entity.ChainEventSyncState)
	}{
		{"not initialized", func(s *entity.ChainEventSyncState) { s.Initialized = false }},
		{"not registered", func(s *entity.ChainEventSyncState) { s.V2Registered = false }},
		{"wrong proxy", func(s *entity.ChainEventSyncState) { s.V2ContractAddress = implementation.Hex() }},
		{"wrong start block", func(s *entity.ChainEventSyncState) { s.V2StartBlock++ }},
		{"wrong implementation", func(s *entity.ChainEventSyncState) { s.V2ImplementationAddress = address.Hex() }},
		{"wrong code hash", func(s *entity.ChainEventSyncState) { s.V2ImplementationCodeHash = common.Hash{}.Hex() }},
		{"cursor before source", func(s *entity.ChainEventSyncState) { s.CursorBlock = 8 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := good
			tc.edit(&state)
			if idoV2ReadinessSourceMatches(state, address, 10, implementation, hash) {
				t.Fatal("mismatched scanner source reported ready")
			}
		})
	}
	if idoV2ReadinessSourceMatches(good, address, 0, implementation, hash) || idoV2ReadinessSourceMatches(good, address, 10, common.Address{}, hash) || idoV2ReadinessSourceMatches(good, address, 10, implementation, common.Hash{}) {
		t.Fatal("missing configured source identity reported ready")
	}
}

func TestJobReadinessRejectsUnadmittedModesWithoutDatabase(t *testing.T) {
	for _, tc := range []struct {
		name, environment, mode, v1 string
		scanner, exclusive          bool
	}{
		{"unspecified", "prod", "", "false", false, false},
		{"development", "local", "ido", "false", false, false},
		{"legacy default", "prod", "ido", "", false, false},
		{"mixed worker", "prod", "ido", "false", true, true},
		{"unreviewed fair launch", "prod", "fair-launch", "false", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.Set("application.environment", tc.environment)
			viper.Set("application.jobMode", tc.mode)
			viper.Set("chain.project.v1.enabled", tc.v1)
			viper.Set("fairLaunchMainnet.scanner.enabled", tc.scanner)
			viper.Set("fairLaunchMainnet.scanner.exclusiveJobMode", tc.exclusive)
			router := gin.New()
			RegisterHealthRoutes(router)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/job-ready", nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("unadmitted workload readiness=%d: %s", response.Code, response.Body.String())
			}
		})
	}
}
