package taskhall

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/points"
	"joincare_server/pkg/core/repository/user"
	"joincare_server/pkg/core/service/notification"
	thirdChain "joincare_server/pkg/core/third/chain"
	"joincare_server/pkg/core/third/twitter"
)

// PointFlows returns only the signed-in user's positive point records. The
// dashboard uses this as an activity feed; deductions remain available to
// administrators but are not mixed into the acquisition history.
func PointFlows(uid, page, pageSize int) (*PointFlowListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}

	query := db.Mysql().Table("user_point_flow").
		Where("uid = ? AND change_point > 0", uid)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]PointFlowItem, 0)
	if err := query.
		Select("id, change_point, remain, type, `desc`, created_at").
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error; err != nil {
		return nil, err
	}

	return &PointFlowListResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPoint: points.GetUserPoints(uid),
	}, nil
}

// CheckIn 采用 "用户自付 gas" 模型：
//  1. 用户在前端直接调合约 checkIn()，钱包发交易付 gas
//  2. 拿到 txHash 后 POST /client/taskhall/v1/checkIn { txHash }
//  3. 后端拉 receipt，验证 status=1、UserCheckedIn event 存在且 user 匹配用户钱包
//  4. 从 event.day 反推 checkInDate（UTC）→ DB 事务里落记录 + 积分 + 首签邀请
//
// V1 (relayer 版) 的 CheckInNonce/EIP-712 流程已下线；仅保留兼容 stub 返回错误。
func CheckIn(uid int, txHash string) (*CheckInResponse, error) {
	txHash = strings.ToLower(strings.TrimSpace(txHash))
	if !strings.HasPrefix(txHash, "0x") || len(txHash) != 66 {
		return nil, ret.CusError(i18n.ERR_000_00003)
	}

	userInfo := user.Info(uid)
	if userInfo.ID == 0 || strings.TrimSpace(userInfo.WalletAddress) == "" {
		return nil, ret.CusError(i18n.ERR_001_00007)
	}
	walletHex := common.HexToAddress(userInfo.WalletAddress).Hex()

	chainClient := thirdChain.GetClient(constant.ChainTask)
	if chainClient == nil {
		return nil, ret.CusError(i18n.ERR_001_00011)
	}

	// 幂等：DB 里 tx_hash 已落 → 直接返回既有结果，不再重复处理链上事件
	existing, err := findRecordByTxHash(txHash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.UID != uid {
			// 别人的 tx 不能借用
			return nil, ret.CusError(i18n.ERR_001_00010)
		}
		return buildResponseFromRecord(existing), nil
	}

	// 拉 receipt（最多 30s，处理 mempool → mined 延迟；hardhat 上通常 <2s）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	receipt, err := waitForReceipt(ctx, chainClient, common.HexToHash(txHash))
	if err != nil {
		log.GetLogger().Warnf("check in receipt lookup failed uid=%d tx=%s err=%s", uid, txHash, err)
		return nil, ret.CusError(i18n.ERR_001_00010)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, ret.CusError(i18n.ERR_001_00010)
	}

	// 从 receipt.Logs 中找 UserCheckedIn 事件（合约地址 + topic 匹配）
	event := parseCheckInEvent(chainClient, receipt.Logs)
	if event == nil {
		return nil, ret.CusError(i18n.ERR_001_00010)
	}

	// 校验 event.user 必须等于当前用户绑定的钱包
	if !strings.EqualFold(event.User.Hex(), walletHex) {
		log.GetLogger().Warnf("check in wallet mismatch uid=%d tx=%s event.user=%s expect=%s", uid, txHash, event.User.Hex(), walletHex)
		return nil, ret.CusError(i18n.ERR_001_00010)
	}

	// event.day = 从 1970-01-01 UTC 开始的天数
	checkInDate := time.Unix(event.Day*86400, 0).UTC().Format("2006-01-02")

	// 提前拦重签（DB uk_uid_date 是最终兜底）
	checked, err := hasCheckedIn(uid, checkInDate)
	if err != nil {
		return nil, err
	}
	if checked {
		return nil, ret.CusError(i18n.ERR_001_00009)
	}

	input := checkInInput{
		UID:           uid,
		WalletAddress: walletHex,
		CheckInDate:   checkInDate,
		Day:           event.Day,
		TxHash:        txHash,
	}
	return settleVerifiedCheckIn(uid, input)
}

func settleVerifiedCheckIn(uid int, input checkInInput) (*CheckInResponse, error) {
	_, totalPoint, totalCheckInDays, continuousCheckInDays, awardedPoint, err := createSuccessfulCheckInRecord(uid, input)
	if err != nil {
		return nil, err
	}

	notification.Push(uid, 0, constant.CheckInSuccessful, map[string][]string{
		constant.NoticeLangEn: {"Daily Check-in Successful", fmt.Sprintf("You earned %d points (streak %d days).", awardedPoint, continuousCheckInDays)},
		constant.NoticeLangTw: {"每日簽到成功", fmt.Sprintf("獲得 %d 點積分（連續 %d 天）。", awardedPoint, continuousCheckInDays)},
		constant.NoticeLangCn: {"每日签到成功", fmt.Sprintf("获得 %d 点积分（连续 %d 天）。", awardedPoint, continuousCheckInDays)},
	})

	return &CheckInResponse{
		TxHash:                input.TxHash,
		Point:                 awardedPoint,
		TotalPoint:            totalPoint,
		CheckInDate:           input.CheckInDate,
		ChainStatus:           "confirmed",
		TotalCheckInDays:      totalCheckInDays,
		ContinuousCheckInDays: continuousCheckInDays,
	}, nil
}

const checkInRecoveryBlockBatch uint64 = 5000

// ReconcileCheckIn repairs a confirmed on-chain check-in when the browser did
// not finish POSTing its transaction hash. It only searches today's event for
// the authenticated user's bound wallet; clients cannot nominate another user.
func ReconcileCheckIn(uid int) (*CheckInResponse, error) {
	userInfo := user.Info(uid)
	if userInfo.ID == 0 || !common.IsHexAddress(strings.TrimSpace(userInfo.WalletAddress)) {
		return nil, ret.CusError(i18n.ERR_001_00007)
	}
	wallet := common.HexToAddress(userInfo.WalletAddress)

	checkInDate, _ := checkInDateAndDay(time.Now())
	existing, err := findRecordByUIDDate(uid, checkInDate)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return buildResponseFromRecord(existing), nil
	}
	walletRecord, err := findRecordByWalletDate(wallet.Hex(), checkInDate)
	if err != nil {
		return nil, err
	}
	if walletRecord != nil {
		if walletRecord.UID == uid {
			return buildResponseFromRecord(walletRecord), nil
		}
		return nil, ret.CusError(i18n.ERR_001_00010)
	}

	chainClient := thirdChain.GetClient(constant.ChainTask)
	if chainClient == nil {
		return nil, ret.CusError(i18n.ERR_001_00011)
	}

	utcNow := time.Now().UTC()
	day := utcNow.Unix() / 86400
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	checkedOnChain, err := chainClient.HasCheckedInToday(ctx, wallet)
	if err != nil {
		log.GetLogger().Warnf("check in reconcile state read failed uid=%d wallet=%s err=%s", uid, userInfo.WalletAddress, err)
		return nil, ret.CusError(i18n.ERR_001_00012)
	}
	if !checkedOnChain {
		return nil, ret.CusError(i18n.ERR_001_00012)
	}

	txHash, scanErr := findCurrentDayCheckInTx(ctx, chainClient, wallet, day)
	if scanErr != nil {
		log.GetLogger().Warnf("check in reconcile scan failed, using verified state uid=%d wallet=%s err=%s", uid, userInfo.WalletAddress, scanErr)
	}
	if txHash != "" {
		res, checkInErr := CheckIn(uid, txHash)
		if checkInErr == nil {
			log.GetLogger().Infof("check in reconciled from event uid=%d wallet=%s tx=%s", uid, userInfo.WalletAddress, txHash)
		}
		return res, checkInErr
	}

	// Some BSC RPC providers disable eth_getLogs. The contract view still gives
	// authoritative proof for the bound wallet, so use a deterministic recovery
	// identifier while preserving the same DB uniqueness and scoring transaction.
	recoveryID := crypto.Keccak256Hash([]byte(fmt.Sprintf(
		"joincare-checkin-recovery:%s:%s:%d",
		chainClient.ChainID.String(), strings.ToLower(wallet.Hex()), day,
	))).Hex()
	input := checkInInput{
		UID:           uid,
		WalletAddress: wallet.Hex(),
		CheckInDate:   checkInDate,
		Day:           day,
		TxHash:        recoveryID,
	}
	res, settleErr := settleVerifiedCheckIn(uid, input)
	if settleErr == nil {
		log.GetLogger().Infof("check in reconciled from contract state uid=%d wallet=%s recovery_id=%s", uid, userInfo.WalletAddress, recoveryID)
	}
	return res, settleErr
}

func findCurrentDayCheckInTx(ctx context.Context, client *commonChain.Client, wallet common.Address, day int64) (string, error) {
	latest, err := client.Client.BlockNumber(ctx)
	if err != nil {
		return "", err
	}

	eventTopic := commonChain.EventTopic(commonChain.UserCheckedIn)
	userTopic := common.BytesToHash(wallet.Bytes())
	dayTopic := common.BigToHash(big.NewInt(day))
	dayStartUnix := day * 86400

	toBlock := latest
	for {
		fromBlock := uint64(0)
		if toBlock+1 > checkInRecoveryBlockBatch {
			fromBlock = toBlock - checkInRecoveryBlockBatch + 1
		}

		logs, filterErr := client.Client.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(fromBlock),
			ToBlock:   new(big.Int).SetUint64(toBlock),
			Addresses: []common.Address{client.ContractAddress},
			Topics: [][]common.Hash{
				{eventTopic},
				{userTopic},
				{dayTopic},
			},
		})
		if filterErr != nil {
			return "", filterErr
		}

		for i := len(logs) - 1; i >= 0; i-- {
			event := client.ProcessLog(logs[i])
			if event != nil && event.Event == commonChain.UserCheckedIn &&
				strings.EqualFold(event.User.Hex(), wallet.Hex()) && event.Day == day {
				return event.TxHash, nil
			}
		}

		if fromBlock == 0 {
			return "", nil
		}
		header, headerErr := client.Client.HeaderByNumber(ctx, new(big.Int).SetUint64(fromBlock))
		if headerErr != nil {
			return "", headerErr
		}
		if int64(header.Time) < dayStartUnix {
			return "", nil
		}
		toBlock = fromBlock - 1
	}
}

// waitForReceipt 等到 mempool 中 tx 被打包后返回 receipt；每秒 poll 一次。
func waitForReceipt(ctx context.Context, c *commonChain.Client, hash common.Hash) (*types.Receipt, error) {
	for {
		receipt, err := c.Client.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		if err != ethereum.NotFound {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// parseCheckInEvent 在 receipt.Logs 里定位 UserCheckedIn 事件（合约地址 + topic 匹配）
func parseCheckInEvent(client *commonChain.Client, logs []*types.Log) *commonChain.Data {
	topic := commonChain.EventTopic(commonChain.UserCheckedIn)
	for _, vLog := range logs {
		if vLog == nil {
			continue
		}
		if !strings.EqualFold(vLog.Address.Hex(), client.ContractAddress.Hex()) {
			continue
		}
		if len(vLog.Topics) == 0 || vLog.Topics[0] != topic {
			continue
		}
		data := client.ProcessLog(*vLog)
		if data != nil && data.Event == commonChain.UserCheckedIn {
			return data
		}
	}
	return nil
}

func findRecordByTxHash(txHash string) (*entity.UserCheckInRecord, error) {
	if txHash == "" {
		return nil, nil
	}
	var rec entity.UserCheckInRecord
	err := db.Mysql().Where("tx_hash = ?", txHash).Limit(1).Find(&rec).Error
	if err != nil {
		return nil, err
	}
	if rec.ID == 0 {
		return nil, nil
	}
	return &rec, nil
}

func findRecordByUIDDate(uid int, checkInDate string) (*entity.UserCheckInRecord, error) {
	var rec entity.UserCheckInRecord
	err := db.Mysql().Where("uid = ? AND check_in_date = ?", uid, checkInDate).Limit(1).Find(&rec).Error
	if err != nil {
		return nil, err
	}
	if rec.ID == 0 {
		return nil, nil
	}
	return &rec, nil
}

func findRecordByWalletDate(walletAddress, checkInDate string) (*entity.UserCheckInRecord, error) {
	var rec entity.UserCheckInRecord
	err := db.Mysql().Where("LOWER(wallet_address) = ? AND check_in_date = ?", strings.ToLower(walletAddress), checkInDate).
		Limit(1).Find(&rec).Error
	if err != nil {
		return nil, err
	}
	if rec.ID == 0 {
		return nil, nil
	}
	return &rec, nil
}

func buildResponseFromRecord(rec *entity.UserCheckInRecord) *CheckInResponse {
	return &CheckInResponse{
		TxHash:                rec.TxHash,
		Point:                 rec.Point,
		TotalPoint:            points.GetUserPoints(rec.UID),
		CheckInDate:           rec.CheckInDate,
		ChainStatus:           "confirmed",
		TotalCheckInDays:      rec.TotalCheckInDays,
		ContinuousCheckInDays: rec.ContinuousCheckInDays,
	}
}

// checkInInput 承载写 user_check_in_record 所需的入参
type checkInInput struct {
	UID           int
	WalletAddress string
	CheckInDate   string
	Day           int64
	TxHash        string
}

// CheckInNonce 已废弃：V2 用户自付 gas 模型不再需要后端下发 nonce。
// 保留 stub 让前端旧版本降级到明确错误提示，而不是 404。
func CheckInNonce(uid int) (*CheckInNonceResponse, error) {
	_ = uid
	return nil, ret.CusError(i18n.ERR_001_00013)
}

// createSuccessfulCheckInRecord 落地一条签到记录（tx_hash 已在上层验证）+ 结算积分 + 首签邀请奖励。
// 只在 totalCheckInDays == 1（首次签到）时结算邀请奖励。
// 返回：recordID, 总积分, 总签到天数, 连续签到天数, 本轮奖励
func createSuccessfulCheckInRecord(uid int, input checkInInput) (int, int, int, int, int, error) {
	now := time.Now()
	recordID := 0
	totalPoint := 0
	totalCheckInDays := 0
	continuousCheckInDays := 0
	awardedThisRound := 0

	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		checked, err := hasCheckedInTx(tx, uid, input.CheckInDate)
		if err != nil {
			return err
		}
		if checked {
			return ret.CusError(i18n.ERR_001_00009)
		}

		totalCheckInDays, continuousCheckInDays, err = checkInStats(tx, uid, input.CheckInDate)
		if err != nil {
			return err
		}

		// 取 streak 当日积分：按 day_threshold <= streak ORDER BY day_threshold DESC LIMIT 1
		// 若无规则匹配 / 表为空，回退到 constant.TaskCheckInPoint
		awardPoint := constant.TaskCheckInPoint
		var rule entity.CheckInRule
		ruleErr := tx.Where("is_enable = 1 AND day_threshold <= ?", continuousCheckInDays).
			Order("day_threshold DESC").
			First(&rule).Error
		if ruleErr == nil {
			awardPoint = rule.BonusPoint
		}
		awardedThisRound = awardPoint

		checkInRecord := entity.UserCheckInRecord{
			UID:                   uid,
			WalletAddress:         input.WalletAddress,
			CheckInDate:           input.CheckInDate,
			CheckInDay:            input.Day,
			CheckInAt:             now,
			Point:                 awardPoint,
			TotalCheckInDays:      totalCheckInDays,
			ContinuousCheckInDays: continuousCheckInDays,
			Nonce:                 "",           // V2 无 nonce
			Deadline:              0,            // V2 无 deadline
			Signature:             "",           // V2 无 sig（用户自签自付）
			TxHash:                input.TxHash, // V2 由前端提交
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		if err = tx.Create(&checkInRecord).Error; err != nil {
			return err
		}
		recordID = checkInRecord.ID

		rollback := true
		rollback, err = points.ChangePoints(uid, awardPoint, constant.PointFlowTypeCheckIn, "Daily check-in", *tx)
		if err != nil {
			points.RollbackChangePoints(uid, awardPoint, constant.PointFlowTypeCheckIn, rollback, false)
			return err
		}
		// 首次签到才结算邀请奖励；同时 settleInviteReward 内部用抢占式 UPDATE 防并发 double-award
		if totalCheckInDays == 1 {
			if err = settleInviteReward(tx, uid); err != nil {
				points.RollbackChangePoints(uid, awardPoint, constant.PointFlowTypeCheckIn, rollback, false)
				return err
			}
		}
		totalPoint, err = points.GetUserPointsTx(tx, uid)
		return err
	})
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return recordID, totalPoint, totalCheckInDays, continuousCheckInDays, awardedThisRound, nil
}

// settleInviteReward 邀请奖励结算：抢占式 UPDATE receive_amount 0→1，仅在成功抢占（RowsAffected==1）后才 ChangePoints。
// 避免并发下多个 tx 同时读到 receive_amount=0 造成邀请人被 double-award。
func settleInviteReward(tx *gorm.DB, inviteeUID int) error {
	inviteInfo := entity.InviteRecord{}
	err := tx.Model(&entity.InviteRecord{}).
		Select("id,inviter_uid").
		Where("uid = ? and receive_amount = 0", inviteeUID).
		Limit(1).
		Scan(&inviteInfo).Error
	if err != nil {
		return err
	}
	if inviteInfo.ID == 0 || inviteInfo.InviterUID == 0 {
		return nil
	}

	// 先抢占：UPDATE ... WHERE receive_amount=0；只有 RowsAffected==1 才是本 tx 结算，才继续 ChangePoints
	res := tx.Model(&entity.InviteRecord{}).
		Where("id = ? and receive_amount = 0", inviteInfo.ID).
		Update("receive_amount", 1)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return nil
	}

	rollback := true
	rollback, err = points.ChangePoints(inviteInfo.InviterUID, constant.TaskInvitePoint, constant.PointFlowTypeInvite, "Invite user", *tx)
	if err != nil {
		points.RollbackChangePoints(inviteInfo.InviterUID, constant.TaskInvitePoint, constant.PointFlowTypeInvite, rollback, false)
		return err
	}
	return nil
}

func checkInStats(tx *gorm.DB, uid int, checkInDate string) (int, int, error) {
	var previousTotal int64
	err := tx.Model(&entity.UserCheckInRecord{}).
		Where("uid = ?", uid).
		Count(&previousTotal).Error
	if err != nil {
		return 0, 0, err
	}

	continuousDays := 1
	// checkInDate 一律 UTC (跟合约 block.timestamp / 86400 对齐)
	currentDate, err := time.ParseInLocation("2006-01-02", checkInDate, time.UTC)
	if err != nil {
		return 0, 0, err
	}
	yesterday := currentDate.AddDate(0, 0, -1).Format("2006-01-02")
	previous := entity.UserCheckInRecord{}
	err = tx.Model(&entity.UserCheckInRecord{}).
		Select("continuous_check_in_days").
		Where("uid = ? and check_in_date = ?", uid, yesterday).
		Limit(1).
		Scan(&previous).Error
	if err != nil {
		return 0, 0, err
	}
	if previous.ContinuousCheckInDays > 0 {
		continuousDays = previous.ContinuousCheckInDays + 1
	}

	return int(previousTotal) + 1, continuousDays, nil
}

func hasCheckedIn(uid int, checkInDate string) (bool, error) {
	return hasCheckedInTx(db.Mysql(), uid, checkInDate)
}

func hasCheckedInTx(tx *gorm.DB, uid int, checkInDate string) (bool, error) {
	info := entity.UserCheckInRecord{}
	err := tx.Model(&entity.UserCheckInRecord{}).
		Select("id").
		Where("uid = ? and check_in_date = ?", uid, checkInDate).
		Limit(1).
		Scan(&info).Error
	if err != nil {
		return false, err
	}
	return info.ID != 0, nil
}

func checkInDateAndDay(t time.Time) (string, int64) {
	// 必须 UTC：合约里 day = block.timestamp / 86400 也是 UTC 天数，
	// 用本地时区判"今日已签"会跟合约错开一天，导致按钮显示"可签"但合约 revert。
	utc := t.In(time.UTC)
	day, _ := strconv.ParseInt(utc.Format("20060102"), 10, 64)
	return utc.Format("2006-01-02"), day
}

// CompleteTask is the legacy non-versioned task path retained for compatibility.
//   - platform: twitter / telegram（默认由 handler 传 twitter 兼容旧客户端）
//   - taskKey 为空：取该 platform 下第一个启用任务（老逻辑）
//   - taskKey 非空：精确匹配 task_key + platform + is_enable=1
//   - replyText is ignored and retained only for old clients.
func CompleteTask(uid int, platform, taskKey, _ string) error {
	if platform == "" {
		platform = "twitter"
	}
	// twitter token / twitter_id 只在 platform=twitter 时拿；telegram 任务不需要 X 绑定
	token := &twitter.Token{}
	userInfo := entity.User{}
	if platform == "twitter" {
		err := db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, uid), token)
		if err != nil {
			return ret.CusError(i18n.ERR_001_00015)
		}
		if !token.SupportsReadOnlyVerification() {
			return ret.CusError(i18n.ERR_001_00015)
		}
		refresh, err := token.CheckExpireTime()
		if err != nil {
			return ret.CusError(i18n.ERR_001_00015)
		}
		if refresh {
			db.Redis().SetByJson(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, uid), token, 20*86400*time.Second)
		}
		err = db.Mysql().Model(&entity.User{}).
			Select("twitter_id").Where("id = ?", uid).First(&userInfo).Error
		if err != nil {
			return err
		}
	}

	// 获取任务信息：taskKey 非空时精确匹配，为空回退到 first enabled
	taskInfo := entity.Task{}
	q := db.Mysql().Model(&entity.Task{}).
		Select("id,platform,task_key,verify_kind,url,amount_point,is_enable").
		Where("is_enable = 1 AND platform = ?", platform)
	if taskKey != "" {
		q = q.Where("task_key = ?", taskKey)
	}
	err := q.Order("sort ASC, id ASC").First(&taskInfo).Error
	if err != nil || taskInfo.ID == 0 {
		return err
	}
	// 获取是否完成过该任务
	info := entity.TaskRecord{}
	db.Mysql().Model(&entity.TaskRecord{}).
		Select("id").
		Where("uid = ? and task_id = ?", uid, taskInfo.ID).Scan(&info)
	if info.ID != 0 {
		return ret.CusError(i18n.ERR_001_00016)
	}
	// Twitter 校验（严格模式）：只读检查用户是否已手工完成 X 动作。
	//   - twitter_pass: 只读 API 检测到动作 → 加分
	//   - twitter_err : API 异常（速率限制、网络抖动、权限缺失等）→ 立即返回错误，不加分
	//   - manual_pass : 非 twitter_api verify_kind（如 telegram_bot），跳过 API 调用直接过
	verifyStatus := "manual_pass"
	if taskInfo.VerifyKind == "twitter_api" && taskInfo.URL != "" && userInfo.TwitterID != "" {
		var apiErr error
		switch strings.ToLower(strings.TrimSpace(taskInfo.TaskKey)) {
		case "follow":
			apiErr = token.VerifyFollowing(userInfo.TwitterID, taskInfo.URL)
		case "like":
			apiErr = token.VerifyLike(userInfo.TwitterID, taskInfo.URL)
		case "retweet":
			apiErr = token.VerifyRetweet(userInfo.TwitterID, taskInfo.URL)
		case "reply":
			apiErr = token.VerifyReply(userInfo.TwitterID, taskInfo.URL)
		default:
			apiErr = ret.CusError(i18n.ERR_001_00020)
		}
		var notCompleted *twitter.TaskNotCompletedError
		if errors.As(apiErr, &notCompleted) {
			apiErr = ret.CusError(i18n.ERR_001_00019)
		}
		if apiErr == nil {
			verifyStatus = "twitter_pass"
		} else {
			// 严格模式：API 失败直接拒发分
			log.GetLogger().Warnf("twitter task verify err uid=%d task=%d key=%s err=%v",
				uid, taskInfo.ID, taskInfo.TaskKey, apiErr)
			return apiErr
		}
	}

	tx := db.Mysql().Begin()
	insert := &entity.TaskRecord{
		TaskID:       int(taskInfo.ID),
		UID:          uid,
		AmountPoint:  taskInfo.AmountPoint,
		VerifyStatus: verifyStatus,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = tx.Create(insert).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	rollback := true
	rollback, err = points.ChangePoints(uid, taskInfo.AmountPoint, constant.PointFlowTypeTask, "Complete task", *tx)
	if err != nil {
		points.RollbackChangePoints(uid, taskInfo.AmountPoint, constant.PointFlowTypeTask, rollback, false)
		tx.Rollback()
		return err
	}
	tx.Commit()

	// Phase 1: 任务完成 → 站内信
	notification.Push(uid, int(taskInfo.ID), constant.TaskCompleted, map[string][]string{
		constant.NoticeLangEn: {"Task Completed", fmt.Sprintf("You earned %d points from task #%d.", taskInfo.AmountPoint, taskInfo.ID)},
		constant.NoticeLangTw: {"任務完成", fmt.Sprintf("完成任務 #%d，獲得 %d 點積分。", taskInfo.ID, taskInfo.AmountPoint)},
		constant.NoticeLangCn: {"任务完成", fmt.Sprintf("完成任务 #%d，获得 %d 点积分。", taskInfo.ID, taskInfo.AmountPoint)},
	})
	return nil
}

// XBinding 查询用户 X 账号绑定状态：
//   - 未绑过 (user.twitter_id == "") → isBound=false
//   - 绑过但 Redis 里 token 缺失/无法 refresh → isBound=true, tokenValid=false
//   - 绑过且 token 有效 → isBound=true, tokenValid=true
func XBinding(uid int) (*XBindingResponse, error) {
	userInfo := entity.User{}
	err := db.Mysql().Model(&entity.User{}).
		Select("twitter_id, twitter_handle, twitter_display_name, twitter_avatar").
		Where("id = ?", uid).First(&userInfo).Error
	if err != nil {
		return nil, err
	}
	resp := &XBindingResponse{
		IsBound:            userInfo.TwitterID != "",
		TwitterID:          userInfo.TwitterID,
		TwitterHandle:      userInfo.TwitterHandle,
		TwitterDisplayName: userInfo.TwitterDisplayName,
		TwitterAvatar:      userInfo.TwitterAvatar,
	}
	if !resp.IsBound {
		return resp, nil
	}
	// 探测 token 是否可用（不做真实 API 调用，只检查 Redis 存在 + 未过期）
	token := &twitter.Token{}
	if err := db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, uid), token); err != nil {
		return resp, nil
	}
	if !token.SupportsReadOnlyVerification() {
		return resp, nil
	}
	if _, err := token.CheckExpireTime(); err == nil {
		resp.TokenValid = true
	}
	return resp, nil
}

func Info(uid int) (*InfoResponse, error) {
	userInfo := user.Info(uid)
	if userInfo.ID == 0 {
		return nil, ret.CusError(i18n.ERR_001_00007)
	}

	checkInDate, _ := checkInDateAndDay(time.Now())
	totalCheckInDays, continuousCheckInDays, err := currentCheckInSummary(uid, checkInDate)
	if err != nil {
		return nil, err
	}
	checkedToday, err := hasCheckedIn(uid, checkInDate)
	if err != nil {
		return nil, err
	}

	twitterTask, err := twitterTaskInfo(uid)
	if err != nil {
		return nil, err
	}
	telegramTask, err := telegramTaskInfo(uid)
	if err != nil {
		return nil, err
	}
	leaderboard, err := checkInLeaderboard(uid)
	if err != nil {
		// Ranking is display-only. A slow/failed global GROUP BY must not make
		// the authenticated user's check-in status endpoint unavailable.
		log.GetLogger().Warn("check-in leaderboard unavailable; omitting display ranking")
		leaderboard = []CheckInLeaderboard{}
	}

	checkInStatus := taskStatusAvailable
	if checkedToday {
		checkInStatus = taskStatusCompleted
	}
	totalPoint := points.GetUserPoints(uid)

	// 本月已签到的具体日期号列表（给前端画日历用，无需再靠 streak "近似"）
	monthChecked, err := currentMonthCheckedDates(uid, time.Now())
	if err != nil {
		monthChecked = []int{}
	}

	return &InfoResponse{
		PointOverview: PointOverview{
			WalletAddress:         userInfo.WalletAddress,
			TotalPoint:            totalPoint,
			TotalCheckInDays:      totalCheckInDays,
			ContinuousCheckInDays: continuousCheckInDays,
		},
		CheckInTask: CheckInTaskInfo{
			Status:       checkInStatus,
			CheckedDates: monthChecked,
		},
		TwitterTask:        twitterTask,
		TelegramTask:       telegramTask,
		InviteTask:         inviteTaskInfo(uid),
		CheckInLeaderboard: leaderboard,
	}, nil
}

// currentMonthCheckedDates 返回当前 UTC 月份内 uid 已签到的"号"（1..31）。
// 用于前端日历画绿方块，替代 streak 近似。
func currentMonthCheckedDates(uid int, now time.Time) ([]int, error) {
	nowUTC := now.UTC()
	monthStart := time.Date(nowUTC.Year(), nowUTC.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := monthStart.AddDate(0, 1, 0)

	type row struct {
		CheckInDate string `gorm:"column:check_in_date"`
	}
	var rows []row
	err := db.Mysql().Model(&entity.UserCheckInRecord{}).
		Select("check_in_date").
		Where("uid = ? AND check_in_date >= ? AND check_in_date < ?",
			uid,
			monthStart.Format("2006-01-02"),
			nextMonth.Format("2006-01-02")).
		Order("check_in_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	dates := make([]int, 0, len(rows))
	for _, r := range rows {
		if len(r.CheckInDate) == 10 {
			// YYYY-MM-DD → int(DD)
			day, convErr := strconv.Atoi(r.CheckInDate[8:10])
			if convErr == nil {
				dates = append(dates, day)
			}
		}
	}
	return dates, nil
}

func currentCheckInSummary(uid int, today string) (int, int, error) {
	latest := entity.UserCheckInRecord{}
	err := db.Mysql().Model(&entity.UserCheckInRecord{}).
		Select("id,check_in_date,total_check_in_days,continuous_check_in_days").
		Where("uid = ?", uid).
		Order("check_in_date desc,id desc").
		Limit(1).
		Scan(&latest).Error
	if err != nil {
		return 0, 0, err
	}
	if latest.ID == 0 {
		return 0, 0, nil
	}

	totalCheckInDays := latest.TotalCheckInDays
	if totalCheckInDays == 0 {
		var count int64
		err = db.Mysql().Model(&entity.UserCheckInRecord{}).
			Where("uid = ?", uid).
			Count(&count).Error
		if err != nil {
			return 0, 0, err
		}
		totalCheckInDays = int(count)
	}

	continuousCheckInDays := 0
	if isCurrentContinuousDate(latest.CheckInDate, today) {
		continuousCheckInDays = latest.ContinuousCheckInDays
	}
	return totalCheckInDays, continuousCheckInDays, nil
}

func isCurrentContinuousDate(checkInDate, today string) bool {
	if checkInDate == today {
		return true
	}
	// today 由 checkInDateAndDay 生成，已是 UTC 日期字符串
	currentDate, err := time.ParseInLocation("2006-01-02", today, time.UTC)
	if err != nil {
		return false
	}
	return checkInDate == currentDate.AddDate(0, 0, -1).Format("2006-01-02")
}

func telegramTaskInfo(uid int) (TelegramTaskInfo, error) {
	tw, err := platformTaskInfoVersioned(uid, "telegram")
	if err != nil {
		return TelegramTaskInfo{}, err
	}
	return TelegramTaskInfo{
		Status:          tw.Status,
		CompletedKeys:   tw.CompletedKeys,
		ChatEarnedPoint: tgChatEarnedPoint(uid),
	}, nil
}

func tgChatEarnedPoint(uid int) int {
	var total int
	err := db.Mysql().Model(&entity.UserPointFlow{}).
		Select("COALESCE(SUM(change_point), 0)").
		Where("uid = ? AND type = ? AND `desc` = ? AND change_point > 0",
			uid, constant.PointFlowTypeTask, "TG chat activity").
		Scan(&total).Error
	if err != nil {
		log.GetLogger().Warnf("get tg chat earned point failed uid=%d err=%s", uid, err)
		return 0
	}
	return total
}

func twitterTaskInfo(uid int) (TwitterTaskInfo, error) {
	return platformTaskInfoVersioned(uid, "twitter")
}

func platformTaskInfo(uid int, platform string) (TwitterTaskInfo, error) {
	res := TwitterTaskInfo{
		Status:        taskStatusDisabled,
		CompletedKeys: []string{},
	}

	// 拉所有启用的对应 platform 任务
	tasks := make([]entity.Task, 0, 8)
	err := db.Mysql().Model(&entity.Task{}).
		Select("id, task_key").
		Where("platform = ? AND is_enable = 1", platform).
		Find(&tasks).Error
	if err != nil {
		return res, err
	}
	if len(tasks) == 0 {
		return res, nil
	}
	res.Status = taskStatusAvailable

	// task_id → task_key 映射
	idToKey := make(map[uint64]string, len(tasks))
	ids := make([]uint64, 0, len(tasks))
	for _, t := range tasks {
		idToKey[t.ID] = t.TaskKey
		ids = append(ids, t.ID)
	}

	// 查用户完成过的 records
	records := make([]entity.TaskRecord, 0, len(tasks))
	err = db.Mysql().Model(&entity.TaskRecord{}).
		Select("task_id").
		Where("uid = ? AND task_id IN ?", uid, ids).
		Find(&records).Error
	if err != nil {
		return res, err
	}
	for _, r := range records {
		if key, ok := idToKey[uint64(r.TaskID)]; ok {
			res.CompletedKeys = append(res.CompletedKeys, key)
		}
	}
	// 全部完成时状态为 completed（前端主要看 CompletedKeys，Status 保留向后兼容）
	if len(res.CompletedKeys) == len(tasks) {
		res.Status = taskStatusCompleted
	}
	return res, nil
}

func inviteTaskInfo(uid int) InviteTaskInfo {
	inviteCode := user.GetUserInviteCode(uid)
	res := InviteTaskInfo{
		InviteCode:  inviteCode,
		RewardPoint: constant.TaskInvitePoint,
	}

	var successfulInviteCount int64
	err := db.Mysql().Model(&entity.InviteRecord{}).
		Where("inviter_uid = ? and receive_amount = 1", uid).
		Count(&successfulInviteCount).Error
	if err != nil {
		log.GetLogger().Errorf("get successful invite count failed uid=%d err=%s", uid, err)
		return res
	}
	res.SuccessfulInviteCount = int(successfulInviteCount)
	res.EarnedPoint = int(successfulInviteCount) * constant.TaskInvitePoint
	return res
}

type checkInLeaderboardRow struct {
	UID           int    `gorm:"column:uid"`
	WalletAddress string `gorm:"column:wallet_address"`
	CheckInCount  int    `gorm:"column:check_in_count"`
}

// The leaderboard is global display data, not the authority for a user's
// check-in. Coalesce concurrent readers rather than re-running a full GROUP BY
// for every three-second task-page poll and exhausting the DB connection pool.
var checkInLeaderboardSnapshot = struct {
	sync.RWMutex
	rows    []checkInLeaderboardRow
	expires time.Time
	flight  singleflight.Group
}{}

func checkInLeaderboard(uid int) ([]CheckInLeaderboard, error) {
	checkInLeaderboardSnapshot.RLock()
	rows, fresh := checkInLeaderboardSnapshot.rows, time.Now().Before(checkInLeaderboardSnapshot.expires)
	checkInLeaderboardSnapshot.RUnlock()
	if !fresh {
		value, err, _ := checkInLeaderboardSnapshot.flight.Do("leaderboard", func() (any, error) {
			checkInLeaderboardSnapshot.RLock()
			cached, valid := checkInLeaderboardSnapshot.rows, time.Now().Before(checkInLeaderboardSnapshot.expires)
			checkInLeaderboardSnapshot.RUnlock()
			if valid {
				return cached, nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			loaded, err := loadCheckInLeaderboard(ctx, db.Mysql())
			if err != nil {
				return nil, err
			}
			checkInLeaderboardSnapshot.Lock()
			checkInLeaderboardSnapshot.rows = loaded
			checkInLeaderboardSnapshot.expires = time.Now().Add(time.Minute)
			checkInLeaderboardSnapshot.Unlock()
			return loaded, nil
		})
		if err != nil {
			return nil, err
		}
		rows = value.([]checkInLeaderboardRow)
	}
	return formatCheckInLeaderboard(rows, uid), nil
}

func loadCheckInLeaderboard(ctx context.Context, database *gorm.DB) ([]checkInLeaderboardRow, error) {
	if database == nil {
		return nil, errors.New("check-in leaderboard database unavailable")
	}
	rows := make([]checkInLeaderboardRow, 0, 20)
	err := database.WithContext(ctx).Model(&entity.UserCheckInRecord{}).
		Select("uid,MAX(wallet_address) as wallet_address,COUNT(*) as check_in_count,MAX(created_at) as reached_at").
		Group("uid").
		Order("check_in_count desc").
		Order("reached_at asc").
		Limit(20).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func formatCheckInLeaderboard(rows []checkInLeaderboardRow, uid int) []CheckInLeaderboard {
	res := make([]CheckInLeaderboard, 0, len(rows))
	for i, row := range rows {
		res = append(res, CheckInLeaderboard{
			Rank:          i + 1,
			WalletAddress: maskWalletAddress(row.WalletAddress),
			CheckInCount:  row.CheckInCount,
			IsCurrentUser: row.UID == uid,
		})
	}
	return res
}

func maskWalletAddress(address string) string {
	address = strings.TrimSpace(address)
	if len(address) <= 10 {
		return address
	}
	return address[:6] + "..." + address[len(address)-4:]
}

// CheckTgJoinResult 检查用户是否加入 TG 群的结果
type CheckTgJoinResult struct {
	Verified    bool `json:"verified"`     // 是否检测到用户在群
	AlreadyDone bool `json:"already_done"` // 之前已完成
	NotBound    bool `json:"not_bound"`    // 用户还没绑定 TG
	Awarded     int  `json:"awarded"`      // 本次奖励积分（0 = 未奖励）
}

// CheckTgJoin 主动调 Telegram getChatMember 检查用户是否已经在群，
// 如果在群且任务未完成，直接 insert TaskRecord + 加分。
// 前端 TG join 点击后，可以按此接口 poll 直到用户加群。
func CheckTgJoin(uid int) (*CheckTgJoinResult, error) {
	res := &CheckTgJoinResult{}

	// 1) 拿 user.telegram_id
	var u entity.User
	if err := db.Mysql().Model(&entity.User{}).
		Select("id, telegram_id").Where("id = ?", uid).First(&u).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(u.TelegramID) == "" {
		res.NotBound = true
		return res, nil
	}

	// 2) 查 TG join 任务
	var taskInfo entity.Task
	if err := db.Mysql().Model(&entity.Task{}).
		Select("id, task_key, amount_point, version, url").
		Where("is_enable = 1 AND platform = ? AND task_key = ?", "telegram", "join").
		First(&taskInfo).Error; err != nil || taskInfo.ID == 0 {
		return res, nil // 没配置 join 任务，直接返回
	}

	// 3) 检查是否已经完成过
	var rec entity.TaskRecord
	db.Mysql().Model(&entity.TaskRecord{}).Select("id").
		Where("uid = ? AND task_id = ?", uid, taskInfo.ID).Scan(&rec)
	if rec.ID != 0 {
		res.AlreadyDone = true
		res.Verified = true
		return res, nil
	}

	// 4) 调 Telegram getChatMember 判断用户是否在群
	botToken := viper.GetString("telegram.botToken")
	if botToken == "" {
		return nil, fmt.Errorf("telegram.botToken not configured")
	}
	chatIDStr := readTgChatID()
	if chatIDStr == "" {
		return nil, fmt.Errorf("tg.chat_id not set in settings")
	}

	inGroup, err := telegramCheckMember(botToken, chatIDStr, u.TelegramID)
	if err != nil {
		return nil, err
	}
	if !inGroup {
		return res, nil // 未在群
	}

	// 5) 在群 → insert TaskRecord + 加分（用与 CompleteTask 一致的 ChangePoints 流程）
	tx := db.Mysql().Begin()
	insert := &entity.TaskRecord{
		TaskID:       int(taskInfo.ID),
		Platform:     "telegram",
		TaskVersion:  currentTaskVersion(taskInfo.Version),
		UID:          uid,
		AmountPoint:  taskInfo.AmountPoint,
		VerifyStatus: "manual_pass",
		TargetURL:    taskInfo.URL,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := tx.Create(insert).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	rollback := true
	rollback, err = points.ChangePoints(uid, taskInfo.AmountPoint, constant.PointFlowTypeTask, "TG join auto-verified", *tx)
	if err != nil {
		points.RollbackChangePoints(uid, taskInfo.AmountPoint, constant.PointFlowTypeTask, rollback, false)
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	res.Verified = true
	res.Awarded = taskInfo.AmountPoint
	return res, nil
}

// readTgChatID 从 settings 表读 tg.chat_id
func readTgChatID() string {
	var s entity.Settings
	if err := db.Mysql().Model(&entity.Settings{}).
		Select("value").Where("`key` = ?", "tg.chat_id").First(&s).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(s.Value)
}

// telegramCheckMember 调 Telegram Bot API getChatMember 判断是否在群
func telegramCheckMember(botToken, chatID, tgUserID string) (bool, error) {
	apiURL := fmt.Sprintf(
		"https://api.telegram.org/bot%s/getChatMember?chat_id=%s&user_id=%s",
		botToken, url.QueryEscape(chatID), url.QueryEscape(tgUserID),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("telegram api %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			Status string `json:"status"` // creator/administrator/member/restricted/left/kicked
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, err
	}
	if !out.OK {
		return false, fmt.Errorf("telegram api ok=false: %s", string(body))
	}
	s := out.Result.Status
	return s == "creator" || s == "administrator" || s == "member" || s == "restricted", nil
}
