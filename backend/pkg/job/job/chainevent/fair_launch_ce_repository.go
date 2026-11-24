package chainevent

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
)

type FairLaunchCEWrite struct {
	Event            *commonchain.FairLaunchTypedEvent
	Context          FairLaunchMaterializationContext
	ProviderIdentity string
	TopicsDataDigest string
	RawLogDigest     string
	TransactionIndex uint32
	SourceID         uint64
	LeaseToken       string
	FenceEpoch       uint64
	LocalArchive     *FairLaunchLocalE2EArchive
}
type FairLaunchCEWriteResult struct {
	IdentityID    uint64
	PlacementID   uint64
	ObservationID uint64
	ApplicationID uint64
	Effect        FairLaunchMaterializationResult
}

const fairLaunchCEV2PersistenceEnabled = constant.FairLaunchMaterializationEnabled

func ApplyFairLaunchCEV2Tx(tx *gorm.DB, in FairLaunchCEWrite, now time.Time) (FairLaunchCEWriteResult, error) {
	if in.Event == nil || in.Context.ChainID == 0 || in.Context.SourceRevisionID == 0 || in.SourceID == 0 || in.LeaseToken == "" || in.FenceEpoch == 0 || now.IsZero() {
		return FairLaunchCEWriteResult{}, errors.New("invalid CE-V2 write")
	}
	if !fairLaunchCEV2PersistenceEnabled {
		return FairLaunchCEWriteResult{}, errors.New("fair_launch_materialization_unavailable")
	}
	if !in.Context.Canonical || !in.Context.Finalized {
		return FairLaunchCEWriteResult{}, errors.New("fair launch canonical finalized placement required")
	}
	cursor := entity.SourceMaterializationCursor{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_id=?", in.SourceID).Take(&cursor).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	if cursor.LeaseToken != in.LeaseToken || cursor.FenceEpoch != in.FenceEpoch {
		return FairLaunchCEWriteResult{}, errors.New("source materialization fence lost")
	}
	txHash := strings.ToLower(in.Event.Log.TxHash.Hex())
	identity := entity.ChainEventIdentityV2{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND tx_hash=? AND tx_log_ordinal=?", in.Context.ChainID, txHash, in.Context.TxLogOrdinal).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		identity = entity.ChainEventIdentityV2{ChainID: in.Context.ChainID, TxHash: txHash, TxLogOrdinal: in.Context.TxLogOrdinal, CreatedAt: now}
		if err = tx.Create(&identity).Error; err != nil {
			return FairLaunchCEWriteResult{}, err
		}
	} else if err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(in.Event)
	if err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	var existingGuard entity.FairLaunchEffectApplication
	guardErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_identity_id=? AND effect_version=?", identity.ID, in.Context.EffectVersion).Take(&existingGuard).Error
	if guardErr == nil {
		return applyFairLaunchReplayTx(tx, in, identity, existingGuard, payloadDigest, now)
	}
	if !errors.Is(guardErr, gorm.ErrRecordNotFound) {
		return FairLaunchCEWriteResult{}, guardErr
	}
	var generation uint32
	if err := tx.Model(&entity.ChainEventPlacementV2{}).Where("event_identity_id=?", identity.ID).Select("COALESCE(MAX(generation),0)").Scan(&generation).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	generation++
	in.Context.PlacementGeneration = generation
	effect, err := MaterializeFairLaunchEvent(in.Event, in.Context)
	if err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	canonical := uint8(1)
	placement := entity.ChainEventPlacementV2{EventIdentityID: identity.ID, Generation: generation, BlockNumber: in.Event.Log.BlockNumber, BlockHash: strings.ToLower(in.Event.Log.BlockHash.Hex()), TransactionIndex: in.TransactionIndex, RPCLogIndex: uint32(in.Event.Log.Index), SourceRevisionID: in.Context.SourceRevisionID}
	if in.Context.Canonical {
		placement.CanonicalSlot = &canonical
	}
	if in.Context.Finalized {
		placement.FinalizedAt = &now
	}
	if err := tx.Create(&placement).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	obs := entity.ChainEventPlacementObservationV2{PlacementID: placement.ID, EventIdentityID: identity.ID, ProviderIdentity: in.ProviderIdentity, EmittingAddress: strings.ToLower(in.Event.Log.Address.Hex()), TopicsDataDigest: stripCE(in.TopicsDataDigest), RawLogDigest: stripCE(in.RawLogDigest), ObservedAt: now}
	if err := tx.Create(&obs).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	var attempt uint32
	if err := tx.Model(&entity.ChainEventApplicationAttemptV2{}).Where("event_identity_id=? AND placement_id=? AND effect_version=?", identity.ID, placement.ID, in.Context.EffectVersion).Select("COALESCE(MAX(attempt_no),0)").Scan(&attempt).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	attempt++
	effectDigest := effect.EffectDigest
	application := entity.ChainEventApplicationAttemptV2{EventIdentityID: identity.ID, PlacementID: placement.ID, EffectVersion: in.Context.EffectVersion, ExpectedEffect: 1, AttemptNo: attempt, Disposition: string(effect.Disposition), EffectDigest: effectDigest}
	if effect.Disposition == FairLaunchEffectExact || effect.Disposition == FairLaunchEffectNoOp {
		application.AppliedAt = &now
	}
	if err := tx.Create(&application).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	guard := entity.FairLaunchEffectApplication{EventIdentityID: identity.ID, EffectVersion: in.Context.EffectVersion, ApplicationAttemptID: application.ID, ObservationID: obs.ID, EventName: in.Event.Name, PayloadDigest: payloadDigest, EffectDigest: effectDigest, AppliedAt: now}
	if err := tx.Create(&guard).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	if effect.ProjectChanged {
		if err := applyFairLaunchProjectEffectTx(tx, in.Event, now); err != nil {
			return FairLaunchCEWriteResult{}, err
		}
	}
	if effect.PositionChanged {
		if err := applyFairLaunchPositionEffectTx(tx, in.Event, in.Context, identity.ID, now); err != nil {
			return FairLaunchCEWriteResult{}, err
		}
	}
	if err := advanceFairLaunchCursorTx(tx, &cursor, in, identity.ID, now); err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	return FairLaunchCEWriteResult{IdentityID: identity.ID, PlacementID: placement.ID, ObservationID: obs.ID, ApplicationID: application.ID, Effect: effect}, nil
}

func advanceFairLaunchCursorTx(tx *gorm.DB, cursor *entity.SourceMaterializationCursor, in FairLaunchCEWrite, identityID uint64, now time.Time) error {
	cursor.SourceRevisionID = in.Context.SourceRevisionID
	cursor.MaterializedThroughBlock = in.Event.Log.BlockNumber
	cursor.MaterializedThroughBlockHash = strings.ToLower(in.Event.Log.BlockHash.Hex())
	cursor.MaterializedThroughTxIndex = in.TransactionIndex
	cursor.MaterializedThroughTxLogOrdinal = in.Context.TxLogOrdinal
	cursor.LastEventIdentityID = &identityID
	cursor.CursorGeneration++
	cursor.UpdatedAt = now
	if err := tx.Save(cursor).Error; err != nil {
		return err
	}
	return nil
}
func applyFairLaunchProjectEffectTx(tx *gorm.DB, event *commonchain.FairLaunchTypedEvent, now time.Time) error {
	if event.ProjectID == nil {
		return nil
	}
	updates := map[string]any{"updated_at": now}
	switch event.Name {
	case "FairLaunchContribution":
		if raw := event.RawDecimal["arg3"]; raw != "" {
			updates["total_raised_raw"] = gorm.Expr("CAST(total_raised_raw AS DECIMAL(65,0)) + CAST(? AS DECIMAL(65,0))", raw)
		}
	case "FairLaunchEnded":
		// FairLaunchEnded arg2 is terminalReason; arg4 is finalTotalRaised.
		raw := event.RawDecimal["arg4"]
		if raw == "" {
			return errors.New("FairLaunchEnded finalTotalRaised is missing")
		}
		updates["final_total_raised_raw"] = raw
	case "FairLaunchClaimed":
	case "FairLaunchRefunded":
	case "FairLaunchProjectProceedsWithdrawn":
		updates["fund_withdrawn"] = 1
	case "FairLaunchPlatformFeeWithdrawn":
		updates["platform_fee_withdrawn"] = 1
	case "FairLaunchFailedProjectTokensWithdrawn":
		updates["token_withdrawn"] = 1
	}
	return tx.Model(&entity.Project{}).Where("project_id=? AND funding_mode='fair_launch'", event.ProjectID.Uint64()).Updates(updates).Error
}
func applyFairLaunchPositionEffectTx(tx *gorm.DB, event *commonchain.FairLaunchTypedEvent, ctx FairLaunchMaterializationContext, eventIdentityID uint64, now time.Time) error {
	if event.ProjectID == nil {
		return nil
	}
	if eventIdentityID == 0 {
		return errors.New("fair launch position event identity missing")
	}
	wallet := eventAddress(event)
	if wallet == "" {
		return errors.New("fair launch position event wallet missing")
	}
	var project entity.Project
	if err := tx.Where("chain_id=? AND project_id=? AND funding_mode='fair_launch' AND contract_version='fair_launch_v1'", ctx.ChainID, event.ProjectID.Uint64()).Take(&project).Error; err != nil {
		return err
	}
	var row entity.FairLaunchPosition
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND project_id=? AND wallet_address=? AND status='canonical'", ctx.ChainID, event.ProjectID.Uint64(), wallet).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = entity.FairLaunchPosition{ChainID: ctx.ChainID, ContractAddress: strings.ToLower(project.ContractAddress), ContractVersion: project.ContractVersion, ProjectID: event.ProjectID.Uint64(), WalletAddress: wallet, PaymentTokenAddress: strings.ToLower(project.PaymentTokenAddress), PaymentTokenDecimals: uint8(project.PaymentTokenDecimals), ProjectTokenAddress: strings.ToLower(project.TokenAddress), ProjectTokenDecimals: uint8(project.TokenDecimals), ContributionAmountRaw: "0", ContributionAmountDisplay: "0", ClaimedTokensRaw: "0", ClaimedTokensDisplay: "0", Status: "canonical", CreatedAt: now}
		if err = tx.Create(&row).Error; err != nil {
			if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND contract_address=? AND contract_version=? AND project_id=? AND wallet_address=?", ctx.ChainID, row.ContractAddress, row.ContractVersion, row.ProjectID, row.WalletAddress).Take(&row).Error; lockErr != nil {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(row.PaymentTokenAddress, project.PaymentTokenAddress) || row.PaymentTokenDecimals != uint8(project.PaymentTokenDecimals) || !strings.EqualFold(row.ProjectTokenAddress, project.TokenAddress) || row.ProjectTokenDecimals != uint8(project.TokenDecimals) {
		return errors.New("fair launch position token identity mismatch")
	}
	amountRaw := "0"
	if event.Name == "FairLaunchContribution" || event.Name == "FairLaunchClaimed" {
		amountRaw = event.RawDecimal["arg3"]
		if amountRaw == "" || amountRaw == "0" {
			return errors.New("fair launch position event amount missing")
		}
	}
	switch event.Name {
	case "FairLaunchContribution":
		row.ContributionAmountRaw = addRawCE(row.ContributionAmountRaw, amountRaw)
		if row.FirstContributionEventID == nil {
			row.FirstContributionEventID = &eventIdentityID
		}
		row.LastContributionEventID = &eventIdentityID
	case "FairLaunchClaimed":
		row.ClaimedTokensRaw = addRawCE(row.ClaimedTokensRaw, amountRaw)
		row.LastClaimEventID = &eventIdentityID
	case "FairLaunchRefunded":
		row.Refunded = true
		row.LastRefundEventID = &eventIdentityID
	}
	var formatErr error
	row.ContributionAmountDisplay, formatErr = money.FormatRaw(row.ContributionAmountRaw, row.PaymentTokenDecimals)
	if formatErr != nil {
		return fmt.Errorf("format fair launch contribution projection: %w", formatErr)
	}
	row.ClaimedTokensDisplay, formatErr = money.FormatRaw(row.ClaimedTokensRaw, row.ProjectTokenDecimals)
	if formatErr != nil {
		return fmt.Errorf("format fair launch claimed projection: %w", formatErr)
	}
	row.UpdatedAt = now
	return tx.Save(&row).Error
}
func eventAddress(event *commonchain.FairLaunchTypedEvent) string {
	for _, key := range []string{"arg1", "arg2"} {
		if v, ok := event.Indexed[key].(common.Address); ok && v != (common.Address{}) {
			return strings.ToLower(v.Hex())
		}
	}
	return ""
}
func addRawCE(a, b string) string {
	x, _ := new(big.Int).SetString(a, 10)
	if x == nil {
		x = new(big.Int)
	}
	y, _ := new(big.Int).SetString(b, 10)
	if y == nil {
		y = new(big.Int)
	}
	return x.Add(x, y).String()
}
func stripCE(v string) string { return strings.TrimPrefix(strings.ToLower(v), "0x") }

func applyFairLaunchReplayTx(tx *gorm.DB, in FairLaunchCEWrite, identity entity.ChainEventIdentityV2, guard entity.FairLaunchEffectApplication, payloadDigest string, now time.Time) (FairLaunchCEWriteResult, error) {
	if guard.EventName != in.Event.Name || guard.PayloadDigest != payloadDigest {
		return FairLaunchCEWriteResult{}, errors.New("fair launch event effect identity conflict")
	}
	var original entity.ChainEventApplicationAttemptV2
	if err := tx.Where("id=? AND event_identity_id=? AND effect_version=?", guard.ApplicationAttemptID, identity.ID, in.Context.EffectVersion).Take(&original).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	if original.EffectDigest != guard.EffectDigest || original.ReversedAt != nil {
		return FairLaunchCEWriteResult{}, errors.New("fair launch event effect replay unavailable")
	}
	var originalPlacement entity.ChainEventPlacementV2
	if err := tx.Where("id=? AND event_identity_id=?", original.PlacementID, identity.ID).Take(&originalPlacement).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	if originalPlacement.BlockNumber != in.Event.Log.BlockNumber || originalPlacement.BlockHash != strings.ToLower(in.Event.Log.BlockHash.Hex()) || originalPlacement.TransactionIndex != in.TransactionIndex || originalPlacement.RPCLogIndex != uint32(in.Event.Log.Index) || originalPlacement.SourceRevisionID != in.Context.SourceRevisionID || originalPlacement.CanonicalSlot == nil || originalPlacement.FinalizedAt == nil || originalPlacement.NoncanonicalAt != nil {
		return FairLaunchCEWriteResult{}, errors.New("fair launch event placement replay mismatch")
	}
	obs := entity.ChainEventPlacementObservationV2{PlacementID: originalPlacement.ID, EventIdentityID: identity.ID, ProviderIdentity: in.ProviderIdentity, EmittingAddress: strings.ToLower(in.Event.Log.Address.Hex()), TopicsDataDigest: stripCE(in.TopicsDataDigest), RawLogDigest: stripCE(in.RawLogDigest), ObservedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&obs).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	var attempt uint32
	if err := tx.Model(&entity.ChainEventApplicationAttemptV2{}).Where("event_identity_id=? AND placement_id=? AND effect_version=?", identity.ID, originalPlacement.ID, in.Context.EffectVersion).Select("COALESCE(MAX(attempt_no),0)").Scan(&attempt).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	attempt++
	application := entity.ChainEventApplicationAttemptV2{EventIdentityID: identity.ID, PlacementID: originalPlacement.ID, EffectVersion: in.Context.EffectVersion, ExpectedEffect: 0, AttemptNo: attempt, PredecessorApplicationID: &guard.ApplicationAttemptID, Disposition: string(FairLaunchEffectNoOp), EffectDigest: guard.EffectDigest, AppliedAt: &now}
	if err := tx.Create(&application).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	return FairLaunchCEWriteResult{IdentityID: identity.ID, PlacementID: originalPlacement.ID, ObservationID: obs.ID, ApplicationID: application.ID, Effect: FairLaunchMaterializationResult{EventName: in.Event.Name, Disposition: FairLaunchEffectNoOp}}, nil
}
