package project

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

type v2CreateReservation struct {
	project      entity.Project
	times        []*big.Int
	percents     []*big.Int
	submissionID uint64
}

func createV2(projectID int64, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	trusted, err := trustedForV2Project(projectID)
	if err != nil {
		return nil, err
	}
	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 relayer client is not configured")
	}
	reservation, err := reserveV2Create(projectID, trusted, guard)
	if err != nil {
		return nil, err
	}
	if err := verifyProjectTokensForCreate(reservation.project.TokenAddress, reservation.project.TokenDecimals, trusted, client.Address); err != nil {
		if releaseErr := releaseV2CreateReservation(reservation.submissionID, projectID); releaseErr != nil {
			return nil, fmt.Errorf("V2 create preflight failed: %v; release reservation: %w", err, releaseErr)
		}
		return nil, fmt.Errorf("V2 create preflight: %w", err)
	}
	if err := db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.MarkReadyTx(tx, reservation.submissionID) }); err != nil {
		return nil, fmt.Errorf("mark V2 create intent ready: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	submission, sendErr := chaintx.Dispatch(ctx, db.Mysql(), client, reservation.submissionID)
	txHash := submissionHash(submission)
	if sendErr != nil {
		return nil, fmt.Errorf("V2 create durable dispatch incomplete; tx_hash=%s: %w", txHash, sendErr)
	}
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusSet, Submitted: true}, nil
}

func activateV2(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	trusted, err := trustedForV2Project(int64(projectID))
	if err != nil {
		return nil, err
	}
	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 relayer client is not configured")
	}
	project, recordID, err := reserveV2Activation(projectID, trusted, guard)
	if err != nil {
		return nil, err
	}
	if err := verifyProjectTokensForActivation(project.TokenAddress, project.TokenDecimals, trusted, client.Address); err != nil {
		if releaseErr := deleteEmptySubmission(recordID); releaseErr != nil {
			return nil, fmt.Errorf("V2 activation preflight failed: %v; release reservation: %w", err, releaseErr)
		}
		return nil, fmt.Errorf("V2 activation preflight: %w", err)
	}
	// Business preflight before any gas estimation: the IDO V2 proxy reverts
	// activateProject with InsufficientProjectTokens() when the deposited
	// project tokens are below the soft-cap-equivalent amount. Detecting this
	// before chaintx.Dispatch keeps the stable business error (with exact
	// current/required/shortfall raw values) from ever needing to parse RPC
	// internals, and releases the reserved intent instead of leaking it.
	if err := requireV2TokenSufficiency(project); err != nil {
		if releaseErr := deleteEmptySubmission(recordID); releaseErr != nil {
			return nil, fmt.Errorf("V2 activation token sufficiency failed: %v; release reservation: %w", err, releaseErr)
		}
		return nil, err
	}
	if err := db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.MarkReadyTx(tx, recordID) }); err != nil {
		return nil, fmt.Errorf("mark V2 activation intent ready: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	submission, sendErr := chaintx.Dispatch(ctx, db.Mysql(), client, recordID)
	txHash := submissionHash(submission)
	if sendErr != nil {
		return nil, fmt.Errorf("V2 activation durable dispatch incomplete; tx_hash=%s: %w", txHash, sendErr)
	}
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusPending, Submitted: true}, nil
}

func endV2(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	trusted, err := trustedForV2Project(int64(projectID))
	if err != nil {
		return nil, err
	}
	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 relayer client is not configured")
	}

	var recordID uint64
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, trusted.ChainID, trusted.Contract); err != nil {
			return err
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		var project entity.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
			return err
		}
		if project.Status != constant.ProjectStatusActive || !project.OnchainLocked {
			return fmt.Errorf("V2 end requires a locked Active project, got status=%s locked=%t", project.Status, project.OnchainLocked)
		}
		if err := constant.RequireExactV2ProjectIdentity(project.FundingMode, project.ContractVersion); err != nil {
			return err
		}
		if project.ChainID != trusted.ChainID || !strings.EqualFold(project.ContractAddress, trusted.Contract) {
			return errors.New("project V2 chain identity mismatch")
		}
		data, err := client.PackEndProject(new(big.Int).SetInt64(int64(projectID)))
		if err != nil {
			return err
		}
		intent, err := chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: uint64(projectID), Action: constant.TransactionTypeProjectEnd, ChainID: trusted.ChainID, SenderAddress: client.Address, ContractAddress: common.HexToAddress(trusted.Contract), CallData: data})
		if err != nil {
			return err
		}
		recordID = intent.ID
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("project not found")
	}
	if err != nil {
		return nil, err
	}
	if err := db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.MarkReadyTx(tx, recordID) }); err != nil {
		return nil, fmt.Errorf("mark V2 end intent ready: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	submission, sendErr := chaintx.Dispatch(ctx, db.Mysql(), client, recordID)
	txHash := submissionHash(submission)
	if sendErr != nil {
		return nil, fmt.Errorf("V2 end durable dispatch incomplete; tx_hash=%s: %w", txHash, sendErr)
	}
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusEnded, Submitted: true}, nil
}

func cancelV2(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	trusted, err := trustedForV2Project(int64(projectID))
	if err != nil {
		return nil, err
	}
	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 relayer client is not configured")
	}

	var recordID uint64
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, trusted.ChainID, trusted.Contract); err != nil {
			return err
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		var project entity.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
			return err
		}
		if project.Status != constant.ProjectStatusActive && project.Status != constant.ProjectStatusPending {
			return fmt.Errorf("V2 cancel requires a Pending or Active project, got status=%s", project.Status)
		}
		if err := constant.RequireExactV2ProjectIdentity(project.FundingMode, project.ContractVersion); err != nil {
			return err
		}
		if project.ChainID != trusted.ChainID || !strings.EqualFold(project.ContractAddress, trusted.Contract) {
			return errors.New("project V2 chain identity mismatch")
		}
		data, err := client.PackCancelProject(new(big.Int).SetInt64(int64(projectID)))
		if err != nil {
			return err
		}
		intent, err := chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: uint64(projectID), Action: constant.TransactionTypeProjectCancel, ChainID: trusted.ChainID, SenderAddress: client.Address, ContractAddress: common.HexToAddress(trusted.Contract), CallData: data})
		if err != nil {
			return err
		}
		recordID = intent.ID
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("project not found")
	}
	if err != nil {
		return nil, err
	}
	if err := db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.MarkReadyTx(tx, recordID) }); err != nil {
		return nil, fmt.Errorf("mark V2 cancel intent ready: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	submission, sendErr := chaintx.Dispatch(ctx, db.Mysql(), client, recordID)
	txHash := submissionHash(submission)
	if sendErr != nil {
		return nil, fmt.Errorf("V2 cancel durable dispatch incomplete; tx_hash=%s: %w", txHash, sendErr)
	}
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusCanceled, Submitted: true}, nil
}

// withdrawPlatformFeeV2 平台侧提取成功项目的平台费（合约 owner-only，收益人 feeReceiver）。
// 前置：项目已 Successful 且 platform_fee_withdrawn=0。
func withdrawPlatformFeeV2(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	trusted, err := trustedForV2Project(int64(projectID))
	if err != nil {
		return nil, err
	}
	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 relayer client is not configured")
	}

	var recordID uint64
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, trusted.ChainID, trusted.Contract); err != nil {
			return err
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		var project entity.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
			return err
		}
		if project.Status != constant.ProjectStatusSuccessful || project.PlatformFeeWithdrawn != 0 {
			return fmt.Errorf("platform fee withdraw requires a Successful project with fee unwithdrawn, got status=%s withdrawn=%d", project.Status, project.PlatformFeeWithdrawn)
		}
		if err := constant.RequireExactV2ProjectIdentity(project.FundingMode, project.ContractVersion); err != nil {
			return err
		}
		if project.ChainID != trusted.ChainID || !strings.EqualFold(project.ContractAddress, trusted.Contract) {
			return errors.New("project V2 chain identity mismatch")
		}
		data, err := client.PackWithdrawPlatformFee(new(big.Int).SetInt64(int64(projectID)))
		if err != nil {
			return err
		}
		intent, err := chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: uint64(projectID), Action: constant.TransactionTypePlatformFee, ChainID: trusted.ChainID, SenderAddress: client.Address, ContractAddress: common.HexToAddress(trusted.Contract), CallData: data})
		if err != nil {
			return err
		}
		recordID = intent.ID
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("project not found")
	}
	if err != nil {
		return nil, err
	}
	if err := db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.MarkReadyTx(tx, recordID) }); err != nil {
		return nil, fmt.Errorf("mark V2 platform fee intent ready: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	submission, sendErr := chaintx.Dispatch(ctx, db.Mysql(), client, recordID)
	txHash := submissionHash(submission)
	if sendErr != nil {
		return nil, fmt.Errorf("V2 platform fee durable dispatch incomplete; tx_hash=%s: %w", txHash, sendErr)
	}
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusSuccessful, Submitted: true}, nil
}

func trustedForV2Project(projectID int64) (trustedPaymentToken, error) {
	info := entity.Project{}
	if err := db.Mysql().Select("payment_token", "funding_mode", "contract_version").Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return trustedPaymentToken{}, errors.New("project not found")
		}
		return trustedPaymentToken{}, err
	}
	if err := constant.RequireExactV2ProjectIdentity(info.FundingMode, info.ContractVersion); err != nil {
		return trustedPaymentToken{}, err
	}
	return resolveTrustedPaymentToken(info.PaymentToken)
}

func reserveV2Create(projectID int64, trusted trustedPaymentToken, guard OnChainMutationGuard) (*v2CreateReservation, error) {
	reservation := &v2CreateReservation{}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, trusted.ChainID, trusted.Contract); err != nil {
			return err
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&reservation.project).Error; err != nil {
			return err
		}
		if reservation.project.Status != constant.ProjectStatusSet {
			return fmt.Errorf("project status must be %s, got %s", constant.ProjectStatusSet, reservation.project.Status)
		}
		if err := validateProjectV2IdentityFields(reservation.project, trusted); err != nil {
			return err
		}
		if reservation.project.OnchainLocked && (reservation.project.FeeReceiver == "" || !strings.EqualFold(reservation.project.FeeReceiver, trusted.FeeReceiver) || reservation.project.SuccessFeeRate != trusted.SuccessFeeRate) {
			return errors.New("locked V2 create does not match the approved fee snapshot")
		}
		tges := make([]entity.ProjectTge, 0, 10)
		if err := tx.Where("project_id = ?", projectID).Order("`index` asc").Find(&tges).Error; err != nil {
			return err
		}
		times, percents, err := validateStoredV2Create(reservation.project, tges, time.Now())
		if err != nil {
			return err
		}
		reservation.times, reservation.percents = times, percents
		client := chainManager.GetClient(constant.ChainProjectV2)
		if client == nil {
			return errors.New("IDO V2 relayer client is not configured")
		}
		wasLocked := reservation.project.OnchainLocked
		storedFeeReceiver := reservation.project.FeeReceiver
		storedFeeRate := reservation.project.SuccessFeeRate
		if wasLocked && (!strings.EqualFold(storedFeeReceiver, trusted.FeeReceiver) || storedFeeRate != trusted.SuccessFeeRate) {
			return errors.New("locked V2 create fee snapshot differs from approved configuration")
		}
		reservation.project.OnchainLocked = true
		reservation.project.FeeReceiver = trusted.FeeReceiver
		reservation.project.SuccessFeeRate = trusted.SuccessFeeRate
		params, err := createV2Params(reservation)
		if err != nil {
			return err
		}
		data, err := client.PackV2CreateProject(params)
		if err != nil {
			return err
		}
		intent, err := chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: uint64(projectID), Action: constant.TransactionTypeV2Create, ChainID: trusted.ChainID, SenderAddress: client.Address, ContractAddress: common.HexToAddress(trusted.Contract), CallData: data})
		if err != nil {
			return err
		}
		reservation.submissionID = intent.ID
		if !wasLocked {
			result := tx.Model(&entity.Project{}).Where("project_id = ? AND status = ? AND onchain_locked = ?", projectID, constant.ProjectStatusSet, false).Updates(map[string]interface{}{"onchain_locked": true, "fee_receiver": trusted.FeeReceiver, "success_fee_rate": trusted.SuccessFeeRate})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("project changed while reserving V2 create")
			}
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("project not found")
	}
	if err != nil {
		return nil, err
	}
	reservation.project.OnchainLocked = true
	reservation.project.FeeReceiver = trusted.FeeReceiver
	reservation.project.SuccessFeeRate = trusted.SuccessFeeRate
	return reservation, nil
}

func reserveV2Activation(projectID int, trusted trustedPaymentToken, guard OnChainMutationGuard) (entity.Project, uint64, error) {
	project := entity.Project{}
	var recordID uint64
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, trusted.ChainID, trusted.Contract); err != nil {
			return err
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
			return err
		}
		if project.Status != constant.ProjectStatusPending || !project.OnchainLocked {
			return fmt.Errorf("V2 activation requires a locked Pending project, got status=%s locked=%t", project.Status, project.OnchainLocked)
		}
		if err := constant.RequireExactV2ProjectIdentity(project.FundingMode, project.ContractVersion); err != nil {
			return err
		}
		if project.ChainID != trusted.ChainID || !strings.EqualFold(project.ContractAddress, trusted.Contract) {
			return errors.New("project V2 chain identity mismatch")
		}
		if project.PaymentToken != trusted.Symbol || !strings.EqualFold(project.PaymentTokenAddress, trusted.Address) || project.PaymentTokenDecimals != trusted.Decimals {
			return errors.New("project V2 payment token identity mismatch")
		}
		if err := validateDepositedProjectTokens(project); err != nil {
			return err
		}
		client := chainManager.GetClient(constant.ChainProjectV2)
		if client == nil {
			return errors.New("IDO V2 relayer client is not configured")
		}
		data, err := client.PackV2ActivateProject(new(big.Int).SetInt64(int64(projectID)))
		if err != nil {
			return err
		}
		intent, err := chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: uint64(projectID), Action: constant.TransactionTypeV2Activate, ChainID: trusted.ChainID, SenderAddress: client.Address, ContractAddress: common.HexToAddress(trusted.Contract), CallData: data})
		if err != nil {
			return err
		}
		recordID = intent.ID
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Project{}, 0, errors.New("project not found")
	}
	return project, recordID, err
}

func validateStoredV2Create(project entity.Project, tges []entity.ProjectTge, now time.Time) ([]*big.Int, []*big.Int, error) {
	values := []struct {
		name, raw string
		display   decimal.Decimal
	}{
		{"token price", project.TokenPriceRaw, project.TokenPrice}, {"hard cap", project.HardCapRaw, project.HardCap}, {"soft cap", project.SoftCapRaw, project.SoftCap}, {"max per user", project.MaxPerUserRaw, project.MaxPerUser},
	}
	rawInts := make(map[string]*big.Int, len(values))
	for _, value := range values {
		canonical, err := money.NormalizeRawUint256(value.raw)
		if err != nil {
			return nil, nil, fmt.Errorf("%s raw: %w", value.name, err)
		}
		parsed, _ := new(big.Int).SetString(canonical, 10)
		if parsed.Sign() <= 0 {
			return nil, nil, fmt.Errorf("%s raw must be positive", value.name)
		}
		display, err := money.FormatRaw(canonical, uint8(project.PaymentTokenDecimals))
		if err != nil {
			return nil, nil, err
		}
		if !value.display.Equal(decimal.RequireFromString(display)) {
			return nil, nil, fmt.Errorf("%s raw/display mismatch", value.name)
		}
		rawInts[value.name] = parsed
	}
	if rawInts["soft cap"].Cmp(rawInts["hard cap"]) > 0 || rawInts["max per user"].Cmp(rawInts["hard cap"]) > 0 {
		return nil, nil, errors.New("project cap invariants are invalid")
	}
	if !project.StartTime.After(now) || !project.EndTime.After(project.StartTime) {
		return nil, nil, errors.New("project sale times are no longer valid for create")
	}
	if len(tges) == 0 || len(tges) > 10 {
		return nil, nil, errors.New("vesting schedule must contain 1 to 10 periods")
	}
	times := make([]*big.Int, 0, len(tges))
	percents := make([]*big.Int, 0, len(tges))
	total := big.NewInt(0)
	previous := int64(0)
	for i, tge := range tges {
		if tge.Index != i+1 {
			return nil, nil, errors.New("vesting indexes must be contiguous from 1")
		}
		unix := tge.VestingTime.Unix()
		if unix <= project.EndTime.Unix() || (i > 0 && unix <= previous) {
			return nil, nil, errors.New("vesting times must be after sale end and strictly ascending")
		}
		basis := tge.VestingPercent.Shift(2)
		if !basis.IsInteger() || !basis.IsPositive() {
			return nil, nil, fmt.Errorf("vesting percent at index %d is invalid", i)
		}
		basisInt := basis.BigInt()
		total.Add(total, basisInt)
		times = append(times, big.NewInt(unix))
		percents = append(percents, basisInt)
		previous = unix
	}
	if total.Cmp(big.NewInt(10_000)) != 0 {
		return nil, nil, errors.New("total vesting percent must equal 10000 basis points")
	}
	return times, percents, nil
}

func validateDepositedProjectTokens(project entity.Project) error {
	canonical, err := money.NormalizeRawUint256(project.TotalDepositedRaw)
	if err != nil {
		return fmt.Errorf("total deposited raw: %w", err)
	}
	value, _ := new(big.Int).SetString(canonical, 10)
	if value.Sign() <= 0 {
		return errors.New("V2 activation requires deposited project tokens")
	}
	display, err := money.FormatRaw(canonical, uint8(project.TokenDecimals))
	if err != nil {
		return err
	}
	parsed, err := decimal.NewFromString(display)
	if err != nil || !project.TotalDeposited.Equal(parsed) {
		return errors.New("total deposited raw/display mismatch")
	}
	return nil
}

func createV2Params(reservation *v2CreateReservation) (baseChain.CreateProjectV2Params, error) {
	p := reservation.project
	parse := func(raw string) (*big.Int, error) {
		canonical, err := money.NormalizeRawUint256(raw)
		if err != nil {
			return nil, err
		}
		value, ok := new(big.Int).SetString(canonical, 10)
		if !ok {
			return nil, errors.New("invalid raw uint256")
		}
		return value, nil
	}
	tokenPrice, err := parse(p.TokenPriceRaw)
	if err != nil {
		return baseChain.CreateProjectV2Params{}, err
	}
	hardCap, err := parse(p.HardCapRaw)
	if err != nil {
		return baseChain.CreateProjectV2Params{}, err
	}
	softCap, err := parse(p.SoftCapRaw)
	if err != nil {
		return baseChain.CreateProjectV2Params{}, err
	}
	maxPerUser, err := parse(p.MaxPerUserRaw)
	if err != nil {
		return baseChain.CreateProjectV2Params{}, err
	}
	return baseChain.CreateProjectV2Params{ProjectID: new(big.Int).SetInt64(int64(p.ProjectID)), ProjectToken: common.HexToAddress(p.TokenAddress), PaymentToken: common.HexToAddress(p.PaymentTokenAddress), ProjectOwner: common.HexToAddress(p.OwnerAddress), ExpectedFeeReceiver: common.HexToAddress(p.FeeReceiver), ExpectedSuccessFeeRate: p.SuccessFeeRate, TokenPrice: tokenPrice, HardCap: hardCap, SoftCap: softCap, MaxPerUser: maxPerUser, StartTime: big.NewInt(p.StartTime.Unix()), EndTime: big.NewInt(p.EndTime.Unix()), VestingTimes: reservation.times, VestingPercents: reservation.percents}, nil
}

func releaseV2CreateReservation(recordID uint64, projectID int64) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chaintx.CancelIntentTx(tx, recordID); err != nil {
			return err
		}
		unlock := tx.Model(&entity.Project{}).Where("project_id = ? AND status = ? AND onchain_locked = ?", projectID, constant.ProjectStatusSet, true).Updates(map[string]interface{}{"onchain_locked": false, "fee_receiver": "", "success_fee_rate": 0})
		if unlock.Error != nil {
			return unlock.Error
		}
		if unlock.RowsAffected != 1 {
			return errors.New("project create lock is no longer releasable")
		}
		return nil
	})
}

func deleteEmptySubmission(recordID uint64) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.CancelIntentTx(tx, recordID) })
}

func submissionHash(submission *entity.ChainTxSubmission) string {
	if submission == nil || submission.TxHash == nil {
		return ""
	}
	return *submission.TxHash
}
