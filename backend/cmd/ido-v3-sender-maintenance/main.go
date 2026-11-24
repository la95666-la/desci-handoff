// Explicit idle-sender maintenance; no signing/send method is constructed.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/config"
	dbclient "joincare_server/pkg/common/database/db"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
)

func run(args []string) error {
	f := flag.NewFlagSet("sender-maintenance", flag.ContinueOnError)
	dir := f.String("config-dir", "", "isolated job configuration")
	confirm := f.String("confirm-sender", "", "fixed sender identity; required for explicit evidence refresh")
	if f.Parse(args) != nil || f.NArg() != 0 || *dir == "" || strings.ToLower(*confirm) != "0xfd712061ee08e070a930ecd45e935ae5d7f5445c" {
		return errors.New("explicit config-dir and sender confirmation required")
	}
	loc, e := time.LoadLocation("Asia/Shanghai")
	if e != nil {
		return errors.New("timezone")
	}
	time.Local = loc
	for _, n := range []string{"job", "common", "secret"} {
		if config.InitConfig(strings.TrimRight(*dir, "/\\")+"/"+n, "config.yaml") != nil {
			return errors.New("config unavailable")
		}
	}
	url := viper.GetString("idoV3TokenAdmission.primaryRPC")
	if viper.GetString("application.environment") != "prod" || !viper.GetBool("idoV3RetiredNonce.enabled") || idov3whitelist.ValidateApprovedV3RPCEndpoint(url) != nil {
		return errors.New("production source unavailable")
	}
	d, e := dbclient.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), "", "", "", viper.GetString("db.mysql.databaseName"), nil, "ERROR")
	if e != nil {
		return errors.New("database unavailable")
	}
	d.Logger = logger.Default.LogMode(logger.Silent)
	sql, e := d.DB()
	if e != nil {
		return e
	}
	defer sql.Close()
	var identity struct{ UUID, DB string }
	if d.Raw("SELECT @@server_uuid AS uuid,DATABASE() AS db").Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" {
		return errors.New("production database mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	a, e := chainrpc.DialScanner(ctx, url)
	if e != nil {
		return errors.New("reader unavailable")
	}
	defer a.Close()
	b, e := chainrpc.DialScanner(ctx, "https://bsc-dataseed1.bnbchain.org")
	if e != nil {
		return errors.New("independent unavailable")
	}
	defer b.Close()
	out, e := chaintx.RefreshIdleV3SenderEvidence(ctx, d, 56, common.HexToAddress(*confirm), a, b)
	if e != nil {
		return fmt.Errorf("sender maintenance refused reason=%s stage=%s", v3diagnostic.Reason(e), v3diagnostic.Stage(e))
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
