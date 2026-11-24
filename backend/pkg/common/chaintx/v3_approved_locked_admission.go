package chaintx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// AdmitApprovedLockedProject checks the executed Set, original applied create,
// canonical ProjectCreated receipt, creation economics, running cursor and
// reviewed runtime. It never signs, sends or changes state. A returned value
// is point-in-time evidence: each consumer still checks its own policy and
// cursor and repeats admission before a write. Normal contiguous scanner
// progress is accepted by ALL consumers, never only preview consumers.
func AdmitApprovedLockedProject(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, error) {
	return admitApprovedLockedProject(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash, false, nil, false)
}

// AuditApprovedLockedProjectForScanner performs the SAME full original-create
// audit and returns its evidence snapshot for a process-local scanner history
// cache. It grants no signing, lifecycle, review, wallet or TTL authority.
// Scanner callers must enforce their own bounded lifetime, dynamic live checks
// and precommit evidence/epoch/CAS fences. Financial entrypoints stay full-only.
func AuditApprovedLockedProjectForScanner(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	var snapshot idov3whitelist.ApprovedLockedCandidate
	admitted, err := admitApprovedLockedProject(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash, false, &snapshot, false)
	return admitted, snapshot, err
}

// AdmitApprovedLockedProjectForPreview is ONLY for an initial, read-only
// policy candidate. The approved scanner may advance its canonical cursor
// during the historical receipt audit. This does not authorize a review,
// transaction, proof or wallet entitlement.
func AdmitApprovedLockedProjectForPreview(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	var snapshot idov3whitelist.ApprovedLockedCandidate
	admitted, err := admitApprovedLockedProject(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash, true, &snapshot, false)
	return admitted, snapshot, err
}

// AdmitApprovedLockedProjectForPolicyReview is restricted to the off-chain
// first-enable decision. It performs the same original create/receipt/runtime
// audit as general admission, with a returned evidence snapshot for review.
// All paths tolerate independently checked contiguous forward scanner progress.
func AdmitApprovedLockedProjectForPolicyReview(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	return AdmitApprovedLockedProjectForPreview(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash)
}

// AdmitApprovedLockedProjectForPolicyObservation authenticates the original
// create and canonical running cursor for an already-executed external Safe
// receipt. The scanner may advance during the historical RPC audit; every
// forward tip must still be contiguous and independently checked on-chain.
// This never authorizes a Safe send, lifecycle action, or wallet proof.
func AdmitApprovedLockedProjectForPolicyObservation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	return AdmitApprovedLockedProjectForPreview(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash)
}

// AdmitApprovedLockedProjectForActivation fully authenticates the original
// applied create while allowing a separately verified, contiguous canonical
// scanner advance during the historical RPC audit. It grants no signer or
// broadcast authority: Core must still simulate the current activation,
// lock the cursor and nonce, and recheck before sending the original attempt.
func AdmitApprovedLockedProjectForActivation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	return AdmitApprovedLockedProjectForLifecycle(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash)
}

// All operator lifecycle methods authenticate the same original create.
// Normal canonical forward progress is not a reorg. This evidence alone
// cannot sign/send: current method-specific simulation and locked outbox
// checks remain mandatory, including immediately before the one allowed send.
func AdmitApprovedLockedProjectForLifecycle(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	var snapshot idov3whitelist.ApprovedLockedCandidate
	admitted, err := admitApprovedLockedProject(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash, true, &snapshot, true)
	return admitted, snapshot, err
}

// Reuse the original full receipt audit, not its authority for any action.
// The lifecycle caller still re-simulates that action and locks its nonce.
func RecheckApprovedLifecycleCreation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, before idov3whitelist.ApprovedLockedCandidate) error {
	return RecheckApprovedPolicyObservation(ctx, database, rpc, projectID, before)
}

type activationAdmissionCheck string

func (e activationAdmissionCheck) Error() string {
	return "V3 activation creation admission unavailable"
}

// Only fixed check names; underlying database or commercial RPC errors never
// cross this diagnostic boundary.
func ActivationAdmissionFailureCheck(err error) string {
	var check activationAdmissionCheck
	if errors.As(err, &check) {
		switch check {
		case "candidate", "original_attempt", "chain", "confirmations", "creation_header",
			"creation_receipt", "creation_logs", "creation_event", "creation_project",
			"creation_runtime", "cursor_header", "confirmed_runtime", "confirmed_project",
			"repeat_receipt", "final_confirmations", "final_headers", "candidate_recheck":
			return string(check)
		}
	}
	return "unavailable"
}

// RecheckPolicyObservationCandidate fences immutable Set/create evidence and
// monotone canonical cursor progress between the first and final receipt
// audits. A rewind, halt/resume or revision-only rewrite still fails closed.
func RecheckPolicyObservationCandidate(before, after idov3whitelist.ApprovedLockedCandidate) error {
	if before.SubmissionID == 0 || !sameApprovedV3PreviewEvidence(before, after) ||
		!approvedV3PreviewCursorProgress(before, after, true) {
		return errors.New("V3 policy observation creation admission changed")
	}
	return nil
}

// RecheckApprovedPolicyObservation keeps the original full create audit as
// the admission authority. Before the immutable observation write, re-read
// the approved Set/create/cursor join, verify contiguous canonical progress,
// and pin both the current cursor and original creation receipt to headers.
// It does not repeat historical runtime/economics simulations or authorize
// any wallet action. The write transaction separately locks the live cursor.
func RecheckApprovedPolicyObservation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, before idov3whitelist.ApprovedLockedCandidate) error {
	return recheckApprovedPolicyObservation(ctx, database, rpc, projectID, before, false)
}

// Display-only: a halted snapshot is historical evidence, never write/proof
// authority. It must remain exactly halted at the audited cursor revision.
func RecheckApprovedV3DisplayObservation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC, projectID uint64, before idov3whitelist.ApprovedLockedCandidate, halted bool) error {
	return recheckApprovedPolicyObservation(ctx, database, rpc, projectID, before, halted)
}

func recheckApprovedPolicyObservation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC, projectID uint64, before idov3whitelist.ApprovedLockedCandidate, halted bool) error {
	invalid := errors.New("V3 policy observation final admission unavailable")
	if ctx == nil || ctx.Err() != nil || database == nil || rpc == nil || projectID == 0 ||
		before.SubmissionID == 0 || before.CreationHash == (common.Hash{}) || before.CreationTx == (common.Hash{}) {
		return invalid
	}
	reader := idov3whitelist.ReadApprovedLockedCandidate
	if halted {
		reader = idov3whitelist.ReadApprovedHaltedCandidate
	}
	after, err := reader(ctx, database, projectID)
	if err != nil || RecheckPolicyObservationCandidate(before, after) != nil {
		return invalid
	}
	if !approvedV3DisplayCursorProgress(before, after, halted) {
		return invalid
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, after.CursorBlock, 15) ||
		!idov3whitelist.HasConfirmations(head, before.CreationBlock, 15) {
		return invalid
	}
	for _, anchor := range []struct {
		block uint64
		hash  common.Hash
	}{{before.CreationBlock, before.CreationHash}, {before.CursorBlock, before.CursorHash}, {after.CursorBlock, after.CursorHash}} {
		block := new(big.Int).SetUint64(anchor.block)
		header, e := rpc.HeaderByNumber(ctx, block)
		if e != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 || header.Hash() != anchor.hash {
			return invalid
		}
	}
	receipt, err := rpc.TransactionReceipt(ctx, before.CreationTx)
	if err != nil || receipt == nil || receipt.TxHash != before.CreationTx ||
		receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil ||
		!receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != before.CreationBlock ||
		receipt.BlockHash != before.CreationHash {
		return invalid
	}
	var journal basechain.V3ReplayEvent
	if json.Unmarshal([]byte(before.CreationPayload), &journal) != nil || journal.Created == nil ||
		journal.Purchase != nil || journal.Lifecycle != nil || journal.Created.LogIndex != before.CreationLog ||
		basechain.MatchV3CreatedReceipt(journal.Created, before.Binding, receipt) != nil {
		return invalid
	}
	return nil
}

func admitApprovedLockedProject(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, ownerCodeHash, governorCodeHash common.Hash, _ bool,
	snapshot *idov3whitelist.ApprovedLockedCandidate, diagnose bool) (admitted idov3whitelist.AdmittedProject, resultErr error) {
	return auditApprovedLockedProject(ctx, database, rpc, projectID, ownerCodeHash, governorCodeHash, snapshot, diagnose, false)
}

// ReadApprovedV3DisplayAdmission audits the same original signature, receipt,
// economics and runtime for an Admin display. Halted is never accepted by the
// existing lifecycle/scanner/review admission entrypoints.
func ReadApprovedV3DisplayAdmission(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC, projectID uint64, owner, governor common.Hash, halted bool) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	var snapshot idov3whitelist.ApprovedLockedCandidate
	a, e := auditApprovedLockedProject(ctx, database, rpc, projectID, owner, governor, &snapshot, true, halted)
	return a, snapshot, e
}

func auditApprovedLockedProject(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC, projectID uint64, ownerCodeHash, governorCodeHash common.Hash, snapshot *idov3whitelist.ApprovedLockedCandidate, diagnose, halted bool) (admitted idov3whitelist.AdmittedProject, resultErr error) {
	var empty idov3whitelist.AdmittedProject
	invalid := errors.New("V3 approved locked admission unavailable")
	check := activationAdmissionCheck("candidate")
	defer func() {
		if diagnose && resultErr != nil {
			resultErr = check
		}
	}()
	if ctx == nil || database == nil || rpc == nil || projectID == 0 ||
		ownerCodeHash == (common.Hash{}) || governorCodeHash == (common.Hash{}) {
		return empty, invalid
	}
	reader := idov3whitelist.ReadApprovedLockedCandidate
	if halted {
		reader = idov3whitelist.ReadApprovedHaltedCandidate
	}
	candidate, err := reader(ctx, database, projectID)
	if err != nil || candidate.SubmissionID == 0 || candidate.CreationBlock < 128805696 ||
		candidate.CreationBlock > candidate.CursorBlock {
		return empty, invalid
	}
	check = "original_attempt"
	var attempts []entity.ChainTxSubmission
	query := `SELECT * FROM chain_tx_submission WHERE id=? AND chain_id=?
		AND project_id=? AND BINARY action=BINARY 'ido_v3_create'
		AND BINARY contract_address=BINARY ? AND BINARY LOWER(sender_address)=BINARY ?
		AND BINARY state=BINARY 'applied' AND active_slot IS NULL LIMIT 2`
	if halted {
		query = strings.Replace(query, "BINARY 'applied'", "BINARY 'v3_reorg_suspended'", 1)
	}
	read := database.WithContext(ctx).Raw(query,
		candidate.SubmissionID, candidate.Binding.ChainID, projectID,
		candidate.Record.Identity.Proxy, candidate.Record.Identity.Operator).Scan(&attempts)
	if read.Error != nil || len(attempts) != 1 {
		return empty, invalid
	}
	row := attempts[0]
	if row.TxHash == nil || row.Nonce == nil || row.V3SignedAt == nil || row.ReceiptBlockNumber == nil ||
		row.ReceiptStatus == nil || *row.ReceiptStatus != types.ReceiptStatusSuccessful ||
		*row.ReceiptBlockNumber != candidate.CreationBlock ||
		!strings.EqualFold(row.ReceiptBlockHash, candidate.CreationHash.Hex()) ||
		!strings.EqualFold(*row.TxHash, candidate.CreationTx.Hex()) ||
		!strings.EqualFold(row.SenderAddress, candidate.Record.Identity.Operator) ||
		!V3SignedBeforeSet(row, candidate.Record.Identity.CreateDeadline) ||
		VerifyV3PersistedAppliedAttempt(row, candidate.Binding) != nil ||
		strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) != candidate.Record.Identity.CreateCallHash {
		return empty, invalid
	}
	check = "chain"
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || chainID.Cmp(new(big.Int).SetUint64(candidate.Binding.ChainID)) != 0 {
		return empty, invalid
	}
	check = "confirmations"
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, candidate.CursorBlock, 15) ||
		!idov3whitelist.HasConfirmations(head, candidate.CreationBlock, 15) {
		return empty, invalid
	}
	check = "creation_header"
	block := new(big.Int).SetUint64(candidate.CreationBlock)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 ||
		header.Hash() != candidate.CreationHash {
		return empty, invalid
	}
	check = "creation_receipt"
	receipt, err := rpc.TransactionReceipt(ctx, candidate.CreationTx)
	if err != nil || receipt == nil || receipt.TxHash != candidate.CreationTx ||
		receipt.BlockNumber == nil || receipt.BlockNumber.Cmp(block) != 0 ||
		receipt.BlockHash != candidate.CreationHash || receipt.Status != types.ReceiptStatusSuccessful {
		return empty, invalid
	}
	// Snapshot the complete receipt log set before further provider calls. A
	// later answer with the same created log but changed policy/purchase logs
	// is not the same canonical receipt, even if a reused RPC slice mutates.
	check = "creation_logs"
	initialLogs, err := json.Marshal(receipt.Logs)
	if err != nil || len(initialLogs) == 0 || len(initialLogs) > 1<<20 {
		return empty, invalid
	}
	initialLogsHash := sha256.Sum256(initialLogs)
	check = "creation_event"
	var created *basechain.V3CreatedEvent
	for _, log := range receipt.Logs {
		if log == nil || log.Address != candidate.Binding.ProxyAddress || log.Removed ||
			len(log.Topics) == 0 || log.Topics[0] != crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")) {
			continue
		}
		item, decodeErr := basechain.DecodeV3CreatedLog(*log, candidate.Binding.ChainID, candidate.Binding.ProxyAddress)
		if decodeErr != nil || item.ProjectID != projectID || created != nil {
			return empty, invalid
		}
		created = item
	}
	var journal basechain.V3ReplayEvent
	if json.Unmarshal([]byte(candidate.CreationPayload), &journal) != nil || journal.Created == nil ||
		journal.Purchase != nil || journal.Lifecycle != nil || created == nil ||
		*journal.Created != *created || created.LogIndex != candidate.CreationLog ||
		basechain.MatchV3CreatedReceipt(created, candidate.Binding, receipt) != nil ||
		created.ProjectOwner != common.HexToAddress(candidate.Record.Identity.ProjectOwner) ||
		created.FeeReceiver != common.HexToAddress(candidate.Record.Identity.FeeReceiver) ||
		created.ProjectDecimals != candidate.Record.Identity.TokenDecimals ||
		created.PaymentDecimals != candidate.Record.Identity.PaymentDecimals ||
		created.FeeRate != candidate.Record.Identity.FeeRate ||
		created.StartTimeRaw != new(big.Int).SetInt64(candidate.Record.Identity.CreateDeadline).String() {
		return empty, invalid
	}
	// No topic-only admission: match the pinned project's full immutable
	// economics against getProject at the creation block.
	scope := idov3whitelist.Scope{ChainID: chainID, ProjectID: new(big.Int).SetUint64(projectID),
		ProxyAddress: candidate.Binding.ProxyAddress}
	check = "creation_project"
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, candidate.Runtime,
		candidate.CreationBlock, candidate.CreationHash)
	if err != nil || basechain.MatchV3CreationProjectAtBlock(created, candidate.Binding, observed) != nil {
		return empty, invalid
	}
	check = "creation_runtime"
	if idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, candidate.Record, ownerCodeHash,
		governorCodeHash, candidate.CreationBlock, candidate.CreationHash) != nil {
		return empty, invalid
	}
	check = "cursor_header"
	cursorBlock := new(big.Int).SetUint64(candidate.CursorBlock)
	cursor, err := rpc.HeaderByNumber(ctx, cursorBlock)
	if err != nil || cursor == nil || cursor.Number == nil || cursor.Number.Cmp(cursorBlock) != 0 ||
		cursor.Hash() != candidate.CursorHash {
		return empty, invalid
	}
	// The creation block was canonical, but an upgrade or altered project
	// identity at the current confirmed head must stop lifecycle, scanner and
	// proof consumers. A policy root may change independently; immutable
	// economics and the reviewed deployment may not.
	check = "confirmed_runtime"
	confirmed := new(big.Int).SetUint64(head - 14)
	currentHeader, err := rpc.HeaderByNumber(ctx, confirmed)
	if err != nil || currentHeader == nil || currentHeader.Number == nil ||
		currentHeader.Number.Cmp(confirmed) != 0 ||
		idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, candidate.Record,
			ownerCodeHash, governorCodeHash, confirmed.Uint64(), currentHeader.Hash()) != nil {
		return empty, invalid
	}
	check = "confirmed_project"
	current, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope,
		candidate.Runtime, confirmed.Uint64(), currentHeader.Hash())
	if err != nil || basechain.MatchV3CreationProjectImmutable(created, candidate.Binding, current) != nil {
		return empty, invalid
	}
	// Detect an RPC reorg or DB state change across the multi-round-trip audit.
	check = "repeat_receipt"
	again, err := rpc.TransactionReceipt(ctx, candidate.CreationTx)
	var repeatedLogsHash [sha256.Size]byte
	if err == nil && again != nil {
		repeatedLogs, marshalErr := json.Marshal(again.Logs)
		if marshalErr != nil || len(repeatedLogs) == 0 || len(repeatedLogs) > 1<<20 {
			return empty, invalid
		}
		repeatedLogsHash = sha256.Sum256(repeatedLogs)
	}
	if err != nil || again == nil || again.TxHash != receipt.TxHash ||
		again.BlockHash != receipt.BlockHash || again.Status != receipt.Status ||
		again.TransactionIndex != receipt.TransactionIndex ||
		again.BlockNumber == nil || again.BlockNumber.Cmp(block) != 0 ||
		repeatedLogsHash != initialLogsHash ||
		basechain.MatchV3CreatedReceipt(created, candidate.Binding, again) != nil {
		return empty, invalid
	}
	check = "final_confirmations"
	finalHead, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(finalHead, candidate.CursorBlock, 15) ||
		!idov3whitelist.HasConfirmations(finalHead, candidate.CreationBlock, 15) {
		return empty, invalid
	}
	check = "final_headers"
	stable, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || stable == nil || stable.Hash() != candidate.CreationHash {
		return empty, invalid
	}
	currentAgain, err := rpc.HeaderByNumber(ctx, confirmed)
	if err != nil || currentAgain == nil || currentAgain.Number == nil ||
		currentAgain.Number.Cmp(confirmed) != 0 || currentAgain.Hash() != currentHeader.Hash() {
		return empty, invalid
	}
	cursorAgain, err := rpc.HeaderByNumber(ctx, cursorBlock)
	if err != nil || cursorAgain == nil || cursorAgain.Number == nil ||
		cursorAgain.Number.Cmp(cursorBlock) != 0 || cursorAgain.Hash() != candidate.CursorHash {
		return empty, invalid
	}
	check = "candidate_recheck"
	fresh, err := reader(ctx, database, projectID)
	if err != nil || !sameApprovedV3PreviewEvidence(candidate, fresh) ||
		!approvedV3PreviewCursorProgress(candidate, fresh, true) {
		return empty, invalid
	}
	if !approvedV3DisplayCursorProgress(candidate, fresh, halted) {
		return empty, invalid
	}
	if fresh.CursorBlock != candidate.CursorBlock {
		// The new tip must itself have the canonical hash recorded by its
		// checkpoint. The old tip and original receipt were checked above;
		// no backwards movement, halt/resume or revision-only rewrite passes.
		latestHead, e := rpc.BlockNumber(ctx)
		if e != nil || !idov3whitelist.HasConfirmations(latestHead, fresh.CursorBlock, 15) {
			return empty, invalid
		}
		newBlock := new(big.Int).SetUint64(fresh.CursorBlock)
		newHeader, e := rpc.HeaderByNumber(ctx, newBlock)
		if e != nil || newHeader == nil || newHeader.Number == nil || newHeader.Number.Cmp(newBlock) != 0 ||
			newHeader.Hash() != fresh.CursorHash {
			return empty, invalid
		}
	}
	if snapshot != nil {
		*snapshot = fresh
	}
	return idov3whitelist.AdmittedProject{Binding: candidate.Binding, Runtime: candidate.Runtime,
		StartBlock: 128805696, CreationBlock: candidate.CreationBlock, Confirmations: 15}, nil
}

func approvedV3DisplayCursorProgress(before, after idov3whitelist.ApprovedLockedCandidate, halted bool) bool {
	if halted {
		return before.CursorBlock == after.CursorBlock && before.CursorHash == after.CursorHash && before.CursorRevision == after.CursorRevision
	}
	return approvedV3PreviewCursorProgress(before, after, true)
}

func approvedV3PreviewCursorProgress(before, after idov3whitelist.ApprovedLockedCandidate, _ bool) bool {
	if after.CursorBlock == before.CursorBlock {
		return after.CursorHash == before.CursorHash && after.CursorRevision == before.CursorRevision
	}
	return after.CursorBlock > before.CursorBlock &&
		after.CursorRevision > before.CursorRevision &&
		after.CursorRevision-before.CursorRevision == after.CursorBlock-before.CursorBlock &&
		after.CursorHash != (common.Hash{})
}

func sameApprovedV3PreviewEvidence(before, after idov3whitelist.ApprovedLockedCandidate) bool {
	return after.Record.Identity == before.Record.Identity && bytes.Equal(after.Record.Request, before.Record.Request) &&
		after.Binding == before.Binding && after.Runtime == before.Runtime &&
		after.SubmissionID == before.SubmissionID && after.CreationBlock == before.CreationBlock &&
		after.CreationHash == before.CreationHash && after.CreationTx == before.CreationTx &&
		after.CreationLog == before.CreationLog && after.CreationPayload == before.CreationPayload
}

// RecheckApprovedLockedPreview is a cheap final fence for the first-enable
// candidate. The original transaction, receipt, runtime and old cursor were
// fully audited by AdmitApprovedLockedProjectForPreview; this only checks that
// the immutable candidate is still present and its current canonical cursor
// has advanced without a halt/resume or rewrite. Never use for review/proof.
func RecheckApprovedLockedPreview(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID uint64, before idov3whitelist.ApprovedLockedCandidate) error {
	invalid := errors.New("V3 preview admission changed")
	if ctx == nil || database == nil || rpc == nil || projectID == 0 || before.SubmissionID == 0 {
		return invalid
	}
	after, err := idov3whitelist.ReadApprovedLockedCandidate(ctx, database, projectID)
	if err != nil || !sameApprovedV3PreviewEvidence(before, after) ||
		!approvedV3PreviewCursorProgress(before, after, true) {
		return invalid
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, after.CursorBlock, 15) {
		return invalid
	}
	height := new(big.Int).SetUint64(after.CursorBlock)
	header, err := rpc.HeaderByNumber(ctx, height)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(height) != 0 ||
		header.Hash() != after.CursorHash {
		return invalid
	}
	return nil
}
