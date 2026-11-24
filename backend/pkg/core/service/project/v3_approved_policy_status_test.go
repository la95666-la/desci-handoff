package project

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedPolicyStatusRequiresExecutedCreationBeforeChainRead(t *testing.T) {
	if _, err := readApprovedV3PolicyStatus(nil, nil, nil, 19, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("nil admission accepted")
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	// ethclient with a nil RPC transport would panic if admission consulted
	// current policy before the executed approval / applied-create DB join.
	rpc := &ethclient.Client{}
	if _, err := readApprovedV3PolicyStatus(context.Background(), database, rpc, 19,
		common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatal("candidate policy read admitted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFirstEnableReviewStatusBudgetSeparateFromStrictReaders(t *testing.T) {
	if got := v3ApprovedPolicyStatusBudget(false); got != 40*time.Second {
		t.Fatalf("strict status budget changed: %s", got)
	}
	if got := v3ApprovedPolicyStatusBudget(true); got != 120*time.Second {
		t.Fatalf("first-enable status budget changed: %s", got)
	}
}

func TestApprovedV3SafeHashesRequireBothSeparatePinnedIdentities(t *testing.T) {
	ownerKey := "idoV3TokenAdmission.ownerSafeCodeHash"
	governorKey := "idoV3TokenAdmission.governorSafeCodeHash"
	oldOwner, oldGovernor := viper.Get(ownerKey), viper.Get(governorKey)
	t.Cleanup(func() { viper.Set(ownerKey, oldOwner); viper.Set(governorKey, oldGovernor) })
	viper.Set(ownerKey, "0x"+strings.Repeat("a", 64))
	viper.Set(governorKey, "")
	if _, _, err := approvedV3SafeHashes(); err == nil {
		t.Fatal("missing governor trust root accepted")
	}
	viper.Set(governorKey, "0x"+strings.Repeat("B", 64))
	if _, _, err := approvedV3SafeHashes(); err == nil {
		t.Fatal("noncanonical code hash accepted")
	}
	viper.Set(governorKey, "0x"+strings.Repeat("b", 64))
	owner, governor, err := approvedV3SafeHashes()
	if err != nil || owner == (common.Hash{}) || governor == (common.Hash{}) || owner == governor {
		t.Fatalf("independent hashes rejected: %v", err)
	}
}
