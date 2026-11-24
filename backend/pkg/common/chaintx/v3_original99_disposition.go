package chaintx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

const Original99Hash = "0x5bb02ae9cc1e61cf0f352c83ad536b776622b0ca8cdd9002974f18ef92d3158b"
const Original99Project uint64 = 43880023173627905
const original99Sender = "0xfd712061ee08e070a930ecd45e935ae5d7f5445c"
const original99Authorization = "operator-authorized-original99-disposition-20261031"
const Original99DispositionGas uint64 = 21000
const Original99DispositionGasPrice int64 = 100000000 // 0.1 gwei; max cost0.0000021 BNB

type Original99RPC interface {
	V3FirstBroadcastRPC
	V3ReceiptSource
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
	EstimateGas(context.Context, ethereum.CallMsg) (uint64, error)
}
type Original99Disposition struct {
	OriginalID                      uint64
	OriginalHash, OriginalRawSHA256 string
	ChainID                         uint64
	SenderAddress                   string
	Nonce                           uint64
	TargetAddress                   string
	GasLimit                        uint64
	GasPriceRaw, ValueRaw           string
	RawTx                           []byte `json:"-"`
	TxHash, State                   string
	BroadcastCount                  uint
	BroadcastMarkedAt               *time.Time
	ReceiptStatus, ReceiptBlock     *uint64
	ReceiptHash                     *string
	ConfirmedBlock                  *uint64
	ConfirmedHash                   *string
	StreamEpoch                     *uint64
	AuthorizationRef                string
	CreatedAt, UpdatedAt            time.Time
}

func (Original99Disposition) TableName() string { return "ido_v3_original99_disposition" }

type Original99Report struct {
	OriginalID      uint64 `json:"original_id"`
	OriginalHash    string `json:"original_hash"`
	DispositionHash string `json:"disposition_hash,omitempty"`
	State           string `json:"state"`
	Nonce           uint64 `json:"nonce"`
	Target          string `json:"target"`
	GasLimit        uint64 `json:"gas_limit"`
	GasPriceRaw     string `json:"gas_price_raw"`
	MaxCostRaw      string `json:"max_cost_wei"`
	Value           string `json:"value"`
}

func original99Report(d Original99Disposition) Original99Report {
	return Original99Report{99, Original99Hash, d.TxHash, d.State, 81, original99Sender, Original99DispositionGas, big.NewInt(Original99DispositionGasPrice).String(), new(big.Int).Mul(big.NewInt(Original99DispositionGasPrice), new(big.Int).SetUint64(Original99DispositionGas)).String(), "0"}
}

var errOriginal99 = errors.New("original99 disposition refused; reconcile original and disposition hashes; never resend")
var errOriginal99Dry = errors.New("original99 dryrun rollback")

// Safe readback after ambiguous HTTP/COMMIT outcome; no raw bytes exposed.
func ReadOriginal99Disposition(ctx context.Context, db *gorm.DB) (Original99Report, error) {
	if ctx == nil || db == nil {
		return Original99Report{}, errOriginal99
	}
	var d Original99Disposition
	e := db.WithContext(ctx).Where("original_id=99").Take(&d).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		out := original99Report(d)
		out.State = "not_prepared"
		return out, nil
	}
	if e != nil || verifyOriginal99Disposition(d) != nil {
		return Original99Report{}, errOriginal99
	}
	return original99Report(d), nil
}

// No key, signing or send capability. Verify approval + persisted original
// identity then two independent read-only nodes. Caller supplies pinned RPCs.
func inspectOriginal99(ctx context.Context, db *gorm.DB, providers []Original99RPC) (entity.ChainTxSubmission, idov3whitelist.ApprovedSetRecord, error) {
	var row entity.ChainTxSubmission
	if ctx == nil || ctx.Err() != nil || db == nil || len(providers) != 2 {
		return row, idov3whitelist.ApprovedSetRecord{}, errOriginal99
	}
	var identity struct{ UUID, DB string }
	if db.WithContext(ctx).Raw(`SELECT @@server_uuid AS uuid,DATABASE() AS db`).Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" {
		return row, idov3whitelist.ApprovedSetRecord{}, errOriginal99
	}
	if db.WithContext(ctx).Where("id=99").Take(&row).Error != nil {
		return row, idov3whitelist.ApprovedSetRecord{}, errOriginal99
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, db, Original99Project)
	if err != nil {
		return row, record, errOriginal99
	}
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: Original99Project, ProxyAddress: common.HexToAddress(record.Identity.Proxy), ProjectToken: common.HexToAddress(record.Identity.ProjectToken), PaymentToken: common.HexToAddress(record.Identity.PaymentToken)}
	if approvedV3OriginalAttempt(row, b, record.Identity) != nil || row.ID != 99 || row.ProjectID != Original99Project || row.ChainID != 56 || row.Action != "ido_v3_create" || row.State != "signed" || row.TxHash == nil || *row.TxHash != Original99Hash || row.Nonce == nil || *row.Nonce != 81 || row.BroadcastCount != 0 || row.LastBroadcastAt != nil || row.ReceiptStatus != nil || row.ReceiptBlockNumber != nil || row.FinalEventRecordID != nil || row.ActiveSlot == nil || *row.ActiveSlot != 1 || !strings.EqualFold(row.SenderAddress, original99Sender) || time.Now().Unix() <= record.Identity.CreateDeadline {
		return row, record, errOriginal99
	}
	for _, rpc := range providers {
		if err := checkOriginal99Node(ctx, row, record.Identity.CreateDeadline, rpc); err != nil {
			return row, record, err
		}
	}
	return row, record, nil
}
func checkOriginal99Node(ctx context.Context, row entity.ChainTxSubmission, deadline int64, rpc Original99RPC) error {
	if rpc == nil || row.TxHash == nil {
		return errOriginal99
	}
	id, e := rpc.ChainID(ctx)
	if e != nil || id == nil || id.Cmp(big.NewInt(56)) != 0 {
		return errOriginal99
	}
	sender := common.HexToAddress(original99Sender)
	latest, e := rpc.NonceAt(ctx, sender, nil)
	if e != nil || latest != 81 {
		return errOriginal99
	}
	pending, e := rpc.PendingNonceAt(ctx, sender)
	if e != nil || pending != 81 {
		return errOriginal99
	}
	if r, e := rpc.TransactionReceipt(ctx, common.HexToHash(Original99Hash)); !errors.Is(e, ethereum.NotFound) || r != nil {
		return errOriginal99
	}
	if tx, _, e := rpc.TransactionByHash(ctx, common.HexToHash(Original99Hash)); !errors.Is(e, ethereum.NotFound) || tx != nil {
		return errOriginal99
	}
	head, e := rpc.BlockNumber(ctx)
	if e != nil || head < 15 {
		return errOriginal99
	}
	header, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(head-14))
	if e != nil || header == nil || header.Number == nil || header.Number.Uint64() != head-14 || header.Time < uint64(deadline) {
		return errOriginal99
	}
	code, e := rpc.CodeAt(ctx, sender, nil)
	if e != nil || len(code) != 0 {
		return errOriginal99
	}
	cost := new(big.Int).Mul(big.NewInt(Original99DispositionGasPrice), new(big.Int).SetUint64(Original99DispositionGas))
	balance, e := rpc.BalanceAt(ctx, sender, nil)
	if e != nil || balance == nil || balance.Cmp(cost) < 0 {
		return errOriginal99
	}
	msg := ethereum.CallMsg{From: sender, To: &sender, Gas: Original99DispositionGas, GasPrice: big.NewInt(Original99DispositionGasPrice), Value: big.NewInt(0)}
	out, e := rpc.CallContract(ctx, msg, nil)
	if e != nil || len(out) != 0 {
		return errOriginal99
	}
	gas, e := rpc.EstimateGas(ctx, msg)
	if e != nil || gas != Original99DispositionGas {
		return errOriginal99
	}
	return nil
}

// Once-only signer inside durable application transaction. It signs ONLY this
// exact identity and fixed fee cap. A committed row forbids signing again.
// dry-run cannot invoke signer. signer is the existing application wallet,
// never a key supplied on CLI or an exported private key.
func PrepareOriginal99Disposition(ctx context.Context, db *gorm.DB, providers []Original99RPC, signer interface {
	SignLegacyTransaction(uint64, uint64, *big.Int, common.Address, []byte) ([]byte, string, error)
}, execute bool, guard func(*gorm.DB) error) (Original99Report, error) {
	if guard == nil {
		return Original99Report{}, errOriginal99
	}
	row, record, err := inspectOriginal99(ctx, db, providers)
	if err != nil {
		return Original99Report{}, err
	}
	sum := sha256.Sum256(row.RawTx)
	d := Original99Disposition{OriginalID: 99, OriginalHash: Original99Hash, OriginalRawSHA256: hex.EncodeToString(sum[:]), ChainID: 56, SenderAddress: original99Sender, Nonce: 81, TargetAddress: original99Sender, GasLimit: Original99DispositionGas, GasPriceRaw: big.NewInt(Original99DispositionGasPrice).String(), ValueRaw: "0", State: "signed", AuthorizationRef: original99Authorization}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if lockOriginal99(ctx, tx, row, record) != nil {
			return errOriginal99
		}
		if guard(tx) != nil {
			return errOriginal99
		}
		var existing int64
		if tx.Model(&Original99Disposition{}).Where("original_id=99").Count(&existing).Error != nil || existing != 0 {
			return errOriginal99
		}
		if !execute {
			return errOriginal99Dry
		}
		if signer == nil {
			return errOriginal99
		}
		raw, hash, e := signer.SignLegacyTransaction(81, Original99DispositionGas, big.NewInt(Original99DispositionGasPrice), common.HexToAddress(original99Sender), nil)
		if e != nil {
			return errOriginal99
		}
		d.RawTx = raw
		d.TxHash = hash
		if verifyOriginal99Disposition(d) != nil {
			return errOriginal99
		}
		d.CreatedAt = time.Now().UTC()
		d.UpdatedAt = d.CreatedAt
		if tx.Create(&d).Error != nil {
			return errOriginal99
		}
		return nil
	})
	if !execute && errors.Is(err, errOriginal99Dry) {
		return original99Report(d), nil
	}
	if err != nil {
		return Original99Report{}, errOriginal99
	}
	return original99Report(d), nil
}
func verifyOriginal99Disposition(d Original99Disposition) error {
	if d.OriginalID != 99 || d.OriginalHash != Original99Hash || d.ChainID != 56 || d.Nonce != 81 || d.SenderAddress != original99Sender || d.TargetAddress != original99Sender || d.GasLimit != Original99DispositionGas || d.GasPriceRaw != big.NewInt(Original99DispositionGasPrice).String() || d.ValueRaw != "0" || d.AuthorizationRef != original99Authorization {
		return errOriginal99
	}
	var signed types.Transaction
	if signed.UnmarshalBinary(d.RawTx) != nil || signed.Type() != types.LegacyTxType || signed.ChainId().Cmp(big.NewInt(56)) != 0 || signed.Nonce() != 81 || signed.To() == nil || *signed.To() != common.HexToAddress(original99Sender) || signed.Gas() != Original99DispositionGas || signed.GasPrice().Cmp(big.NewInt(Original99DispositionGasPrice)) != 0 || signed.Value().Sign() != 0 || len(signed.Data()) != 0 || signed.Hash().Hex() != d.TxHash {
		return errOriginal99
	}
	sender, e := types.Sender(types.NewEIP155Signer(big.NewInt(56)), &signed)
	if e != nil || sender != common.HexToAddress(original99Sender) {
		return errOriginal99
	}
	return nil
}
func lockOriginal99(ctx context.Context, tx *gorm.DB, expected entity.ChainTxSubmission, record idov3whitelist.ApprovedSetRecord) error {
	if chainreorg.LockNoActiveIncidentTx(tx, 56, record.Identity.Proxy) != nil {
		return errOriginal99
	}
	var project struct {
		Status string
		Locked bool
	}
	if tx.Raw(`SELECT status,onchain_locked AS locked FROM projects WHERE project_id=? FOR UPDATE`, Original99Project).Scan(&project).Error != nil || project.Status != "Set" || project.Locked {
		return errOriginal99
	}
	fresh, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, Original99Project)
	if e != nil || fresh.Identity != record.Identity {
		return errOriginal99
	}
	var row entity.ChainTxSubmission
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=99").Take(&row).Error != nil || !bytes.Equal(row.RawTx, expected.RawTx) || row.TxHash == nil || *row.TxHash != Original99Hash || row.State != "signed" || row.BroadcastCount != 0 || row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.Nonce == nil || *row.Nonce != 81 {
		return errOriginal99
	}
	var fence entity.ChainRelayerNonce
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=56 AND sender_address=?", expected.SenderAddress).Take(&fence).Error != nil || !activeRetirementFence(row, fence) {
		return errOriginal99
	}
	return nil
}

// Marker commits before at-most-one send. Reinvocation can never resend an
// already marked child. Unknown COMMIT/send outcomes reconcile both hashes.
func BroadcastOriginal99Disposition(ctx context.Context, db *gorm.DB, providers []Original99RPC, sender interface {
	SendTransaction(context.Context, *types.Transaction) error
}, expectedHash string, guard func(*gorm.DB) error) (Original99Report, error) {
	if guard == nil {
		return Original99Report{}, errOriginal99
	}
	row, record, err := inspectOriginal99(ctx, db, providers)
	if err != nil || sender == nil {
		return Original99Report{}, errOriginal99
	}
	var d Original99Disposition
	if db.WithContext(ctx).Where("original_id=99").Take(&d).Error != nil || verifyOriginal99Disposition(d) != nil || d.TxHash != expectedHash || d.State != "signed" || d.BroadcastCount != 0 {
		return Original99Report{}, errOriginal99
	}
	// Neither child can have appeared between preparation and marker.
	for _, rpc := range providers {
		if r, e := rpc.TransactionReceipt(ctx, common.HexToHash(d.TxHash)); !errors.Is(e, ethereum.NotFound) || r != nil {
			return original99Report(d), errOriginal99
		}
		if t, _, e := rpc.TransactionByHash(ctx, common.HexToHash(d.TxHash)); !errors.Is(e, ethereum.NotFound) || t != nil {
			return original99Report(d), errOriginal99
		}
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if lockOriginal99(ctx, tx, row, record) != nil {
			return errOriginal99
		}
		if guard(tx) != nil {
			return errOriginal99
		}
		r := tx.Model(&Original99Disposition{}).Where("original_id=99 AND state='signed' AND broadcast_count=0 AND tx_hash=?", d.TxHash).Updates(map[string]any{"state": "broadcast_unknown", "broadcast_count": 1, "broadcast_marked_at": time.Now().UTC(), "updated_at": time.Now().UTC()})
		if r.Error != nil || r.RowsAffected != 1 {
			return errOriginal99
		}
		return nil
	})
	if err != nil {
		return original99Report(d), errOriginal99
	}
	d.State = "broadcast_unknown"
	report := original99Report(d)
	var signed types.Transaction
	if signed.UnmarshalBinary(d.RawTx) != nil {
		return report, errOriginal99
	}
	if sender.SendTransaction(ctx, &signed) != nil {
		return report, errOriginal99
	}
	return report, nil
}

// Dual-node confirmed receipt agreement plus producer epoch/coverage anchor.
// A self-transfer emits no IDO event: record its own receipt, NOT a fake
// project create/revert. The original receipt fields remain untouched.
func ReconcileOriginal99Disposition(ctx context.Context, db *gorm.DB, providers []Original99RPC, guard func(*gorm.DB) error) (Original99Report, error) {
	if guard == nil {
		return Original99Report{}, errOriginal99
	}
	if ctx == nil || db == nil || len(providers) != 2 {
		return Original99Report{}, errOriginal99
	}
	var d Original99Disposition
	if db.WithContext(ctx).Where("original_id=99").Take(&d).Error != nil || verifyOriginal99Disposition(d) != nil || d.State != "broadcast_unknown" || d.BroadcastCount != 1 {
		return Original99Report{}, errOriginal99
	}
	var audits []original99ReceiptAudit
	for _, rpc := range providers {
		a, e := auditOriginal99DispositionReceipt(ctx, d, rpc)
		if e != nil {
			return original99Report(d), e
		}
		audits = append(audits, a)
	}
	if audits[0].Block != audits[1].Block || audits[0].BlockHash != audits[1].BlockHash || audits[0].Hash != audits[1].Hash || audits[0].ConfirmedHash != audits[1].ConfirmedHash {
		return original99Report(d), errOriginal99
	}
	a := audits[0]
	record, e := idov3whitelist.ReadApprovedSetRecord(ctx, db, Original99Project)
	if e != nil {
		return original99Report(d), errOriginal99
	}
	var original entity.ChainTxSubmission
	if db.WithContext(ctx).Where("id=99").Take(&original).Error != nil {
		return original99Report(d), errOriginal99
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, 56, record.Identity.Proxy) != nil {
			return errOriginal99
		}
		var anchor struct {
			Epoch uint64
			Hash  string
		}
		r := tx.Raw(`SELECT s.epoch,c.block_hash AS hash FROM ido_v3_proxy_stream s JOIN ido_v3_proxy_coverage c
   ON c.chain_id=s.chain_id AND c.proxy_address=s.proxy_address AND c.contract_version=s.contract_version AND c.epoch=s.epoch
   WHERE s.chain_id=56 AND s.proxy_address=? AND s.state='running' AND c.block_number=? AND c.block_hash=?
   AND s.cursor_block>=? FOR UPDATE`, record.Identity.Proxy, a.Block, strings.ToLower(a.BlockHash.Hex()), a.Block).Scan(&anchor)
		if r.Error != nil || r.RowsAffected != 1 || anchor.Epoch == 0 {
			return errOriginal99
		}
		if lockOriginal99(ctx, tx, original, record) != nil {
			return errOriginal99
		}
		if guard(tx) != nil {
			return errOriginal99
		}
		sum := sha256.Sum256(original.RawTx)
		if hex.EncodeToString(sum[:]) != d.OriginalRawSHA256 {
			return errOriginal99
		}
		r = tx.Model(&Original99Disposition{}).Where("original_id=99 AND state='broadcast_unknown' AND tx_hash=? AND broadcast_count=1", d.TxHash).Updates(map[string]any{"state": "confirmed", "receipt_status": 1, "receipt_block": a.Block, "receipt_hash": strings.ToLower(a.BlockHash.Hex()), "confirmed_block": a.Block + 14, "confirmed_hash": strings.ToLower(audits[0].ConfirmedHash.Hex()), "stream_epoch": anchor.Epoch, "updated_at": time.Now().UTC()})
		if r.Error != nil || r.RowsAffected != 1 {
			return errOriginal99
		}
		r = tx.Model(&entity.ChainTxSubmission{}).Where("id=99 AND state='signed' AND active_slot=1 AND tx_hash=? AND broadcast_count=0", Original99Hash).Updates(map[string]any{"state": "v3_nonce_disposed", "active_slot": nil, "last_error": "original nonce consumed by linked confirmed disposition; original create not executed", "updated_at": time.Now()})
		if r.Error != nil || r.RowsAffected != 1 {
			return errOriginal99
		}
		r = tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id=56 AND sender_address=? AND active_submission_id=99 AND next_nonce=82", original.SenderAddress).Updates(map[string]any{"active_submission_id": nil, "updated_at": time.Now()})
		if r.Error != nil || r.RowsAffected != 1 {
			return errOriginal99
		}
		return nil
	})
	if err != nil {
		return original99Report(d), errOriginal99
	}
	d.State = "confirmed"
	return original99Report(d), nil
}

type original99ReceiptAudit struct {
	V3ReceiptAudit
	ConfirmedHash common.Hash
}

func auditOriginal99DispositionReceipt(ctx context.Context, d Original99Disposition, rpc Original99RPC) (original99ReceiptAudit, error) {
	empty := original99ReceiptAudit{}
	if rpc == nil {
		return empty, errOriginal99
	}
	chain, e := rpc.ChainID(ctx)
	if e != nil || chain == nil || chain.Cmp(big.NewInt(56)) != 0 {
		return empty, errOriginal99
	}
	hash := common.HexToHash(d.TxHash)
	r, e := rpc.TransactionReceipt(ctx, hash)
	if e != nil || !validV3Receipt(r, hash) || r.Status != 1 || r.GasUsed != 21000 || len(r.Logs) != 0 {
		return empty, errOriginal99
	}
	tx, pending, e := rpc.TransactionByHash(ctx, hash)
	if e != nil || pending || tx == nil {
		return empty, errOriginal99
	}
	raw, e := tx.MarshalBinary()
	if e != nil || !bytes.Equal(raw, d.RawTx) {
		return empty, errOriginal99
	}
	head, e := rpc.BlockNumber(ctx)
	if e != nil || !idov3whitelist.HasConfirmations(head, r.BlockNumber.Uint64(), 15) {
		return empty, errOriginal99
	}
	h, e := rpc.HeaderByNumber(ctx, r.BlockNumber)
	if e != nil || h == nil || h.Number == nil || h.Number.Cmp(r.BlockNumber) != 0 || h.Hash() != r.BlockHash {
		return empty, errOriginal99
	}
	conf, e := rpc.HeaderByNumber(ctx, new(big.Int).Add(r.BlockNumber, big.NewInt(14)))
	if e != nil || conf == nil || conf.Number == nil || conf.Number.Uint64() != r.BlockNumber.Uint64()+14 {
		return empty, errOriginal99
	}
	nonce, e := rpc.NonceAt(ctx, common.HexToAddress(original99Sender), conf.Number)
	if e != nil || nonce != 82 {
		return empty, errOriginal99
	}
	if receipt, e := rpc.TransactionReceipt(ctx, common.HexToHash(Original99Hash)); !errors.Is(e, ethereum.NotFound) || receipt != nil {
		return empty, errOriginal99
	}
	again, e := rpc.TransactionReceipt(ctx, hash)
	if e != nil || !validV3Receipt(again, hash) || again.BlockHash != r.BlockHash || again.Status != 1 || again.BlockNumber.Cmp(r.BlockNumber) != 0 {
		return empty, errOriginal99
	}
	stable, e := rpc.HeaderByNumber(ctx, r.BlockNumber)
	if e != nil || stable == nil || stable.Number == nil || stable.Number.Cmp(r.BlockNumber) != 0 || stable.Hash() != r.BlockHash {
		return empty, errOriginal99
	}
	stableConf, e := rpc.HeaderByNumber(ctx, conf.Number)
	if e != nil || stableConf == nil || stableConf.Number == nil || stableConf.Number.Cmp(conf.Number) != 0 || stableConf.Hash() != conf.Hash() {
		return empty, errOriginal99
	}
	return original99ReceiptAudit{V3ReceiptAudit: V3ReceiptAudit{Hash: hash, Nonce: 81, Block: r.BlockNumber.Uint64(), BlockHash: r.BlockHash, GasUsed: r.GasUsed}, ConfirmedHash: conf.Hash()}, nil
}
