package project

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

// ApprovedV3ModeEnabled is a coordinated, default-off rollout fence. A
// partial Core configuration may not accept a four-eyes Set which Job or the
// wallet-proof reader cannot consume. The job has an independent startup
// check for the same schema, endpoint and trust roots. No legacy per-project
// release manifest is consulted in this mode.
func ApprovedV3ModeConfigured() bool { return viper.GetBool("idoV3Approved.enabled") }

func ApprovedV3ModeEnabled() bool {
	if !ApprovedV3ModeConfigured() ||
		!viper.GetBool("idoV3TokenAdmission.enabled") ||
		!viper.GetBool("idoV3Execution.enabled") ||
		!viper.GetBool("idoV3Lifecycle.enabled") ||
		!viper.GetBool("idoV3PolicyPreview.enabled") ||
		!viper.GetBool("idoV3PolicyReview.enabled") ||
		!viper.GetBool("idoV3PolicyExecution.enabled") ||
		!viper.GetBool("idoV3PolicyProof.enabled") ||
		!viper.GetBool("idoV3PolicyJournal.enabled") ||
		viper.GetBool("idoV3Scanner.enabled") {
		return false
	}
	if idov3whitelist.ValidateApprovedV3RPCEndpoint(strings.TrimSpace(viper.GetString("idoV3TokenAdmission.primaryRPC"))) != nil {
		return false
	}
	_, _, err := approvedV3SafeHashes()
	return err == nil
}

// ApprovedV3ModeReady refuses partial schema installation. This check is
// intentionally repeated at each entry point; a config toggle alone cannot
// authorize a Set or a signed outbox attempt before the immutable evidence
// store and its database fences exist.
func ApprovedV3ModeReady() bool {
	if !ApprovedV3ModeEnabled() || !approvedV3ExecutionIdentityConfigured() || db.Mysql() == nil {
		return false
	}
	client := chainManager.GetClient(constant.ChainProjectV3)
	if client == nil || client.PrivateKey == nil || client.ChainID == nil ||
		!client.ChainID.IsUint64() || client.ChainID.Uint64() != 56 ||
		client.ContractAddress != common.HexToAddress(v3TokenProxy) ||
		client.Address != common.HexToAddress(v3DiagnosticOperator) ||
		crypto.PubkeyToAddress(client.PrivateKey.PublicKey) != client.Address {
		return false
	}
	return idov3whitelist.ApprovedV3SchemaReady(db.Mysql()) == nil
}

// Refuse an approved Set before it can become an immutable binding if Core
// cannot later construct a create with the separately deployed V3 operator.
// Checking this at request time also covers a configuration reload after
// startup; startup still verifies possession of the actual signing key.
func approvedV3ExecutionIdentityConfigured() bool {
	proxyRaw := strings.TrimSpace(viper.GetString("idoV3Execution.proxyAddress"))
	operatorRaw := strings.TrimSpace(viper.GetString("idoV3Execution.operatorAddress"))
	proxy := common.HexToAddress(v3TokenProxy)
	return common.IsHexAddress(proxyRaw) && common.IsHexAddress(operatorRaw) &&
		viper.GetInt64("chain.project.chainID") == 56 &&
		common.HexToAddress(proxyRaw) == proxy &&
		common.HexToAddress(operatorRaw) == common.HexToAddress(v3DiagnosticOperator) &&
		common.HexToAddress(strings.TrimSpace(viper.GetString("chain.project.v2.contractAddress"))) != proxy
}

// ApprovedV3RouteReady allows the legacy routes to remain isolated until the
// new schema, code hashes, policy journal and scanner are all configured.
// Callers MUST reject an explicitly configured but incomplete mode; never
// fall back to a signed single-project release for the same bound project.
func ApprovedV3RouteReady() bool { return ApprovedV3ModeReady() }
