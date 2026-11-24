package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

// Seven independent durable consumers share public evidence, not project
// authority. Uses the network-none MySQL fixture's already verified seed; the
// full admission path remains independently tested by its existing regressions.
func testV3SevenConsumerBudgetMySQL(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, s v3StreamScope, base idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	rollback := errors.New("seven consumer budget fixture rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		var payload string
		if err := tx.Raw(`SELECT payload FROM ido_v3_event_journal WHERE project_id=? AND event_kind='created'`, base.ProjectID).Scan(&payload).Error; err != nil {
			return err
		}
		bindings := map[uint64]idov3whitelist.ExpectedProjectBinding{}
		var ids []uint64
		for i := uint64(0); i < 7; i++ {
			b := base
			b.ProjectID += 1000 + i
			ids = append(ids, b.ProjectID)
			bindings[b.ProjectID] = b
			var event commonchain.V3ReplayEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil || event.Created == nil {
				t.Fatal("creation seed unavailable", err)
			}
			event.Created.ProjectID = b.ProjectID
			event.Created.TxHash = common.BigToHash(new(big.Int).SetUint64(b.ProjectID))
			raw, err := json.Marshal(event)
			if err != nil {
				return err
			}
			if err := v3StreamWriterTx(context.Background(), tx, func(w *gorm.DB) error {
				for _, q := range []*gorm.DB{
					w.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners) SELECT ?,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners FROM projects WHERE project_id=?`, b.ProjectID, base.ProjectID),
					w.Exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(?,?,?)`, b.ProjectID, b.ChainID, v3Lower(b.ProxyAddress)),
					w.Exec(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) SELECT chain_id,proxy_address,?,cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE project_id=?`, b.ProjectID, base.ProjectID),
					w.Exec(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash,status) SELECT chain_id,proxy_address,?,block_number,block_hash,status FROM ido_v3_block_checkpoint WHERE project_id=?`, b.ProjectID, base.ProjectID),
					w.Exec(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,tx_hash,log_index,event_kind,payload) SELECT chain_id,proxy_address,?,block_number,block_hash,?,log_index,event_kind,? FROM ido_v3_event_journal WHERE project_id=?`, b.ProjectID, event.Created.TxHash.Hex(), string(raw), base.ProjectID),
				} {
					if q.Error != nil {
						return q.Error
					}
				}
				return rebuildV3ProjectTx(w, b, 90, rpc.blocks[90].head.Hash(), 1)
			}); err != nil {
				return err
			}
		}
		parent := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
		remaining := 50 * time.Second
		after := uint64(0)
		result, err := fairApprovedV3ScanWithStartBudget(parent, ids, &after, approvedV3ProjectBudget, approvedV3StreamAdmissionFloor, 1,
			func(ctx context.Context, id uint64) error {
				b := bindings[id]
				projectRPC := &v3EmptyWindowRPC{blocks: map[uint64]*v3PinnedScanStub{}, latest: rpc.latest, headerReads: map[uint64]int{}}
				for height, source := range rpc.blocks {
					block := *source
					block.project = id
					block.tuple = v3PendingTuple(id, b.ProjectToken, b.PaymentToken)
					projectRPC.blocks[height] = &block
				}
				if err := consumeV3StreamWindows(ctx, tx, projectRPC, s, b, rt); err != nil {
					return err
				}
				cursor, err := readV3ConsumerCursor(tx, b, false)
				if err != nil || cursor.CursorBlock != 93 || cursor.Revision != 4 {
					t.Fatal("consumer did not advance", id, cursor, err)
				}
				remaining -= 4 * time.Second
				parent.set(remaining)
				return nil
			}, func(uint64) error { t.Fatal("valid history revoked"); return nil })
		if err != nil || result.Completed != 7 || result.Skipped != 0 {
			t.Fatal("full seven-project coverage unavailable", result, err)
		}
		t.Log("seven durable project cursors advanced 90->93 in one serial round; maximum timeout reservation no longer skips tail")
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
