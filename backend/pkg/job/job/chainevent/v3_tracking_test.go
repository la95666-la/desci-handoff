package chainevent

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/projecttracking"
	"math/big"
	"testing"
	"time"
)

func TestTrackingModeFenceRejectsConcurrentAdminChange(t *testing.T) {
	db, m := recoveryMockDB(t)
	ctx := context.WithValue(context.Background(), trackingFenceKey{}, trackingFence{revision: 7, mode: "normal"})
	m.ExpectQuery("SELECT t.*FROM ido_v3_project_tracking").WillReturnRows(sqlmock.NewRows([]string{"project_id", "mode", "revision", "due", "evidence_current"}).AddRow("19", "maintenance", 8, 0, 1))
	if err := checkV3TrackingFence(ctx, db, 19); err == nil || !v3Deferred(err) {
		t.Fatal(err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestTrackingProducerWakeIsPersistentAndIdempotent(t *testing.T) {
	old := viper.Get("idoV3ProjectTracking.enabled")
	defer viper.Set("idoV3ProjectTracking.enabled", old)
	viper.Set("idoV3ProjectTracking.enabled", true)
	db, m := recoveryMockDB(t)
	s := productionV3StreamScope()
	id := uint64(9007199254740999)
	l := types.Log{BlockNumber: 100, Topics: []common.Hash{crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")), common.BigToHash(new(big.Int).SetUint64(id))}}
	for i := 0; i < 2; i++ {
		m.ExpectExec("UPDATE ido_v3_project_tracking SET wake_block=GREATEST").WithArgs(uint64(100), id, s.Chain, v3Lower(s.Proxy), v3StreamVersion).WillReturnResult(sqlmock.NewResult(0, 1))
		if err := wakeV3TrackingFromCoverageTx(db, s, []v3StreamBlock{{Logs: []types.Log{l}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestTwelveHourIntervalDoesNotChangeFreshnessTTL(t *testing.T) {
	if projecttracking.MaintenanceInterval != 12*time.Hour {
		t.Fatal(projecttracking.MaintenanceInterval)
	}
}

func TestMaintenanceBacklogDefersWithoutClosureButDueDrains(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  projecttracking.Row
		skip bool
	}{
		{"initial no evidence", projecttracking.Row{Mode: "maintenance"}, true},
		{"stale evidence", projecttracking.Row{Mode: "maintenance", EvidenceCurrent: false}, true},
		{"acknowledged historical wake", projecttracking.Row{Mode: "maintenance", WakeBlock: 100}, true},
		{"due backlog", projecttracking.Row{Mode: "maintenance", Due: true, WakeBlock: 100}, false},
		{"partial batch remains due", projecttracking.Row{Mode: "maintenance", Due: true, EvidenceCurrent: false}, false},
		{"new event due", projecttracking.Row{Mode: "maintenance", Due: true, WakeBlock: 200}, false},
		{"pending transaction", projecttracking.Row{Mode: "maintenance", Urgent: true}, false},
		{"source recovery", projecttracking.Row{Mode: "maintenance", RecoveryRequired: true}, false},
		{"archived stale", projecttracking.Row{Mode: "archived", Finished: true}, false},
		{"archived valid", projecttracking.Row{Mode: "archived", Finished: true, EvidenceCurrent: true}, true},
		{"normal stale", projecttracking.Row{Mode: "normal"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skipV3TrackingTurn(tc.row); got != tc.skip {
				t.Fatal(got, tc.row)
			}
		})
	}
}
