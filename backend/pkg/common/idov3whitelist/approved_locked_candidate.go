package idov3whitelist

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// ApprovedLockedCandidate is a DB join for a project already bridged by Job.
// It is NOT scanner, lifecycle or wallet admission: callers must independently
// audit the original signed create, canonical receipt, runtime and cursor at
// confirmed blocks. Never use this result as signer or proof authority alone.
type ApprovedLockedCandidate struct {
	Record          ApprovedSetRecord
	Binding         ExpectedProjectBinding
	Runtime         RuntimeIdentity
	SubmissionID    uint64
	CreationBlock   uint64
	CreationHash    common.Hash
	CreationTx      common.Hash
	CreationLog     uint
	CreationPayload string
	CursorBlock     uint64
	CursorHash      common.Hash
	CursorRevision  uint64
}

func ReadApprovedLockedCandidate(ctx context.Context, database *gorm.DB, projectID uint64) (ApprovedLockedCandidate, error) {
	return readApprovedLockedCandidate(ctx, database, projectID, false)
}

// ReadApprovedHaltedCandidate is only for bounded reorg recovery. Its cursor
// hash is historical DB evidence, not a currently canonical chain anchor;
// never pass it to a signer, ordinary scanner or wallet proof.
func ReadApprovedHaltedCandidate(ctx context.Context, database *gorm.DB, projectID uint64) (ApprovedLockedCandidate, error) {
	return readApprovedLockedCandidate(ctx, database, projectID, true)
}

func readApprovedLockedCandidate(ctx context.Context, database *gorm.DB, projectID uint64, halted bool) (ApprovedLockedCandidate, error) {
	invalid := errors.New("V3 locked approval candidate unavailable")
	if ctx == nil || database == nil || projectID == 0 {
		return ApprovedLockedCandidate{}, invalid
	}
	record, err := ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil {
		return ApprovedLockedCandidate{}, invalid
	}
	var rows []struct {
		Block         uint64 `gorm:"column:block_number"`
		BlockHash     string `gorm:"column:block_hash"`
		TxHash        string `gorm:"column:tx_hash"`
		LogIndex      uint   `gorm:"column:log_index"`
		Payload       string `gorm:"column:payload"`
		CursorBlock   uint64 `gorm:"column:cursor_block"`
		CursorHash    string `gorm:"column:cursor_hash"`
		Revision      uint64 `gorm:"column:revision"`
		State         string `gorm:"column:state"`
		Checkpoint    string `gorm:"column:checkpoint_hash"`
		ProjectToken  string `gorm:"column:token_address"`
		PaymentToken  string `gorm:"column:payment_token_address"`
		Owner         string `gorm:"column:owner_address"`
		Receiver      string `gorm:"column:fee_receiver"`
		FeeRate       uint16 `gorm:"column:success_fee_rate"`
		TokenDecimals uint8  `gorm:"column:token_decimals"`
		PayDecimals   uint8  `gorm:"column:payment_token_decimals"`
		CallHash      string `gorm:"column:create_calldata_hash"`
		Sender        string `gorm:"column:sender_address"`
		ReceiptBlock  uint64 `gorm:"column:receipt_block_number"`
		ReceiptHash   string `gorm:"column:receipt_block_hash"`
		ReceiptStatus uint64 `gorm:"column:receipt_status"`
		SubmissionID  uint64 `gorm:"column:submission_id"`
	}
	// Plain proxy equality supplies the existing composite PK's index prefix.
	// BINARY equality remains mandatory as an exact residual identity guard.
	// Without the plain equality MySQL can scan every checkpoint on the chain.
	const query = `SELECT j.block_number,j.block_hash,j.tx_hash,j.log_index,j.payload,
		s.cursor_block,s.cursor_hash,s.revision,s.state,
		ck.block_hash AS checkpoint_hash,p.token_address,p.payment_token_address,p.owner_address,
		p.fee_receiver,p.success_fee_rate,p.token_decimals,p.payment_token_decimals,
		c.create_calldata_hash,t.sender_address,t.id AS submission_id,
		t.receipt_block_number,t.receipt_block_hash,t.receipt_status
		FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
		 AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		 AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		JOIN ido_v3_scan_cursor s ON s.project_id=b.project_id AND s.chain_id=b.chain_id
		 AND BINARY s.proxy_address=BINARY b.proxy_address AND BINARY s.contract_version=BINARY b.contract_version
		JOIN ido_v3_block_checkpoint ck ON ck.project_id=s.project_id AND ck.chain_id=s.chain_id
		 AND ck.proxy_address=s.proxy_address
		 AND BINARY ck.proxy_address=BINARY s.proxy_address AND BINARY ck.contract_version=BINARY s.contract_version
		 AND ck.block_number=s.cursor_block AND BINARY ck.block_hash=BINARY s.cursor_hash
		 AND BINARY ck.status=BINARY 'canonical'
		JOIN ido_v3_event_journal j ON j.project_id=b.project_id AND j.chain_id=b.chain_id
		 AND BINARY j.proxy_address=BINARY b.proxy_address AND BINARY j.contract_version=BINARY b.contract_version
		 AND BINARY j.status=BINARY 'canonical' AND BINARY j.event_kind=BINARY 'created'
		JOIN ido_v3_block_checkpoint k ON k.project_id=j.project_id AND k.chain_id=j.chain_id
		 AND k.proxy_address=j.proxy_address
		 AND BINARY k.proxy_address=BINARY j.proxy_address AND BINARY k.contract_version=BINARY j.contract_version
		 AND k.block_number=j.block_number AND BINARY k.block_hash=BINARY j.block_hash AND BINARY k.status=BINARY 'canonical'
		JOIN chain_tx_submission t ON t.project_id=b.project_id AND t.chain_id=b.chain_id
		 AND BINARY t.contract_address=BINARY b.proxy_address AND BINARY t.action=BINARY 'ido_v3_create'
		 AND BINARY t.tx_hash=BINARY j.tx_hash AND BINARY t.state=BINARY ?
		 AND t.final_event_record_id=j.id AND t.active_slot IS NULL
		 AND t.receipt_status=1 AND t.receipt_block_number=j.block_number
		 AND BINARY t.receipt_block_hash=BINARY j.block_hash
		WHERE p.project_id=? AND p.onchain_locked=1 AND p.chain_id=56
		 AND BINARY p.contract_address=BINARY ? AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		 AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY s.state=BINARY ?
		 AND s.revision>0 AND s.cursor_block>=j.block_number LIMIT 2`
	state := "running"
	attemptState := "applied"
	if halted {
		state = "reorg_halted"
		attemptState = "v3_reorg_suspended"
	}
	read := database.WithContext(ctx).Raw(query, attemptState, projectID, record.Identity.Proxy, state).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return ApprovedLockedCandidate{}, invalid
	}
	r := rows[0]
	var creation struct {
		Created   json.RawMessage `json:"Created"`
		Purchase  json.RawMessage `json:"Purchase"`
		Lifecycle json.RawMessage `json:"Lifecycle"`
	}
	if !decodeStrictJSON(r.Payload, &creation) || len(creation.Created) == 0 || string(creation.Created) == "null" ||
		(len(creation.Purchase) != 0 && string(creation.Purchase) != "null") ||
		(len(creation.Lifecycle) != 0 && string(creation.Lifecycle) != "null") {
		return ApprovedLockedCandidate{}, invalid
	}
	if r.SubmissionID == 0 || len(r.Payload) == 0 || len(r.Payload) > 16384 ||
		r.Block < 128805696 || r.CursorBlock < r.Block || r.Revision == 0 || r.State != state ||
		r.Checkpoint != r.CursorHash ||
		r.ProjectToken != record.Identity.ProjectToken || r.PaymentToken != record.Identity.PaymentToken ||
		r.Owner != record.Identity.ProjectOwner || r.Receiver != record.Identity.FeeReceiver ||
		r.FeeRate != record.Identity.FeeRate || r.TokenDecimals != record.Identity.TokenDecimals ||
		r.PayDecimals != record.Identity.PaymentDecimals ||
		r.CallHash != record.Identity.CreateCallHash || !strings.EqualFold(r.Sender, record.Identity.Operator) ||
		r.ReceiptBlock != r.Block || r.ReceiptHash != r.BlockHash || r.ReceiptStatus != 1 ||
		!canonicalAdmissionHash(r.BlockHash) || !canonicalAdmissionHash(r.TxHash) ||
		!canonicalAdmissionHash(r.CursorHash) || strings.EqualFold(r.CursorHash, "0x"+strings.Repeat("0", 64)) {
		return ApprovedLockedCandidate{}, invalid
	}
	return ApprovedLockedCandidate{Record: record,
		Binding: ExpectedProjectBinding{ChainID: 56, ProjectID: projectID,
			ProxyAddress: common.HexToAddress(record.Identity.Proxy), ProjectToken: common.HexToAddress(r.ProjectToken),
			PaymentToken: common.HexToAddress(r.PaymentToken)},
		Runtime: RuntimeIdentity{Implementation: common.HexToAddress("0x887fcc6e54fc8229919166ad90b16c19b4992560"),
			ProxyCodeHash: common.HexToHash("0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8"),
			CodeHash:      common.HexToHash("0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590")},
		SubmissionID: r.SubmissionID, CreationBlock: r.Block, CreationHash: common.HexToHash(r.BlockHash), CreationTx: common.HexToHash(r.TxHash),
		CreationLog:     r.LogIndex,
		CreationPayload: r.Payload,
		CursorBlock:     r.CursorBlock, CursorHash: common.HexToHash(r.CursorHash), CursorRevision: r.Revision}, nil
}
