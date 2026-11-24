package platform

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

func Settings() map[string]interface{} {
	res := make(map[string]interface{}, 8)
	err := db.Redis().GetByJson(context.Background(), constant.KEY_PLATFORM_SETTINGS, &res)
	if err != nil {
		infos := []entity.Settings{}
		err = db.Mysql().Model(&entity.Settings{}).
			Select("`key`,`value`,`type`").Where("`key` in ?", []string{"PlatformFee"}).
			Scan(&infos).Error
		if err != nil {
			return nil
		}
		for _, info := range infos {
			res[info.Key] = info.Value
		}
		db.Redis().SetByJson(context.Background(), constant.KEY_PLATFORM_SETTINGS, &res, 86400*time.Second)
	}
	chainPrefix := "chain.project"
	if strings.TrimSpace(viper.GetString(chainPrefix+".url")) == "" || viper.GetUint64(chainPrefix+".chainID") == 0 {
		chainPrefix = "chain"
	}
	projectContract := strings.TrimSpace(viper.GetString("chain.project.contractAddress"))
	if viper.GetString("chain.project.v1.enabled") == "false" {
		projectContract = strings.TrimSpace(viper.GetString("chain.project.v2.contractAddress"))
	}
	res["ChainEnvironment"] = viper.GetString("application.environment")
	res["ChainUrl"] = viper.GetString(chainPrefix + ".url")
	res["ChainID"] = viper.GetString(chainPrefix + ".chainID")
	res["JccAddress"] = viper.GetString("chain.jccAddress")
	res["USDTAddress"] = viper.GetString("chain.project.v2.paymentTokens.USDT.address")
	res["USDCAddress"] = viper.GetString("chain.project.v2.paymentTokens.USDC.address")
	res["ContractAddress"] = projectContract
	res["IDOV2ContractAddress"] = viper.GetString("chain.project.v2.contractAddress")
	// CheckIn (task) 合约地址下发给前端；避免前端每次换合约都要 rebuild
	res["CheckinAddress"] = viper.GetString("chain.task.checkinAddress")

	// 双链结构化下发（安全审计双链拆分）：task 链（CheckIn）与 project 链（Launchpad/IDO）
	// 是两条独立链。旧扁平字段把 project 的 ChainID/ChainUrl 和 task 的 CheckinAddress
	// 拼在同一对象里，前端曾把 CheckIn 地址解释为 project 链合约。结构化对象让前端
	// 分别消费；旧字段保留一个发布周期供回滚。
	res["TaskChain"] = BuildTaskChainInfo(
		viper.GetUint64("chain.task.chainID"),
		strings.TrimSpace(viper.GetString("chain.task.url")),
		strings.TrimSpace(viper.GetString("chain.task.checkinAddress")),
	)
	res["ProjectChain"] = BuildProjectChainInfo(
		viper.GetUint64(chainPrefix+".chainID"),
		strings.TrimSpace(viper.GetString(chainPrefix+".url")),
		projectContract,
	)
	return res
}

// BuildTaskChainInfo 描述 CheckIn 所在链。Enabled 要求 chainID 与合约地址同时存在；
// 缺任一视为该链未配置（前端保持构建期默认，不落任何运行期地址）。
func BuildTaskChainInfo(chainID uint64, url, checkinAddress string) map[string]interface{} {
	return map[string]interface{}{
		"Enabled":        chainID != 0 && checkinAddress != "",
		"ChainID":        formatChainID(chainID),
		"ChainUrl":       url,
		"CheckinAddress": checkinAddress,
	}
}

// BuildProjectChainInfo 描述 Launchpad/IDO 所在链。Enabled 语义与 TaskChain 一致。
func BuildProjectChainInfo(chainID uint64, url, contractAddress string) map[string]interface{} {
	return map[string]interface{}{
		"Enabled":         chainID != 0 && contractAddress != "",
		"ChainID":         formatChainID(chainID),
		"ChainUrl":        url,
		"ContractAddress": contractAddress,
	}
}

func formatChainID(chainID uint64) string {
	if chainID == 0 {
		return ""
	}
	return strconv.FormatUint(chainID, 10)
}
