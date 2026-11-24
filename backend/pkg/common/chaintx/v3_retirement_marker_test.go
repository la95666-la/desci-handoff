package chaintx

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"joincare_server/pkg/common/v3diagnostic"
)

func TestRetirementMarkerIndexedCountKeepsMembershipAndFailureClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		count  int
		fail   bool
		reason string
	}{{"exact", 1, false, ""}, {"missing", 0, false, "SENDER_RETIREMENT_REVIEW_REQUIRED"}, {"duplicate", 2, false, "SENDER_RETIREMENT_REVIEW_REQUIRED"}, {"missing index/schema/timeout", 0, true, "SENDER_EVIDENCE_UNAVAILABLE"}} {
		t.Run(tc.name, func(t *testing.T) {
			d, m := recoveryFlowDB(t)
			q := m.ExpectQuery("SELECT COUNT.*FORCE INDEX \\(ix_v3_coverage_retirement_marker\\).*contract_version='ido_v3_stablecoin' AND retirement_marker=1.*JSON_CONTAINS").WithArgs(uint64(56), "0xabc", uint64(9007199254740999))
			if tc.fail {
				q.WillReturnError(errors.New("SECRET SQL/private rpc"))
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
			}
			e := requireV3RetirementMarker(d, 56, "0xAbC", 9007199254740999)
			if tc.reason == "" && e != nil || tc.reason != "" && (e == nil || v3diagnostic.Reason(e) != tc.reason) {
				t.Fatal(e)
			}
			if e != nil && strings.Contains(e.Error(), "SECRET") {
				t.Fatal("secret leaked")
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
