package chainevent

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

// v3BootstrapCreation is an UNREGISTERED bootstrap prototype. The exact
// project, runtime, creation block and confirmation policy must first be
// admitted by an independently approved source; unverified runtime rows,
// candidate snapshots and a caller-supplied receipt cannot provide that.
// This function has no Core route, signer, Safe or Job scheduler.
func v3BootstrapCreation(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	creationBlock, minConfirmations uint64) error {
	return v3BootstrapCreationWithLock(ctx, db, rpc, binding, runtime, creationBlock, minConfirmations, nil)
}

// lockCreated is invoked in the same transaction as the first journal and
// cursor insert. Only the registered, separately admitted creation bridge may
// supply it; a bare prototype bootstrap does not mutate the Core project.
func v3BootstrapCreationWithLock(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	creationBlock, minConfirmations uint64, lockCreated func(*gorm.DB, *commonchain.V3CreatedEvent) error) error {
	if ctx == nil || db == nil || rpc == nil || creationBlock < 2 || minConfirmations == 0 ||
		binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) || binding.PaymentToken == binding.ProjectToken {
		return errors.New("V3 creation bootstrap scope invalid")
	}
	var complete []types.Log
	parent, hash, logs, err := v3ScanBlock(ctx, rpc, binding, runtime, creationBlock, minConfirmations, &complete)
	if err != nil {
		return err
	}
	var creation *commonchain.V3CreatedEvent
	for _, log := range logs {
		event, decodeErr := commonchain.DecodeV3ReplayLog(log, binding)
		if decodeErr != nil {
			return errors.New("V3 creation bootstrap event invalid")
		}
		if event.Created != nil {
			if creation != nil {
				return errors.New("V3 creation bootstrap ambiguous")
			}
			creation = event.Created
		}
	}
	if creation == nil {
		return errors.New("V3 creation bootstrap missing event")
	}
	previous := new(big.Int).SetUint64(creationBlock - 1)
	parentHeader, err := rpc.HeaderByNumber(ctx, previous)
	if err != nil || parentHeader == nil || parentHeader.Number == nil || parentHeader.Number.Cmp(previous) != 0 || parentHeader.Hash() != parent {
		return errors.New("V3 creation bootstrap parent unavailable")
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(binding.ChainID), ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, creationBlock, hash)
	if err != nil || commonchain.MatchV3CreationProjectAtBlock(creation, binding, observed) != nil {
		return errors.New("V3 creation bootstrap economics mismatched")
	}
	if err := v3RecheckCompleteFilter(ctx, rpc, binding, creationBlock, complete); err != nil {
		return err
	}
	// The initial scanner checked all selected receipts twice, but the later
	// project read is another provider round trip. Recheck EVERY event (not
	// just ProjectCreated): same-block deposits and purchases affect the
	// projection seeded by applyChecked as well.
	for _, log := range logs {
		event, decodeErr := commonchain.DecodeV3ReplayLog(log, binding)
		if decodeErr != nil {
			return errors.New("V3 creation bootstrap event changed")
		}
		receipt, readErr := rpc.TransactionReceipt(ctx, log.TxHash)
		if readErr != nil {
			return errors.New("V3 creation bootstrap receipt changed")
		}
		var matchErr error
		switch {
		case event.Created != nil:
			matchErr = commonchain.MatchV3CreatedReceipt(event.Created, binding, receipt)
		case event.Purchase != nil:
			matchErr = commonchain.MatchV3PurchaseReceipt(event.Purchase, commonchain.V3PurchaseBinding{
				ChainID: binding.ChainID, ProxyAddress: binding.ProxyAddress, ProjectID: binding.ProjectID,
				PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
			}, receipt)
		case event.Lifecycle != nil:
			matchErr = commonchain.MatchV3LifecycleReceipt(event.Lifecycle, binding, receipt)
		default:
			return errors.New("V3 creation bootstrap event unsupported")
		}
		if matchErr != nil {
			return errors.New("V3 creation bootstrap receipt changed")
		}
	}
	// Recheck both anchors after the final historical read. Ongoing canonicality
	// after commit remains the responsibility of the registered scanner Job.
	for _, anchor := range []struct {
		number uint64
		digest common.Hash
	}{{creationBlock - 1, parent}, {creationBlock, hash}} {
		header, readErr := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(anchor.number))
		if readErr != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != anchor.number || header.Hash() != anchor.digest {
			return errors.New("V3 creation bootstrap anchor changed")
		}
	}
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(latest, creationBlock, minConfirmations) {
		return errors.New("V3 creation bootstrap confirmation changed")
	}
	// The final head request can race with replacement of the creation block.
	// Do not insert the initial cursor/checkpoint based only on headers read
	// before that request. This is a point-in-time fence, not finality: the
	// future registered monitor must keep checking after commit.
	for _, anchor := range []struct {
		number uint64
		digest common.Hash
	}{{creationBlock - 1, parent}, {creationBlock, hash}} {
		header, readErr := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(anchor.number))
		if readErr != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != anchor.number || header.Hash() != anchor.digest {
			return errors.New("V3 creation bootstrap final anchor changed")
		}
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if lockCreated != nil {
			if err := lockCreated(tx, creation); err != nil {
				return errors.New("V3 creation project lock unavailable")
			}
		}
		proxy := v3Lower(binding.ProxyAddress)
		if err := tx.Exec(`INSERT INTO ido_v3_scan_cursor (chain_id,proxy_address,project_id,contract_version,cursor_block,cursor_hash,state)
			VALUES (?,?,?,'ido_v3_stablecoin',?,?,'running')`, binding.ChainID, proxy, binding.ProjectID, creationBlock-1, v3LowerHashHeader(parentHeader)).Error; err != nil {
			return errors.New("V3 creation bootstrap cursor unavailable")
		}
		if err := tx.Exec(`INSERT INTO ido_v3_block_checkpoint (chain_id,proxy_address,project_id,contract_version,block_number,block_hash,status)
			VALUES (?,?,?,'ido_v3_stablecoin',?,?,'canonical')`, binding.ChainID, proxy, binding.ProjectID, creationBlock-1, v3LowerHashHeader(parentHeader)).Error; err != nil {
			return errors.New("V3 creation bootstrap checkpoint unavailable")
		}
		return (v3Journal{db: tx, requireCreateAttempt: lockCreated != nil, policyRPC: rpc, policyRuntime: runtime}).applyChecked(ctx, binding, creationBlock, parent, hash, logs, observed)
	}); err != nil {
		return err
	}
	// The initial projection is committed before the caller can resume a
	// monitor. Check the newly persisted cursor, including its checkpoint,
	// immediately after commit. A concrete different canonical header halts
	// and revokes the projection. An unavailable RPC is unknown: the caller
	// must not treat this bootstrap as successful or publish its result.
	// This cannot replace a continuously running monitor after registration.
	if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
		if haltErr := (v3Journal{db: db}).haltBootstrapUnknown(ctx, binding, creationBlock, hash); haltErr != nil {
			return errors.New("V3 creation bootstrap post-commit revocation unavailable")
		}
		return errors.New("V3 creation bootstrap post-commit canonicality unavailable")
	}
	confirmedHead, headErr := rpc.BlockNumber(ctx)
	if headErr != nil || !idov3whitelist.HasConfirmations(confirmedHead, creationBlock, minConfirmations) {
		// Matching hashes do not restore a lost confirmation depth. Keep the
		// creation journal canonical, but stop exposing its projection until
		// independent recovery establishes a new stable head.
		if haltErr := (v3Journal{db: db}).haltBootstrapUnknown(ctx, binding, creationBlock, hash); haltErr != nil {
			return errors.New("V3 creation bootstrap post-commit depth revocation unavailable")
		}
		return errors.New("V3 creation bootstrap post-commit confirmations unavailable")
	}
	// The creation header alone is not a complete canonical anchor: a
	// disagreeing parent response after commit also invalidates publication.
	// In particular, do not accept an inconsistent RPC merely because its
	// creation header still carries the old ParentHash.
	confirmedParent, parentErr := rpc.HeaderByNumber(ctx, previous)
	if parentErr != nil || confirmedParent == nil || confirmedParent.Number == nil || confirmedParent.Number.Cmp(previous) != 0 || confirmedParent.Hash() != parent {
		if haltErr := (v3Journal{db: db}).haltBootstrapUnknown(ctx, binding, creationBlock, hash); haltErr != nil {
			return errors.New("V3 creation bootstrap post-commit parent revocation unavailable")
		}
		return errors.New("V3 creation bootstrap post-commit parent unavailable")
	}
	return nil
}
