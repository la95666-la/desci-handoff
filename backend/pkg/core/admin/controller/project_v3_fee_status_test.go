package controller

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"joincare_server/pkg/core/db"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestV3FeeListUsesCanonicalEventIncludingZeroAmount(t *testing.T) {
	for _, tc := range []struct {
		name        string
		event, fail bool
	}{{"paid", true, false}, {"zero amount executed event", true, false}, {"not withdrawn or orphaned", false, false}, {"source unavailable", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			conn, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			d, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			expected := m.ExpectQuery("SELECT DISTINCT b.project_id.*j.event_kind='PlatformFeeWithdrawn'.*j.status='canonical'.*e.status='canonical'")
			rows := sqlmock.NewRows([]string{"project_id"})
			if tc.event {
				rows.AddRow(uint64(43368660559134718))
			}
			if tc.fail {
				expected.WillReturnError(errors.New("unavailable"))
			} else {
				expected.WillReturnRows(rows)
			}
			result, e := readV3PlatformFeeWithdrawn(context.Background(), d, []uint64{43368660559134718})
			if (e != nil) != tc.fail || !tc.fail && result[43368660559134718] != tc.event {
				t.Fatal(result, e)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
	if result, e := readV3PlatformFeeWithdrawn(context.Background(), nil, nil); e != nil || len(result) != 0 {
		t.Fatal(result, e)
	}
}

func TestProjectListV3FeeOverridesLegacyFlagAndKeepsV2(t *testing.T) {
	for _, tc := range []struct {
		name, version       string
		legacy, event, want int
	}{{"V3 stale zero", "ido_v3_stablecoin", 0, 1, 1}, {"V3 no event ignores old flag", "ido_v3_stablecoin", 1, 0, 0}, {"V2 unchanged", "v2_stablecoin", 1, 0, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			conn, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			d, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			prior := db.Client.Mysql
			db.Client.Mysql = d
			defer func() { db.Client.Mysql = prior }()
			m.ExpectQuery("SELECT count.*FROM `projects`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			m.ExpectQuery("SELECT .*FROM `projects`").WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "status", "funding_mode", "contract_version", "onchain_locked", "platform_fee_withdrawn"}).AddRow(1, uint64(43368660559134718), "Successful", "capped_presale", tc.version, 1, tc.legacy))
			if tc.version == "ido_v3_stablecoin" {
				rows := sqlmock.NewRows([]string{"project_id"})
				if tc.event == 1 {
					rows.AddRow(uint64(43368660559134718))
				}
				m.ExpectQuery("SELECT DISTINCT b.project_id").WillReturnRows(rows)
			}
			m.ExpectQuery("SELECT .*FROM `chain_tx_submission`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/admin/v1/projects", nil)
			ListProjects(c)
			want := `"platform_fee_withdrawn":0`
			if tc.want == 1 {
				want = `"platform_fee_withdrawn":1`
			}
			if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
				t.Fatal(w.Code, w.Body.String())
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
