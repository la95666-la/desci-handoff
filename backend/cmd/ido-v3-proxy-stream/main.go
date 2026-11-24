// Controlled local operator tool. Defaults to read-only progress; no signing,
// transaction submission, schema installation or automatic project recovery.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/database/db"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/job/job/chainevent"
)

func run(args []string) error {
	flags := flag.NewFlagSet("ido-v3-proxy-stream", flag.ContinueOnError)
	dir := flags.String("config-dir", "", "isolated Job config directory (never logged)")
	action := flags.String("action", "inspect", "inspect, audit-project, check-project, recover-project, backfill, switch-stream, switch-shadow, recover")
	execute := flags.Bool("execute", false, "required for separately authorized writer mutation")
	revision := flags.Uint64("expected-revision", 0, "exact stream revision")
	ancestor := flags.Uint64("ancestor", 0, "independently selected recovery ancestor")
	hash := flags.String("ancestor-hash", "", "exact recovery ancestor hash")
	project := flags.Uint64("project-id", 0, "exact halted project for coverage recovery")
	retired := flags.String("retired-tests", "", "exact sorted comma-separated test project IDs")
	confirmRetired := flags.String("confirm-retired-digest", "", "SHA256 of exact retired ID JSON array")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dir == "" || flags.NArg() != 0 {
		return fmt.Errorf("config-dir required")
	}
	if *action != "inspect" && *action != "check-project" && *action != "audit-project" && (!*execute || *revision == 0) {
		return fmt.Errorf("mutation requires execute and exact expected-revision")
	}
	if *action == "inspect" && *execute {
		return fmt.Errorf("inspect cannot execute mutation")
	}
	if *action != "inspect" && *action != "switch-stream" && *action != "switch-shadow" && *action != "recover" && *action != "check-project" && *action != "recover-project" && *action != "backfill" && *action != "audit-project" && *action != "retire-tests-rebaseline" {
		return fmt.Errorf("unsupported action")
	}
	var ids []uint64
	if *action == "retire-tests-rebaseline" {
		for _, v := range strings.Split(*retired, ",") {
			id, e := strconv.ParseUint(v, 10, 64)
			if e != nil || id == 0 || (len(ids) > 0 && ids[len(ids)-1] >= id) {
				return fmt.Errorf("exact sorted retired IDs required")
			}
			ids = append(ids, id)
		}
		if *ancestor == 0 || len(*hash) != 66 || chainevent.RetiredV3TestDigest(ids) != *confirmRetired {
			return fmt.Errorf("exact confirmed baseline and retirement digest required")
		}
	}
	if (*action == "check-project" || *action == "recover-project") && (*project == 0 || *revision == 0) {
		return fmt.Errorf("project recovery requires exact project-id/revision")
	}
	if *action == "check-project" && *execute {
		return fmt.Errorf("check-project is read-only")
	}
	if *action == "audit-project" && (*project == 0 || *execute) {
		return fmt.Errorf("audit-project requires project-id and is read-only")
	}
	if *action == "recover" && (*ancestor == 0 || len(*hash) != 66 || common.HexToHash(*hash) == (common.Hash{})) {
		return fmt.Errorf("recovery requires exact ancestor/hash")
	}
	for _, part := range []string{"job", "common", "secret"} {
		if err := config.InitConfig(strings.TrimRight(*dir, "/\\")+"/"+part, "config.yaml"); err != nil {
			return fmt.Errorf("configuration unavailable")
		}
	}
	if viper.GetString("application.environment") != "prod" || viper.GetString("application.jobMode") != "ido" || !viper.GetBool("idoV3Approved.enabled") || viper.GetUint64("chain.project.chainID") != 56 || idov3whitelist.ValidateApprovedV3RPCEndpoint(viper.GetString("idoV3TokenAdmission.primaryRPC")) != nil {
		return fmt.Errorf("production Job identity unavailable")
	}
	database, err := db.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), "", "", "", viper.GetString("db.mysql.databaseName"), nil, "ERROR")
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	defer sqlDB.Close()
	var identity struct{ UUID, Name string }
	if database.Raw(`SELECT @@server_uuid AS uuid,DATABASE() AS name`).Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.Name != "joincare" {
		return fmt.Errorf("production database identity mismatch")
	}
	operationBudget := 150 * time.Second
	if *action == "audit-project" || *action == "check-project" || *action == "recover-project" {
		operationBudget = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationBudget)
	defer cancel()
	rpc, err := chainrpc.DialScanner(ctx, viper.GetString("idoV3TokenAdmission.primaryRPC"))
	if err != nil {
		return fmt.Errorf("RPC unavailable")
	}
	defer rpc.Close()
	if *action != "inspect" {
		var err error
		if *action == "retire-tests-rebaseline" {
			err = chainevent.OperatorV3TestRebaseline(ctx, database, rpc, *revision, ids, *ancestor, common.HexToHash(*hash), *confirmRetired)
		} else if *action == "audit-project" {
			err = chainevent.OperatorV3ProjectParity(ctx, database, rpc, *project)
		} else if *action == "check-project" || *action == "recover-project" {
			owner, governor, e := idov3whitelist.ParseApprovedV3SafeCodeHashes(viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash"), viper.GetString("idoV3TokenAdmission.governorSafeCodeHash"))
			if e != nil {
				return fmt.Errorf("Safe identity unavailable")
			}
			err = chainevent.OperatorV3ProjectCoverageRecovery(ctx, database, rpc, *project, *revision, *execute, owner, governor)
		} else {
			err = chainevent.OperatorV3ProxyStream(ctx, database, rpc, *action, *revision, *ancestor, common.HexToHash(*hash))
		}
		if err != nil {
			return fmt.Errorf("stream operation refused")
		}
	}
	progress, err := chainevent.V3ProxyStreamProgress(ctx, database)
	if err != nil {
		return fmt.Errorf("stream progress unavailable")
	}
	return json.NewEncoder(os.Stdout).Encode(progress)
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
