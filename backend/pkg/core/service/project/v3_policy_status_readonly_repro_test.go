package project

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestPolicyStatusProductionReadOnlyRepro(t *testing.T) {
	dir := os.Getenv("JC_POLICY_STATUS_READONLY_CONFIG")
	if dir == "" {
		t.Skip("explicit production readonly diagnostic only")
	}
	for _, part := range []string{"core", "common", "secret"} {
		if e := config.InitConfig(dir+"/"+part, "config.yaml"); e != nil {
			t.Fatal("configuration unavailable")
		}
	}
	c := drivermysql.NewConfig()
	c.User = viper.GetString("db.mysql.user")
	c.Passwd = viper.GetString("db.mysql.password")
	c.Net = "tcp"
	c.Addr = viper.GetString("db.mysql.host")
	c.DBName = viper.GetString("db.mysql.databaseName")
	c.ParseTime = true
	c.Loc = time.Local
	c.Timeout = 10 * time.Second
	c.ReadTimeout = 30 * time.Second
	c.Params = map[string]string{"transaction_read_only": "1"}
	database, e := gorm.Open(mysql.Open(c.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("DB unavailable")
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()
	var identity struct{ UUID, Name string }
	if database.Raw(`SELECT @@server_uuid AS uuid,DATABASE() AS name`).Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.Name != "joincare" {
		t.Fatal("identity mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), v3PolicyDisplayBudget)
	defer cancel()
	const id = uint64(43359438123302913)
	t.Logf("old global scan gate accepts=%t", idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil)
	var health struct {
		State string
		Age   int64
	}
	database.Raw(`SELECT state,TIMESTAMPDIFF(SECOND,last_success_at,UTC_TIMESTAMP()) AS age FROM ido_v3_approved_scan_health WHERE chain_id=56`).Scan(&health)
	t.Logf("global health state=%s last_success_age_seconds=%d", health.State, health.Age)
	owner, governor, e := approvedV3SafeHashes()
	if e != nil {
		t.Fatal("hash configuration unavailable")
	}
	rpc, e := v3TokenGateRPC(ctx)
	if e != nil {
		t.Fatal("RPC unavailable")
	}
	defer rpc.Close()
	started := time.Now()
	status, e := readApprovedV3Display(ctx, database, rpc, id, owner, governor)
	if e != nil {
		t.Fatalf("display status failed elapsed=%s check=%v", time.Since(started), e)
	}
	t.Logf("display passed elapsed=%s project=%s state=%s scanner=%s enabled=%t version=%s root=%s block=%s hash=%s approval=%s", time.Since(started), status.ProjectID, status.ProjectState, status.ScannerState, status.Enabled, status.Version, status.Root, status.Block, status.BlockHash, status.ApprovalState)
	if status.ProjectID != "43359438123302913" || !strings.EqualFold(status.Proxy, v3TokenProxy) {
		t.Fatal("scope mismatch")
	}
}
