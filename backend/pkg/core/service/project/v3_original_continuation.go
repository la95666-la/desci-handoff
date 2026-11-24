package project

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/db"
)

// Explicit authenticated continuation, not a new attempt or automatic Job
// broadcast. Default dry-run is exposed separately by the controller. The
// hash and submission are caller confirmations, never transaction authority.
func ContinueApprovedV3Original(ctx context.Context, id uint64, expectedHash string, execute bool, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	invalid := errors.New("V3 original continuation unavailable; inspect original attempt")
	if ctx == nil || ctx.Err() != nil || id == 0 || !v3CanonicalHash(expectedHash) || guard == nil || !ApprovedV3ModeReady() || db.Mysql() == nil {
		return nil, invalid
	}
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	var row entity.ChainTxSubmission
	database := db.Mysql()
	if database.WithContext(ctx).Where("id=?", id).Take(&row).Error != nil || row.TxHash == nil || !strings.EqualFold(*row.TxHash, expectedHash) || row.State != "signed" || row.BroadcastCount != 0 {
		return nil, invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, row.ProjectID)
	if err != nil {
		return nil, invalid
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: record.Identity.ChainID, ProjectID: row.ProjectID, ProxyAddress: common.HexToAddress(record.Identity.Proxy), ProjectToken: common.HexToAddress(record.Identity.ProjectToken), PaymentToken: common.HexToAddress(record.Identity.PaymentToken)}
	if !strings.EqualFold(row.SenderAddress, v3DiagnosticOperator) || !strings.EqualFold(row.ContractAddress, v3TokenProxy) || chaintx.VerifyV3PersistedAppliedAttempt(row, binding) != nil {
		return nil, invalid
	}
	if row.Action == "ido_v3_create" && time.Now().Unix() >= record.Identity.CreateDeadline {
		return nil, wrapV3OperationFailure(ctx, database, id, "signed_preflight", v3diagnostic.New("CREATE_DEADLINE_EXPIRED", "signed_preflight", nil))
	}
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return guard(tx) }); err != nil {
		return nil, err
	}
	reader, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, invalid
	}
	defer reader.Close()
	// No private key or Core Client is passed to the first-broadcast transport.
	independent, err := chainrpc.Dial(ctx, "https://bsc-dataseed1.bnbchain.org")
	if err != nil {
		return nil, invalid
	}
	defer independent.Close()
	owner, governor, err := approvedV3SafeHashes()
	if err != nil {
		return nil, invalid
	}
	recheck := func(tx *gorm.DB) error {
		if err := guard(tx); err != nil {
			return err
		}
		fresh, err := idov3whitelist.ReadApprovedSetRecord(ctx, tx, row.ProjectID)
		if err != nil || fresh.Identity != record.Identity {
			return invalid
		}
		var current entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).Take(&current).Error != nil || !bytes.Equal(current.RawTx, row.RawTx) || current.TxHash == nil || !strings.EqualFold(*current.TxHash, expectedHash) {
			return invalid
		}
		if row.Action == "ido_v3_create" && checkApprovedV3SignedContinuation(ctx, tx, row.ProjectID, id, record.Identity, binding) != nil {
			return invalid
		}
		return nil
	}
	status := ""
	if row.Action == "ido_v3_create" {
		plan, e := previewApprovedV3CoreCreate(ctx, database, int64(row.ProjectID), common.HexToAddress(row.SenderAddress), basechain.V3Operator)
		if e != nil || !bytes.Equal(plan.CallData, row.CallData) || checkApprovedV3SignedContinuation(ctx, database, row.ProjectID, id, record.Identity, binding) != nil {
			return nil, invalid
		}
		status = "Set"
	} else {
		method := map[string]string{"ido_v3_activate": "activateProject", "ido_v3_end": "endProject", "ido_v3_withdraw_platform_fee": "withdrawPlatformFee"}[row.Action]
		if method == "" {
			return nil, invalid
		}
		admitted, candidate, e := chaintx.AdmitApprovedLockedProjectForLifecycle(ctx, database, reader, row.ProjectID, owner, governor)
		if e != nil || admitted.Binding != binding {
			return nil, invalid
		}
		governance := idov3whitelist.GovernanceIdentity{FollowOwner: true, Governor: common.HexToAddress(v3DiagnosticGovernor), OwnerCodeHash: owner, GovernorCodeHash: governor}
		plan, _, e := previewApprovedV3Lifecycle(ctx, database, reader, admitted, method, common.HexToAddress(row.SenderAddress), governance)
		if e != nil || !bytes.Equal(plan.CallData, row.CallData) || chaintx.RecheckApprovedLifecycleCreation(ctx, database, reader, row.ProjectID, candidate) != nil {
			return nil, invalid
		}
		prior := recheck
		recheck = func(tx *gorm.DB) error {
			if err := prior(tx); err != nil {
				return err
			}
			fresh, e := idov3whitelist.ReadApprovedLockedCandidate(ctx, tx, row.ProjectID)
			if e != nil || chaintx.RecheckPolicyObservationCandidate(candidate, fresh) != nil {
				return invalid
			}
			return nil
		}
		status = map[string]string{"ido_v3_activate": "Pending", "ido_v3_end": "Active", "ido_v3_withdraw_platform_fee": "Successful"}[row.Action]
	}
	if err := checkOriginalWithIndependentRPC(ctx, row, binding, reader, independent); err != nil {
		return nil, err
	}
	if !execute {
		if _, err := chaintx.PreviewV3PersistedFirstBroadcast(ctx, database, binding, common.HexToAddress(row.SenderAddress), id, recheck); err != nil {
			return nil, invalid
		}
		return &OnChainSubmission{TxHash: expectedHash, Status: status, Submitted: false}, nil
	}
	hash, err := chaintx.BroadcastV3PersistedOnce(ctx, database, independent, binding, common.HexToAddress(row.SenderAddress), id, recheck)
	if err != nil {
		return nil, invalid
	}
	return &OnChainSubmission{TxHash: hash, Status: status, Submitted: true}, nil
}

func checkOriginalWithIndependentRPC(ctx context.Context, row entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding, reader, independent *ethclient.Client) error {
	// Identity must be independently configured by these two endpoint sources.
	if strings.TrimSpace(viper.GetString("idoV3TokenAdmission.primaryRPC")) == "https://bsc-dataseed1.bnbchain.org" {
		return errors.New("independent V3 RPC required")
	}
	return chaintx.CheckV3OriginalFirstBroadcast(ctx, row, b, reader, independent)
}
