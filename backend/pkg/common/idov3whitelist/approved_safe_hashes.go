package idov3whitelist

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// ParseApprovedV3SafeCodeHashes accepts deployment-pinned code identities
// only. Core and Job must read the same independently reviewed configuration;
// an approval, project row, Safe response or RPC getter cannot supply them.
func ParseApprovedV3SafeCodeHashes(ownerRaw, governorRaw string) (common.Hash, common.Hash, error) {
	invalid := errors.New("V3 Safe code identity unavailable")
	parse := func(raw string) (common.Hash, bool) {
		if len(raw) != 66 || !strings.HasPrefix(raw, "0x") || raw != strings.ToLower(raw) {
			return common.Hash{}, false
		}
		for _, digit := range raw[2:] {
			if !strings.ContainsRune("0123456789abcdef", digit) {
				return common.Hash{}, false
			}
		}
		hash := common.HexToHash(raw)
		return hash, hash != (common.Hash{})
	}
	owner, ownerOK := parse(ownerRaw)
	governor, governorOK := parse(governorRaw)
	if !ownerOK || !governorOK {
		return common.Hash{}, common.Hash{}, invalid
	}
	return owner, governor, nil
}
