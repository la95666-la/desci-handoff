package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/third/twitter"
)

var (
	errInvalidTaskTarget = errors.New("invalid task target")
	errTaskTargetReused  = errors.New("task target was already used")
)

type taskRoundChange struct {
	Changed    bool
	OldVersion uint32
	NewVersion uint32
	TargetRef  string
}

func normalizedTaskVersion(version uint32) uint32 {
	if version == 0 {
		return 1
	}
	return version
}

func isTwitterPostTask(platform, taskKey string) bool {
	if !strings.EqualFold(strings.TrimSpace(platform), "twitter") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(taskKey)) {
	case "like", "retweet", "reply", "bookmark":
		return true
	default:
		return false
	}
}

func normalizedTaskTargetSlot(slot uint8) uint8 {
	if slot == 0 {
		return 1
	}
	return slot
}

func supportsSecondTaskTargetSlot(platform, taskKey string) bool {
	if !strings.EqualFold(strings.TrimSpace(platform), "twitter") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(taskKey)) {
	case "like", "retweet", "reply":
		return true
	default:
		return false
	}
}

func validateTaskTargetSlot(platform, taskKey string, slot uint8) error {
	slot = normalizedTaskTargetSlot(slot)
	if slot == 1 {
		return nil
	}
	if slot == 2 && supportsSecondTaskTargetSlot(platform, taskKey) {
		return nil
	}
	if isTwitterPostTask(platform, taskKey) {
		return fmt.Errorf("%w: X post target_slot must be 1 or 2", errInvalidTaskTarget)
	}
	return fmt.Errorf("%w: non-post tasks only support target_slot 1", errInvalidTaskTarget)
}

// taskTargetRef deliberately uses the tweet ID rather than the literal URL.
// x.com/twitter.com and query-string variants therefore remain the same round.
func taskTargetRef(platform, taskKey, rawURL string) (string, error) {
	if !isTwitterPostTask(platform, taskKey) {
		return "", nil
	}
	_, tweetID, err := twitter.ParseTweetURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: invalid X post URL: %v", errInvalidTaskTarget, err)
	}
	if strings.TrimSpace(tweetID) == "" {
		return "", fmt.Errorf("%w: invalid X post URL: tweet ID is empty", errInvalidTaskTarget)
	}
	return tweetID, nil
}

func ensureTaskRound(tx *gorm.DB, task entity.Task, targetRef string) error {
	if targetRef == "" {
		return nil
	}
	startedAt := task.RoundStartedAt
	if startedAt.IsZero() {
		startedAt = task.CreatedAt
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	round := entity.TaskRound{
		TaskID:      task.ID,
		Version:     normalizedTaskVersion(task.Version),
		TargetRef:   targetRef,
		TargetURL:   task.URL,
		AmountPoint: task.AmountPoint,
		StartedAt:   startedAt,
		CreatedAt:   startedAt,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&round).Error; err != nil {
		return err
	}
	if !supportsSecondTaskTargetSlot(task.Platform, task.TaskKey) {
		return nil
	}
	history := entity.TaskTargetHistory{
		Platform:    task.Platform,
		TaskKey:     strings.ToLower(strings.TrimSpace(task.TaskKey)),
		TargetRef:   targetRef,
		TaskID:      task.ID,
		TaskVersion: normalizedTaskVersion(task.Version),
		TargetSlot:  normalizedTaskTargetSlot(task.TargetSlot),
		TargetURL:   task.URL,
		CreatedAt:   startedAt,
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&history).Error
}

// Cross-slot reuse is an operator decision. A previous target from slot 1 can
// be assigned to slot 2 after slot 1 rotates; per-task round history still
// prevents awarding the same task/post twice.
func rejectDuplicateTwitterTarget(tx *gorm.DB, task entity.Task, excludeTaskID uint64) error {
	_, err := taskTargetRef(task.Platform, task.TaskKey, task.URL)
	return err
}

// applyTaskRoundMutation updates round-related fields in updates and records
// the old/new targets. Call it while the task row is locked in a transaction.
func applyTaskRoundMutation(
	tx *gorm.DB,
	before entity.Task,
	effective entity.Task,
	updates map[string]interface{},
) (taskRoundChange, error) {
	oldVersion := normalizedTaskVersion(before.Version)
	change := taskRoundChange{OldVersion: oldVersion, NewVersion: oldVersion}

	oldIsPost := isTwitterPostTask(before.Platform, before.TaskKey)
	newIsPost := isTwitterPostTask(effective.Platform, effective.TaskKey)
	oldRef := strings.TrimSpace(before.TargetRef)
	if oldIsPost && oldRef == "" {
		if parsed, err := taskTargetRef(before.Platform, before.TaskKey, before.URL); err == nil {
			oldRef = parsed
		}
	}
	if oldIsPost && oldRef != "" {
		before.TargetRef = oldRef
		before.Version = oldVersion
		if err := ensureTaskRound(tx, before, oldRef); err != nil {
			return change, err
		}
	}

	newRef, err := taskTargetRef(effective.Platform, effective.TaskKey, effective.URL)
	if err != nil {
		return change, err
	}
	change.TargetRef = newRef
	updates["target_ref"] = newRef

	// Same logical post, including URL host/query variations: keep the round.
	if oldIsPost && newIsPost &&
		strings.EqualFold(before.Platform, effective.Platform) &&
		strings.EqualFold(before.TaskKey, effective.TaskKey) &&
		oldRef != "" && oldRef == newRef {
		updates["version"] = oldVersion
		if before.RoundStartedAt.IsZero() {
			updates["round_started_at"] = before.CreatedAt
		}
		return change, nil
	}

	// Non-post tasks (for example follow) do not restart for URL/metadata edits.
	if !oldIsPost && !newIsPost {
		updates["version"] = oldVersion
		return change, nil
	}

	// A changed post or a task identity transition starts a fresh round.
	if newRef != "" {
		var count int64
		if err := tx.Model(&entity.TaskRound{}).
			Where("task_id = ? AND target_ref = ?", before.ID, newRef).
			Count(&count).Error; err != nil {
			return change, err
		}
		if count > 0 {
			return change, fmt.Errorf("%w: tweet %s", errTaskTargetReused, newRef)
		}
	}

	now := time.Now()
	newVersion := oldVersion + 1
	updates["version"] = newVersion
	updates["round_started_at"] = now
	change.Changed = true
	change.NewVersion = newVersion

	if newRef != "" {
		round := entity.TaskRound{
			TaskID:      before.ID,
			Version:     newVersion,
			TargetRef:   newRef,
			TargetURL:   effective.URL,
			AmountPoint: effective.AmountPoint,
			StartedAt:   now,
			CreatedAt:   now,
		}
		if err := tx.Create(&round).Error; err != nil {
			return change, err
		}
		if supportsSecondTaskTargetSlot(effective.Platform, effective.TaskKey) {
			history := entity.TaskTargetHistory{
				Platform:    effective.Platform,
				TaskKey:     strings.ToLower(strings.TrimSpace(effective.TaskKey)),
				TargetRef:   newRef,
				TaskID:      before.ID,
				TaskVersion: newVersion,
				TargetSlot:  normalizedTaskTargetSlot(effective.TargetSlot),
				TargetURL:   effective.URL,
				CreatedAt:   now,
			}
			if err := tx.Create(&history).Error; err != nil {
				return change, err
			}
		}
	}
	return change, nil
}
