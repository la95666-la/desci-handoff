package chainevent

import (
	"errors"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

// fairLaunchEndCandidate uses the stored UTC instant rather than a database
// timezone conversion. The caller must pass the application clock explicitly.
func fairLaunchEndCandidate(project entity.Project, now time.Time, runtime projectruntime.Runtime) error {
	if !runtime.Enabled {
		return errors.New("fair launch runtime is disabled")
	}
	if project.ContractVersion != constant.ContractVersionFairLaunchV1 || project.Status != constant.ProjectStatusActive || project.EndTime.After(now) {
		return errors.New("project is not eligible for fair launch end")
	}
	if project.ChainID != runtime.ChainID || !strings.EqualFold(project.ContractAddress, runtime.ContractAddress) {
		return errors.New("fair launch project runtime identity mismatch")
	}
	return nil
}

// reserveFairLaunchEndTx creates an unsigned durable action. Signing and
// broadcasting remain in the existing outbox recovery loop and are gated by
// the configured remote signer implementation.
func reserveFairLaunchEndTx(tx *gorm.DB, projectID int, runtime projectruntime.Runtime, now time.Time) (*entity.ChainTxSubmission, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return nil, chaintx.ErrFairLaunchMaterializationUnavailable
	}
	project := entity.Project{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
		return nil, err
	}
	if err := fairLaunchEndCandidate(project, now, runtime); err != nil {
		return nil, err
	}
	data, err := baseChain.PackFairLaunchEnd(big.NewInt(int64(projectID)))
	if err != nil {
		return nil, err
	}
	intent, err := chaintx.FairLaunchIntent(runtime, uint64(projectID), constant.TransactionTypeFairLaunchEnd, data)
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
