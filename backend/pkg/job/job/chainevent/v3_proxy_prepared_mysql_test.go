package chainevent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/idov3whitelist"
)

type v3SQLCounter struct {
	logger.Interface
	mu                                    sync.Mutex
	total, coverage, journal, checkpoints int
}

func (c *v3SQLCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	q, _ := fc()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total++
	if strings.Contains(q, "SELECT c.block_number,c.block_hash,c.parent_hash,c.digest,c.payload") {
		c.coverage++
	}
	if strings.Contains(q, "SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal") {
		c.journal++
	}
	if strings.HasPrefix(q, "INSERT INTO ido_v3_block_checkpoint") {
		c.checkpoints++
	}
}

type v3ConsumerFacts struct {
	Cursor     v3ConsumerCursor
	Projection []struct {
		State, RaisedRaw, SoldRaw, DepositedRaw, RefundedRaw, ClaimedRaw, Wallets, ProceedsPaidRaw, FeePaidRaw, ProjectTokensWithdrawnRaw string
		Revision, CursorBlock                                                                                                             uint64
		CursorHash                                                                                                                        string
	}
	Journal []struct {
		BlockNumber                uint64
		BlockHash, TxHash          string
		LogIndex                   uint
		EventKind, Payload, Status string
	}
	Effects []struct {
		BlockNumber                                                                  uint64
		TxHash, EntryKind, AssetRole, WalletAddress, TokenAddress, AmountRaw, Status string
	}
	Policy []struct {
		BlockNumber, Version uint64
		TxHash, Root, State  string
		Enabled              bool
	}
	Checkpoints []struct {
		BlockNumber       uint64
		BlockHash, Status string
	}
	Outbox []struct {
		Action, State, TxHash string
		Nonce                 *uint64
		ActiveSlot            *uint8
		ReceiptStatus         *uint64
		ReceiptBlockNumber    *uint64
		ReceiptBlockHash      string
	}
	Status string
}

func readV3ConsumerFacts(t *testing.T, tx *gorm.DB, b idov3whitelist.ExpectedProjectBinding) v3ConsumerFacts {
	t.Helper()
	var f v3ConsumerFacts
	var e error
	f.Cursor, e = readV3ConsumerCursor(tx, b, false)
	if e != nil {
		t.Fatal(e)
	}
	for _, read := range []*gorm.DB{
		tx.Raw(`SELECT state,raised_raw,sold_raw,deposited_raw,refunded_raw,claimed_raw,wallets,proceeds_paid_raw,fee_paid_raw,project_tokens_withdrawn_raw,revision,cursor_block,cursor_hash FROM ido_v3_project_projection WHERE project_id=?`, b.ProjectID).Scan(&f.Projection),
		tx.Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload,status FROM ido_v3_event_journal WHERE project_id=? ORDER BY block_number,log_index`, b.ProjectID).Scan(&f.Journal),
		tx.Raw(`SELECT block_number,tx_hash,entry_kind,asset_role,wallet_address,token_address,amount_raw,status FROM ido_v3_financial_effect WHERE project_id=? ORDER BY block_number,log_index,asset_role`, b.ProjectID).Scan(&f.Effects),
		tx.Raw(`SELECT block_number,version,tx_hash,root,state,enabled FROM ido_v3_policy_event_journal WHERE project_id=? ORDER BY block_number,log_index`, b.ProjectID).Scan(&f.Policy),
		tx.Raw(`SELECT status FROM projects WHERE project_id=?`, b.ProjectID).Scan(&f.Status),
		tx.Raw(`SELECT block_number,block_hash,status FROM ido_v3_block_checkpoint WHERE project_id=? ORDER BY block_number`, b.ProjectID).Scan(&f.Checkpoints),
		tx.Raw(`SELECT action,state,tx_hash,nonce,active_slot,receipt_status,receipt_block_number,receipt_block_hash FROM chain_tx_submission WHERE project_id=? ORDER BY id`, b.ProjectID).Scan(&f.Outbox),
	} {
		if read.Error != nil {
			t.Fatal(read.Error)
		}
	}
	return f
}

func testV3PreparedConsumerParity(t *testing.T, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity, limit uint64) {
	t.Helper()
	rollback := errors.New("differential consumer rollback")
	var old, new v3ConsumerFacts
	counts := []*v3SQLCounter{}
	for i, consume := range []func(context.Context, *gorm.DB, v3ScannerRPC, v3StreamScope, idov3whitelist.ExpectedProjectBinding, idov3whitelist.RuntimeIdentity, uint64) error{consumeV3StreamLegacy, consumeV3Stream} {
		c := &v3SQLCounter{Interface: logger.Default.LogMode(logger.Silent)}
		counts = append(counts, c)
		e := db.Transaction(func(tx *gorm.DB) error {
			if e := consume(context.Background(), tx.Session(&gorm.Session{Logger: c}), rpc, s, b, rt, limit); e != nil {
				t.Fatalf("differential pass %d: %v", i, e)
			}
			f := readV3ConsumerFacts(t, tx, b)
			if i == 0 {
				old = f
			} else {
				new = f
			}
			return rollback
		})
		if !errors.Is(e, rollback) {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(old, new) {
		t.Fatalf("legacy/prepared facts differ\nold=%+v\nnew=%+v", old, new)
	}
	// Event-driven routing must preserve identical event/effect/outbox facts,
	// not merely match cursors after an empty synthetic interval.
	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := context.WithValue(context.Background(), v3SharedBatchKey{}, true)
		if err := consumeV3Stream(ctx, tx, rpc, s, b, rt, limit); err != nil {
			return err
		}
		if shared := readV3ConsumerFacts(t, tx, b); !reflect.DeepEqual(new, shared) {
			t.Fatalf("shared event batch facts differ\nprepared=%+v\nshared=%+v", new, shared)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if counts[1].total >= counts[0].total || counts[1].coverage >= counts[0].coverage {
		t.Fatalf("SQL not reduced: old=%+v new=%+v", counts[0], counts[1])
	}
	t.Logf("consumer parity limit=%d SQL %d -> %d; coverage SELECT %d -> %d; full-journal SELECT %d -> %d; checkpoint INSERT %d -> %d", limit, counts[0].total, counts[1].total, counts[0].coverage, counts[1].coverage, counts[0].journal, counts[1].journal, counts[0].checkpoints, counts[1].checkpoints)
}
