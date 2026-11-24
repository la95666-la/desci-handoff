package controller

import (
	"encoding/json/v2"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

func TestProjectListSelectsPersistedV3FundingMode(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := db.Client.Mysql
	db.Client.Mysql = database
	t.Cleanup(func() { db.Client.Mysql = previous })

	const projectID int64 = 42630852370235393
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `projects`").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	mock.ExpectQuery("SELECT .*funding_mode.*FROM `projects`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "status", "funding_mode", "contract_version"}).
			AddRow(1, projectID, constant.ProjectStatusSet, constant.FundingModeCappedPresale, constant.ContractVersionIDOV3Stablecoin))
	mock.ExpectQuery("SELECT .*FROM `chain_tx_submission`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "action", "state", "tx_hash"}))

	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest("GET", "/admin/v1/projects?page=1&size=20", nil)
	ListProjects(ctx)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"funding_mode":"capped_presale"`) ||
		!strings.Contains(response.Body.String(), `"project_id":"42630852370235393"`) {
		t.Fatalf("approved V3 project list unavailable: %d %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectListRowPreservesSonyflakeProjectID(t *testing.T) {
	id := int64(40104297089728513)
	row := projectListRow{ProjectID: strconv.FormatInt(id, 10)}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.ProjectID != "40104297089728513" {
		t.Fatalf("unsafe project id in Admin list: %s, %v", encoded, err)
	}
}

func TestProjectContentResponsePreservesSonyflakeProjectID(t *testing.T) {
	row, err := newProjectContentResponse(entity.Project{ProjectID: 40104297089728513, ContractVersion: "v2_stablecoin"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.ProjectID != "40104297089728513" {
		t.Fatalf("unsafe project id in Admin content: %s, %v", encoded, err)
	}
}

func TestSetupContentExposesPersistedCappedPresaleIntent(t *testing.T) {
	row := entity.Project{
		ProjectID:       41197150595448833,
		Status:          constant.ProjectStatusSetup,
		FundingMode:     constant.FundingModeCappedPresale,
		ContractVersion: constant.ContractVersionUnassigned,
	}
	got, err := newProjectContentResponse(row)
	if err != nil {
		t.Fatal(err)
	}
	if got.FundingMode != constant.FundingModeCappedPresale {
		t.Fatalf("Setup intent lost: %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"project_id":"41197150595448833"`, `"funding_mode":"capped_presale"`, `"contract_version":"unassigned"`, `"status":"Setup"`} {
		if !strings.Contains(string(encoded), expected) {
			t.Errorf("missing %s in Setup content", expected)
		}
	}
	row.ContractVersion = constant.ContractVersionV1Legacy
	got, err = newProjectContentResponse(row)
	if err != nil {
		t.Fatal(err)
	}
	if got.FundingMode != "" {
		t.Fatal("must not infer capped-presale mode for legacy contracts")
	}
}

func TestProjectListSubmissionSummaryNeverLeaksOutboxInternals(t *testing.T) {
	hash := "0x" + strings.Repeat("a", 64)
	row := projectListRow{ProjectID: "7", ContractVersion: "v2_stablecoin", OnchainLocked: true, ActiveSubmission: &projectSubmissionSummary{ID: 9, Action: "v2_create", State: "submitted", TxHash: &hash}}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"sender_address", "nonce", "gas_limit", "gas_price_raw", "raw_tx", "call_data", "last_error"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("project list safe summary leaked %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"active_submission", "v2_create", "submitted", hash} {
		if !strings.Contains(text, required) {
			t.Fatalf("project list safe summary omitted %q: %s", required, text)
		}
	}
}

func TestProjectReadResponsesExposeExactFairLaunchProjection(t *testing.T) {
	project := entity.Project{
		ProjectID:            8,
		ContractVersion:      constant.ContractVersionFairLaunchV1,
		Status:               constant.ProjectStatusSet,
		TokenDecimals:        18,
		PaymentTokenDecimals: 6,
		SoftCapRaw:           "10000001",
		OnchainJSON:          `{"fairLaunch":{"saleTokenAmountRaw":"1000000000000000000000001","softCapRaw":"10000001"}}`,
	}
	content, err := newProjectContentResponse(project)
	if err != nil {
		t.Fatal(err)
	}
	if content.FundingMode != constant.FundingModeFairLaunch || content.ChainStatus != "not_created" || content.SaleTokenAmountRaw != "1000000000000000000000001" || content.SoftCapRaw != "10000001" {
		t.Fatalf("wrong content projection: %+v", content)
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`"funding_mode":"fair_launch"`, `"chain_status":"not_created"`, `"sale_token_amount_raw":"1000000000000000000000001"`, `"soft_cap_raw":"10000001"`} {
		if !strings.Contains(string(encoded), required) {
			t.Fatalf("content projection omitted %s: %s", required, encoded)
		}
	}
}

func TestFairLaunchPendingReleaseProjection(t *testing.T) {
	project := entity.Project{
		ProjectID: 9, ContractVersion: constant.ContractVersionFairLaunchV1,
		Status: constant.ProjectStatusSuccessfulPendingRelease, OnchainLocked: true,
		TokenDecimals: 18, PaymentTokenDecimals: 6, SoftCapRaw: "1000000",
		OnchainJSON: `{"fairLaunch":{"saleTokenAmountRaw":"1000000000000000000","softCapRaw":"1000000"}}`,
	}
	content, err := newProjectContentResponse(project)
	if err != nil {
		t.Fatal(err)
	}
	if content.ChainStatus != "successful_pending_release" {
		t.Fatalf("wrong pending release status: %+v", content)
	}
}

func TestProjectRestoreCapability(t *testing.T) {
	tests := []struct {
		name    string
		project entity.Project
		active  bool
		allowed bool
		blocker string
	}{
		{name: "trusted source", project: entity.Project{Status: "archived", PreArchiveStatus: "pre_launch", ContractVersion: constant.ContractVersionUnassigned}, allowed: true},
		{name: "legacy source unknown", project: entity.Project{Status: "archived", ContractVersion: constant.ContractVersionUnassigned}, blocker: "PROJECT_RESTORE_STATE_UNKNOWN"},
		{name: "v2 locked", project: entity.Project{Status: "archived", PreArchiveStatus: "Set", ContractVersion: constant.ContractVersionV2Stablecoin}, blocker: "PROJECT_RESTORE_LIFECYCLE_LOCKED"},
		{name: "fair launch unlocked", project: entity.Project{Status: "archived", PreArchiveStatus: "Set", ContractVersion: constant.ContractVersionFairLaunchV1}, blocker: "PROJECT_RESTORE_LIFECYCLE_LOCKED"},
		{name: "V3 unlocked", project: entity.Project{Status: "archived", PreArchiveStatus: "Set", ContractVersion: constant.ContractVersionIDOV3Stablecoin}, blocker: "PROJECT_RESTORE_LIFECYCLE_LOCKED"},
		{name: "active submission", project: entity.Project{Status: "archived", PreArchiveStatus: "warmup", ContractVersion: constant.ContractVersionUnassigned}, active: true, blocker: "PROJECT_RESTORE_ACTIVE_SUBMISSION"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, blocker := projectRestoreCapability(test.project, test.active)
			if allowed != test.allowed || blocker != test.blocker {
				t.Fatalf("got allowed=%v blocker=%q; want allowed=%v blocker=%q", allowed, blocker, test.allowed, test.blocker)
			}
		})
	}
}
