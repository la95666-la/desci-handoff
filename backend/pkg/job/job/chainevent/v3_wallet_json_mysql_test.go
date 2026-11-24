package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/projecttracking"
)

// Called by the existing network-none MySQL 8.4 runner, starting at the
// independently seeded creation block 90. No production IDs/RPC/signers.
func testV3WalletJSONLifecycleMySQL(t *testing.T, db *gorm.DB, b idov3whitelist.ExpectedProjectBinding, created *commonchain.V3CreatedEvent, parent *types.Header, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	for _, scenario := range []string{"purchase", "claim", "cancel_refund", "failed_refund", "proceeds", "fee", "token_withdraw", "settled_success", "settled_refund"} {
		t.Run("wallet_json_"+scenario, func(t *testing.T) {
			rollback := errors.New("wallet JSON fixture rollback")
			e := db.Transaction(func(tx *gorm.DB) error {
				ctx := newV3ScanTrace(context.Background(), b.ProjectID)
				word := func(s string) []byte {
					n, ok := new(big.Int).SetString(s, 10)
					if !ok {
						t.Fatal(s)
					}
					return n.FillBytes(make([]byte, 32))
				}
				buyer := common.HexToAddress("0x4444444444444444444444444444444444444444")
				headers := make([]*types.Header, 258)
				previous := parent
				for i := range headers {
					headers[i] = &types.Header{Number: new(big.Int).SetUint64(91 + uint64(i)), ParentHash: previous.Hash()}
					previous = headers[i]
				}
				var logs []types.Log
				appendLog := func(signature string, topics []common.Hash, data []byte) {
					logs = append(logs, types.Log{Address: b.ProxyAddress, BlockNumber: 91, BlockHash: headers[0].Hash(), TxHash: common.BigToHash(big.NewInt(int64(len(logs) + 100))), Index: uint(len(logs) + 1), Topics: append([]common.Hash{crypto.Keccak256Hash([]byte(signature)), common.BigToHash(new(big.Int).SetUint64(b.ProjectID))}, topics...), Data: data})
				}
				saleTopic := []common.Hash{common.BytesToHash(b.ProjectToken.Bytes())}
				payTopic := []common.Hash{common.BytesToHash(b.PaymentToken.Bytes())}
				appendLog("ProjectTokensDeposited(uint256,address,uint256)", saleTopic, word("40000000000000000000"))
				appendLog("ProjectActivated(uint256)", nil, nil)
				payment := "600"
				tokens := "30000000000000000000"
				if scenario == "failed_refund" {
					payment = "400"
					tokens = "20000000000000000000"
				}
				purchaseData := append(word(payment), common.LeftPadBytes(b.ProjectToken.Bytes(), 32)...)
				purchaseData = append(purchaseData, word(tokens)...)
				appendLog("Purchased(uint256,address,address,uint256,address,uint256)", []common.Hash{common.BytesToHash(buyer.Bytes()), payTopic[0]}, purchaseData)
				switch scenario {
				case "claim", "proceeds", "fee", "token_withdraw", "settled_success":
					appendLog("ProjectEnded(uint256,uint8,address,uint256)", payTopic, append(word("2"), word(payment)...))
					switch scenario {
					case "claim":
						appendLog("Claimed(uint256,address,address,uint256)", []common.Hash{common.BytesToHash(buyer.Bytes()), saleTopic[0]}, word("10000000000000000000"))
					case "proceeds", "fee":
						appendLog("ProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)", []common.Hash{payTopic[0], common.BytesToHash(created.ProjectOwner.Bytes())}, append(word("588"), word("12")...))
						if scenario == "fee" {
							appendLog("PlatformFeeWithdrawn(uint256,address,address,uint256)", []common.Hash{payTopic[0], common.BytesToHash(created.FeeReceiver.Bytes())}, word("12"))
						}
					case "token_withdraw":
						appendLog("ProjectTokensWithdrawn(uint256,address,address,uint256)", []common.Hash{common.BytesToHash(created.ProjectOwner.Bytes()), saleTopic[0]}, word("10000000000000000000"))
					case "settled_success":
						appendLog("Claimed(uint256,address,address,uint256)", []common.Hash{common.BytesToHash(buyer.Bytes()), saleTopic[0]}, word(tokens))
						appendLog("ProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)", []common.Hash{payTopic[0], common.BytesToHash(created.ProjectOwner.Bytes())}, append(word("588"), word("12")...))
						appendLog("PlatformFeeWithdrawn(uint256,address,address,uint256)", []common.Hash{payTopic[0], common.BytesToHash(created.FeeReceiver.Bytes())}, word("12"))
						appendLog("ProjectTokensWithdrawn(uint256,address,address,uint256)", []common.Hash{common.BytesToHash(created.ProjectOwner.Bytes()), saleTopic[0]}, word("10000000000000000000"))
					}
				case "cancel_refund", "failed_refund", "settled_refund":
					if scenario != "failed_refund" {
						appendLog("ProjectCancelled(uint256,address,uint256)", payTopic, word(payment))
					} else {
						appendLog("ProjectEnded(uint256,uint8,address,uint256)", payTopic, append(word("3"), word(payment)...))
					}
					appendLog("Refunded(uint256,address,address,uint256)", []common.Hash{common.BytesToHash(buyer.Bytes()), payTopic[0]}, word(payment))
					if scenario == "settled_refund" {
						appendLog("ProjectTokensWithdrawn(uint256,address,address,uint256)", []common.Hash{common.BytesToHash(created.ProjectOwner.Bytes()), saleTopic[0]}, word("40000000000000000000"))
					}
				}
				events := []commonchain.V3ReplayEvent{{Created: created}}
				for _, l := range logs {
					event, e := commonchain.DecodeV3ReplayLog(l, b)
					if e != nil {
						t.Fatal(e)
					}
					events = append(events, event)
				}
				totals, e := commonchain.RebuildV3Replay(b, events)
				if e != nil {
					t.Fatal(e)
				}
				observed := idov3whitelist.ProjectIdentityObservation{ChainID: b.ChainID, ProjectID: b.ProjectID, ProxyAddress: b.ProxyAddress, ProjectToken: b.ProjectToken, PaymentToken: b.PaymentToken, ProjectOwner: created.ProjectOwner, FeeReceiver: created.FeeReceiver, ProjectDecimals: created.ProjectDecimals, PaymentDecimals: created.PaymentDecimals, FeeRate: created.FeeRate, TokenPriceRaw: created.TokenPriceRaw, HardCapRaw: created.HardCapRaw, SoftCapRaw: created.SoftCapRaw, MaxPerUserRaw: created.MaxPerUserRaw, StartTimeRaw: created.StartTimeRaw, EndTimeRaw: created.EndTimeRaw, State: totals.State, RaisedAtBlockRaw: totals.Raised.String(), TokensSoldAtBlockRaw: totals.Sold.String(), BlockNumber: 91, BlockHash: headers[0].Hash()}
				journal := v3Journal{db: tx}
				if e := journal.applyChecked(ctx, b, 91, parent.Hash(), headers[0].Hash(), logs, observed); e != nil {
					t.Fatal("event application", e)
				}
				expected, e := v3ExpectedProjection(totals)
				if e != nil {
					t.Fatal(e)
				}
				var stored string
				if e := tx.Raw(`SELECT wallets FROM ido_v3_project_projection WHERE project_id=?`, b.ProjectID).Scan(&stored).Error; e != nil {
					t.Fatal(e)
				}
				if stored == expected.Wallets {
					t.Fatal("fixture did not reproduce MySQL JSON byte difference")
				}
				if !json.Valid([]byte(stored)) {
					t.Fatal("invalid MySQL JSON")
				}
				// Corrupt each wallet counter and each aggregate independently. Every
				// rejection must preserve cursor/checkpoints/effects/outbox exactly.
				before := readV3ConsumerFacts(t, tx, b)
				for _, field := range []string{"payment", "tokens", "refunded", "claimed"} {
					bad := expected
					var wallets map[string]map[string]string
					_ = json.Unmarshal([]byte(bad.Wallets), &wallets)
					wallets[v3Lower(buyer)][field] = "1"
					raw, _ := json.Marshal(wallets)
					bad.Wallets = string(raw)
					if e := replayV3ProjectTx(tx, b, 91, headers[0].Hash(), 2, &bad); e == nil {
						t.Fatal("wallet corruption accepted", field)
					}
					walletRollback := errors.New("corrupt wallet rollback")
					if e := tx.Transaction(func(w *gorm.DB) error {
						if e := w.Exec(`UPDATE ido_v3_project_projection SET wallets=? WHERE project_id=?`, bad.Wallets, b.ProjectID).Error; e != nil {
							return e
						}
						if _, e := (v3Journal{db: w}).applyVerifiedEmptyRangeBounded(ctx, b, headers[1:129], headers[0].Hash(), observed, 128); e == nil {
							t.Fatal("stored wallet corruption advanced empty window", field)
						}
						return walletRollback
					}); !errors.Is(e, walletRollback) {
						t.Fatal(e)
					}
				}
				for _, column := range []string{"raised_raw", "sold_raw", "deposited_raw", "refunded_raw", "claimed_raw", "proceeds_paid_raw", "fee_paid_raw", "project_tokens_withdrawn_raw"} {
					badRollback := errors.New("corrupt counter rollback")
					e := tx.Transaction(func(w *gorm.DB) error {
						if e := w.Exec(`UPDATE ido_v3_project_projection SET `+column+`=CAST(`+column+` AS DECIMAL(65,0))+1 WHERE project_id=?`, b.ProjectID).Error; e != nil {
							return e
						}
						if _, e := (v3Journal{db: w}).applyVerifiedEmptyRangeBounded(ctx, b, headers[1:129], headers[0].Hash(), observed, 128); e == nil {
							t.Fatal("aggregate corruption advanced empty window", column)
						}
						return badRollback
					})
					if !errors.Is(e, badRollback) {
						t.Fatal(e)
					}
				}
				after := readV3ConsumerFacts(t, tx, b)
				if !reflectV3FactsEqual(before, after) {
					t.Fatal("failed validation mutated durable facts")
				}
				// Exercise production prepared-consumer entry as well as the direct
				// journal calls above. Rollback-only stream baseline at the already
				// checked event tip; this is not a production mode-switch certificate.
				rpc := &v3EmptyWindowRPC{blocks: map[uint64]*v3PinnedScanStub{}, latest: 360, headerReads: map[uint64]int{}}
				for _, header := range append([]*types.Header{parent}, headers...) {
					tuple := v3PendingTuple(b.ProjectID, b.ProjectToken, b.PaymentToken)
					if scenario == "settled_success" || scenario == "settled_refund" {
						for _, index := range []int{18, 19, 20} {
							if scenario == "settled_success" || index == 20 {
								copy(tuple[32+index*32:32+(index+1)*32], word("1"))
							}
						}
					}
					for index, value := range map[int]string{8: new(big.Int).SetUint64(uint64(totals.State)).String(), 15: totals.Raised.String(), 16: totals.Sold.String(), 17: totals.Deposited.String()} {
						copy(tuple[32+index*32:32+(index+1)*32], word(value))
					}
					rpc.blocks[header.Number.Uint64()] = &v3PinnedScanStub{chain: b.ChainID, head: header, proxy: b.ProxyAddress, impl: rt.Implementation, payment: b.PaymentToken, sale: b.ProjectToken, project: b.ProjectID, tuple: tuple}
				}
				s := v3StreamScope{Chain: b.ChainID, Proxy: b.ProxyAddress, Runtime: rt, Confirmations: 3}
				if e := bootstrapV3Stream(ctx, tx, rpc, s, "shadow", 91, headers[0].Hash()); e != nil {
					t.Fatal(e)
				}
				for _, header := range headers[1:257] {
					raw, digest, e := v3StreamPayload(v3StreamBlock{Header: (*v3StreamHeader)(header), Logs: []types.Log{}, Receipts: map[string]*v3StreamReceipt{}})
					if e != nil {
						t.Fatal(e)
					}
					if e := insertV3Coverage(tx, s, header, raw, digest); e != nil {
						t.Fatal(e)
					}
				}
				if e := tx.Exec(`UPDATE ido_v3_proxy_stream SET mode='stream',revision=revision+1,cursor_block=347,cursor_hash=? WHERE chain_id=? AND proxy_address=?`, v3LowerHashHeader(headers[256]), b.ChainID, v3Lower(b.ProxyAddress)).Error; e != nil {
					t.Fatal(e)
				}
				sharedRollback := errors.New("shared financial quiet parity")
				var backlogTrackingSchema int64
				tx.Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_v3_project_tracking'`).Scan(&backlogTrackingSchema)
				if backlogTrackingSchema == 1 && totals.State != idov3whitelist.ProjectActive {
					old := viper.Get("idoV3ProjectTracking.enabled")
					viper.Set("idoV3ProjectTracking.enabled", true)
					err := tx.Transaction(func(w *gorm.DB) error {
						if err := projecttracking.Ensure(w, b.ProjectID); err != nil {
							return err
						}
						if err := w.Exec(`UPDATE ido_v3_project_tracking SET mode='maintenance',next_scan_at=UTC_TIMESTAMP(6),wake_block=347 WHERE project_id=?`, b.ProjectID).Error; err != nil {
							return err
						}
						if err := consumeV3Stream(ctx, w, rpc, s, b, rt, 128); err != nil {
							return err
						}
						cursor, err := readV3ConsumerCursor(w, b, false)
						if err != nil {
							return err
						}
						row, err := projecttracking.Read(ctx, w, b.ProjectID, false)
						if err != nil || cursor.CursorBlock != 219 || !row.Due || row.WakeBlock != 347 || row.Mode != "maintenance" {
							return errors.New("partial maintenance batch postponed or lost backlog")
						}
						return sharedRollback
					})
					viper.Set("idoV3ProjectTracking.enabled", old)
					if !errors.Is(err, sharedRollback) {
						t.Fatal("due maintenance partial consumption", err)
					}
				}
				var sharedFacts v3ConsumerFacts
				if err := tx.Transaction(func(w *gorm.DB) error {
					shared := context.WithValue(ctx, v3SharedBatchKey{}, true)
					if err := consumeV3StreamWindows(shared, w, rpc, s, b, rt); err != nil {
						return err
					}
					sharedFacts = readV3ConsumerFacts(t, w, b)
					return sharedRollback
				}); !errors.Is(err, sharedRollback) {
					t.Fatal(err)
				}
				for n := 0; n < 2; n++ {
					if e := consumeV3Stream(ctx, tx, rpc, s, b, rt, 128); e != nil {
						t.Fatal("post-event prepared empty window", e)
					}
				}
				after = readV3ConsumerFacts(t, tx, b)
				if !reflect.DeepEqual(sharedFacts, after) {
					t.Fatal("shared batch changed post-financial facts", scenario)
				}
				if after.Cursor.CursorBlock != 347 || after.Cursor.Revision != 258 || after.Cursor.State != "running" {
					t.Fatal(after.Cursor)
				}
				if len(after.Effects) != len(before.Effects) || len(after.Journal) != len(before.Journal) {
					t.Fatal("empty range duplicated economic events")
				}
				if !reflect.DeepEqual(before.Effects, after.Effects) || !reflect.DeepEqual(before.Journal, after.Journal) || !reflect.DeepEqual(before.Outbox, after.Outbox) || before.Status != after.Status {
					t.Fatal("empty range changed financial/history/outbox/business facts")
				}
				if len(after.Projection) != 1 {
					t.Fatal("projection disappeared")
				}
				p := after.Projection[0]
				if !v3ProjectionEqual(expected, v3ParityProjection{State: p.State, RaisedRaw: p.RaisedRaw, SoldRaw: p.SoldRaw, DepositedRaw: p.DepositedRaw, RefundedRaw: p.RefundedRaw, ClaimedRaw: p.ClaimedRaw, Wallets: p.Wallets, ProceedsPaidRaw: p.ProceedsPaidRaw, FeePaidRaw: p.FeePaidRaw, ProjectTokensWithdrawnRaw: p.ProjectTokensWithdrawnRaw}) {
					t.Fatal("empty window altered totals")
				}
				var trackingSchema int64
				tx.Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_v3_project_tracking'`).Scan(&trackingSchema)
				if trackingSchema == 1 {
					old := viper.Get("idoV3ProjectTracking.enabled")
					defer viper.Set("idoV3ProjectTracking.enabled", old)
					viper.Set("idoV3ProjectTracking.enabled", true)
					cursor, err := readV3ConsumerCursor(tx, b, false)
					if err != nil {
						t.Fatal(err)
					}
					stream, err := v3StreamCursorRead(tx, s, false)
					if err != nil {
						t.Fatal(err)
					}
					if err := verifyV3ProjectClosure(ctx, tx, rpc, s, b, rt, cursor, stream); err != nil {
						t.Fatal("actual closure evaluation", err)
					}
					tracking, err := projecttracking.Read(ctx, tx, b.ProjectID, false)
					if err != nil {
						t.Fatal(err)
					}
					wantFinished := scenario == "settled_success" || scenario == "settled_refund"
					if tracking.Finished != wantFinished || !tracking.EvidenceCurrent {
						t.Fatal("closure conclusion", scenario, tracking)
					}
					if wantFinished && (tracking.FinishedAt == nil || len(tracking.EvidenceDigest) != 64) {
						t.Fatal("missing closure evidence", tracking)
					}
					sharedTracking := context.WithValue(ctx, v3SharedBatchKey{}, true)
					sharedTracking = context.WithValue(sharedTracking, trackingFenceKey{}, trackingFence{revision: tracking.Revision, mode: tracking.Mode})
					if err := consumeV3Stream(sharedTracking, tx, rpc, s, b, rt, 128); err != nil {
						t.Fatal("tracking-enabled no-new-evidence consumer", err)
					}
					if totals.State != idov3whitelist.ProjectActive {
						if err := tx.Exec(`UPDATE ido_v3_project_tracking SET mode='maintenance',next_scan_at=TIMESTAMPADD(HOUR,12,UTC_TIMESTAMP(6)) WHERE project_id=?`, b.ProjectID).Error; err != nil {
							t.Fatal(err)
						}
						ids, err := selectV3TrackingCandidates(ctx, tx, []uint64{b.ProjectID})
						if err != nil || len(ids) != 0 {
							t.Fatal("not-due project scanned", ids, err)
						}
						// A deliberately stale/missing closure certificate must not
						// force acknowledged maintenance backlog into every tick.
						if err := tx.Transaction(func(w *gorm.DB) error {
							if err := w.Exec(`UPDATE ido_v3_project_tracking SET finished=0,finished_at=NULL,evidence_epoch=0,evidence_block=0,evidence_revision=0,wake_block=348 WHERE project_id=?`, b.ProjectID).Error; err != nil {
								return err
							}
							row, err := projecttracking.Read(ctx, w, b.ProjectID, false)
							if err != nil || row.EvidenceCurrent || row.RecoveryRequired || !skipV3TrackingTurn(row) {
								return errors.New("stale maintenance was not deferred")
							}
							if err := w.Exec(`UPDATE ido_v3_project_tracking SET next_scan_at=UTC_TIMESTAMP(6) WHERE project_id=?`, b.ProjectID).Error; err != nil {
								return err
							}
							row, err = projecttracking.Read(ctx, w, b.ProjectID, false)
							if err != nil || !row.Due || skipV3TrackingTurn(row) {
								return errors.New("due backlog not selected")
							}
							if err := w.Exec(`UPDATE ido_v3_project_tracking SET next_scan_at=TIMESTAMPADD(HOUR,12,UTC_TIMESTAMP(6)),evidence_epoch=? WHERE project_id=?`, stream.Epoch+1, b.ProjectID).Error; err != nil {
								return err
							}
							row, err = projecttracking.Read(ctx, w, b.ProjectID, false)
							if err != nil || !row.RecoveryRequired || skipV3TrackingTurn(row) {
								return errors.New("epoch recovery was deferred")
							}
							return sharedRollback
						}); !errors.Is(err, sharedRollback) {
							t.Fatal("maintenance backlog scheduling", err)
						}
						if wantFinished {
							if err := tx.Exec(`UPDATE ido_v3_project_tracking SET mode='archived' WHERE project_id=?`, b.ProjectID).Error; err != nil {
								t.Fatal(err)
							}
							ids, err = selectV3TrackingCandidates(ctx, tx, []uint64{b.ProjectID})
							if err != nil || len(ids) != 0 {
								t.Fatal("archive scanned", ids, err)
							}
						}
						if err := wakeV3TrackingFromCoverageTx(tx, s, []v3StreamBlock{{Logs: []types.Log{{BlockNumber: 348, Topics: []common.Hash{crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")), common.BigToHash(new(big.Int).SetUint64(b.ProjectID))}}}}}); err != nil {
							t.Fatal(err)
						}
						ids, err = selectV3TrackingCandidates(ctx, tx, []uint64{b.ProjectID})
						if err != nil || len(ids) != 1 {
							t.Fatal("durable event failed to wake", ids, err)
						}
					}
					facts := readV3ConsumerFacts(t, tx, b)
					if !reflect.DeepEqual(facts, after) {
						t.Fatal("closure evaluation changed financial facts")
					}
				}
				t.Logf("%s -> two 128-block empty windows; original JSON bytes differed; effects=%d; corruption rejected", scenario, len(before.Effects))
				return rollback
			})
			if !errors.Is(e, rollback) {
				t.Fatal(e)
			}
		})
	}
}

func reflectV3FactsEqual(a, b v3ConsumerFacts) bool { return reflect.DeepEqual(a, b) }
