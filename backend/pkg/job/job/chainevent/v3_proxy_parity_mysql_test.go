package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

func testV3CompleteParityMySQL(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	s, rpc := streamTestFixture()
	s.Proxy = common.HexToAddress("0x7878787878787878787878787878787878787878")
	for _, block := range rpc.blocks {
		block.proxy = s.Proxy
	}
	// All fixture writes roll back; production IDs/config/signers are absent.
	b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: 7, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
	rollback := errors.New("parity fixture rollback")
	oldPolicyFlag := viper.Get("idoV3PolicyJournal.enabled")
	viper.Set("idoV3PolicyJournal.enabled", true)
	defer viper.Set("idoV3PolicyJournal.enabled", oldPolicyFlag)
	e := db.Transaction(func(tx *gorm.DB) error {
		exec := func(q string, args ...any) {
			t.Helper()
			if e := tx.Exec(q, args...).Error; e != nil {
				t.Fatal(e)
			}
		}
		// The parent is block 90, actual creation is 91. This exercises a REAL
		// covered creation event, unlike the older empty-at-creation fixture.
		exec(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block) VALUES(31337,?,?,?,?,?,1)`, v3Lower(s.Proxy), v3Lower(s.Runtime.Implementation), strings.ToLower(s.Runtime.ProxyCodeHash.Hex()), strings.ToLower(s.Runtime.CodeHash.Hex()), v3Lower(b.ProjectToken))
		exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners) VALUES(7,'Pending','capped_presale','ido_v3_stablecoin',31337,?,1,?,?,?,?,'','','','','')`, v3Lower(s.Proxy), v3Lower(b.ProjectToken), v3Lower(b.PaymentToken), v3Lower(b.ProjectToken), v3Lower(b.PaymentToken))
		exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(7,31337,?)`, v3Lower(s.Proxy))
		word := func(v int64) []byte { return big.NewInt(v).FillBytes(make([]byte, 32)) }
		data := append(common.LeftPadBytes(b.ProjectToken.Bytes(), 32), common.LeftPadBytes(b.PaymentToken.Bytes(), 32)...)
		for _, v := range []int64{18, 18, 200, 20, 1000, 500, 900, 100, 200} {
			data = append(data, word(v)...)
		}
		l := types.Log{Address: s.Proxy, BlockNumber: 91, BlockHash: rpc.blocks[91].head.Hash(), TxHash: common.HexToHash("0xc1"), Index: 1, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")), common.BigToHash(big.NewInt(7)), common.BytesToHash(b.ProjectToken.Bytes()), common.BytesToHash(b.PaymentToken.Bytes())}, Data: data}
		rpc.logs = []types.Log{l}
		rpc.receipts[l.TxHash] = &types.Receipt{Status: 1, TxHash: l.TxHash, BlockHash: l.BlockHash, BlockNumber: big.NewInt(91), Logs: []*types.Log{&l}}
		root := common.HexToHash("0xabc1")
		policy := types.Log{Address: s.Proxy, BlockNumber: 91, BlockHash: l.BlockHash, TxHash: common.HexToHash("0xc2"), Index: 2, Topics: []common.Hash{v3PolicyUpdateTopic, common.BigToHash(big.NewInt(7)), common.BigToHash(big.NewInt(1))}, Data: append(word(1), root.Bytes()...)}
		rpc.logs = append(rpc.logs, policy)
		rpc.receipts[policy.TxHash] = &types.Receipt{Status: 1, TxHash: policy.TxHash, BlockHash: l.BlockHash, BlockNumber: big.NewInt(91), Logs: []*types.Log{&policy}}
		rpc.blocks[91].policy = append(append(word(1), root.Bytes()...), word(1)...)
		event, e := commonchain.DecodeV3ReplayLog(l, b)
		if e != nil {
			t.Fatal(e)
		}
		payload, e := json.Marshal(event)
		if e != nil {
			t.Fatal(e)
		}
		exec(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) VALUES(31337,?,7,91,?,1,'running')`, v3Lower(s.Proxy), strings.ToLower(l.BlockHash.Hex()))
		exec(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash) VALUES(31337,?,7,91,?)`, v3Lower(s.Proxy), strings.ToLower(l.BlockHash.Hex()))
		exec(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,tx_hash,log_index,event_kind,payload) VALUES(31337,?,7,91,?,?,1,'created',?)`, v3Lower(s.Proxy), strings.ToLower(l.BlockHash.Hex()), strings.ToLower(l.TxHash.Hex()), string(payload))
		exec(`INSERT INTO ido_v3_policy_event_journal(chain_id,proxy_address,project_id,tx_hash,block_number,block_hash,log_index,version,enabled,root,state) VALUES(31337,?,7,?,91,?,2,1,1,?,'canonical')`, v3Lower(s.Proxy), strings.ToLower(policy.TxHash.Hex()), strings.ToLower(l.BlockHash.Hex()), strings.ToLower(root.Hex()))
		if e = rebuildV3ProjectTx(tx, b, 91, l.BlockHash, 1); e != nil {
			t.Fatal(e)
		}
		if e = bootstrapV3Stream(ctx, tx, rpc, s, "shadow", 90, rpc.blocks[90].head.Hash()); e != nil {
			t.Fatal(e)
		}
		if e = produceV3Stream(ctx, tx, rpc, s, 1); e != nil {
			t.Fatal(e)
		}
		if _, e = auditV3CoveredProject(ctx, tx, rpc, s, b, 91, 91, l.BlockHash, false); e != nil {
			t.Fatalf("full parity %v", e)
		}
		originalPolicy := rpc.blocks[91].policy
		rpc.blocks[91].policy = make([]byte, 96)
		if _, e = auditV3CoveredProject(ctx, tx, rpc, s, b, 91, 91, l.BlockHash, false); e == nil {
			t.Fatal("stale policy state passed parity")
		}
		rpc.blocks[91].policy = originalPolicy
		exec(`UPDATE ido_v3_project_projection SET claimed_raw='1' WHERE project_id=7`)
		if e = switchV3StreamMode(ctx, tx, rpc, s, 2, "stream"); e == nil {
			t.Fatal("economic mismatch passed switch")
		}
		exec(`UPDATE ido_v3_project_projection SET claimed_raw='0' WHERE project_id=7`)
		if e = switchV3StreamMode(ctx, tx, rpc, s, 2, "stream"); e != nil {
			t.Fatalf("complete parity switch %v", e)
		}
		if e = haltV3Stream(ctx, tx, s); e != nil {
			t.Fatal(e)
		}
		exec(`UPDATE ido_v3_proxy_stream SET state='running' WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...)
		candidate := idov3whitelist.ApprovedLockedCandidate{Binding: b, Runtime: s.Runtime, CreationBlock: 91, CursorBlock: 91, CursorHash: l.BlockHash, CursorRevision: 2}
		if e = v3StreamWriterTx(ctx, tx, func(w *gorm.DB) error { return restoreV3CoveredCandidate(ctx, w, rpc, s, candidate) }); e != nil {
			t.Fatalf("full covered paused-project restore %v", e)
		}
		if _, e = readV3PublishedProjection(ctx, tx, b); e != nil {
			t.Fatal("restored projection absent", e)
		}
		if e = v3StreamWriterTx(ctx, tx, func(w *gorm.DB) error { return restoreV3CoveredCandidate(ctx, w, rpc, s, candidate) }); e == nil {
			t.Fatal("stale recovery revision replayed")
		}
		return rollback
	})
	if !errors.Is(e, rollback) {
		t.Fatal(e)
	}
}
