package idov3candidate

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// AuditInertProjectIdentity reads an exactly bound, locked V3 project and its
// unverified runtime in one repeatable-read transaction. This is a diagnostic
// for M2 preparation, NOT a trusted runtime admission or scanner registration.
// Even a matching on-chain observation cannot promote these rows to trusted.
// The distinct type prevents accidentally passing these unverified rows as
// the caller-supplied trusted RuntimeIdentity/ExpectedProjectBinding to readers.
type InertProjectIdentityAudit struct {
	ChainID                uint64
	ProjectID              uint64
	ProxyAddress           common.Address
	ProjectToken           common.Address
	PaymentToken           common.Address
	Implementation         common.Address
	Governor               common.Address
	StartBlock             uint64
	ProxyCodeHash          common.Hash
	ImplementationCodeHash common.Hash
}

func AuditInertProjectIdentity(ctx context.Context, database *gorm.DB, scope idov3whitelist.Scope) (InertProjectIdentityAudit, error) {
	var empty InertProjectIdentityAudit
	if database == nil || scope.ChainID == nil || !scope.ChainID.IsUint64() || scope.ChainID.Sign() <= 0 ||
		scope.ProjectID == nil || !scope.ProjectID.IsUint64() || scope.ProjectID.Sign() <= 0 || scope.ProxyAddress == (common.Address{}) {
		return empty, errors.New("V3 project identity audit scope invalid")
	}
	tx := database.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if tx.Error != nil {
		return empty, errors.New("V3 project identity audit unavailable")
	}
	defer tx.Rollback()
	type identityRow struct {
		ProjectToken       string `gorm:"column:project_token"`
		PaymentToken       string `gorm:"column:payment_token"`
		Implementation     string `gorm:"column:implementation_address"`
		Governor           string `gorm:"column:governor_address"`
		StartBlock         uint64 `gorm:"column:start_block"`
		ProxyHash          string `gorm:"column:proxy_code_hash"`
		ImplementationHash string `gorm:"column:implementation_code_hash"`
	}
	// Do not infer an implementation or a token from a submitted log. Exact
	// joined persistence identity is mandatory, including onchain_locked.
	const query = `SELECT p.token_address AS project_token,p.payment_token_address AS payment_token,
		r.implementation_address,r.governor_address,r.start_block,r.proxy_code_hash,r.implementation_code_hash
		FROM projects p
		JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
			AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_runtime r ON r.chain_id=b.chain_id AND r.proxy_address=b.proxy_address
			AND r.contract_version=b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.funding_mode=BINARY 'capped_presale'
			AND p.onchain_locked=1 AND r.state='unverified' LIMIT 2`
	var rows []identityRow
	if err := tx.Raw(query, scope.ProjectID.Uint64(), scope.ChainID.Uint64(), strings.ToLower(scope.ProxyAddress.Hex())).Scan(&rows).Error; err != nil || len(rows) != 1 {
		return empty, errors.New("V3 project identity audit unavailable or ambiguous")
	}
	row := rows[0]
	address := func(text string) (common.Address, bool) {
		if len(text) != 42 || text != strings.ToLower(text) || !common.IsHexAddress(text) {
			return common.Address{}, false
		}
		a := common.HexToAddress(text)
		return a, a != (common.Address{}) && a != scope.ProxyAddress
	}
	hash := func(text string) (common.Hash, bool) {
		if !canonicalHash.MatchString(text) {
			return common.Hash{}, false
		}
		h := common.HexToHash(text)
		return h, h != (common.Hash{})
	}
	projectToken, okProject := address(row.ProjectToken)
	paymentToken, okPayment := address(row.PaymentToken)
	implementation, okImplementation := address(row.Implementation)
	governor, okGovernor := address(row.Governor)
	proxyHash, okProxyHash := hash(row.ProxyHash)
	implementationHash, okImplementationHash := hash(row.ImplementationHash)
	if !okProject || !okPayment || !okImplementation || !okGovernor || row.StartBlock == 0 || !okProxyHash || !okImplementationHash ||
		projectToken == paymentToken || implementation == projectToken || implementation == paymentToken ||
		governor == implementation || governor == projectToken || governor == paymentToken {
		return empty, errors.New("V3 project identity audit row invalid")
	}
	if err := tx.Commit().Error; err != nil {
		return empty, errors.New("V3 project identity audit unavailable")
	}
	return InertProjectIdentityAudit{ChainID: scope.ChainID.Uint64(), ProxyAddress: scope.ProxyAddress,
		ProjectID: scope.ProjectID.Uint64(), ProjectToken: projectToken, PaymentToken: paymentToken,
		Implementation: implementation, Governor: governor, StartBlock: row.StartBlock,
		ProxyCodeHash: proxyHash, ImplementationCodeHash: implementationHash}, nil
}
