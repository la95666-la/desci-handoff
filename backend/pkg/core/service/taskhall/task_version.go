package taskhall

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	driverMysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/points"
	settingsRepo "joincare_server/pkg/core/repository/settings"
	"joincare_server/pkg/core/service/notification"
	"joincare_server/pkg/core/third/twitter"
)

func currentTaskVersion(version uint32) uint32 {
	if version == 0 {
		return 1
	}
	return version
}

func taskVersionMismatch() error {
	return ret.CusError(i18n.ERR_001_00021)
}

func taskRecordTargetRef(task entity.Task) string {
	if strings.TrimSpace(task.TargetRef) != "" {
		return strings.TrimSpace(task.TargetRef)
	}
	switch strings.ToLower(strings.TrimSpace(task.TaskKey)) {
	case "like", "retweet", "reply", "bookmark":
		_, tweetID, err := twitter.ParseTweetURL(task.URL)
		if err == nil {
			return tweetID
		}
	}
	return ""
}

func isDuplicateTaskRecord(err error) bool {
	var mysqlErr *driverMysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

type strictTwitterTaskVerifierFunc func(database *gorm.DB, uid int, task entity.Task) error

var strictTwitterTaskVerifier strictTwitterTaskVerifierFunc = verifyTwitterTaskWithAPI

func isSupportedTwitterTask(task entity.Task) bool {
	if !strings.EqualFold(strings.TrimSpace(task.Platform), "twitter") {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(task.TaskKey))
	switch key {
	case "follow", "like", "retweet", "reply":
		// Bypass eligibility must retain the same target-shape requirements as
		// strict verification, otherwise a malformed task could receive points
		// only while the external API is disabled.
		if strings.TrimSpace(task.URL) == "" {
			return false
		}
		if key == "follow" {
			return true
		}
		_, tweetID, err := twitter.ParseTweetURL(task.URL)
		return err == nil && strings.TrimSpace(tweetID) != ""
	default:
		return false
	}
}

// twitterTaskVerifyStatus only bypasses the external X API. Task identity,
// round checks, the completion transaction and the database uniqueness guard
// remain in CompleteTaskVersioned. Policy read failures reject completion so a
// broken setting can never become an implicit bypass.
func twitterTaskVerifyStatus(database *gorm.DB, uid int, task entity.Task) (string, error) {
	if !isSupportedTwitterTask(task) {
		return "", ret.CusError(i18n.ERR_001_00020)
	}

	disabled, policyErr := settingsRepo.TwitterVerificationDisabled(database)
	if policyErr != nil {
		if logger := log.GetLogger(); logger != nil {
			logger.Warnf("twitter verification policy read failed; task completion rejected task=%d err=%v", task.ID, policyErr)
		}
		return "", ret.CusError(i18n.ERR_001_00020)
	}
	if disabled {
		return "manual_pass", nil
	}

	verifyErr := strictTwitterTaskVerifier(database, uid, task)
	var notCompleted *twitter.TaskNotCompletedError
	if errors.As(verifyErr, &notCompleted) {
		verifyErr = ret.CusError(i18n.ERR_001_00019)
	}
	if verifyErr != nil {
		if logger := log.GetLogger(); logger != nil {
			logger.Warnf("twitter task verify err uid=%d task=%d version=%d key=%s err=%v",
				uid, task.ID, currentTaskVersion(task.Version), task.TaskKey, verifyErr)
		}
		return "", verifyErr
	}
	return "twitter_pass", nil
}

func verifyTwitterTaskWithAPI(database *gorm.DB, uid int, task entity.Task) error {
	if task.URL == "" {
		return ret.CusError(i18n.ERR_001_00020)
	}
	token := &twitter.Token{}
	if err := db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, uid), token); err != nil {
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

	var userInfo entity.User
	if err := database.Model(&entity.User{}).
		Select("twitter_id").Where("id = ?", uid).First(&userInfo).Error; err != nil {
		return err
	}
	if userInfo.TwitterID == "" {
		return ret.CusError(i18n.ERR_001_00015)
	}

	switch strings.ToLower(strings.TrimSpace(task.TaskKey)) {
	case "follow":
		return token.VerifyFollowing(userInfo.TwitterID, task.URL)
	case "like":
		return token.VerifyLike(userInfo.TwitterID, task.URL)
	case "retweet":
		return token.VerifyRetweet(userInfo.TwitterID, task.URL)
	case "reply":
		return token.VerifyReply(userInfo.TwitterID, task.URL)
	default:
		return ret.CusError(i18n.ERR_001_00020)
	}
}

// CompleteTaskVersioned completes exactly the task round rendered by the
// client. taskID/taskVersion may be zero only for backward-compatible clients.
func CompleteTaskVersioned(
	uid int,
	platform string,
	taskID uint64,
	taskVersion uint32,
	taskKey string,
	_ string,
) error {
	if platform == "" {
		platform = "twitter"
	}

	taskInfo := entity.Task{}
	q := db.Mysql().Model(&entity.Task{}).
		Select("id,platform,task_key,target_slot,verify_kind,url,amount_point,is_enable,version,target_ref,round_started_at").
		Where("is_enable = 1 AND platform = ?", platform)
	if taskID != 0 {
		q = q.Where("id = ?", taskID)
	} else if taskKey != "" {
		// Legacy clients cannot identify slot 2. Keep their historical behavior
		// deterministic by resolving only slot 1.
		q = q.Where("task_key = ? AND target_slot = ?", taskKey, 1)
	}
	if err := q.Order("sort ASC, id ASC").First(&taskInfo).Error; err != nil {
		return err
	}
	if taskKey != "" && !strings.EqualFold(taskInfo.TaskKey, taskKey) {
		return taskVersionMismatch()
	}

	currentVersion := currentTaskVersion(taskInfo.Version)
	if taskVersion == 0 {
		// Legacy clients still work, but only new clients receive stale-tab
		// protection because they submit the rendered version explicitly.
		taskVersion = currentVersion
	}
	if taskVersion != currentVersion {
		return taskVersionMismatch()
	}

	var existing entity.TaskRecord
	err := db.Mysql().Model(&entity.TaskRecord{}).
		Select("id").
		Where("uid = ? AND task_id = ? AND task_version = ?", uid, taskInfo.ID, currentVersion).
		Take(&existing).Error
	if err == nil {
		return ret.CusError(i18n.ERR_001_00016)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if strings.EqualFold(taskInfo.Platform, "twitter") && taskInfo.VerifyKind == "twitter_api" && !isSupportedTwitterTask(taskInfo) {
		return ret.CusError(i18n.ERR_001_00020)
	}

	verifyStatus := "manual_pass"
	if taskInfo.VerifyKind == "twitter_api" {
		var err error
		verifyStatus, err = twitterTaskVerifyStatus(db.Mysql(), uid, taskInfo)
		if err != nil {
			return err
		}
	}

	tx := db.Mysql().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	var locked entity.Task
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id,platform,task_key,target_slot,url,amount_point,is_enable,version,target_ref").
		Where("id = ? AND is_enable = 1", taskInfo.ID).Take(&locked).Error; err != nil {
		tx.Rollback()
		return err
	}
	if currentTaskVersion(locked.Version) != taskVersion ||
		locked.Platform != taskInfo.Platform ||
		locked.TaskKey != taskInfo.TaskKey ||
		locked.TargetSlot != taskInfo.TargetSlot ||
		taskRecordTargetRef(locked) != taskRecordTargetRef(taskInfo) {
		tx.Rollback()
		return taskVersionMismatch()
	}

	now := time.Now()
	insert := &entity.TaskRecord{
		TaskID:       int(locked.ID),
		Platform:     locked.Platform,
		TaskVersion:  taskVersion,
		UID:          uid,
		AmountPoint:  locked.AmountPoint,
		VerifyStatus: verifyStatus,
		TargetRef:    taskRecordTargetRef(locked),
		TargetURL:    locked.URL,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := tx.Create(insert).Error; err != nil {
		tx.Rollback()
		if isDuplicateTaskRecord(err) {
			return ret.CusError(i18n.ERR_001_00016)
		}
		return err
	}

	desc := fmt.Sprintf("Complete %s/%s task #%d v%d", locked.Platform, locked.TaskKey, locked.ID, taskVersion)
	rollbackRedis, err := points.ChangePoints(
		uid, locked.AmountPoint, constant.PointFlowTypeTask, desc, *tx,
	)
	if err != nil {
		points.RollbackChangePoints(uid, locked.AmountPoint, constant.PointFlowTypeTask, rollbackRedis, false)
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		points.RollbackChangePoints(uid, locked.AmountPoint, constant.PointFlowTypeTask, rollbackRedis, false)
		return err
	}

	notification.Push(uid, int(locked.ID), constant.TaskCompleted, map[string][]string{
		constant.NoticeLangEn: {"Task Completed", fmt.Sprintf("You earned %d points from task #%d (round %d).", locked.AmountPoint, locked.ID, taskVersion)},
		constant.NoticeLangTw: {"任務完成", fmt.Sprintf("完成任務 #%d（第 %d 輪），獲得 %d 點積分。", locked.ID, taskVersion, locked.AmountPoint)},
		constant.NoticeLangCn: {"任务完成", fmt.Sprintf("完成任务 #%d（第 %d 轮），获得 %d 点积分。", locked.ID, taskVersion, locked.AmountPoint)},
	})
	return nil
}

// platformTaskInfoVersioned marks a task complete only when the user's record
// belongs to the task's current round.
func platformTaskInfoVersioned(uid int, platform string) (TwitterTaskInfo, error) {
	res := TwitterTaskInfo{Status: taskStatusDisabled, CompletedKeys: []string{}, CompletedTaskIDs: []uint64{}}
	tasks := make([]entity.Task, 0, 8)
	if err := db.Mysql().Model(&entity.Task{}).
		Select("id,task_key,version").
		Where("platform = ? AND is_enable = 1", platform).
		Find(&tasks).Error; err != nil {
		return res, err
	}
	if len(tasks) == 0 {
		return res, nil
	}
	ids := make([]uint64, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}

	var records []entity.TaskRecord
	if err := db.Mysql().Model(&entity.TaskRecord{}).
		Select("task_id,task_version").
		Where("uid = ? AND task_id IN ?", uid, ids).
		Find(&records).Error; err != nil {
		return res, err
	}
	return buildPlatformTaskInfo(tasks, records), nil
}

func buildPlatformTaskInfo(tasks []entity.Task, records []entity.TaskRecord) TwitterTaskInfo {
	res := TwitterTaskInfo{Status: taskStatusDisabled, CompletedKeys: []string{}, CompletedTaskIDs: []uint64{}}
	if len(tasks) == 0 {
		return res
	}
	res.Status = taskStatusAvailable
	type currentTask struct {
		Key     string
		Version uint32
	}
	currentByID := make(map[uint64]currentTask, len(tasks))
	for _, task := range tasks {
		currentByID[task.ID] = currentTask{Key: task.TaskKey, Version: currentTaskVersion(task.Version)}
	}
	for _, record := range records {
		current, ok := currentByID[uint64(record.TaskID)]
		if ok && currentTaskVersion(record.TaskVersion) == current.Version {
			res.CompletedKeys = append(res.CompletedKeys, current.Key)
			res.CompletedTaskIDs = append(res.CompletedTaskIDs, uint64(record.TaskID))
		}
	}
	if len(res.CompletedTaskIDs) == len(tasks) {
		res.Status = taskStatusCompleted
	}
	return res
}
