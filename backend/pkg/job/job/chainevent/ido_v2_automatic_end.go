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

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
)

// A V2 end is permissionless on chain. Job uses its own signer and nonce lane;
// it never assumes the Core operator identity or consumes Core's outbox slot.
func reserveIDOV2End(projectID int, client *baseChain.Client, now time.Time) (*entity.ChainTxSubmission, error) {
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() || client.ChainID.Sign() <= 0 || client.Address == (common.Address{}) || !chain.V2Enabled() || client.ContractAddress != chain.V2ContractAddress() || projectID <= 0 {
		return nil, errors.New("IDO V2 end job is not admitted")
	}
	data, err := client.PackEndProject(big.NewInt(int64(projectID)))
	if err != nil {
		return nil, err
	}
	var submission *entity.ChainTxSubmission
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		submission, reserveErr = reserveIDOV2EndTx(tx, projectID, client, data, now)
		return reserveErr
	})
	if err != nil {
		return nil, err
	}
	return submission, err
}

// The reorg gate, project row lock and shared business-action slot are one
// transaction. Core's manual EndWithGuard uses the same slot with a different
// sender; its outstanding intent must never be replaced by Job's signer.
func reserveIDOV2EndTx(tx *gorm.DB, projectID int, client *baseChain.Client, data []byte, now time.Time) (*entity.ChainTxSubmission, error) {
	if tx == nil || client == nil || client.ChainID == nil || !client.ChainID.IsUint64() || client.ChainID.Sign() <= 0 || client.Address == (common.Address{}) || len(data) == 0 || projectID <= 0 {
		return nil, errors.New("IDO V2 end reservation identity is incomplete")
	}
	if err := chainreorg.LockNoActiveIncidentTx(tx, client.ChainID.Uint64(), client.ContractAddress.Hex()); err != nil {
		return nil, err
	}
	project := entity.Project{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
		return nil, err
	}
	if err := validateIDOV2EndCandidate(project, client.ChainID.Uint64(), client.ContractAddress, now); err != nil {
		return nil, err
	}
	submission, err := chaintx.CreateIntentTx(tx, chaintx.Intent{
		ProjectID: uint64(projectID), Action: constant.TransactionTypeProjectEnd,
		ChainID: client.ChainID.Uint64(), SenderAddress: client.Address,
		ContractAddress: common.HexToAddress(project.ContractAddress), CallData: data,
	})
	if err != nil {
		return nil, err
	}
	if err := chaintx.MarkReadyTx(tx, submission.ID); err != nil {
		return nil, err
	}
	// MarkReadyTx updates the database row, not this in-memory record. Do not
	// rewrite already signed/submitted/terminal states on a retry.
	if submission.State == constant.ChainTxStateIntent {
		submission.State = constant.ChainTxStateReady
	}
	return submission, nil
}

func validateIDOV2EndCandidate(project entity.Project, chainID uint64, contract common.Address, now time.Time) error {
	if chainID == 0 || contract == (common.Address{}) || now.IsZero() {
		return errors.New("IDO V2 end runtime identity is missing")
	}
	if project.Status != constant.ProjectStatusActive || !project.OnchainLocked || project.EndTime.After(now) {
		return errors.New("IDO V2 project is not due to end")
	}
	if err := constant.RequireExactV2ProjectIdentity(project.FundingMode, project.ContractVersion); err != nil {
		return err
	}
	if project.ChainID != chainID || !strings.EqualFold(project.ContractAddress, contract.Hex()) {
		return fmt.Errorf("IDO V2 end project chain identity mismatch")
	}
	return nil
}
