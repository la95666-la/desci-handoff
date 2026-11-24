package chainevent

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/model"
)

// V3ScanAdmission is independent of the unverified runtime table. The key
// must be pinned by a deployment trust root separate from the signed payload.
// No candidate row, Admin form or RPC response can supply the verifier key.
type V3ScanAdmission struct {
	PublicKey ed25519.PublicKey
	Manifest  idov3whitelist.AdmissionManifest
	Signature []byte
}

// ParseV3ScanAdmission decodes only; it never authorizes scanning. PublicKey
// comes from a separately pinned trust root, not from manifestJSON.
func ParseV3ScanAdmission(publicKey ed25519.PublicKey, manifestJSON, signatureBase64 string) (V3ScanAdmission, error) {
	if len(publicKey) != ed25519.PublicKeySize || strings.TrimSpace(manifestJSON) == "" || strings.TrimSpace(signatureBase64) == "" {
		return V3ScanAdmission{}, errors.New("V3 scanner admission configuration unavailable")
	}
	var manifest idov3whitelist.AdmissionManifest
	decoder := json.NewDecoder(strings.NewReader(manifestJSON))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil {
		return V3ScanAdmission{}, errors.New("V3 scanner admission manifest malformed")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return V3ScanAdmission{}, errors.New("V3 scanner admission manifest malformed")
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(signatureBase64)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return V3ScanAdmission{}, errors.New("V3 scanner admission signature malformed")
	}
	return V3ScanAdmission{PublicKey: append(ed25519.PublicKey(nil), publicKey...), Manifest: manifest, Signature: signature}, nil
}

// V3ScannerJob is intentionally a separate source from V1/V2 EventAddresses.
// It can be registered only with a signed project admission, and re-admits
// from the exact persisted project/runtime binding on every tick. It has no
// signer or send capability; the outbox is consulted only for original-hash
// receipt recovery. Run overlap is suppressed for catch-up and cron.
type V3ScannerJob struct {
	Database     *gorm.DB
	RPCURL       string
	Admission    V3ScanAdmission
	SetKey       ed25519.PublicKey
	SetManifest  idov3whitelist.SetManifest
	SetSignature []byte
	// Operator is release-pinned independently of the scanner manifest. It
	// authenticates the durable create attempt during the pre-lock bridge.
	Operator common.Address
	Context  context.Context
	running  chan struct{}
	initOnce sync.Once
}

func (j *V3ScannerJob) tryRun() bool {
	j.initOnce.Do(func() {
		j.running = make(chan struct{}, 1)
	})
	select {
	case j.running <- struct{}{}:
		return true
	default:
		return false
	}
}

func (j *V3ScannerJob) Add(crontab *cron.Cron) {
	crontab.AddJob("25,55 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
}

var _ model.Add = (*V3ScannerJob)(nil)

func (j *V3ScannerJob) Run() {
	if j == nil || !j.tryRun() {
		return
	}
	defer func() { <-j.running }()
	ctx := j.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	// A previously signed create may have reverted while the Set expired.
	// Its original-hash recovery is independent of permission to scan or
	// issue a new create; do it before the live-Set scanner admission.
	if j.reconcilePrelockCreate(ctx) != nil {
		log.GetLogger().Warn("V3 original-hash creation receipt recovery unavailable")
	}
	if err := j.run(ctx); err != nil {
		// The scan context may have timed out. Revocation must still be
		// attempted with an independent bounded DB context before returning.
		haltCtx, haltCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer haltCancel()
		if haltErr := j.haltPublishedProjection(haltCtx); haltErr != nil {
			log.GetLogger().Error("V3 scanner projection revocation unavailable")
		}
		// Never print raw RPC, DB or admission material in the Job log.
		log.GetLogger().Warn("V3 scanner tick unavailable; project projection must not be promoted")
	}
}

// A failed tick cannot silently leave yesterday's wallet projection readable.
// This fence only halts the currently running revision; it never rewrites a
// journal, invents a canonical ancestor, or revokes a concurrent successor.
func (j *V3ScannerJob) haltPublishedProjection(ctx context.Context) error {
	if j == nil || j.Database == nil || j.Admission.Manifest.ChainID == 0 || j.Admission.Manifest.ProjectID == 0 ||
		!common.IsHexAddress(j.Admission.Manifest.ProxyAddress) {
		return errors.New("V3 scanner projection scope unavailable")
	}
	var cursor struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	m := j.Admission.Manifest
	read := j.Database.WithContext(ctx).Raw(`SELECT cursor_block,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`,
		m.ChainID, m.ProxyAddress, m.ProjectID).Scan(&cursor)
	if read.Error != nil {
		return errors.New("V3 scanner cursor unavailable")
	}
	if read.RowsAffected == 0 || cursor.State == "reorg_halted" {
		return nil
	}
	if read.RowsAffected != 1 || cursor.Block == 0 || cursor.Revision == 0 || cursor.State != "running" {
		return errors.New("V3 scanner cursor invalid")
	}
	if !common.IsHexAddress(m.ProjectToken) || !common.IsHexAddress(m.PaymentToken) ||
		common.HexToAddress(m.ProjectToken) == (common.Address{}) || common.HexToAddress(m.PaymentToken) == (common.Address{}) ||
		common.HexToAddress(m.ProjectToken) == common.HexToAddress(m.PaymentToken) {
		return errors.New("V3 scanner halt token identity unavailable")
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: m.ChainID, ProjectID: m.ProjectID,
		ProxyAddress: common.HexToAddress(m.ProxyAddress), ProjectToken: common.HexToAddress(m.ProjectToken),
		PaymentToken: common.HexToAddress(m.PaymentToken)}
	return (v3Journal{db: j.Database}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision)
}

func (j *V3ScannerJob) run(ctx context.Context) error {
	if j == nil || ctx == nil || j.Database == nil || strings.TrimSpace(j.RPCURL) == "" {
		return errors.New("V3 scanner unavailable")
	}
	admitted, err := idov3whitelist.AdmitSignedProject(ctx, j.Database, j.Admission.PublicKey,
		j.Admission.Manifest, j.Admission.Signature, time.Now())
	creationOnly := false
	historical := j.SetManifest.ExpiresAt <= time.Now().Unix()
	// Recheck the release-pinned historical Set even for locked projects:
	// lifecycle receipts require its original operator and governance code
	// identity. It is not a renewal of permission to create a transaction.
	if idov3whitelist.VerifySignedSetManifest(j.SetKey, j.SetManifest, j.SetSignature,
		common.HexToHash(j.SetManifest.RequestDigest), historicalV3SetReviewTime(j.SetManifest.ExpiresAt)) != nil ||
		idov3whitelist.CheckCreationSetCommitment(ctx, j.Database, j.SetManifest, j.Admission.Manifest) != nil ||
		common.HexToAddress(j.SetManifest.Operator) != j.Operator {
		return errors.New("V3 scanner Set identity unavailable")
	}
	if err != nil {
		admitted, err = idov3whitelist.AdmitSignedCreation(ctx, j.Database, j.Admission.PublicKey,
			j.Admission.Manifest, j.Admission.Signature, time.Now())
		if err != nil {
			return errors.New("V3 scanner admission unavailable")
		}
		creationOnly = true
	}
	rpc, err := chainrpc.Dial(ctx, j.RPCURL)
	if err != nil {
		return errors.New("V3 scanner RPC unavailable")
	}
	defer rpc.Close()
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != admitted.Binding.ChainID {
		return errors.New("V3 scanner chain mismatch")
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || head < admitted.Confirmations-1 {
		return errors.New("V3 scanner head unavailable")
	}
	finalized := head - (admitted.Confirmations - 1)
	if finalized < admitted.CreationBlock {
		return errors.New("V3 creation not yet confirmed")
	}
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(finalized))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != finalized {
		return errors.New("V3 scanner canonical header unavailable")
	}
	scope := idov3whitelist.Scope{ChainID: chainID, ProxyAddress: admitted.Binding.ProxyAddress,
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID)}
	if _, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, finalized, header.Hash()); err != nil {
		return errors.New("V3 scanner runtime identity unavailable")
	}
	if err := idov3whitelist.ReadScanGovernorAt(ctx, rpc, scope, admitted.Runtime,
		common.HexToAddress(j.Admission.Manifest.Governor), finalized, header.Hash()); err != nil {
		return errors.New("V3 scanner governance identity unavailable")
	}
	if creationOnly {
		if err := idov3whitelist.ReadScanOperatorAt(ctx, rpc, scope, admitted.Runtime,
			j.Operator, finalized, header.Hash()); err != nil {
			return errors.New("V3 creation operator identity unavailable")
		}
	}
	// A signed manifest and candidate binding alone cannot authorize even
	// cancellation. Recheck the independent chain, runtime and governance at
	// a confirmed block first. The locked storage primitive then checks the
	// exact unsigned row under the reorg/cursor/project locks. No signed row
	// is eligible for cleanup or replay.
	if err := j.cancelStaleUnsignedIntent(ctx); err != nil {
		return errors.New("V3 stale unsigned intent cleanup unavailable")
	}
	var cursors []struct {
		State string `gorm:"column:state"`
	}
	read := j.Database.WithContext(ctx).Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' LIMIT 2`,
		admitted.Binding.ChainID, strings.ToLower(admitted.Binding.ProxyAddress.Hex()), admitted.Binding.ProjectID).Scan(&cursors)
	if read.Error != nil || len(cursors) > 1 {
		return errors.New("V3 scanner cursor unavailable")
	}
	if creationOnly {
		if len(cursors) != 0 {
			return errors.New("V3 pre-lock cursor must be absent")
		}
		if historical {
			// A historical Set cannot authorize creation. Only a separately
			// audited, confirmed success for the exact original attempt may
			// bootstrap the event and journal after the review expires.
			if err := requireV3ConfirmedCreation(ctx, j.Database, admitted.Binding, j.Operator,
				j.SetManifest.ExpiresAt); err != nil {
				return errors.New("V3 historical create receipt unavailable")
			}
		}
		return bridgeV3Creation(ctx, j.Database, rpc, admitted, j.Operator,
			j.Admission.Manifest.ExpiresAt, j.SetManifest.ExpiresAt, historical)
	}
	if len(cursors) == 0 {
		// A locked project without its initial journal is not an authorized
		// bootstrap source. The pre-lock signed bridge commits both atomically.
		return errors.New("V3 locked project missing creation cursor")
	}
	if cursors[0].State != "running" {
		if cursors[0].State != "reorg_halted" {
			return errors.New("V3 scanner cursor state unavailable")
		}
		// Re-admitted runtime and persisted checkpoints bound the only recovery
		// source. The bounded coordinator rechecks the whole surviving prefix.
		if err := recoverV3HaltedPrefix(ctx, j.Database, rpc, admitted); err != nil {
			return errors.New("V3 scanner recovery unavailable")
		}
	}
	_, err = v3ScanConfirmedRange(ctx, j.Database, rpc, admitted.Binding, admitted.Runtime, admitted.Confirmations, 128)
	if err != nil {
		return errors.New("V3 scanner range unavailable")
	}
	// Receipt recovery is strictly read-only on chain and never a reason to
	// revoke an otherwise canonical scanner projection. An absent/unknown
	// receipt leaves the signed sender fence intact for a later tick.
	if j.reconcileSignedReceipt(ctx, rpc, "") != nil {
		log.GetLogger().Warn("V3 original-hash receipt recovery unavailable")
	}
	return nil
}

// Prevent accidental linkage of a V2 proxy into a separately admitted V3
// source when the Job still has its existing V2 source registered.
func (j *V3ScannerJob) DistinctFrom(v1, v2 common.Address) bool {
	proxy := common.HexToAddress(j.Admission.Manifest.ProxyAddress)
	return proxy != (common.Address{}) && proxy != v1 && proxy != v2
}
