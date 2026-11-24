package idov3whitelist

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReadApprovedSetRecordRejectsMissingChangedAndAmbiguousEvidence(t *testing.T) {
	addr := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	identity := ApprovedSetIdentity{ProjectID: 19, ChainID: 56,
		Proxy: "0x0d4040875742975269c55e16c8884d3ee9510b8e", ProjectToken: addr("2"),
		TokenDecimals: 18, PaymentToken: "0x55d398326f99059ff775485246999027b3197955",
		PaymentDecimals: 18, ProjectOwner: addr("4"),
		FeeReceiver: addr("5"), FeeRate: 100, Operator: "0xfd712061ee08e070a930ecd45e935ae5d7f5445c", RequestDigest: hash("a"),
		CreateCallHash: hash("b"), CreateDeadline: 1800000000,
		ApprovalID: 7, InitiatorID: 8, ApproverID: 9}
	request := struct {
		StartTime string `json:"start_time"`
	}{StartTime: time.Unix(identity.CreateDeadline, 0).UTC().Format(time.RFC3339)}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	identity.RequestDigest = fmt.Sprintf("0x%x", sha256.Sum256(requestJSON))
	digest, err := DigestApprovedSetIdentity(identity)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	approvedPayload, err := json.Marshal(struct {
		ProjectID uint64 `json:"project_id"`
		Params    struct {
			StartTime string `json:"start_time"`
		} `json:"params"`
	}{ProjectID: 19, Params: struct {
		StartTime string `json:"start_time"`
	}{StartTime: time.Unix(identity.CreateDeadline, 0).UTC().Format(time.RFC3339)}})
	if err != nil {
		t.Fatal(err)
	}
	columns := []string{"chain_id", "proxy_address", "token_address", "payment_token_address",
		"owner_address", "fee_receiver", "success_fee_rate", "token_decimals", "payment_token_decimals",
		"request_digest", "manifest_digest", "create_calldata_hash", "operator_address",
		"identity_json", "identity_digest", "approval_id", "initiator_id", "approver_id", "payload"}
	base := []driver.Value{uint64(56), identity.Proxy, identity.ProjectToken, identity.PaymentToken,
		identity.ProjectOwner, identity.FeeReceiver, 100, 18, 18, identity.RequestDigest,
		digest.Hex(), identity.CreateCallHash, identity.Operator, string(encoded), digest.Hex(),
		uint64(7), uint64(8), uint64(9), string(approvedPayload)}
	for _, tc := range []struct {
		name   string
		mutate func([]driver.Value)
		count  int
		want   bool
	}{
		{name: "exact", count: 1, want: true},
		{name: "absent", count: 0},
		{name: "ambiguous", count: 2},
		{name: "foreign project", count: 1, mutate: func(v []driver.Value) { v[18] = `{"project_id":20,"params":{}}` }},
		{name: "duplicate project ID", count: 1, mutate: func(v []driver.Value) {
			v[18] = strings.Replace(v[18].(string), `{"project_id":19,`, `{"project_id":20,"project_id":19,`, 1)
		}},
		{name: "noncanonical approval envelope", count: 1, mutate: func(v []driver.Value) { v[18] = " " + v[18].(string) }},
		{name: "different approver", count: 1, mutate: func(v []driver.Value) { v[17] = uint64(10) }},
		{name: "unbound digest", count: 1, mutate: func(v []driver.Value) { v[14] = hash("c") }},
		{name: "legacy manifest digest", count: 1, mutate: func(v []driver.Value) { v[10] = hash("d") }},
		{name: "changed approval ID", count: 1, mutate: func(v []driver.Value) { v[15] = uint64(11) }},
		{name: "changed calldata", count: 1, mutate: func(v []driver.Value) { v[11] = hash("c") }},
		{name: "changed project token", count: 1, mutate: func(v []driver.Value) { v[2] = addr("a") }},
		{name: "changed payment token", count: 1, mutate: func(v []driver.Value) { v[3] = addr("3") }},
		{name: "changed operator", count: 1, mutate: func(v []driver.Value) { v[12] = addr("6") }},
		{name: "changed deadline", count: 1, mutate: func(v []driver.Value) { v[18] = `{"project_id":19,"params":{"start_time":"2020-01-01T00:00:00Z"}}` }},
		{name: "changed terms with same deadline", count: 1, mutate: func(v []driver.Value) {
			v[18] = `{"project_id":19,"params":{"start_time":"` + request.StartTime + `","hard_cap":"999"}}`
		}},
		{name: "unknown identity field", count: 1, mutate: func(v []driver.Value) { v[13] = string(encoded[:len(encoded)-1]) + `,"new":1}` }},
		{name: "noncanonical identity", count: 1, mutate: func(v []driver.Value) { v[13] = " " + string(encoded) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			database, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			rows := sqlmock.NewRows(columns)
			for i := 0; i < tc.count; i++ {
				values := append([]driver.Value(nil), base...)
				if tc.mutate != nil {
					tc.mutate(values)
				}
				rows.AddRow(values...)
			}
			mock.ExpectQuery("FROM projects p").WithArgs(uint64(19), identity.Proxy,
				"0x887fcc6e54fc8229919166ad90b16c19b4992560",
				"0xc46659e67543aaaec553f7f3fc8f004b7dd286ef",
				"0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8",
				"0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590",
				uint64(128805696)).WillReturnRows(rows)
			got, e := ReadApprovedSetRecord(context.Background(), database, 19)
			if (e == nil) != tc.want || (e != nil && errors.Is(e, gorm.ErrRecordNotFound)) {
				t.Fatalf("got %+v, error %v", got, e)
			}
			if tc.want && (got.Identity != identity || len(got.Request) == 0) {
				t.Fatalf("wrong evidence: %+v", got)
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestListApprovedV3ProjectIDsIsBoundedAndOrdered(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ids   []uint64
		limit int
		want  bool
	}{
		{name: "two projects", ids: []uint64{21, 24}, limit: 2, want: true},
		{name: "empty page", limit: 2, want: true},
		{name: "duplicate", ids: []uint64{21, 21}, limit: 2},
		{name: "out of order", ids: []uint64{24, 21}, limit: 2},
		{name: "before cursor", ids: []uint64{20}, limit: 2},
		{name: "too many", ids: []uint64{21, 24, 25}, limit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			rows := sqlmock.NewRows([]string{"project_id"})
			for _, id := range tc.ids {
				rows.AddRow(id)
			}
			mock.ExpectQuery("SELECT b.project_id FROM ido_v3_project_binding b").WithArgs(uint64(20),
				"0x0d4040875742975269c55e16c8884d3ee9510b8e",
				tc.limit).WillReturnRows(rows)
			ids, err := ListApprovedV3ProjectIDs(context.Background(), database, 20, tc.limit)
			if (err == nil) != tc.want || (tc.want && len(ids) != len(tc.ids)) {
				t.Fatalf("ids=%v err=%v", ids, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := ListApprovedV3ProjectIDs(context.Background(), nil, 0, 10); err == nil {
		t.Fatal("nil DB accepted")
	}
	if _, err := ListApprovedV3ProjectIDs(context.Background(), nil, 0, 129); err == nil {
		t.Fatal("unbounded page accepted")
	}
}
