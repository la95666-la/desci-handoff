package project

import "github.com/shopspring/decimal"

type ListResponse struct {
	Cursor int         `json:"cursor"` // 游标 翻页使用
	List   []BriefInfo `json:"list"`
}

type BriefInfo struct {
	ProjectID               int              `json:"projectID,string"`
	Name                    string           `json:"name"`
	Logo                    File             `json:"logo"`
	Tagline                 string           `json:"tagline"`
	Tags                    []string         `json:"tags"`
	Status                  string           `json:"status"`
	PaymentToken            string           `json:"paymentToken"`
	PaymentTokenAddress     string           `json:"paymentTokenAddress"`
	PaymentTokenDecimals    int              `json:"paymentTokenDecimals"`
	ChainID                 uint64           `json:"chainId"`
	ContractAddress         string           `json:"contractAddress"`
	ContractVersion         string           `json:"contractVersion"`
	FundingMode             string           `json:"fundingMode,omitempty"`
	ChainStatus             string           `json:"chainStatus,omitempty"`
	SaleTokenAmount         string           `json:"saleTokenAmount,omitempty"`
	SaleTokenAmountRaw      string           `json:"saleTokenAmountRaw,omitempty"`
	TotalTokensDeposited    string           `json:"totalTokensDeposited,omitempty"`
	TotalTokensDepositedRaw string           `json:"totalTokensDepositedRaw,omitempty"`
	OnchainLocked           bool             `json:"onchainLocked"`
	FeeReceiver             string           `json:"feeReceiver"`
	SuccessFeeRate          uint16           `json:"successFeeRate"`
	PlatformFeeWithdrawn    bool             `json:"platformFeeWithdrawn"`
	TokenSymbol             string           `json:"tokenSymbol"`
	TokenAddress            string           `json:"tokenAddress"`
	TokenDecimals           int              `json:"tokenDecimals"`
	SoldToken               string           `json:"soldToken"`
	TokenPrice              decimal.Decimal  `json:"tokenPrice"`
	TokenPriceRaw           string           `json:"tokenPriceRaw"`
	HardCap                 decimal.Decimal  `json:"hardCap"`
	HardCapRaw              string           `json:"hardCapRaw"`
	SoftCap                 decimal.Decimal  `json:"softCap"`
	SoftCapRaw              string           `json:"softCapRaw"`
	MaxPerUser              decimal.Decimal  `json:"maxPerUser"`
	MaxPerUserRaw           string           `json:"maxPerUserRaw"`
	TotalRaised             *decimal.Decimal `json:"totalRaised"`
	FundingDataStatus       string           `json:"fundingDataStatus,omitempty"`
	TotalRaisedRaw          string           `json:"totalRaisedRaw"`
	TokenWithdrawn          bool             `json:"tokenWithdrawn"`
	FundWithdrawn           bool             `json:"fundWithdrawn"`
	StartTime               int64            `json:"startTime"`
	EndTime                 int64            `json:"endTime"`
	TgeList                 []TgeInfo        `json:"tgeList"`
	// Sprint D 列表展示字段
	Category  string `json:"category"`
	CoverURL  string `json:"coverUrl"`
	LaunchAt  int64  `json:"launchAt"`
	TaglineZh string `json:"taglineZh"`
}

type File struct {
	Url      string `json:"url"`
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type TgeInfo struct {
	TgeTime         int64           `json:"tgeTime"`
	TgePercent      decimal.Decimal `json:"tgePercent"`
	VestingInterval int             `json:"vestingInterval"`
	VestingPercent  decimal.Decimal `json:"vestingPercent"`
	VestingDuration int             `json:"vestingDuration"`
}

type DetailResponse struct {
	Name                    string           `json:"name"`
	Logo                    File             `json:"logo"`
	Banner                  File             `json:"banner"`
	Tagline                 string           `json:"tagline"`
	Tags                    []string         `json:"tags"`
	Description             string           `json:"description"`
	TeamInfo                string           `json:"teamInfo"`
	Roadmap                 string           `json:"roadmap"`
	Partners                string           `json:"partners"`
	TokenAddress            string           `json:"tokenAddress"`
	TokenSymbol             string           `json:"tokenSymbol"`
	TokenDecimals           int              `json:"tokenDecimals"`
	PaymentToken            string           `json:"paymentToken"`
	PaymentTokenAddress     string           `json:"paymentTokenAddress"`
	PaymentTokenDecimals    int              `json:"paymentTokenDecimals"`
	ChainID                 uint64           `json:"chainId"`
	ContractAddress         string           `json:"contractAddress"`
	ContractVersion         string           `json:"contractVersion"`
	FundingMode             string           `json:"fundingMode,omitempty"`
	ChainStatus             string           `json:"chainStatus,omitempty"`
	SaleTokenAmount         string           `json:"saleTokenAmount,omitempty"`
	SaleTokenAmountRaw      string           `json:"saleTokenAmountRaw,omitempty"`
	TotalTokensDeposited    string           `json:"totalTokensDeposited,omitempty"`
	TotalTokensDepositedRaw string           `json:"totalTokensDepositedRaw,omitempty"`
	OnchainLocked           bool             `json:"onchainLocked"`
	FeeReceiver             string           `json:"feeReceiver"`
	SuccessFeeRate          uint16           `json:"successFeeRate"`
	PlatformFeeWithdrawn    bool             `json:"platformFeeWithdrawn"`
	Status                  string           `json:"status"`
	TokenPrice              decimal.Decimal  `json:"tokenPrice"`
	TokenPriceRaw           string           `json:"tokenPriceRaw"`
	HardCap                 decimal.Decimal  `json:"hardCap"`
	HardCapRaw              string           `json:"hardCapRaw"`
	SoftCap                 decimal.Decimal  `json:"softCap"`
	SoftCapRaw              string           `json:"softCapRaw"`
	MaxPerUser              decimal.Decimal  `json:"maxPerUser"`
	MaxPerUserRaw           string           `json:"maxPerUserRaw"`
	MinPurchaseAmount       string           `json:"minPurchaseAmount"`
	MinPurchaseAmountRaw    string           `json:"minPurchaseAmountRaw"`
	TotalRaised             *decimal.Decimal `json:"totalRaised"`
	FundingDataStatus       string           `json:"fundingDataStatus,omitempty"`
	TotalRaisedRaw          string           `json:"totalRaisedRaw"`
	StartTime               int64            `json:"startTime"`
	EndTime                 int64            `json:"endTime"`
	TgeList                 []TgeInfo        `json:"tgeList"`

	// Sprint A/D 展示型字段（Phase 1 内容驱动，Phase 2 交易时补 stage 联动）
	Category      string `json:"category"`
	CoverURL      string `json:"coverUrl"`
	CoverVideoURL string `json:"coverVideoUrl"`
	LaunchAt      int64  `json:"launchAt"` // unix，0 表示未设
	StoryHTML     string `json:"storyHtml"`
	ResearchHTML  string `json:"researchHtml"`
	RisksHTML     string `json:"risksHtml"`
	// 双语并列（繁體中文）；前端根据 language pick，空则 fallback 到英语版
	TaglineZh      string `json:"taglineZh"`
	StoryHTMLZh    string `json:"storyHtmlZh"`
	ResearchHTMLZh string `json:"researchHtmlZh"`
	RisksHTMLZh    string `json:"risksHtmlZh"`
	TeamJSON       string `json:"teamJson"` // 原样 JSON 字符串，前端 parse
	TokenomicsJSON string `json:"tokenomicsJson"`
	RoadmapJSON    string `json:"roadmapJson"`
	DocumentsJSON  string `json:"documentsJson"`
	FAQJSON        string `json:"faqJson"`
	OnchainJSON    string `json:"onchainJson"`

	// 展示型 stats（不走链上，backend 聚合）
	SubscriberCount int64 `json:"subscriberCount"`
	WatchCount      int64 `json:"watchCount"`
	WhitelistCount  int64 `json:"whitelistCount"`
}

type PendingClaimListResponse struct {
	Cursor int                `json:"cursor"` // 游标 翻页使用
	List   []PendingClaimData `json:"list"`
}

type PendingClaimData struct {
	Name          string          `json:"name"`
	ProjectID     int             `json:"projectID,string"`
	TokenAddress  string          `json:"tokenAddress"`
	TokenSymbol   string          `json:"tokenSymbol"`
	TokenDecimals int             `json:"tokenDecimals"`
	ClaimNum      decimal.Decimal `json:"claimNum"`
	NextClaimTime int64           `json:"nextClaimTime"`
	NextClaimNum  decimal.Decimal `json:"nextClaimNum"`
}

type PurchaseRecordListResponse struct {
	Cursor int                  `json:"cursor"` // 游标 翻页使用
	List   []PurchaseRecordData `json:"list"`
}

type PurchaseRecordData struct {
	Name          string          `json:"name"`
	ProjectID     int             `json:"projectID,string"`
	TokenAddress  string          `json:"tokenAddress"`
	TokenSymbol   string          `json:"tokenSymbol"`
	TokenDecimals int             `json:"tokenDecimals"`
	PurchaseToken decimal.Decimal `json:"purchaseToken"`
	ClaimToken    decimal.Decimal `json:"claimNum"`
	Status        string          `json:"status"`
	CreatedAt     int64           `json:"createdAt"`
}

type ClaimRecordResponse struct {
	Cursor int          `json:"cursor"` // 游标 翻页使用
	List   []RecordData `json:"list"`
}

type RecordData struct {
	Name          string          `json:"name"`
	ProjectID     int             `json:"projectID,string"`
	TokenAddress  string          `json:"tokenAddress"`
	TokenSymbol   string          `json:"tokenSymbol"`
	TokenDecimals int             `json:"tokenDecimals"`
	ClaimToken    decimal.Decimal `json:"claimNum"`
	TxHash        string          `json:"txHash"`
	CreatedAt     int64           `json:"createdAt"`
}

type RefundRecordResponse struct {
	Cursor int          `json:"cursor"` // 游标 翻页使用
	List   []RecordData `json:"list"`
}

type NewProjectRequest struct {
	Name            string   `json:"name"`                     // 姓名
	City            string   `json:"city"`                     // 所在城市/国家
	Email           string   `json:"email"`                    // 邮箱
	Wechat          string   `json:"wechat"`                   // 微信
	Telegram        string   `json:"telegram"`                 // Telegram
	Major           string   `json:"major"`                    // 专业/研究方向
	Identity        string   `json:"identity"`                 // 当前身份（医生/科研人员/医学生/创业者/独立开发者/其他）
	Organization    string   `json:"organization,omitempty"`   // 所属机构/学校/公司（选填）
	ProjectName     string   `json:"projectName"`              // 项目名称
	ProjectDesc     string   `json:"projectDesc"`              // 用一句话介绍你的项目
	ProjectCategory string   `json:"projectCategory"`          // 项目所属方向
	CurrentStep     string   `json:"currentStep"`              // 目前做到哪一步了
	Possessions     []string `json:"possessions"`              // 目前已经具备哪些内容
	ApplyReason     string   `json:"applyReason"`              // 为什么想申请 JoinCare IDO
	SupportWanted   []string `json:"supportWanted"`            // 希望获得什么支持
	ProjectLink     string   `json:"projectLink,omitempty"`    // 项目介绍/PDF链接
	AdditionalInfo  string   `json:"additionalInfo,omitempty"` // 其他想补充的内容
	WalletAddress   string   `json:"walletAddress"`
}

type SubmitProjectReviewRequest struct {
	ProjectID      int64           `json:"projectID,string"`
	NameCH         string          `json:"nameCh"`
	NameEn         string          `json:"nameEn"`
	Logo           string          `json:"logo"`
	Banner         string          `json:"banner"`
	Tagline        string          `json:"tagline"`
	Tags           []string        `json:"tags"`
	Description    string          `json:"description"`
	TeamInfo       string          `json:"teamInfo"`
	Roadmap        string          `json:"roadmap"`
	Partners       string          `json:"partners"`
	WalletAddress  string          `json:"walletAddress"`
	TokenAddress   string          `json:"tokenAddress"`
	TokenSymbol    string          `json:"tokenSymbol"`
	TokenDecimals  int             `json:"tokenDecimals"`
	TokenPrice     decimal.Decimal `json:"tokenPrice"`
	PaymentToken   string          `json:"paymentToken"`
	HardCap        decimal.Decimal `json:"hardCap"`
	SoftCap        decimal.Decimal `json:"softCap"`
	MaxPerUser     decimal.Decimal `json:"maxPerUser"`
	StartTime      int64           `json:"startTime"`
	EndTime        int64           `json:"endTime"`
	VestingTimes   []int64         `json:"vestingTimes"`
	VestingPercent []float64       `json:"vestingPercent"`
}

// ProjectInvestorItem is one canonical purchase record of an owned project.
// V2 uses purchase_payment ledger facts; V3 uses reconciled financial effects.
// Display amounts are deterministic decimal strings, never floating point.
// Social identity fields are joined from the user table by wallet address.
type ProjectInvestorItem struct {
	ID            uint64 `json:"id,string"`
	WalletAddress string `json:"walletAddress"`
	AmountDisplay string `json:"amountDisplay"`
	TokenSymbol   string `json:"tokenSymbol"`
	TxHash        string `json:"txHash"`
	BlockNumber   uint64 `json:"blockNumber"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	// 社交身份（按钱包地址关联 user 表，未绑定时为空）
	TwitterHandle    string `json:"twitterHandle"`
	TwitterName      string `json:"twitterName"`
	TwitterAvatar    string `json:"twitterAvatar"`
	TelegramUsername string `json:"telegramUsername"`
	TelegramName     string `json:"telegramName"`
	TelegramAvatar   string `json:"telegramAvatar"`
}

type ProjectInvestorsResponse struct {
	ProjectID int                   `json:"projectID,string"`
	Items     []ProjectInvestorItem `json:"items"`
	Total     int                   `json:"total"`
	Page      int                   `json:"page"`
	PageSize  int                   `json:"pageSize"`
}
