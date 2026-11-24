package taskhall

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	driverMysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestCurrentTaskVersionDefaultsToOne(t *testing.T) {
	if got := currentTaskVersion(0); got != 1 {
		t.Fatalf("currentTaskVersion(0) = %d, want 1", got)
	}
	if got := currentTaskVersion(7); got != 7 {
		t.Fatalf("currentTaskVersion(7) = %d, want 7", got)
	}
}

func TestTaskRecordTargetRef(t *testing.T) {
	task := entity.Task{
		TaskKey: "retweet",
		URL:     "https://x.com/joincare/status/99887766?ref=home",
	}
	if got := taskRecordTargetRef(task); got != "99887766" {
		t.Fatalf("taskRecordTargetRef() = %q, want 99887766", got)
	}
	task.TargetRef = "stored-target"
	if got := taskRecordTargetRef(task); got != "stored-target" {
		t.Fatalf("stored target = %q, want stored-target", got)
	}
}

func TestPlatformTaskInfoSupportsDuplicateTaskKeysByID(t *testing.T) {
	tasks := []entity.Task{
		{ID: 11, TaskKey: "like", TargetSlot: 1, Version: 4},
		{ID: 12, TaskKey: "like", TargetSlot: 2, Version: 2},
	}
	partial := buildPlatformTaskInfo(tasks, []entity.TaskRecord{{TaskID: 11, TaskVersion: 4}})
	if partial.Status != taskStatusAvailable || len(partial.CompletedTaskIDs) != 1 || partial.CompletedTaskIDs[0] != 11 {
		t.Fatalf("partial completion = %#v", partial)
	}
	if len(partial.CompletedKeys) != 1 || partial.CompletedKeys[0] != "like" {
		t.Fatalf("legacy completed keys = %#v", partial.CompletedKeys)
	}

	complete := buildPlatformTaskInfo(tasks, []entity.TaskRecord{
		{TaskID: 11, TaskVersion: 4},
		{TaskID: 12, TaskVersion: 2},
	})
	if complete.Status != taskStatusCompleted || len(complete.CompletedTaskIDs) != 2 {
		t.Fatalf("complete status = %#v", complete)
	}
}

func TestTwitterVerificationPolicyControlsExternalVerifier(t *testing.T) {
	for _, test := range []struct {
		name              string
		settingValue      *string
		settingErr        error
		wantStatus        string
		wantVerifierCalls int
		wantErr           bool
	}{
		{name: "disabled skips X API", settingValue: taskStringPtr("1"), wantStatus: "manual_pass"},
		{name: "enabled calls X API", settingValue: taskStringPtr("0"), wantStatus: "twitter_pass", wantVerifierCalls: 1},
		{name: "missing calls X API", wantStatus: "twitter_pass", wantVerifierCalls: 1},
		{name: "invalid rejects without X API", settingValue: taskStringPtr("invalid"), wantErr: true},
		{name: "read failure rejects without X API", settingErr: errors.New("settings unavailable"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock := taskVersionPolicyDB(t)
			query := mock.ExpectQuery("SELECT .*value.* FROM `settings` WHERE `key` = .* LIMIT").
				WithArgs(constant.TwitterVerificationDisabledSettingKey, 1)
			if test.settingErr != nil {
				query.WillReturnError(test.settingErr)
			} else {
				rows := sqlmock.NewRows([]string{"value"})
				if test.settingValue != nil {
					rows.AddRow(*test.settingValue)
				}
				query.WillReturnRows(rows)
			}

			oldVerifier := strictTwitterTaskVerifier
			calls := 0
			strictTwitterTaskVerifier = func(_ *gorm.DB, _ int, _ entity.Task) error {
				calls++
				return nil
			}
			t.Cleanup(func() { strictTwitterTaskVerifier = oldVerifier })

			status, err := twitterTaskVerifyStatus(database, 42, entity.Task{
				ID: 7, Platform: "twitter", TaskKey: "like", URL: "https://x.com/joincare/status/123", Version: 3,
			})
			if (err != nil) != test.wantErr || status != test.wantStatus || calls != test.wantVerifierCalls {
				t.Fatalf("status=%q err=%v verifierCalls=%d, want status=%q calls=%d wantErr=%v", status, err, calls, test.wantStatus, test.wantVerifierCalls, test.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTwitterVerificationPolicyRejectsUnknownTaskWithoutExternalCall(t *testing.T) {
	database, mock := taskVersionPolicyDB(t)
	oldVerifier := strictTwitterTaskVerifier
	calls := 0
	strictTwitterTaskVerifier = func(_ *gorm.DB, _ int, _ entity.Task) error {
		calls++
		return nil
	}
	t.Cleanup(func() { strictTwitterTaskVerifier = oldVerifier })

	if _, err := twitterTaskVerifyStatus(database, 42, entity.Task{Platform: "twitter", TaskKey: "unknown"}); err == nil {
		t.Fatal("unknown X task was accepted")
	}
	if calls != 0 {
		t.Fatalf("external verifier called %d times", calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTwitterVerificationPolicyRejectsMalformedSupportedTarget(t *testing.T) {
	database, mock := taskVersionPolicyDB(t)
	oldVerifier := strictTwitterTaskVerifier
	calls := 0
	strictTwitterTaskVerifier = func(_ *gorm.DB, _ int, _ entity.Task) error {
		calls++
		return nil
	}
	t.Cleanup(func() { strictTwitterTaskVerifier = oldVerifier })

	if _, err := twitterTaskVerifyStatus(database, 42, entity.Task{Platform: "twitter", TaskKey: "like", URL: "https://x.com/joincare"}); err == nil {
		t.Fatal("malformed X post target was accepted")
	}
	if calls != 0 {
		t.Fatalf("external verifier called %d times", calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSupportedTwitterTaskAllowlist(t *testing.T) {
	validPostURL := "https://x.com/joincare/status/123456"
	for _, test := range []struct {
		task entity.Task
		want bool
	}{
		{task: entity.Task{Platform: "twitter", TaskKey: "follow", URL: "https://x.com/joincare"}, want: true},
		{task: entity.Task{Platform: "twitter", TaskKey: "like", URL: validPostURL}, want: true},
		{task: entity.Task{Platform: "twitter", TaskKey: "retweet", URL: validPostURL}, want: true},
		{task: entity.Task{Platform: "twitter", TaskKey: "reply", URL: validPostURL}, want: true},
		{task: entity.Task{Platform: "twitter", TaskKey: "bookmark", URL: validPostURL}, want: false},
		{task: entity.Task{Platform: "twitter", TaskKey: "like", URL: "https://x.com/joincare"}, want: false},
		{task: entity.Task{Platform: "telegram", TaskKey: "follow", URL: "https://x.com/joincare"}, want: false},
	} {
		if got := isSupportedTwitterTask(test.task); got != test.want {
			t.Fatalf("task=%+v supported=%v want=%v", test.task, got, test.want)
		}
	}
}

func TestDuplicateTaskRecordUsesDatabaseUniquenessError(t *testing.T) {
	if !isDuplicateTaskRecord(&driverMysql.MySQLError{Number: 1062, Message: "duplicate task/version/uid"}) {
		t.Fatal("MySQL duplicate key was not recognized")
	}
	if isDuplicateTaskRecord(errors.New("other")) {
		t.Fatal("unrelated error was treated as duplicate")
	}
}

func taskVersionPolicyDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}

func taskStringPtr(value string) *string { return &value }
