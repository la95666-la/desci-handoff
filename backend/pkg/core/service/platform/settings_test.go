package platform

import "testing"

// 双链结构化下发（安全审计双链拆分）：task 链与 project 链必须独立描述，
// Enabled 语义一致 —— chainID 与合约地址缺任一即视为未配置。
func TestBuildTaskChainInfo(t *testing.T) {
	info := BuildTaskChainInfo(56, "https://bsc-dataseed.bnbchain.org", "0xE029161be55922edf3eC9D222142edf057d196ee")
	if info["Enabled"] != true {
		t.Fatalf("fully configured task chain reported disabled: %v", info)
	}
	if info["ChainID"] != "56" {
		t.Fatalf("chain id = %v, want \"56\"", info["ChainID"])
	}
	if info["CheckinAddress"] != "0xE029161be55922edf3eC9D222142edf057d196ee" {
		t.Fatalf("checkin address = %v", info["CheckinAddress"])
	}

	noAddress := BuildTaskChainInfo(56, "https://bsc-dataseed.bnbchain.org", "")
	if noAddress["Enabled"] != false {
		t.Fatal("task chain without contract address reported enabled")
	}
	if noAddress["ChainID"] != "56" {
		t.Fatalf("chain id should still be surfaced for diagnostics, got %v", noAddress["ChainID"])
	}

	noChain := BuildTaskChainInfo(0, "https://bsc-dataseed.bnbchain.org", "0xE029161be55922edf3eC9D222142edf057d196ee")
	if noChain["Enabled"] != false {
		t.Fatal("task chain without chain id reported enabled")
	}
	if noChain["ChainID"] != "" {
		t.Fatalf("unset chain id must serialize empty, got %v", noChain["ChainID"])
	}
}

func TestBuildProjectChainInfo(t *testing.T) {
	info := BuildProjectChainInfo(97, "https://data-seed-prebsc-1-s1.binance.org:8545", "0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A")
	if info["Enabled"] != true {
		t.Fatalf("fully configured project chain reported disabled: %v", info)
	}
	if info["ChainID"] != "97" || info["ContractAddress"] == "" {
		t.Fatalf("project chain fields = %v", info)
	}

	// 测试服务器双链形态：project 链未配置（IDO 未上线）时 Enabled=false，
	// 前端不得据此落任何运行期合约地址。
	disabled := BuildProjectChainInfo(0, "", "")
	if disabled["Enabled"] != false || disabled["ChainID"] != "" || disabled["ContractAddress"] != "" {
		t.Fatalf("unconfigured project chain must be fully empty/disabled: %v", disabled)
	}
}
