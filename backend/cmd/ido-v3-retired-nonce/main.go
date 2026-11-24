// Read-only dry-run unless --execute. Never loads a private key into a signer,
// sends a transaction, or changes an outbox/project/nonce row.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

func run(args []string) error {
	fs := flag.NewFlagSet("ido-v3-retired-nonce", flag.ContinueOnError)
	dir := fs.String("config-dir", "", "existing isolated runtime config, never logged")
	id := fs.Uint64("submission-id", 0, "exact original suspended attempt")
	hash := fs.String("confirm-hash", "", "exact original hash required even for dry-run")
	execute := fs.Bool("execute", false, "persist short-lived verified evidence only")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if fs.NArg() != 0 || *dir == "" || *id == 0 || len(*hash) != 66 || !strings.HasPrefix(*hash, "0x") || len(common.FromHex(*hash)) != 32 || common.HexToHash(*hash) == (common.Hash{}) {
		return fmt.Errorf("exact config-dir, submission-id and original confirm-hash required")
	}
	for _, part := range []string{"job", "common", "secret"} {
		if e := config.InitConfig(strings.TrimRight(*dir, "/\\")+"/"+part, "config.yaml"); e != nil {
			return fmt.Errorf("configuration unavailable")
		}
	}
	if viper.GetString("application.environment") != "prod" || viper.GetUint64("chain.project.chainID") != 56 || idov3whitelist.ValidateApprovedV3RPCEndpoint(viper.GetString("idoV3TokenAdmission.primaryRPC")) != nil {
		return fmt.Errorf("production identity unavailable")
	}
	database, e := db.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), "", "", "", viper.GetString("db.mysql.databaseName"), nil, "ERROR")
	if e != nil {
		return fmt.Errorf("database unavailable")
	}
	sqlDB, e := database.DB()
	if e != nil {
		return e
	}
	defer sqlDB.Close()
	var identity struct{ UUID, Name, User string }
	if database.Raw(`SELECT @@server_uuid AS uuid,DATABASE() AS name,CURRENT_USER() AS user`).Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.Name != "joincare" {
		return fmt.Errorf("production database identity mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	rpc, e := chainrpc.DialScanner(ctx, viper.GetString("idoV3TokenAdmission.primaryRPC"))
	if e != nil {
		return fmt.Errorf("RPC unavailable")
	}
	defer rpc.Close()
	proof, e := chaintx.ReconcileRetiredV3Nonce(ctx, database, rpc, *id, common.HexToHash(*hash), *execute)
	if e != nil {
		return fmt.Errorf("original-hash retired nonce reconciliation refused")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"execute": *execute, "evidence": proof, "outbox_changed": false, "nonce_changed": false})
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
