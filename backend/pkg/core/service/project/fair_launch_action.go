package project

import (
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

// PrepareFairLaunchOperatorIntentTx performs strict version/runtime routing and
// stores an unsigned durable intent. It never signs or broadcasts.
func PrepareFairLaunchOperatorIntentTx(tx *gorm.DB, projectID int64, action string, reasonCode, evidenceHash common.Hash) (*entity.ChainTxSubmission, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return nil, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if projectID <= 0 {
		return nil, errors.New("project id must be positive")
	}
	runtime, err := projectruntime.FairLaunchV1()
	if err != nil {
		return nil, err
	}
	project := entity.Project{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
		return nil, err
	}
	if project.ContractVersion != constant.ContractVersionFairLaunchV1 || project.ChainID != runtime.ChainID || !strings.EqualFold(project.ContractAddress, runtime.ContractAddress) {
		return nil, errors.New("project is not bound to the trusted fair launch runtime")
	}
	var data []byte
	switch action {
	case constant.TransactionTypeFairLaunchActivate:
		if project.Status != constant.ProjectStatusPending {
			return nil, errors.New("fair launch activation requires Pending status")
		}
		data, err = baseChain.PackFairLaunchActivate(big.NewInt(projectID))
	case constant.TransactionTypeFairLaunchEnd:
		if project.Status != constant.ProjectStatusActive {
			return nil, errors.New("fair launch end requires Active status")
		}
		data, err = baseChain.PackFairLaunchEnd(big.NewInt(projectID))
	case constant.TransactionTypeFairLaunchCancel:
		if project.Status != constant.ProjectStatusPending && project.Status != constant.ProjectStatusActive {
			return nil, errors.New("fair launch cancellation requires Pending or Active status")
		}
		if reasonCode == (common.Hash{}) || evidenceHash == (common.Hash{}) {
			return nil, errors.New("fair launch cancellation requires reason and evidence hashes")
		}
		data, err = baseChain.PackFairLaunchCancel(big.NewInt(projectID), reasonCode, evidenceHash)
	default:
		return nil, errors.New("unsupported fair launch Operator action")
	}
	if err != nil {
		return nil, err
	}
	intent, err := chaintx.FairLaunchIntent(runtime, uint64(projectID), action, data)
	if err != nil {
		return nil, err
	}
	submission, err := chaintx.CreateIntentTx(tx, intent)
	if err != nil {
		return nil, err
	}
	if err := chaintx.MarkReadyTx(tx, submission.ID); err != nil {
		return nil, err
	}
	return submission, nil
}
