// Bounded operational recovery: refresh enrolled evidence, never sign/send.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/config"
	dbclient "joincare_server/pkg/common/database/db"
	"os"
	"strings"
	"time"
)

const hash = "0xd07cecf548b4d7e11cf6fc749cd078a0ef0bb354b808ceabfc1117b2f370433d"

func run(args []string) error {
	f := flag.NewFlagSet("refresh-fee74-retirement", flag.ContinueOnError)
	dir := f.String("config-dir", "", "existing Core config")
	execute := f.Bool("execute", false, "append independently verified evidence only")
	confirm := f.String("confirm-hash", "", "exact active original hash")
	if f.Parse(args) != nil || f.NArg() != 0 || *dir == "" || *confirm != hash {
		return errors.New("exact config and original hash required")
	}
	loc, e := time.LoadLocation("Asia/Shanghai")
	if e != nil {
		return errors.New("timezone")
	}
	time.Local = loc
	for _, n := range []string{"core", "common", "secret"} {
		if config.InitConfig(strings.TrimRight(*dir, "/\\")+"/"+n, "config.yaml") != nil {
			return errors.New("config unavailable")
		}
	}
	if viper.GetString("application.environment") != "prod" || !viper.GetBool("idoV3RetiredNonce.enabled") || !viper.GetBool("idoV3Approved.enabled") || viper.GetUint64("chain.project.chainID") != 56 {
		return errors.New("runtime identity")
	}
	db, e := dbclient.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), "", "", "", viper.GetString("db.mysql.databaseName"), nil, "ERROR")
	if e != nil {
		return errors.New("database")
	}
	db.Logger = logger.Default.LogMode(logger.Silent)
	sql, e := db.DB()
	if e != nil {
		return errors.New("database")
	}
	defer sql.Close()
	var identity struct{ UUID, DB, Account string }
	if db.Raw("SELECT @@server_uuid AS uuid,DATABASE() AS db,CURRENT_USER() AS account").Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" || identity.Account != "dbmasteruser@%" {
		return errors.New("database identity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	reader, e := chainrpc.DialScanner(ctx, viper.GetString("idoV3TokenAdmission.primaryRPC"))
	if e != nil {
		return errors.New("reader")
	}
	defer reader.Close()
	independent, e := chainrpc.DialScanner(ctx, "https://bsc-dataseed1.bnbchain.org")
	if e != nil {
		return errors.New("independent reader")
	}
	defer independent.Close()
	report, e := chaintx.RefreshRetirementForActiveOriginal(ctx, db, 74, common.HexToHash(hash), []uint64{41, 43, 44, 45, 48, 49}, reader, independent, *execute)
	if e != nil {
		return errors.New("fee74 retirement refresh refused; no send was possible")
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
