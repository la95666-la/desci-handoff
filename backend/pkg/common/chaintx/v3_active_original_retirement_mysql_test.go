package chaintx

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"math/big"
	"strings"
	"testing"
)

func testActiveOriginalRefreshMySQL(t *testing.T, tx *gorm.DB, retiredID uint64, b idov3whitelist.ExpectedProjectBinding, proof retiredNonceEvidence) {
	t.Helper()
	// Fixture-only legacy seeding after stream activation; restore the session
	// marker before evaluating the production recovery function.
	if e := tx.Exec(`SET @jc_v3_stream_writer=1`).Error; e != nil {
		t.Fatal(e)
	}
	defer tx.Exec(`SET @jc_v3_stream_writer=0`)
	ctx := context.Background()
	exec := func(s string, args ...any) {
		if e := tx.Exec(s, args...).Error; e != nil {
			t.Fatal(e)
		}
	}
	activeBinding := b
	activeBinding.ProjectID = 998
	proxy := strings.ToLower(b.ProxyAddress.Hex())
	key, _ := crypto.HexToECDSA(strings.Repeat("7", 64))
	operator := crypto.PubkeyToAddress(key.PublicKey).Hex()
	data, e := basechain.PackV3Lifecycle("activateProject", big.NewInt(998))
	if e != nil {
		t.Fatal(e)
	}
	signed, e := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 8, To: &b.ProxyAddress, Gas: 100000, GasPrice: big.NewInt(1), Data: data}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := signed.MarshalBinary()
	exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,token_address,payment_token_address,description,tags,team_info,roadmap,partners) VALUES(998,'Set','capped_presale','ido_v3_stablecoin',31337,?,?,?,'','','','','')`, proxy, strings.ToLower(b.ProjectToken.Hex()), strings.ToLower(b.PaymentToken.Hex()))
	exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(998,31337,?)`, proxy)
	exec(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,manifest_digest,create_calldata_hash,operator_address) VALUES(998,31337,?,CONCAT('0x',REPEAT('1',64)),CONCAT('0x',REPEAT('2',64)),CONCAT('0x',REPEAT('3',64)),?)`, proxy, strings.ToLower(operator))
	exec(`UPDATE projects SET status='Pending',onchain_locked=1 WHERE project_id=998`)
	exec(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,contract_address,call_data,state,active_slot,last_error) VALUES(998,'ido_v3_activate',1,31337,?,?,?,'ready',1,'')`, operator, proxy, data)
	exec(`UPDATE chain_tx_submission SET state='signed',nonce=8,gas_limit=100000,gas_price_raw='1',raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6) WHERE project_id=998 AND state='ready'`, raw, signed.Hash().Hex())
	var active entity.ChainTxSubmission
	if tx.Where("project_id=998").Take(&active).Error != nil {
		t.Fatal("active")
	}
	exec(`UPDATE chain_relayer_nonce SET next_nonce=9,active_submission_id=? WHERE chain_id=31337 AND sender_address=?`, active.ID, operator)
	h := common.HexToHash("0x123")
	exec(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) VALUES(31337,?,998,110,?,1,'running')`, proxy, h.Hex())
	exec(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash,status) VALUES(31337,?,998,110,?,'canonical')`, proxy, h.Hex())
	// Expire by inserting a new independently shaped historical fixture, never
	// by altering immutable expiry. Latest state remains enrolled/non-revoked.
	exec(`INSERT INTO ido_v3_retired_nonce_evidence(submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,evidence_digest,expires_at,created_at,state) SELECT submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,REPEAT('e',64),UTC_TIMESTAMP(6)-INTERVAL 1 SECOND,UTC_TIMESTAMP(6)-INTERVAL 10 MINUTE,'valid' FROM ido_v3_retired_nonce_evidence WHERE submission_id=? ORDER BY id DESC LIMIT 1`, retiredID)
	var retired entity.ChainTxSubmission
	if tx.Where("id=?", retiredID).Take(&retired).Error != nil {
		t.Fatal("retired")
	}
	in, e := readActiveRetirementInput(tx, retired, false)
	if e != nil {
		t.Fatal(e)
	}
	proof.SubmissionID = retiredID
	proof.JournalID = *retired.FinalEventRecordID
	proof.ConfirmedBlock = 112
	proof.ConsumedNonce = 8
	proof.EvidenceDigest = strings.Repeat("f", 64)
	in.Proof = proof
	var before int64
	tx.Raw(`SELECT COUNT(*) FROM ido_v3_retired_nonce_evidence`).Scan(&before)
	if e := commitActiveRetirement(ctx, tx, active, activeBinding, []activeRetirementInput{in}, false); e != nil {
		t.Fatal("dryrun", e)
	}
	var after int64
	tx.Raw(`SELECT COUNT(*) FROM ido_v3_retired_nonce_evidence`).Scan(&after)
	if before != after {
		t.Fatal("dry wrote")
	}
	stop := errors.New("negative rollback")
	for _, kind := range []string{"revoked", "nonce", "broadcast"} {
		e := tx.Transaction(func(w *gorm.DB) error {
			switch kind {
			case "revoked":
				if e := w.Exec(`UPDATE ido_v3_retired_nonce_evidence SET state='revoked' WHERE id=?`, in.PreviousID).Error; e != nil {
					return e
				}
			case "nonce":
				if e := w.Exec(`UPDATE chain_relayer_nonce SET next_nonce=10 WHERE chain_id=31337 AND sender_address=?`, operator).Error; e != nil {
					return e
				}
			case "broadcast":
				if e := w.Exec(`UPDATE chain_tx_submission SET state='broadcast_unknown',broadcast_count=1,last_broadcast_at=NOW(6) WHERE id=?`, active.ID).Error; e != nil {
					return e
				}
			}
			if commitActiveRetirement(ctx, w, active, activeBinding, []activeRetirementInput{in}, true) == nil {
				t.Fatal("unsafe commit", kind)
			}
			return stop
		})
		if !errors.Is(e, stop) {
			t.Fatal(e)
		}
	}
	if e := commitActiveRetirement(ctx, tx, active, activeBinding, []activeRetirementInput{in}, true); e != nil {
		t.Fatal("refresh", e)
	}
	tx.Raw(`SELECT COUNT(*) FROM ido_v3_retired_nonce_evidence`).Scan(&after)
	if after != before+1 {
		t.Fatal("count")
	}
	var now entity.ChainTxSubmission
	tx.Where("id=?", active.ID).Take(&now)
	if !sameActiveOriginal(active, now, activeBinding) {
		t.Fatal("active changed")
	}
	var fence entity.ChainRelayerNonce
	tx.Where("chain_id=31337 AND sender_address=?", operator).Take(&fence)
	if !activeRetirementFence(active, fence) {
		t.Fatal("fence changed")
	}
	t.Log("expired enrolled retirement refreshed with exact active original; dryrun/revocation/nonce/marked-send fences retained; outbox and nonce unchanged")
}
