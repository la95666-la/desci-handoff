package project

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/db"
	chainmanager "joincare_server/pkg/core/third/chain"
	"math/big"
	"strings"
	"time"
)

// Fixed incident scope, explicit admin execution only. No startup/background
// signer or sender. Existing Core wallet stays inside the application process.
func Original99Disposition(ctx context.Context, action, hash string, guard OnChainMutationGuard) (chaintx.Original99Report, error) {
	invalid := errors.New("original99 disposition unavailable; reconcile both hashes")
	if ctx == nil || guard == nil || db.Mysql() == nil || !ApprovedV3ModeReady() || !viper.GetBool("idoV3Execution.enabled") {
		return chaintx.Original99Report{}, invalid
	}
	if action != "preflight" && action != "prepare" && action != "broadcast" && action != "reconcile" && action != "status" {
		return chaintx.Original99Report{}, invalid
	}
	if (action == "prepare" && hash != chaintx.Original99Hash) || (action == "broadcast" && !v3CanonicalHash(hash)) {
		return chaintx.Original99Report{}, invalid
	}
	ctx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	// SQL statements can include signed bytes on failure. Never let recovery
	// logging print those bytes or underlying credentials/provider text.
	database := db.Mysql().Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error { return guard(tx) }) != nil {
		return chaintx.Original99Report{}, invalid
	}
	url := viper.GetString("idoV3TokenAdmission.primaryRPC")
	if idov3whitelist.ValidateApprovedV3RPCEndpoint(url) != nil || strings.TrimSpace(url) == "https://bsc-dataseed1.bnbchain.org" {
		return chaintx.Original99Report{}, invalid
	}
	primary, e := chainrpc.DialScanner(ctx, url)
	if e != nil {
		return chaintx.Original99Report{}, invalid
	}
	defer primary.Close()
	second, e := chainrpc.DialScanner(ctx, "https://bsc-dataseed1.bnbchain.org")
	if e != nil {
		return chaintx.Original99Report{}, invalid
	}
	defer second.Close()
	providers := []chaintx.Original99RPC{primary, second}
	if action == "status" {
		return chaintx.ReadOriginal99Disposition(ctx, db.Mysql())
	}
	// Recovery may not borrow a generic RPC identity or trust the transaction
	// row as its own deployment authority. Both independently configured
	// readers verify the pinned V3 runtime against the immutable executed Set.
	record, e := idov3whitelist.ReadApprovedSetRecord(ctx, db.Mysql(), chaintx.Original99Project)
	if e != nil {
		return chaintx.Original99Report{}, invalid
	}
	owner, governor, e := approvedV3SafeHashes()
	if e != nil {
		return chaintx.Original99Report{}, invalid
	}
	for _, rpc := range []idov3whitelist.PolicyRPC{primary, second} {
		h, e := rpc.HeaderByNumber(ctx, nil)
		if e != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() < 128805710 {
			return chaintx.Original99Report{}, invalid
		}
		height := h.Number.Uint64() - 14
		anchor, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
		if e != nil || anchor == nil || anchor.Number == nil || anchor.Number.Uint64() != height || idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, owner, governor, height, anchor.Hash()) != nil {
			return chaintx.Original99Report{}, invalid
		}
	}
	switch action {
	case "preflight":
		return chaintx.PrepareOriginal99Disposition(ctx, database, providers, nil, false, guard)
	case "prepare":
		client := chainmanager.GetClient(constant.ChainProjectV3)
		if client == nil || client.PrivateKey == nil || client.ChainID == nil || client.ChainID.Cmp(big.NewInt(56)) != 0 || client.Address != common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c") || crypto.PubkeyToAddress(client.PrivateKey.PublicKey) != client.Address || client.ContractAddress != common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e") {
			return chaintx.Original99Report{}, invalid
		}
		return chaintx.PrepareOriginal99Disposition(ctx, database, providers, client, true, guard)
	case "broadcast":
		return chaintx.BroadcastOriginal99Disposition(ctx, database, providers, second, hash, guard)
	case "reconcile":
		return chaintx.ReconcileOriginal99Disposition(ctx, database, providers, guard)
	}
	return chaintx.Original99Report{}, invalid
}
