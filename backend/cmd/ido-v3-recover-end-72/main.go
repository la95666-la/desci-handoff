// A single authorized original outbox broadcast; never loads a signing client.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/spf13/viper"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/database/db"
	"joincare_server/pkg/common/idov3whitelist"
	"os"
	"strings"
	"time"
)

func run(args []string) error {
	f := flag.NewFlagSet("recover-end72", flag.ContinueOnError)
	path := f.String("config-dir", "", "Core config directory")
	execute := f.Bool("execute", false, "one original send")
	confirm := f.String("confirm-hash", "", "exact original hash")
	if f.Parse(args) != nil || f.NArg() != 0 || *path == "" || (*execute && *confirm != chaintx.End72Hash) || (!*execute && *confirm != "") {
		return errors.New("invalid flags")
	}
	loc, e := time.LoadLocation("Asia/Shanghai")
	if e != nil {
		return errors.New("timezone unavailable")
	}
	time.Local = loc
	for _, n := range []string{"core", "common", "secret"} {
		if config.InitConfig(strings.TrimRight(*path, "/\\")+"/"+n, "config.yaml") != nil {
			return errors.New("configuration unavailable")
		}
	}
	if viper.GetString("application.environment") != "prod" || !viper.GetBool("idoV3Approved.enabled") || !viper.GetBool("idoV3Lifecycle.enabled") || viper.GetUint64("chain.project.chainID") != 56 || !strings.EqualFold(viper.GetString("idoV3Execution.proxyAddress"), "0x0d4040875742975269c55e16c8884d3ee9510b8e") || !strings.EqualFold(viper.GetString("idoV3Execution.operatorAddress"), "0xfd712061ee08e070a930ecd45e935ae5d7f5445c") {
		return errors.New("production identity unavailable")
	}
	url := viper.GetString("idoV3TokenAdmission.primaryRPC")
	if idov3whitelist.ValidateApprovedV3RPCEndpoint(url) != nil {
		return errors.New("RPC not approved")
	}
	owner, governor, e := idov3whitelist.ParseApprovedV3SafeCodeHashes(viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash"), viper.GetString("idoV3TokenAdmission.governorSafeCodeHash"))
	if e != nil {
		return errors.New("Safe identity unavailable")
	}
	database, e := db.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), "", "", "", viper.GetString("db.mysql.databaseName"), nil, "ERROR")
	if e != nil {
		return errors.New("database unavailable")
	}
	database.Logger = logger.Default.LogMode(logger.Silent)
	sql, e := database.DB()
	if e != nil {
		return errors.New("database unavailable")
	}
	defer sql.Close()
	var identity struct{ UUID, DB, Account string }
	if database.Raw("SELECT @@server_uuid AS uuid,DATABASE() AS db,CURRENT_USER() AS account").Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" || identity.Account != "dbmasteruser@%" {
		return errors.New("database identity mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	reader, e := chainrpc.Dial(ctx, url)
	if e != nil {
		return errors.New("reader unavailable")
	}
	defer reader.Close()
	// Independent public transport; no key, wallet or signer is constructed.
	sender, e := chainrpc.Dial(ctx, "https://bsc-dataseed1.bnbchain.org")
	if e != nil {
		return errors.New("transport unavailable")
	}
	defer sender.Close()
	report, e := chaintx.RecoverEnd72Once(ctx, database, reader, sender, owner, governor, *execute)
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
	return e
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
