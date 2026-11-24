package project

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	db "joincare_server/pkg/core/db"
)

// OnchainParamsRequest 是 Admin 专用表单（POST /admin/v1/projects/:id/onchain-params）
// 的载荷。链身份字段（chain_id/合约/费率快照/支付币地址与 decimals）**不可由客户端提交**：
// V2 从受信配置注入；显式 V3 的只读预览从固定部署及链上快照读取。
// 表单本身只表达业务语义，不能供应链身份或签名密钥。
type OnchainParamsRequest struct {
	// Explicit V3 writes require the project-scoped four-eyes Set approval.
	// Omitted version retains the existing V2 review payload for compatibility.
	ContractVersion string `json:"contract_version,omitempty"`
	PaymentToken    string `json:"payment_token" binding:"required"` // 仅 symbol：USDT / USDC
	TokenAddress    string `json:"token_address" binding:"required"`
	TokenSymbol     string `json:"token_symbol"`
	OwnerAddress    string `json:"owner_address" binding:"required"`

	TokenPrice string `json:"token_price" binding:"required"`
	HardCap    string `json:"hard_cap" binding:"required"`
	SoftCap    string `json:"soft_cap" binding:"required"`
	MaxPerUser string `json:"max_per_user" binding:"required"`

	// 时间为 ISO8601（含时区或 Z）；创建上链前仍会后端复核 start > now
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`

	TGEAfterEndMin int               `json:"tge_after_end_min"`
	TGEIntervalMin int               `json:"tge_interval_min"`
	TGE            []OnchainTGEEntry `json:"tge" binding:"required,min=1"`
}

type OnchainTGEEntry struct {
	Percent decimal.Decimal `json:"percent" binding:"required"`
}

// OnchainParamsCurrent 是 GET /admin/v1/projects/:id/onchain-params 的响应：
// 回显项目当前已保存的 Set 参数，供 Admin 表单回填。未设置时 Configured=false。
type OnchainParamsCurrent struct {
	Configured   bool              `json:"configured"`
	Locked       bool              `json:"locked"`
	Status       string            `json:"status"`
	FundingMode  string            `json:"funding_mode"`
	Version      string            `json:"contract_version"`
	ChainID      uint64            `json:"chain_id"`
	Contract     string            `json:"contract_address"`
	PaymentToken string            `json:"payment_token"`
	TokenAddress string            `json:"token_address"`
	TokenSymbol  string            `json:"token_symbol"`
	OwnerAddress string            `json:"owner_address"`
	TokenPrice   string            `json:"token_price"`
	HardCap      string            `json:"hard_cap"`
	SoftCap      string            `json:"soft_cap"`
	MaxPerUser   string            `json:"max_per_user"`
	StartTime    string            `json:"start_time"`
	EndTime      string            `json:"end_time"`
	TGE          []OnchainTGEEntry `json:"tge"`
	TGEAfterEnd  int               `json:"tge_after_end_min"`
	TGEInterval  int               `json:"tge_interval_min"`
}

// GetOnchainParams 读取项目当前 Set 参数（表单回填用）。
// 时间经 GORM loc=Local 读出为带偏移的 time.Time，RFC3339 输出由前端 dayjs 还原为浏览器本地时间。
func GetOnchainParams(projectID int64) (*OnchainParamsCurrent, error) {
	var project entity.Project
	if err := db.Mysql().Select("project_id", "status", "onchain_locked", "contract_version", "funding_mode", "chain_id", "contract_address",
		"payment_token", "token_address", "token_symbol", "owner_address",
		"token_price", "hard_cap", "soft_cap", "max_per_user", "start_time", "end_time").
		Where("project_id = ?", projectID).Take(&project).Error; err != nil {
		return nil, err
	}
	current := &OnchainParamsCurrent{
		Configured: (constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) ||
			constant.IsExactIDOV3ProjectIdentity(project.FundingMode, project.ContractVersion)) && project.TokenPrice.IsPositive(),
		Locked:       project.OnchainLocked,
		Status:       project.Status,
		FundingMode:  project.FundingMode,
		Version:      project.ContractVersion,
		ChainID:      project.ChainID,
		Contract:     project.ContractAddress,
		PaymentToken: project.PaymentToken,
		TokenAddress: project.TokenAddress,
		TokenSymbol:  project.TokenSymbol,
		OwnerAddress: project.OwnerAddress,
		TokenPrice:   project.TokenPrice.String(),
		HardCap:      project.HardCap.String(),
		SoftCap:      project.SoftCap.String(),
		MaxPerUser:   project.MaxPerUser.String(),
	}
	if !project.StartTime.IsZero() {
		current.StartTime = project.StartTime.Format(time.RFC3339)
	}
	if !project.EndTime.IsZero() {
		current.EndTime = project.EndTime.Format(time.RFC3339)
	}
	if !current.Configured {
		return current, nil
	}
	var tges []entity.ProjectTge
	if err := db.Mysql().Where("project_id = ?", projectID).Order("`index` ASC").Find(&tges).Error; err != nil {
		return nil, err
	}
	for _, t := range tges {
		current.TGE = append(current.TGE, OnchainTGEEntry{Percent: t.TgePercent})
	}
	if len(tges) > 0 {
		current.TGEAfterEnd = int(tges[0].VestingTime.Sub(project.EndTime).Minutes())
	}
	if len(tges) > 1 {
		gap := int(tges[1].VestingTime.Sub(tges[0].VestingTime).Minutes())
		uniform := true
		for i := 2; i < len(tges); i++ {
			if int(tges[i].VestingTime.Sub(tges[i-1].VestingTime).Minutes()) != gap {
				uniform = false
				break
			}
		}
		if uniform {
			current.TGEInterval = gap
		}
	}
	return current, nil
}

// OnchainParamsPreview 是 dry-run 等价物：换算后的 raw 值、时间表与 TGE 计划。
// 提交审批前前端必须先展示本预览，Admin 确认后才允许发起审批。
type OnchainParamsPreview struct {
	ProjectID    int64                    `json:"project_id"`
	Chain        map[string]interface{}   `json:"chain"`
	PaymentToken map[string]interface{}   `json:"payment_token"`
	ProjectToken map[string]interface{}   `json:"project_token"`
	Amounts      map[string]interface{}   `json:"amounts"`
	Window       map[string]interface{}   `json:"window"`
	TGE          []map[string]interface{} `json:"tge"`
	SoftCapToken string                   `json:"soft_cap_token_estimate"`
	HardCapToken string                   `json:"hard_cap_token_estimate"`
}

// resolvedOnchainParams 是校验通过的归一化结果。
type resolvedOnchainParams struct {
	Trusted          trustedPaymentToken
	Req              OnchainParamsRequest
	TokenDecimals    int
	PriceRaw         string
	HardRaw          string
	SoftRaw          string
	MaxRaw           string
	MinPurchaseRaw   *string
	Start            time.Time
	End              time.Time
	TGERows          []entity.ProjectTge
	TokenSymbolValue string
}

// resolveOnchainParams 执行 Set 阶段的全部静态校验（等价 ido_prepare_set.py），
// 含链上代币身份核查（decimals 与字节码存在性由 verifyProjectTokensOnChain 读取）。
func resolveOnchainParams(projectID int64, req OnchainParamsRequest) (*resolvedOnchainParams, error) {
	trusted, err := resolveTrustedPaymentToken(strings.TrimSpace(req.PaymentToken))
	if err != nil {
		return nil, err
	}
	return resolveOnchainParamsWithInspector(projectID, req, trusted, func(token string, trusted trustedPaymentToken) (int, error) {
		_, decimals, err := verifyProjectTokensOnChain(token, 0, trusted)
		return decimals, err
	})
}

// The caller selects the trusted runtime and token inspector explicitly. The
// V2 wrapper above remains V2-only; V3 must supply its independently checked
// V3 runtime instead of borrowing the V2 proxy or payment-token configuration.
func resolveOnchainParamsWithInspector(projectID int64, req OnchainParamsRequest, trusted trustedPaymentToken,
	inspect func(string, trustedPaymentToken) (int, error)) (*resolvedOnchainParams, error) {
	if inspect == nil || projectID <= 0 || trusted.ChainID == 0 || trusted.Decimals <= 0 || trusted.Decimals > 18 {
		return nil, errors.New("on-chain parameter runtime unavailable")
	}
	tokenAddress := strings.TrimSpace(req.TokenAddress)
	if !common.IsHexAddress(tokenAddress) || common.HexToAddress(tokenAddress) == (common.Address{}) {
		return nil, errors.New("project token address is invalid")
	}
	if strings.EqualFold(tokenAddress, trusted.Address) {
		return nil, errors.New("project token address equals the payment token")
	}
	if !common.IsHexAddress(req.OwnerAddress) || common.HexToAddress(req.OwnerAddress) == (common.Address{}) {
		return nil, errors.New("owner address is invalid")
	}

	// 返回值：(验证后的代币地址, 链上 decimals, err)——参照 service.go create-onchain 的用法
	decimals, err := inspect(tokenAddress, trusted)
	if err != nil {
		return nil, fmt.Errorf("project token inspection failed: %w", err)
	}
	if decimals <= 0 || decimals > 18 {
		return nil, fmt.Errorf("project token decimals invalid: %d", decimals)
	}

	// 金额：十进制字符串，服务端换算 raw（拒绝科学计数法/负数/溢出精度）
	parseAmount := func(name, value string) (decimal.Decimal, string, error) {
		d, perr := decimal.NewFromString(strings.TrimSpace(value))
		if perr != nil || d.Sign() <= 0 {
			return decimal.Decimal{}, "", fmt.Errorf("%s must be a positive decimal string", name)
		}
		raw, rerr := money.DisplayToRaw(d, uint8(trusted.Decimals))
		if rerr != nil {
			return decimal.Decimal{}, "", fmt.Errorf("%s: %w", name, rerr)
		}
		return d, raw, nil
	}
	priceDisp, priceRaw, err := parseAmount("token_price", req.TokenPrice)
	if err != nil {
		return nil, err
	}
	_ = priceDisp // 仅用于下述 caps 比较（price 不参与）
	hardDisp, hardRaw, err := parseAmount("hard_cap", req.HardCap)
	if err != nil {
		return nil, err
	}
	softDisp, softRaw, err := parseAmount("soft_cap", req.SoftCap)
	if err != nil {
		return nil, err
	}
	maxDisp, maxRaw, err := parseAmount("max_per_user", req.MaxPerUser)
	if err != nil {
		return nil, err
	}

	if softDisp.GreaterThan(hardDisp) {
		return nil, fmt.Errorf("soft_cap %s > hard_cap %s", req.SoftCap, req.HardCap)
	}
	if maxDisp.GreaterThan(hardDisp) {
		return nil, fmt.Errorf("max_per_user %s > hard_cap %s", req.MaxPerUser, req.HardCap)
	}

	start, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartTime))
	if err != nil {
		return nil, errors.New("start_time must be ISO8601 with timezone (e.g. 2026-08-24T10:00:00Z)")
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndTime))
	if err != nil {
		return nil, errors.New("end_time must be ISO8601 with timezone")
	}
	if !start.After(time.Now()) {
		return nil, fmt.Errorf("start_time %s is not in the future", req.StartTime)
	}
	if !end.After(start) {
		return nil, errors.New("end_time must be after start_time")
	}

	// TGE：1~10 期，百分比合计严格 100，时间从 end + tge_after_end_min 起按 interval 递推
	if len(req.TGE) > 10 {
		return nil, errors.New("tge must contain 1 to 10 entries")
	}
	total := decimal.Zero
	tgeRows := make([]entity.ProjectTge, 0, len(req.TGE))
	t := end.Add(time.Duration(req.TGEAfterEndMin) * time.Minute)
	for i, entry := range req.TGE {
		if entry.Percent.Sign() <= 0 {
			return nil, fmt.Errorf("tge[%d].percent must be positive", i)
		}
		total = total.Add(entry.Percent)
		tgeRows = append(tgeRows, entity.ProjectTge{
			ProjectID: int(projectID), Index: i + 1,
			TgePercent: entry.Percent, VestingPercent: entry.Percent,
			VestingInterval: uint32(req.TGEIntervalMin), VestingTime: t,
			VestingDuration: 0, IsNotification: 1,
		})
		if req.TGEIntervalMin > 0 {
			t = t.Add(time.Duration(req.TGEIntervalMin) * time.Minute)
		}
	}
	if !total.Equal(decimal.NewFromInt(100)) {
		return nil, fmt.Errorf("tge percents sum to %s, want 100", total.String())
	}

	tokenSymbol := strings.TrimSpace(req.TokenSymbol)
	if tokenSymbol == "" {
		tokenSymbol = "TOKEN"
	}

	return &resolvedOnchainParams{
		Trusted: trusted, Req: req, TokenDecimals: decimals,
		PriceRaw: priceRaw, HardRaw: hardRaw, SoftRaw: softRaw, MaxRaw: maxRaw,
		Start: start, End: end, TGERows: tgeRows, TokenSymbolValue: tokenSymbol,
	}, nil
}

// PrepareOnchainParamsPreview 组装 dry-run 预览（只读，不写库）。
func PrepareOnchainParamsPreview(projectID int64, req OnchainParamsRequest) (*OnchainParamsPreview, error) {
	if req.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		return prepareV3OnchainParamsPreview(projectID, req)
	}
	if req.ContractVersion != "" && req.ContractVersion != constant.ContractVersionV2Stablecoin {
		return nil, errors.New("project contract version unavailable")
	}
	res, err := resolveOnchainParams(projectID, req)
	if err != nil {
		return nil, err
	}
	return previewResolvedOnchainParams(projectID, res), nil
}

func previewResolvedOnchainParams(projectID int64, res *resolvedOnchainParams) *OnchainParamsPreview {
	// soft/hard 对应的项目币数量估算（同合约 activate 校验口径：cap*10^tokenDec/price）
	tokenDecimalDiv := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(res.TokenDecimals)), nil)
	estimate := func(capRaw string) string {
		capBig, _ := new(big.Int).SetString(capRaw, 10)
		priceBig, _ := new(big.Int).SetString(res.PriceRaw, 10)
		if priceBig.Sign() <= 0 {
			return ""
		}
		out := new(big.Int).Mul(capBig, tokenDecimalDiv)
		out.Div(out, priceBig)
		return out.String()
	}
	tge := make([]map[string]interface{}, 0, len(res.TGERows))
	for _, row := range res.TGERows {
		tge = append(tge, map[string]interface{}{
			"index": row.Index, "percent": row.VestingPercent.String(),
			"time_utc": row.VestingTime.UTC().Format(time.RFC3339),
		})
	}
	return &OnchainParamsPreview{
		ProjectID: projectID,
		Chain: map[string]interface{}{
			"chain_id": res.Trusted.ChainID, "contract": res.Trusted.Contract,
			"fee_receiver": res.Trusted.FeeReceiver, "success_fee_rate": res.Trusted.SuccessFeeRate,
		},
		PaymentToken: map[string]interface{}{
			"symbol": res.Trusted.Symbol, "address": res.Trusted.Address, "decimals": res.Trusted.Decimals,
		},
		ProjectToken: map[string]interface{}{
			"symbol": res.TokenSymbolValue, "address": res.Req.TokenAddress, "decimals": res.TokenDecimals,
			"owner": res.Req.OwnerAddress,
		},
		Amounts: map[string]interface{}{
			"token_price": res.Req.TokenPrice, "token_price_raw": res.PriceRaw,
			"hard_cap": res.Req.HardCap, "hard_cap_raw": res.HardRaw,
			"soft_cap": res.Req.SoftCap, "soft_cap_raw": res.SoftRaw,
			"max_per_user": res.Req.MaxPerUser, "max_per_user_raw": res.MaxRaw,
		},
		Window: map[string]interface{}{
			"start_utc": res.Start.UTC().Format(time.RFC3339),
			"end_utc":   res.End.UTC().Format(time.RFC3339),
		},
		TGE:          tge,
		SoftCapToken: estimate(res.SoftRaw),
		HardCapToken: estimate(res.HardRaw),
	}
}

// ApplyOnchainParamsTx 在审批通过后执行写入（等价脚本的守卫 + 单事务重写）。
// 时间按 GORM loc=Local 写入（后端容器墙钟语义，与脚本 db_tz_offset_hours 行为一致）。
func ApplyOnchainParamsTx(tx *gorm.DB, projectID int64, req OnchainParamsRequest) (string, error) {
	if req.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		return applyV3OnchainParamsTx(tx, projectID, req)
	}
	if req.ContractVersion != "" && req.ContractVersion != constant.ContractVersionV2Stablecoin {
		return "", errors.New("project contract version unavailable")
	}
	res, err := resolveOnchainParams(projectID, req)
	if err != nil {
		return "", err
	}
	// 守卫：必须存在、未锁定、状态白名单
	var current entity.Project
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("project_id,status,onchain_locked,funding_mode,contract_version,payment_token,payment_token_decimals,min_purchase_amount_raw,min_purchase_draft_amount,min_purchase_draft_token").
		Where("project_id = ?", projectID).Take(&current).Error; err != nil {
		return "", err
	}
	if current.OnchainLocked {
		return "", errors.New("project is already locked to an on-chain identity")
	}
	if current.ContractVersion != constant.ContractVersionUnassigned && current.ContractVersion != constant.ContractVersionV2Stablecoin {
		return "", errors.New("project is assigned to a non-V2 contract version")
	}
	if current.FundingMode != "" && current.FundingMode != constant.FundingModeCappedPresale {
		return "", errors.New("project is assigned to a non-capped funding mode")
	}
	// The Setup draft has no authoritative decimals or wallet cap. Apply only
	// after comparison with the trusted Set token and its validated maximum.
	res.MinPurchaseRaw, err = resolveMinimumForSet(current, res.Trusted, res.MaxRaw)
	if err != nil {
		return "", err
	}
	switch current.Status {
	case constant.ProjectStatusPreLaunch, constant.ProjectStatusSet, constant.ProjectStatusWarmup, constant.ProjectStatusSetup:
	default:
		return "", fmt.Errorf("project status %s does not allow on-chain parameter updates", current.Status)
	}

	result := tx.Model(&entity.Project{}).
		Where("project_id = ? AND onchain_locked = ? AND funding_mode = ? AND contract_version IN ?", projectID, false, constant.FundingModeCappedPresale, []string{constant.ContractVersionUnassigned, constant.ContractVersionV2Stablecoin}).
		Updates(map[string]interface{}{
			"status":                    constant.ProjectStatusSet,
			"funding_mode":              constant.FundingModeCappedPresale,
			"contract_version":          constant.ContractVersionV2Stablecoin,
			"chain_id":                  res.Trusted.ChainID,
			"contract_address":          res.Trusted.Contract,
			"token_address":             strings.ToLower(common.HexToAddress(res.Req.TokenAddress).Hex()),
			"token_symbol":              res.TokenSymbolValue,
			"token_decimals":            res.TokenDecimals,
			"payment_token":             res.Trusted.Symbol,
			"payment_token_address":     res.Trusted.Address,
			"payment_token_decimals":    res.Trusted.Decimals,
			"owner_address":             strings.ToLower(common.HexToAddress(res.Req.OwnerAddress).Hex()),
			"fee_receiver":              res.Trusted.FeeReceiver,
			"success_fee_rate":          res.Trusted.SuccessFeeRate,
			"token_price":               res.Req.TokenPrice,
			"token_price_raw":           res.PriceRaw,
			"hard_cap":                  res.Req.HardCap,
			"hard_cap_raw":              res.HardRaw,
			"soft_cap":                  res.Req.SoftCap,
			"soft_cap_raw":              res.SoftRaw,
			"max_per_user":              res.Req.MaxPerUser,
			"max_per_user_raw":          res.MaxRaw,
			"min_purchase_amount_raw":   res.MinPurchaseRaw,
			"min_purchase_draft_amount": nil,
			"min_purchase_draft_token":  nil,
			"start_time":                res.Start,
			"end_time":                  res.End,
			"tge_time":                  res.TGERows[0].VestingTime,
		})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", errors.New("project changed while applying on-chain parameters")
	}
	if err := tx.Where("project_id = ?", projectID).Delete(&entity.ProjectTge{}).Error; err != nil {
		return "", err
	}
	for i := range res.TGERows {
		res.TGERows[i].ProjectID = int(projectID)
		if err := tx.Create(&res.TGERows[i]).Error; err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("project %d set to v2_stablecoin on chain %d; price=%s soft=%s hard=%s max=%s; %d tge entries",
		projectID, res.Trusted.ChainID, res.PriceRaw, res.SoftRaw, res.HardRaw, res.MaxRaw, len(res.TGERows)), nil
}
