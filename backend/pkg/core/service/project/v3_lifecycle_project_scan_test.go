package project

import (
	"context"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/idov3whitelist"
)

type settlementHeaders struct {
	idov3whitelist.PolicyRPC // unexpected code/storage/call RPCs panic
	head, cursor             *types.Header
}

func (s settlementHeaders) HeaderByNumber(_ context.Context, block *big.Int) (*types.Header, error) {
	if block == nil {
		return s.head, nil
	}
	return s.cursor, nil
}

func TestSettlementUsesExactCanonicalProjectNotGlobalHealth(t *testing.T) {
	for _, tc := range []struct {
		name              string
		present, mismatch bool
		head, time        uint64
		want              bool
	}{
		{"canonical project despite unhealthy global scan", true, false, 114, 1100, true},
		{"missing or halted project", false, false, 114, 1100, false},
		{"cursor not confirmed", true, false, 113, 1100, false},
		{"scanner lag is not current settlement authority", true, false, 235, 1200, true},
		{"future cursor timestamp rejected", true, false, 114, 1000, false},
		{"replaced canonical header", true, true, 114, 1100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			cursor := &types.Header{Number: big.NewInt(100), Time: 1050}
			hash := strings.ToLower(cursor.Hash().Hex())
			rows := sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision"})
			if tc.present {
				rows.AddRow(100, hash, 3)
			}
			mock.ExpectQuery("SELECT s.cursor_block,s.cursor_hash,s.revision").
				WithArgs(uint64(56), strings.ToLower(v3TokenProxy), strings.ToLower(v3TokenProxy), uint64(43235577608273921)).WillReturnRows(rows)
			if tc.mismatch {
				cursor = &types.Header{Number: big.NewInt(100), Time: 1051}
			}
			admitted := idov3whitelist.AdmittedProject{Binding: idov3whitelist.ExpectedProjectBinding{
				ChainID: 56, ProjectID: 43235577608273921, ProxyAddress: common.HexToAddress(v3TokenProxy)}, CreationBlock: 99, Confirmations: 15}
			rpc := settlementHeaders{head: &types.Header{Number: new(big.Int).SetUint64(tc.head), Time: tc.time}, cursor: cursor}
			err := requireApprovedV3OperatorProjectScan(context.Background(), database, rpc, admitted, "ido_v3_end")
			if (err == nil) != tc.want {
				t.Fatalf("settlement gate %v want=%v", err, tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOperatorCheckpointSQLRetainsIndexedAndExactIdentityGuards(t *testing.T) {
	for _, m := range regexp.MustCompile(`BINARY (ck\.[a-z_]+)=BINARY (s\.[a-z_]+)`).FindAllStringSubmatch(v3OperatorCheckpointSQL, -1) {
		if !strings.Contains(v3OperatorCheckpointSQL, m[1]+"="+m[2]+" AND BINARY ") {
			t.Fatalf("missing indexable equality: %s", m[0])
		}
	}
	for _, guard := range []string{"s.chain_id=?", "s.proxy_address=? AND BINARY s.proxy_address=BINARY ?", "s.project_id=?", "s.contract_version='ido_v3_stablecoin' AND BINARY s.contract_version=BINARY 'ido_v3_stablecoin'", "BINARY s.state=BINARY 'running'", "ck.block_number=s.cursor_block", "BINARY ck.block_hash=BINARY s.cursor_hash", "BINARY ck.status=BINARY 'canonical'", "LIMIT 2"} {
		if !strings.Contains(v3OperatorCheckpointSQL, guard) {
			t.Fatalf("missing identity guard %s", guard)
		}
	}
}

func TestOperatorCheckpointLookupRejectsAmbiguousOrInvalidEvidence(t *testing.T) {
	for _, kind := range []string{"duplicate", "zero-revision", "zero-hash", "before-creation", "db-error"} {
		t.Run(kind, func(t *testing.T) {
			d, m := v2AdmissionMock(t)
			cursor := &types.Header{Number: big.NewInt(100), Time: 1000}
			block, revision, hash := uint64(100), uint64(3), cursor.Hash().Hex()
			if kind == "zero-revision" {
				revision = 0
			}
			if kind == "zero-hash" {
				hash = common.Hash{}.Hex()
			}
			if kind == "before-creation" {
				block = 98
			}
			rows := sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision"}).AddRow(block, hash, revision)
			if kind == "duplicate" {
				rows.AddRow(block, hash, revision)
			}
			e := m.ExpectQuery(regexp.QuoteMeta(v3OperatorCheckpointSQL)).WithArgs(uint64(56), strings.ToLower(v3TokenProxy), strings.ToLower(v3TokenProxy), uint64(19))
			if kind == "db-error" {
				e.WillReturnError(errors.New("lookup unavailable"))
			} else {
				e.WillReturnRows(rows)
			}
			a := idov3whitelist.AdmittedProject{Binding: idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: 19, ProxyAddress: common.HexToAddress(v3TokenProxy)}, CreationBlock: 99, Confirmations: 15}
			// Embedded interface intentionally panics if malformed SQL evidence
			// reaches an RPC call instead of closing the gate immediately.
			if err := requireApprovedV3OperatorProjectScan(context.Background(), d, struct{ idov3whitelist.PolicyRPC }{}, a, "ido_v3_end"); err == nil {
				t.Fatal("bad evidence accepted")
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSettlementDoesNotBroadenFeeOrWalletAuthority(t *testing.T) {
	if err := requireApprovedV3OperatorProjectScan(context.Background(), nil, nil, idov3whitelist.AdmittedProject{}, "ido_v3_withdraw_platform_fee"); err == nil {
		t.Fatal("fee global gate removed")
	}
	if err := requireApprovedV3OperatorProjectScan(context.Background(), nil, nil, idov3whitelist.AdmittedProject{}, "ido_v3_end"); err == nil {
		t.Fatal("missing settlement evidence admitted")
	}
}

func TestAllOperatorActionsIgnoreCertificateGapButKeepCanonicalFence(t *testing.T) {
	old := viper.Get("idoV3ProjectFreshness.enabled")
	defer viper.Set("idoV3ProjectFreshness.enabled", old)
	for _, enabled := range []bool{false, true} {
		viper.Set("idoV3ProjectFreshness.enabled", enabled)
		for _, action := range []string{"ido_v3_activate", "ido_v3_end", "ido_v3_withdraw_platform_fee"} {
			db, m := v2AdmissionMock(t)
			cursor := &types.Header{Number: big.NewInt(100), Time: 1000}
			m.ExpectQuery("SELECT s.cursor_block,s.cursor_hash,s.revision").WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision"}).AddRow(100, cursor.Hash().Hex(), 3))
			a := idov3whitelist.AdmittedProject{Binding: idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: 19, ProxyAddress: common.HexToAddress(v3TokenProxy)}, CreationBlock: 99, Confirmations: 15}
			if err := requireApprovedV3OperatorProjectScan(context.Background(), db, settlementHeaders{head: &types.Header{Number: big.NewInt(1000), Time: 2000}, cursor: cursor}, a, action); err != nil {
				t.Fatal(enabled, action, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
