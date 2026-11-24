package chain

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
)

// V3ShadowSource is configuration data only. Until the independent scanner,
// financial materializer, outbox, and reorg recovery are reviewed, Job MUST
// NOT register this address in the V1/V2 event source list or write purchases.
type V3ShadowSource struct {
	ChainID                uint64
	ProxyAddress           common.Address
	ImplementationAddress  common.Address
	ProxyCodeHash          common.Hash
	ImplementationCodeHash common.Hash
	GovernorAddress        common.Address
	StartBlock             uint64
}

// readV3ShadowSource validates an optional complete, inactive V3 identity.
// Unknown keys, partial identity, mismatched chain and reuse of legacy/V2
// proxies fail closed. This is not a trusted runtime code verification or an
// authorization to enable V3 scanning.
func readV3ShadowSource(config *viper.Viper, chainID uint64, v1, v2 common.Address) (*V3ShadowSource, error) {
	if config == nil || chainID == 0 {
		return nil, errors.New("V3 Job source configuration unavailable")
	}
	const prefix = "chain.project.v3."
	keys := []string{"enabled", "chainId", "contractAddress", "implementationAddress", "proxyCodeHash", "implementationCodeHash", "governorAddress", "startBlock"}
	configured := false
	for _, key := range config.AllKeys() {
		if strings.EqualFold(key, "chain.project.v3") {
			return nil, errors.New("V3 Job source configuration malformed")
		}
		if strings.HasPrefix(strings.ToLower(key), prefix) {
			configured = true
			known := false
			for _, allowed := range keys {
				if strings.EqualFold(key, prefix+allowed) {
					known = true
					break
				}
			}
			if !known {
				return nil, errors.New("unknown V3 Job source configuration")
			}
		}
	}
	if !configured {
		return nil, nil
	}
	if strings.ToLower(strings.TrimSpace(config.GetString(prefix+"enabled"))) != "false" {
		return nil, errors.New("V3 Job source must remain disabled until independent materialization exists")
	}
	parsePositive := func(key string) (uint64, error) {
		value := strings.TrimSpace(config.GetString(prefix + key))
		if value == "" || value[0] == '0' || strings.Trim(value, "0123456789") != "" {
			return 0, errors.New("V3 Job source numeric field invalid")
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			return 0, errors.New("V3 Job source numeric field invalid")
		}
		return parsed, nil
	}
	configuredChain, err := parsePositive("chainId")
	if err != nil || configuredChain != chainID {
		return nil, errors.New("V3 Job source chain mismatch")
	}
	start, err := parsePositive("startBlock")
	if err != nil {
		return nil, err
	}
	parseAddress := func(key string) (common.Address, error) {
		value := strings.TrimSpace(config.GetString(prefix + key))
		if len(value) != 42 || !strings.HasPrefix(value, "0x") || value != strings.ToLower(value) || !common.IsHexAddress(value) || common.HexToAddress(value) == (common.Address{}) {
			return common.Address{}, errors.New("V3 Job source address invalid")
		}
		return common.HexToAddress(value), nil
	}
	proxy, err := parseAddress("contractAddress")
	if err != nil {
		return nil, err
	}
	implementation, err := parseAddress("implementationAddress")
	if err != nil {
		return nil, err
	}
	governor, err := parseAddress("governorAddress")
	if err != nil {
		return nil, err
	}
	if proxy == v1 || proxy == v2 || proxy == implementation || proxy == governor || implementation == governor {
		return nil, errors.New("V3 Job source identities overlap")
	}
	parseHash := func(key string) (common.Hash, error) {
		value := strings.TrimSpace(config.GetString(prefix + key))
		if !isExactHash(value) || value != strings.ToLower(value) || common.HexToHash(value) == (common.Hash{}) {
			return common.Hash{}, errors.New("V3 Job source code hash invalid")
		}
		return common.HexToHash(value), nil
	}
	proxyHash, err := parseHash("proxyCodeHash")
	if err != nil {
		return nil, err
	}
	implementationHash, err := parseHash("implementationCodeHash")
	if err != nil {
		return nil, err
	}
	return &V3ShadowSource{ChainID: chainID, ProxyAddress: proxy, ImplementationAddress: implementation,
		ProxyCodeHash: proxyHash, ImplementationCodeHash: implementationHash, GovernorAddress: governor, StartBlock: start}, nil
}
