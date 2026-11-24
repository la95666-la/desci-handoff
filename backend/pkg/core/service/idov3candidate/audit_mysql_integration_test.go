package idov3candidate

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	mysqlDriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// Explicit disposable MySQL 8.4 verification. Never run against a named
// production/test database. The read DSN has SELECT on five audit inputs only;
// the separate writer is confined to the one network-isolated container.
func TestAuditCandidateDisposableMySQLSnapshotAndPrivileges(t *testing.T) {
	readDSN, writeDSN := os.Getenv("JOINCARE_IDO_V3_MYSQL_READ_DSN"), os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if readDSN == "" && writeDSN == "" {
		t.Skip("disposable V3 MySQL DSNs are not set")
	}
	if readDSN == "" || writeDSN == "" {
		t.Fatal("both disposable V3 DSNs are required")
	}
	open := func(dsn string) *gorm.DB {
		database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
		if err != nil {
			var dbErr *mysqlDriver.MySQLError
			if errors.As(err, &dbErr) {
				t.Fatalf("disposable MySQL connection failed (code %d)", dbErr.Number)
			}
			t.Fatal("disposable MySQL connection failed (transport)")
		}
		var name, version string
		if database.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
			database.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
			t.Fatal("refuse non-disposable or unsupported V3 MySQL")
		}
		return database
	}
	reader, writer := open(readDSN), open(writeDSN)
	scope := NewScope(31337, 41376991395446785, common.HexToAddress("0x4444444444444444444444444444444444444444"))
	tree, err := idov3whitelist.Build(scope, []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	canonical := func(hash common.Hash) string { return strings.ToLower(hash.Hex()) }
	seed := func(query string, args ...any) {
		if err := writer.Exec(query, args...).Error; err != nil {
			t.Fatalf("disposable fixture write failed: %v", err)
		}
	}
	proxy := strings.ToLower(scope.ProxyAddress.Hex())
	seed(`INSERT INTO projects(project_id,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,description,tags,team_info,roadmap,partners)
		VALUES(?,'capped_presale','ido_v3_stablecoin',31337,?,1,'0x1111111111111111111111111111111111111111','0x2222222222222222222222222222222222222222','','','','','')`, scope.ProjectID.Uint64(), proxy)
	seed(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block)
		VALUES(31337,?,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',?,?, '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`, proxy, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64))
	seed(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(?,31337,?)`, scope.ProjectID.Uint64(), proxy)
	ctx := context.Background()
	identity, err := AuditInertProjectIdentity(ctx, reader, scope)
	if err != nil || identity.ProjectID != scope.ProjectID.Uint64() || identity.ProjectToken != common.HexToAddress("0x1111111111111111111111111111111111111111") ||
		identity.PaymentToken != common.HexToAddress("0x2222222222222222222222222222222222222222") || identity.Implementation != common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") ||
		identity.Governor != common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") || identity.StartBlock != 1 {
		t.Fatal("disposable inert V3 identity audit rejected complete locked binding")
	}
	if _, err := AuditInertProjectIdentity(ctx, reader, NewScope(56, scope.ProjectID.Uint64(), scope.ProxyAddress)); err == nil {
		t.Fatal("wrong chain admitted by inert identity audit")
	}
	if _, err := AuditInertProjectIdentity(ctx, reader, NewScope(31337, scope.ProjectID.Uint64(), common.HexToAddress("0x5555555555555555555555555555555555555555"))); err == nil {
		t.Fatal("wrong proxy admitted by inert identity audit")
	}
	seed(`INSERT INTO ido_v3_policy_snapshot(project_id,chain_id,proxy_address,expected_version,target_version,enabled,root,members_digest,snapshot_digest,member_count,initiator_admin_id)
		VALUES(?,31337,?,0,1,1,?,?,?,?,17)`, scope.ProjectID.Uint64(), proxy, canonical(commitment.Root), canonical(commitment.MembersDigest), canonical(commitment.Digest), commitment.MemberCount)
	var snapshotID uint64
	if err := writer.Raw("SELECT id FROM ido_v3_policy_snapshot WHERE project_id=?", scope.ProjectID.Uint64()).Scan(&snapshotID).Error; err != nil || snapshotID == 0 {
		t.Fatal("disposable snapshot ID unavailable")
	}
	members := tree.Members()
	insertMember := func(index int) {
		member := members[index]
		seed(`INSERT INTO ido_v3_policy_member(snapshot_id,leaf_index,wallet_address,leaf_hash) VALUES(?,?,?,?)`, snapshotID, index, strings.ToLower(member.Wallet.Hex()), canonical(member.Leaf))
	}
	insertMember(0)
	if _, err := AuditCandidate(ctx, reader, scope, snapshotID); err == nil {
		t.Fatal("incomplete member set accepted")
	}
	// Snapshot SELECT begins the REPEATABLE READ view. A concurrent committed
	// second member must not be visible to the same read-only transaction.
	if _, err := auditCandidate(ctx, reader, scope, snapshotID, func() error { insertMember(1); return nil }); err == nil {
		t.Fatal("concurrently inserted member crossed the read-only snapshot")
	}
	got, err := AuditCandidate(ctx, reader, scope, snapshotID)
	if err != nil || got != commitment {
		t.Fatalf("complete durable candidate rejected: %+v %v", got, err)
	}
	// SQL CHECK constraints enforce syntax and immutability, not the Merkle
	// commitment. Another append-only candidate can carry syntactically valid
	// but false digests; the audit must recompute all members and reject it.
	second, err := tree.CommitSnapshot(scope, 2)
	if err != nil {
		t.Fatal(err)
	}
	badDigest := "0x" + strings.Repeat("f", 64)
	if badDigest == canonical(second.Digest) {
		t.Fatal("invalid digest fixture collided")
	}
	seed(`INSERT INTO ido_v3_policy_snapshot(project_id,chain_id,proxy_address,expected_version,target_version,enabled,root,members_digest,snapshot_digest,member_count,initiator_admin_id)
		VALUES(?,31337,?,1,2,1,?,?,?,?,17)`, scope.ProjectID.Uint64(), proxy, canonical(second.Root), canonical(second.MembersDigest), badDigest, second.MemberCount)
	var badID uint64
	if err := writer.Raw("SELECT id FROM ido_v3_policy_snapshot WHERE project_id=? AND target_version=2", scope.ProjectID.Uint64()).Scan(&badID).Error; err != nil || badID == 0 {
		t.Fatal("malformed candidate was not stored for negative audit")
	}
	for index, member := range members {
		seed(`INSERT INTO ido_v3_policy_member(snapshot_id,leaf_index,wallet_address,leaf_hash) VALUES(?,?,?,?)`, badID, index, strings.ToLower(member.Wallet.Hex()), canonical(member.Leaf))
	}
	if _, err := AuditCandidate(ctx, reader, scope, badID); err == nil {
		t.Fatal("syntactically valid but false member commitment accepted")
	}
	if got, err := AuditCandidate(ctx, reader, scope, snapshotID); err != nil || got != commitment {
		t.Fatal("invalid later candidate changed earlier complete audit")
	}
	readSQL, err := reader.DB()
	if err != nil {
		t.Fatal("read-only audit DB unavailable")
	}
	if _, err := readSQL.ExecContext(ctx, "UPDATE ido_v3_policy_snapshot SET root=root WHERE id=?", snapshotID); err == nil {
		t.Fatal("read-only audit account was able to write")
	}
	for _, table := range []string{"project_whitelists", "admin_approval", "ido_v3_policy_execution"} {
		// Constant internal identifiers, never caller input. The audit account
		// must not see signup rows, independent approvals or execution state.
		if _, err := readSQL.QueryContext(ctx, "SELECT * FROM "+table+" LIMIT 1"); err == nil {
			t.Fatalf("candidate reader can access forbidden table %s", table)
		}
	}
	if _, err := AuditCandidate(ctx, reader, scope, snapshotID+1); err == nil {
		t.Fatal("wrong snapshot ID accepted")
	}
	if _, err := AuditCandidate(ctx, reader, NewScope(31337, scope.ProjectID.Uint64()+1, scope.ProxyAddress), snapshotID); err == nil {
		t.Fatal("wrong project accepted")
	}
	if _, err := AuditCandidate(ctx, reader, NewScope(56, scope.ProjectID.Uint64(), scope.ProxyAddress), snapshotID); err == nil {
		t.Fatal("wrong chain accepted")
	}
	if _, err := AuditCandidate(ctx, reader, NewScope(31337, scope.ProjectID.Uint64(), common.HexToAddress("0x5555555555555555555555555555555555555555")), snapshotID); err == nil {
		t.Fatal("wrong proxy accepted")
	}
}
