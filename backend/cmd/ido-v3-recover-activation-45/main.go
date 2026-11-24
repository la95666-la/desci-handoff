// One-time recovery for the exact already signed #45 activation attempt.
// No relayer private key is read. Dry-run is the default; --execute can send
// only the original persisted bytes after all source and chain fences pass.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/database/db"
	"joincare_server/pkg/common/idov3whitelist"
)

const originalHash = "0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f"

func run(args []string) error {
	fs := flag.NewFlagSet("ido-v3-recover-activation-45", flag.ContinueOnError)
	path := fs.String("config-dir", "", "production Core config directory (not logged)")
	execute := fs.Bool("execute", false, "commit one original-hash send; default is read-only dry-run")
	confirm := fs.String("confirm-hash", "", "exact original transaction hash required with --execute")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *path == "" || (*execute && *confirm != originalHash) || (!*execute && *confirm != "") {
		return errors.New("invalid recovery flags; execution requires exact original hash")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return errors.New("production time zone unavailable")
	}
	time.Local = location
	for _, item := range []string{"core", "common", "secret"} {
		if err := config.InitConfig(strings.TrimRight(*path, "/\\")+"/"+item, "config.yaml"); err != nil {
			return errors.New("production configuration unavailable")
		}
	}
	if viper.GetString("application.environment") != "prod" || !viper.GetBool("idoV3Approved.enabled") ||
		!viper.GetBool("idoV3Lifecycle.enabled") || viper.GetInt64("chain.project.chainID") != 56 ||
		!strings.EqualFold(viper.GetString("idoV3Execution.proxyAddress"), "0x0d4040875742975269c55e16c8884d3ee9510b8e") ||
		!strings.EqualFold(viper.GetString("idoV3Execution.operatorAddress"), "0xfd712061ee08e070a930ecd45e935ae5d7f5445c") ||
		idov3whitelist.ValidateApprovedV3RPCEndpoint(viper.GetString("idoV3TokenAdmission.primaryRPC")) != nil {
		return errors.New("production V3 identity unavailable")
	}
	owner, governor, err := idov3whitelist.ParseApprovedV3SafeCodeHashes(
		viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash"),
		viper.GetString("idoV3TokenAdmission.governorSafeCodeHash"))
	if err != nil {
		return errors.New("production Safe identity unavailable")
	}
	database, err := db.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"),
		viper.GetString("db.mysql.password"), "", "", "", viper.GetString("db.mysql.databaseName"), nil, "ERROR")
	if err != nil {
		return errors.New("database unavailable")
	}
	sqlDB, err := database.DB()
	if err != nil {
		return errors.New("database unavailable")
	}
	defer sqlDB.Close()
	var identity struct {
		UUID string `gorm:"column:uuid"`
		DB   string `gorm:"column:db_name"`
		User string `gorm:"column:db_user"`
	}
	if database.Raw("SELECT @@server_uuid AS uuid,DATABASE() AS db_name,CURRENT_USER() AS db_user").Scan(&identity).Error != nil ||
		identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" ||
		!strings.HasPrefix(identity.User, "dbmasteruser@") {
		return errors.New("production database identity mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	reader, err := chainrpc.Dial(ctx, viper.GetString("idoV3TokenAdmission.primaryRPC"))
	if err != nil {
		return errors.New("approved read RPC unavailable")
	}
	defer reader.Close()
	sender, err := chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	if err != nil {
		return errors.New("operator transport unavailable")
	}
	defer sender.Close()
	if id, err := sender.ChainID(ctx); err != nil || id.Cmp(big.NewInt(56)) != 0 {
		return errors.New("operator transport chain mismatch")
	}
	candidate, err := idov3whitelist.ReadApprovedLockedCandidate(ctx, database, 43235577608273921)
	if err != nil {
		return errors.New("approved original creation unavailable")
	}
	if err := chaintx.RecoverActivation45Once(ctx, database, sender, reader, candidate.Binding,
		candidate.Runtime, common.HexToHash(originalHash), owner, governor, *execute); err != nil {
		return fmt.Errorf("recovery unavailable at %s; inspect #45 by original hash; never retry", chaintx.Recovery45FailureStage(err))
	}
	if *execute {
		fmt.Println("original transaction submitted once; reconcile #45 by original hash (not yet final)")
	} else {
		fmt.Println("dry-run verified #45; no outbox write or chain send")
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
