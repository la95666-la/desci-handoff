package chainevent

import (
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/common/purchaseentitlement"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
)

func (v *GetChainEventJob) ProcessV2Log(data *commonChain.V2Event) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := requireNoActiveReorgIncidentTx(tx); err != nil {
			return err
		}
		return v.processV2LogTx(tx, data)
	})
}

func (v *GetChainEventJob) processV2LogTx(tx *gorm.DB, data *commonChain.V2Event) error {
	// Source identity is supplied by the admitted scanner client. Keeping the
	// event projection behind this wrapper also permits an isolated real-chain
	// test to exercise the exact consumer without mutating global Job config.
	client := chain.GetClient()
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() {
		return errors.New("V2 event scanner source identity is unavailable")
	}
	return v.processV2LogWithSourceTx(tx, data, client.ChainID.Uint64(), chain.V2ContractAddress())
}

func (v *GetChainEventJob) processV2LogWithSourceTx(tx *gorm.DB, data *commonChain.V2Event, chainID uint64, contract common.Address) error {
	if data == nil {
		return errors.New("V2 event is nil")
	}
	if chainID == 0 || contract == (common.Address{}) || data.ContractAddress != contract {
		return fmt.Errorf("V2 event contract mismatch: got=%s configured=%s", data.ContractAddress.Hex(), contract.Hex())
	}
	if data.ProjectID == 0 || data.ProjectID > math.MaxInt64 {
		return fmt.Errorf("V2 project ID %d is outside database range", data.ProjectID)
	}
	processed := entity.ChainEventRecord{}
	if err := tx.Select("id").Where(
		"block_number = ? AND tx_hash = ? AND log_index = ?", data.BlockNumber, data.TxHash, data.LogIndex,
	).Take(&processed).Error; err == nil {
		return resolveEventFailure(tx, data.BlockNumber, data.TxHash, data.LogIndex)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query V2 chain event record: %w", err)
	}

	now := time.Now()
	project := entity.Project{}
	// Load and validate the complete project identity before any financial,
	// entitlement, lifecycle, or event-record mutation.
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", data.ProjectID).Take(&project).Error; err != nil {
		return fmt.Errorf("load V2 project: %w", err)
	}
	if err := validateV2ProjectSource(project, data, chainID, contract); err != nil {
		return err
	}
	if err := applyV2Event(tx, &project, data, now); err != nil {
		return fmt.Errorf("apply V2 %s: %w", data.Event, err)
	}
	content, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal V2 event: %w", err)
	}
	record := entity.ChainEventRecord{
		Event: data.Event, BlockNumber: int(data.BlockNumber), BlockHash: data.BlockHash,
		TxHash: data.TxHash, LogIndex: data.LogIndex, ContractAddress: data.ContractAddress.Hex(),
		ContractVersion: constant.ContractVersionV2Stablecoin, Data: string(content), CreatedAt: now,
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("create V2 chain event record: %w", err)
	}
	if action, known := v2LifecycleFinalizeAction(data.Event); known {
		if tolerant := v2LifecycleFinalizeTolerant(data.Event); tolerant {
			// 安全审计 MED-1：endProject / withdrawPlatformFee 在合约层是 permissionless，
			// Pending 项目的 projectOwner 也可直接 cancelProject。链上 canonical 外部事件
			// 必须可落库：存在同 txHash 的 durable submission 时推进 applied；否则取消被
			// 取代的 unsigned intent（金额/身份/状态校验已在 applyV2Event 完成）。
			submission, err := chaintx.FindEventSubmissionTx(tx, uint64(data.ProjectID), action, data.TxHash)
			if err != nil {
				return err
			}
			if submission != nil {
				if err := chaintx.MarkAppliedTx(tx, submission.ID, record.ID); err != nil {
					return fmt.Errorf("finalize V2 lifecycle submission: %w", err)
				}
			} else if err := chaintx.CancelUnsignedBusinessTx(tx, uint64(data.ProjectID), action); err != nil {
				return err
			}
		} else {
			// operator/owner-gated 生命周期（create/activate）维持严格匹配：
			// 外部无法发起，事件必须对应一条后端 durable submission。
			sender, err := v2LifecycleSubmissionSender()
			if err != nil {
				return err
			}
			submission, err := chaintx.RequireEventSubmissionTx(tx, uint64(data.ProjectID), action, data.TxHash, project.ChainID, data.ContractAddress, sender)
			if err != nil {
				return err
			}
			if err := chaintx.MarkAppliedTx(tx, submission.ID, record.ID); err != nil {
				return fmt.Errorf("finalize V2 lifecycle submission: %w", err)
			}
		}
	}
	return resolveEventFailure(tx, data.BlockNumber, data.TxHash, data.LogIndex)
}

func validateV2ProjectSource(project entity.Project, data *commonChain.V2Event, expectedChainID uint64, expectedContract common.Address) error {
	if err := constant.RequireExactV2ProjectIdentity(project.FundingMode, project.ContractVersion); err != nil {
		return fmt.Errorf("project %d is not exact V2: %w", data.ProjectID, err)
	}
	if project.ChainID != expectedChainID {
		return fmt.Errorf("project %d chain ID %d does not match consumer %d", data.ProjectID, project.ChainID, expectedChainID)
	}
	if data.ContractAddress != expectedContract || !strings.EqualFold(project.ContractAddress, data.ContractAddress.Hex()) {
		return fmt.Errorf("project %d contract address mismatch", data.ProjectID)
	}
	if data.Event != commonChain.V2ProjectCreated && !project.OnchainLocked {
		return fmt.Errorf("project %d is not locked to an on-chain identity", data.ProjectID)
	}
	return nil
}

func applyV2Event(tx *gorm.DB, project *entity.Project, data *commonChain.V2Event, now time.Time) error {
	switch data.Event {
	case commonChain.V2ProjectCreated:
		if err := requireV2LifecycleSubmission(tx, *project, constant.TransactionTypeV2Create, data.TxHash); err != nil {
			return err
		}
		return applyV2ProjectCreated(tx, project, data)
	case commonChain.V2ProjectActivated:
		if err := requireV2LifecycleSubmission(tx, *project, constant.TransactionTypeV2Activate, data.TxHash); err != nil {
			return err
		}
		return updateProjectStatus(tx, project.ProjectID, constant.ProjectStatusPending, constant.ProjectStatusActive)
	case commonChain.V2ProjectEnded:
		if err := requirePaymentToken(project, data.PaymentToken); err != nil {
			return err
		}
		if data.TotalRaisedRaw != project.TotalRaisedRaw {
			return fmt.Errorf("ended totalRaised raw mismatch: event=%s db=%s", data.TotalRaisedRaw, project.TotalRaisedRaw)
		}
		status, err := v2ProjectStatus(data.Status)
		if err != nil {
			return err
		}
		return updateProjectStatus(tx, project.ProjectID, constant.ProjectStatusActive, status)
	case commonChain.V2ProjectCancelled:
		if err := requirePaymentToken(project, data.PaymentToken); err != nil {
			return err
		}
		if data.TotalRaisedRaw != project.TotalRaisedRaw {
			return fmt.Errorf("cancelled totalRaised raw mismatch: event=%s db=%s", data.TotalRaisedRaw, project.TotalRaisedRaw)
		}
		result := tx.Model(&entity.Project{}).Where("project_id = ? AND status IN ?", project.ProjectID, []string{constant.ProjectStatusPending, constant.ProjectStatusActive}).Update("status", constant.ProjectStatusCanceled)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("project cancellation status transition rejected")
		}
		return nil
	case commonChain.V2ProjectTokensDeposited:
		if project.Status != constant.ProjectStatusPending {
			return errors.New("project token deposit requires Pending status")
		}
		if err := requireProjectToken(project, data.ProjectToken); err != nil {
			return err
		}
		if err := applyV2Movement(tx, project, data, fundledger.EntryProjectTokenDeposited, fundledger.AssetRoleProject, data.ProjectTokenRaw, project.OwnerAddress, data.ContractAddress.Hex(), now); err != nil {
			return err
		}
		return addProjectAggregate(tx, project.ProjectID, "total_deposited_raw", project.TotalDepositedRaw, "total_deposited", project.TotalDeposited, data.ProjectTokenRaw, uint8(project.TokenDecimals))
	case commonChain.V2Purchased:
		if project.Status != constant.ProjectStatusActive {
			return errors.New("purchase requires Active status")
		}
		if err := requirePaymentToken(project, data.PaymentToken); err != nil {
			return err
		}
		if err := requireProjectToken(project, data.ProjectToken); err != nil {
			return err
		}
		var priorPurchases int64
		if err := tx.Model(&entity.FundLedger{}).Where(
			"chain_id = ? AND contract_address = ? AND project_id = ? AND entry_type = ? AND wallet_address = ?",
			project.ChainID, data.ContractAddress.Hex(), project.ProjectID, fundledger.EntryPurchasePayment, data.User.Hex(),
		).Count(&priorPurchases).Error; err != nil {
			return err
		}
		purchaseFact, err := buildV2FundFact(project, data, fundledger.EntryPurchasePayment, fundledger.AssetRolePayment, data.PaymentRaw, data.User.Hex(), data.ContractAddress.Hex(), now)
		if err != nil {
			return err
		}
		if err := tx.Create(purchaseFact).Error; err != nil {
			return err
		}
		entitlement, err := buildV2PurchaseEntitlement(project, data, purchaseFact.ID, now)
		if err != nil {
			return err
		}
		if err := tx.Create(entitlement).Error; err != nil {
			return err
		}
		if err := addProjectAggregate(tx, project.ProjectID, "total_raised_raw", project.TotalRaisedRaw, "total_raised", project.TotalRaised, data.PaymentRaw, uint8(project.PaymentTokenDecimals)); err != nil {
			return err
		}
		if priorPurchases == 0 {
			if err := tx.Model(&entity.Project{}).Where("project_id = ?", project.ProjectID).Update("participant_count", gorm.Expr("participant_count + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	case commonChain.V2Refunded:
		if project.Status != constant.ProjectStatusFailed && project.Status != constant.ProjectStatusCanceled {
			return errors.New("refund requires Failed or Canceled status")
		}
		if err := requirePaymentToken(project, data.PaymentToken); err != nil {
			return err
		}
		return applyV2Movement(tx, project, data, fundledger.EntryPaymentRefunded, fundledger.AssetRolePayment, data.PaymentRaw, data.User.Hex(), data.ContractAddress.Hex(), now)
	case commonChain.V2Claimed:
		if project.Status != constant.ProjectStatusSuccessful {
			return errors.New("claim requires Successful status")
		}
		if err := requireProjectToken(project, data.ProjectToken); err != nil {
			return err
		}
		return applyV2Movement(tx, project, data, fundledger.EntryProjectTokenClaimed, fundledger.AssetRoleProject, data.ProjectTokenRaw, data.User.Hex(), data.ContractAddress.Hex(), now)
	case commonChain.V2ProjectProceedsWithdrawn:
		if project.Status != constant.ProjectStatusSuccessful {
			return errors.New("project proceeds require Successful status")
		}
		if err := requirePaymentToken(project, data.PaymentToken); err != nil {
			return err
		}
		if !strings.EqualFold(project.OwnerAddress, data.ProjectOwner.Hex()) {
			return errors.New("project proceeds owner mismatch")
		}
		expectedFee, expectedOwner, err := expectedV2Settlement(project)
		if err != nil {
			return err
		}
		if data.PlatformFeeRaw != expectedFee || data.PaymentRaw != expectedOwner {
			return errors.New("project proceeds settlement amount mismatch")
		}
		if err := applyV2Movement(tx, project, data, fundledger.EntryProjectProceedsPaid, fundledger.AssetRolePayment, data.PaymentRaw, data.ProjectOwner.Hex(), data.ContractAddress.Hex(), now); err != nil {
			return err
		}
		return setProjectFlag(tx, project.ProjectID, "fund_withdrawn")
	case commonChain.V2PlatformFeeWithdrawn:
		if project.Status != constant.ProjectStatusSuccessful {
			return errors.New("platform fee requires Successful status")
		}
		if err := requirePaymentToken(project, data.PaymentToken); err != nil {
			return err
		}
		if !strings.EqualFold(project.FeeReceiver, data.FeeReceiver.Hex()) {
			return errors.New("platform fee receiver mismatch")
		}
		expectedFee, _, err := expectedV2Settlement(project)
		if err != nil {
			return err
		}
		if data.PaymentRaw != expectedFee {
			return errors.New("platform fee settlement amount mismatch")
		}
		if err := applyV2Movement(tx, project, data, fundledger.EntryPlatformFeePaid, fundledger.AssetRolePayment, data.PaymentRaw, data.FeeReceiver.Hex(), data.ContractAddress.Hex(), now); err != nil {
			return err
		}
		return setProjectFlag(tx, project.ProjectID, "platform_fee_withdrawn")
	case commonChain.V2ProjectTokensWithdrawn:
		if project.Status != constant.ProjectStatusSuccessful && project.Status != constant.ProjectStatusFailed && project.Status != constant.ProjectStatusCanceled {
			return errors.New("project token withdrawal requires terminal status")
		}
		if err := requireProjectToken(project, data.ProjectToken); err != nil {
			return err
		}
		if !strings.EqualFold(project.OwnerAddress, data.ProjectOwner.Hex()) {
			return errors.New("project token withdrawal owner mismatch")
		}
		if err := applyV2Movement(tx, project, data, fundledger.EntryProjectTokensWithdrawn, fundledger.AssetRoleProject, data.ProjectTokenRaw, data.ProjectOwner.Hex(), data.ContractAddress.Hex(), now); err != nil {
			return err
		}
		return setProjectFlag(tx, project.ProjectID, "token_withdrawn")
	default:
		return fmt.Errorf("unsupported V2 event %q", data.Event)
	}
}

func requireV2LifecycleSubmission(tx *gorm.DB, project entity.Project, action, txHash string) error {
	sender, err := v2LifecycleSubmissionSender()
	if err != nil {
		return err
	}
	_, err = chaintx.RequireEventSubmissionTx(tx, uint64(project.ProjectID), action, txHash, project.ChainID, common.HexToAddress(project.ContractAddress), sender)
	return err
}

func v2LifecycleSubmissionSender() (common.Address, error) {
	client := chain.GetClient()
	if client == nil {
		return common.Address{}, errors.New("V2 lifecycle Job client is unavailable")
	}
	return validateV2LifecycleSubmissionSender(client.Address, chain.V2CoreOperatorAddress())
}

func validateV2LifecycleSubmissionSender(jobSigner, coreOperator common.Address) (common.Address, error) {
	if jobSigner == (common.Address{}) || coreOperator == (common.Address{}) {
		return common.Address{}, errors.New("V2 lifecycle sender identity is unavailable")
	}
	if coreOperator == jobSigner {
		return common.Address{}, errors.New("V2 lifecycle Core operator must remain separate from Job signer")
	}
	return coreOperator, nil
}

// v2LifecycleFinalizeAction maps a lifecycle V2 event to its durable action.
func v2LifecycleFinalizeAction(event string) (string, bool) {
	switch event {
	case commonChain.V2ProjectCreated:
		return constant.TransactionTypeV2Create, true
	case commonChain.V2ProjectActivated:
		return constant.TransactionTypeV2Activate, true
	case commonChain.V2ProjectEnded:
		return constant.TransactionTypeProjectEnd, true
	case commonChain.V2ProjectCancelled:
		return constant.TransactionTypeProjectCancel, true
	case commonChain.V2PlatformFeeWithdrawn:
		return constant.TransactionTypePlatformFee, true
	default:
		return "", false
	}
}

// v2LifecycleFinalizeTolerant reports whether the on-chain entry point for the
// event is permissionless, so a canonical external transaction without a durable
// submission must still be applied (MED-1). create/activate stay strict: the
// contract gates them to operator-or-owner, both held by the relayer identity.
func v2LifecycleFinalizeTolerant(event string) bool {
	switch event {
	case commonChain.V2ProjectEnded, commonChain.V2ProjectCancelled, commonChain.V2PlatformFeeWithdrawn:
		return true
	default:
		return false
	}
}

// applyV2GlobalPauseStateTx 将 Paused/Unpaused 治理事件对应的 global_pause /
// global_unpause durable submission 推进 applied 并释放 relayer signer fence
// （安全审计 HIGH-1 配套）。全局暂停无金额语义，不产生 fund ledger 记录；
// final_event_record_id=0 表示由治理事件直接终结（无 material 事件记录）。
// 若链上事件源自 owner 直发（无 durable submission），这里安静跳过。
func (v *GetChainEventJob) applyV2GlobalPauseStateTx(tx *gorm.DB, info types.Log) error {
	if info.Address != chain.V2ContractAddress() {
		return fmt.Errorf("V2 governance event contract mismatch: got=%s configured=%s", info.Address.Hex(), chain.V2ContractAddress().Hex())
	}
	action, event := v2GlobalPauseActionForTopic(info.Topics[0])
	if action == "" {
		return fmt.Errorf("unapproved V2 governance event topic %s", info.Topics[0].Hex())
	}
	submission, err := chaintx.FindGlobalEventSubmissionTx(tx, action, info.TxHash.Hex(), chain.GetClient().ChainID.Uint64(), chain.V2ContractAddress(), chain.GetClient().Address)
	if err != nil {
		return err
	}
	if submission == nil {
		return nil
	}
	if err := chaintx.MarkAppliedTx(tx, submission.ID, 0); err != nil {
		return fmt.Errorf("finalize %s submission: %w", event, err)
	}
	return nil
}

// v2GlobalPauseActionForTopic maps a governance pause topic to its durable action.
func v2GlobalPauseActionForTopic(topic common.Hash) (action, event string) {
	switch topic {
	case commonChain.V2PausedTopic:
		return constant.TransactionTypeGlobalPause, "Paused"
	case commonChain.V2UnpausedTopic:
		return constant.TransactionTypeGlobalUnpause, "Unpaused"
	default:
		return "", ""
	}
}

func validLifecycleTxHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil && common.HexToHash(value) != (common.Hash{})
}

func applyV2ProjectCreated(tx *gorm.DB, project *entity.Project, data *commonChain.V2Event) error {
	if !project.OnchainLocked || project.Status != constant.ProjectStatusSet {
		return errors.New("project is not a locked Set broadcast")
	}
	if err := requirePaymentToken(project, data.PaymentToken); err != nil {
		return err
	}
	if err := requireProjectToken(project, data.ProjectToken); err != nil {
		return err
	}
	if !strings.EqualFold(project.OwnerAddress, data.ProjectOwner.Hex()) {
		return errors.New("project owner mismatch")
	}
	if project.TokenDecimals != int(data.ProjectDecimals) || project.PaymentTokenDecimals != int(data.PaymentDecimals) {
		return errors.New("token decimals snapshot mismatch")
	}
	if err := validateV2FeeSnapshot(*project, data.FeeReceiver, data.FeeRate); err != nil {
		return err
	}
	if project.TokenPriceRaw != data.TokenPriceRaw || project.HardCapRaw != data.HardCapRaw || project.SoftCapRaw != data.SoftCapRaw || project.MaxPerUserRaw != data.MaxPerUserRaw {
		return errors.New("project amount snapshot mismatch")
	}
	start, err := rawUnix(data.StartTimeRaw)
	if err != nil {
		return err
	}
	end, err := rawUnix(data.EndTimeRaw)
	if err != nil {
		return err
	}
	if project.StartTime.Unix() != start || project.EndTime.Unix() != end {
		return errors.New("project time snapshot mismatch")
	}
	result := tx.Model(&entity.Project{}).Where("project_id = ? AND status = ? AND onchain_locked = ?", project.ProjectID, constant.ProjectStatusSet, true).Updates(map[string]interface{}{
		"status": constant.ProjectStatusPending, "onchain_locked": true, "owner_address": data.ProjectOwner.Hex(), "fee_receiver": data.FeeReceiver.Hex(), "success_fee_rate": data.FeeRate,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("project create transition rejected")
	}
	return nil
}

func validateV2FeeSnapshot(project entity.Project, receiver common.Address, rate uint16) error {
	if !strings.EqualFold(project.FeeReceiver, receiver.Hex()) || project.SuccessFeeRate != rate {
		return errors.New("project fee snapshot mismatch")
	}
	return nil
}

func applyV2Movement(tx *gorm.DB, project *entity.Project, data *commonChain.V2Event, entryType, assetRole, raw, wallet, counterparty string, now time.Time) error {
	fact, err := buildV2FundFact(project, data, entryType, assetRole, raw, wallet, counterparty, now)
	if err != nil {
		return err
	}
	return tx.Create(fact).Error
}

func buildV2FundFact(project *entity.Project, data *commonChain.V2Event, entryType, assetRole, raw, wallet, counterparty string, now time.Time) (*entity.FundLedger, error) {
	canonical, err := money.NormalizeRawUint256(raw)
	if err != nil {
		return nil, err
	}
	decimals := project.PaymentTokenDecimals
	tokenAddress := project.PaymentTokenAddress
	symbol := project.PaymentToken
	if assetRole == fundledger.AssetRoleProject {
		decimals = project.TokenDecimals
		tokenAddress = project.TokenAddress
		symbol = project.TokenSymbol
	}
	display, err := money.FormatRaw(canonical, uint8(decimals))
	if err != nil {
		return nil, err
	}
	eventJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	fact := entity.FundLedger{
		ChainID: project.ChainID, ContractAddress: data.ContractAddress.Hex(), ContractVersion: constant.ContractVersionV2Stablecoin,
		ProjectID: data.ProjectID, WalletAddress: wallet, CounterpartyAddress: counterparty, EntryType: entryType,
		AssetRole: assetRole, TokenSymbol: symbol, TokenAddress: tokenAddress, TokenDecimals: uint8(decimals),
		AmountRaw: canonical, AmountDisplay: display, TxHash: data.TxHash, LogIndex: data.LogIndex,
		BlockNumber: data.BlockNumber, BlockHash: data.BlockHash, Status: fundledger.StatusCanonical,
		EventData: string(eventJSON), CreatedAt: now, UpdatedAt: now,
	}
	if err := fundledger.Validate(&fact); err != nil {
		return nil, err
	}
	return &fact, nil
}

func buildV2PurchaseEntitlement(project *entity.Project, data *commonChain.V2Event, fundLedgerID uint64, now time.Time) (*entity.IDOPurchaseEntitlement, error) {
	if !purchaseentitlement.MatchesPurchaseFormula(data.PaymentRaw, data.ProjectTokenRaw, project.TokenPriceRaw, uint8(project.TokenDecimals)) {
		return nil, errors.New("purchase entitlement amount does not match project token-price formula")
	}
	paymentRaw, err := money.NormalizeRawUint256(data.PaymentRaw)
	if err != nil {
		return nil, err
	}
	projectRaw, err := money.NormalizeRawUint256(data.ProjectTokenRaw)
	if err != nil {
		return nil, err
	}
	paymentDisplay, err := money.FormatRaw(paymentRaw, uint8(project.PaymentTokenDecimals))
	if err != nil {
		return nil, err
	}
	projectDisplay, err := money.FormatRaw(projectRaw, uint8(project.TokenDecimals))
	if err != nil {
		return nil, err
	}
	eventJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	fact := entity.IDOPurchaseEntitlement{
		FundLedgerID: fundLedgerID, ChainID: project.ChainID, ContractAddress: data.ContractAddress.Hex(), ContractVersion: constant.ContractVersionV2Stablecoin,
		ProjectID: data.ProjectID, WalletAddress: data.User.Hex(),
		PaymentTokenSymbol: project.PaymentToken, PaymentTokenAddress: project.PaymentTokenAddress, PaymentTokenDecimals: uint8(project.PaymentTokenDecimals), PaymentAmountRaw: paymentRaw, PaymentAmountDisplay: paymentDisplay,
		ProjectTokenSymbol: project.TokenSymbol, ProjectTokenAddress: project.TokenAddress, ProjectTokenDecimals: uint8(project.TokenDecimals), ProjectTokenAmountRaw: projectRaw, ProjectTokenDisplay: projectDisplay,
		TxHash: data.TxHash, LogIndex: data.LogIndex, BlockNumber: data.BlockNumber, BlockHash: data.BlockHash,
		Status: fundledger.StatusCanonical, EventData: string(eventJSON), CreatedAt: now,
	}
	if err := purchaseentitlement.Validate(&fact); err != nil {
		return nil, err
	}
	return &fact, nil
}

func requirePaymentToken(project *entity.Project, address common.Address) error {
	if !strings.EqualFold(project.PaymentTokenAddress, address.Hex()) {
		return errors.New("payment token mismatch")
	}
	return nil
}
func requireProjectToken(project *entity.Project, address common.Address) error {
	if !strings.EqualFold(project.TokenAddress, address.Hex()) {
		return errors.New("project token mismatch")
	}
	return nil
}
func updateProjectStatus(tx *gorm.DB, id int, from, to string) error {
	result := tx.Model(&entity.Project{}).Where("project_id = ? AND status = ?", id, from).Update("status", to)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("project status transition %s -> %s rejected", from, to)
	}
	return nil
}
func setProjectFlag(tx *gorm.DB, id int, column string) error {
	result := tx.Model(&entity.Project{}).Where("project_id = ? AND "+column+" = 0", id).Update(column, 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("project flag %s transition rejected", column)
	}
	return nil
}
func v2ProjectStatus(status uint8) (string, error) {
	switch status {
	case 2:
		return constant.ProjectStatusSuccessful, nil
	case 3:
		return constant.ProjectStatusFailed, nil
	default:
		return "", fmt.Errorf("unexpected terminal V2 project status %d", status)
	}
}
func rawUnix(raw string) (int64, error) {
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok || !value.IsInt64() || value.Sign() < 0 {
		return 0, errors.New("invalid uint timestamp")
	}
	return value.Int64(), nil
}

func expectedV2Settlement(project *entity.Project) (string, string, error) {
	totalRaw, err := money.NormalizeRawUint256(project.TotalRaisedRaw)
	if err != nil {
		return "", "", err
	}
	total, _ := new(big.Int).SetString(totalRaw, 10)
	fee := new(big.Int).Mul(total, new(big.Int).SetUint64(uint64(project.SuccessFeeRate)))
	fee.Div(fee, big.NewInt(10_000))
	owner := new(big.Int).Sub(new(big.Int).Set(total), fee)
	return fee.String(), owner.String(), nil
}
func addProjectAggregate(tx *gorm.DB, projectID int, rawColumn, currentRaw, displayColumn string, currentDisplay decimal.Decimal, deltaRaw string, decimals uint8) error {
	left, err := money.NormalizeRawUint256(currentRaw)
	if err != nil {
		return fmt.Errorf("invalid current %s: %w", rawColumn, err)
	}
	right, err := money.NormalizeRawUint256(deltaRaw)
	if err != nil {
		return fmt.Errorf("invalid delta %s: %w", rawColumn, err)
	}
	a, _ := new(big.Int).SetString(left, 10)
	b, _ := new(big.Int).SetString(right, 10)
	totalRaw, err := money.NormalizeRawUint256(new(big.Int).Add(a, b).String())
	if err != nil {
		return err
	}
	deltaDisplayText, err := money.FormatRaw(right, decimals)
	if err != nil {
		return err
	}
	deltaDisplay, err := decimal.NewFromString(deltaDisplayText)
	if err != nil {
		return err
	}
	result := tx.Model(&entity.Project{}).Where("project_id = ? AND "+rawColumn+" = ?", projectID, currentRaw).Updates(map[string]interface{}{
		rawColumn: totalRaw, displayColumn: currentDisplay.Add(deltaDisplay),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("project aggregate %s update rejected", rawColumn)
	}
	return nil
}
