package entity

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type User struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	TwitterID          string    `gorm:"column:twitter_id;size:50;not null" json:"twitter_id"`
	TwitterHandle      string    `gorm:"column:twitter_handle;size:100;not null" json:"twitter_handle"`
	TwitterAvatar      string    `gorm:"column:twitter_avatar;size:500" json:"twitter_avatar"`
	TwitterDisplayName string    `gorm:"column:twitter_display_name;size:100" json:"twitter_display_name"`
	TelegramID         string    `gorm:"column:telegram_id;size:50;not null;default:''" json:"telegram_id"`
	TelegramUsername   string    `gorm:"column:telegram_username;size:100;not null;default:''" json:"telegram_username"`
	TelegramFirstName  string    `gorm:"column:telegram_first_name;size:100;not null;default:''" json:"telegram_first_name"`
	TelegramLastName   string    `gorm:"column:telegram_last_name;size:100;not null;default:''" json:"telegram_last_name"`
	TelegramAvatar     string    `gorm:"column:telegram_avatar;size:500;not null;default:''" json:"telegram_avatar"`
	WalletAddress      string    `gorm:"column:wallet_address;size:42" json:"wallet_address"`
	Type               string    `gorm:"column:type;size:42" json:"type"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;default:0000-00-00 00:00:00;autoUpdateTime" json:"updated_at"`
	LastLoginAt        time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	LastLoginIP        string    `gorm:"column:last_login_ip;size:45;not null;default:''" json:"last_login_ip"`
	IsEnable           int8      `gorm:"column:is_enable;not null;default:1" json:"is_enable"`
	SessionVersion     uint64    `gorm:"column:session_version;not null;default:1" json:"-"`
}

func (*User) TableName() string {
	return "user"
}

type Project struct {
	ID                          uint64          `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID                   int             `gorm:"column:project_id;not null;uniqueIndex" json:"project_id"`
	OwnerAddress                string          `gorm:"column:owner_address;size:100;not null" json:"owner_address"`
	NameEn                      string          `gorm:"column:name_en;size:255;not null" json:"name_en"`
	NameCn                      string          `gorm:"column:name_cn;size:255;not null" json:"name_cn"`
	Slug                        string          `gorm:"column:slug;size:255;not null" json:"slug"`
	Tagline                     string          `gorm:"column:tagline;size:500;not null" json:"tagline"`
	Description                 string          `gorm:"column:description;type:text;not null" json:"description"`
	Tags                        string          `gorm:"column:tags;type:text;not null" json:"tags"`
	TeamInfo                    string          `gorm:"column:team_info;type:text;not null" json:"team_info"`
	Roadmap                     string          `gorm:"column:roadmap;type:text;not null" json:"roadmap"`
	Partners                    string          `gorm:"column:partners;type:text;not null" json:"partners"`
	TokenAddress                string          `gorm:"column:token_address;size:42;not null" json:"token_address"`
	TokenSymbol                 string          `gorm:"column:token_symbol;size:20;not null" json:"token_symbol"`
	TokenDecimals               int             `gorm:"column:token_decimals;not null;default:18" json:"token_decimals"`
	PaymentToken                string          `gorm:"column:payment_token;size:20;not null;default:''" json:"payment_token"`
	PaymentTokenAddress         string          `gorm:"column:payment_token_address;size:42;not null;default:''" json:"payment_token_address"`
	PaymentTokenDecimals        int             `gorm:"column:payment_token_decimals;not null;default:0" json:"payment_token_decimals"`
	PaymentTokenPolicyRevision  *uint32         `gorm:"column:payment_token_policy_revision" json:"payment_token_policy_revision,omitempty"`
	ProjectTokenPolicyDigest    string          `gorm:"column:project_token_policy_digest;size:64" json:"project_token_policy_digest"`
	ProjectTokenRuntimeCodeHash string          `gorm:"column:project_token_runtime_code_hash;size:64" json:"project_token_runtime_code_hash"`
	ChainID                     uint64          `gorm:"column:chain_id;not null;default:0" json:"chain_id"`
	ContractAddress             string          `gorm:"column:contract_address;size:42;not null;default:''" json:"contract_address"`
	ContractVersion             string          `gorm:"column:contract_version;size:32;not null;default:unassigned" json:"contract_version"`
	FundingMode                 string          `gorm:"column:funding_mode;size:32;not null;default:capped_presale" json:"funding_mode"`
	OnchainLocked               bool            `gorm:"column:onchain_locked;not null;default:0" json:"onchain_locked"`
	FeeReceiver                 string          `gorm:"column:fee_receiver;size:42;not null;default:''" json:"fee_receiver"`
	SuccessFeeRate              uint16          `gorm:"column:success_fee_rate;not null;default:0" json:"success_fee_rate"`
	TokenPrice                  decimal.Decimal `gorm:"column:token_price;type:decimal(65,18);not null" json:"token_price"`
	TokenPriceRaw               string          `gorm:"column:token_price_raw;size:78;not null;default:'0'" json:"token_price_raw"`
	HardCap                     decimal.Decimal `gorm:"column:hard_cap;type:decimal(65,18);not null" json:"hard_cap"`
	HardCapRaw                  string          `gorm:"column:hard_cap_raw;size:78;not null;default:'0'" json:"hard_cap_raw"`
	SoftCap                     decimal.Decimal `gorm:"column:soft_cap;type:decimal(65,18);not null" json:"soft_cap"`
	SoftCapRaw                  string          `gorm:"column:soft_cap_raw;size:78;not null;default:'0'" json:"soft_cap_raw"`
	MaxPerUser                  decimal.Decimal `gorm:"column:max_per_user;type:decimal(65,18);not null" json:"max_per_user"`
	MaxPerUserRaw               string          `gorm:"column:max_per_user_raw;size:78;not null;default:'0'" json:"max_per_user_raw"`
	// Off-chain official-page policy, NOT a contract parameter. NULL means disabled.
	MinPurchaseAmountRaw        *string         `gorm:"column:min_purchase_amount_raw;size:78" json:"min_purchase_amount_raw"`
	MinPurchaseDraftAmount      *string         `gorm:"column:min_purchase_draft_amount;size:78" json:"min_purchase_draft_amount"`
	MinPurchaseDraftToken       *string         `gorm:"column:min_purchase_draft_token;size:8" json:"min_purchase_draft_token"`
	StartTime                   time.Time       `gorm:"column:start_time;not null" json:"start_time"`
	EndTime                     time.Time       `gorm:"column:end_time;not null" json:"end_time"`
	TgeTime                     time.Time       `gorm:"column:tge_time;not null" json:"tge_time"`
	SettlementBlockNumber       *uint64         `gorm:"column:settlement_block_number" json:"settlement_block_number,omitempty"`
	SettlementBlockHash         string          `gorm:"column:settlement_block_hash;size:66" json:"settlement_block_hash"`
	SettlementParentBlockNumber *uint64         `gorm:"column:settlement_parent_block_number" json:"settlement_parent_block_number,omitempty"`
	SettlementParentBlockHash   string          `gorm:"column:settlement_parent_block_hash;size:66" json:"settlement_parent_block_hash"`
	ReleaseAttestationDigest    string          `gorm:"column:release_attestation_digest;size:64" json:"release_attestation_digest"`
	TotalRaised                 decimal.Decimal `gorm:"column:total_raised;type:decimal(65,18);not null;default:0.0" json:"total_raised"`
	TotalRaisedRaw              string          `gorm:"column:total_raised_raw;size:78;not null;default:'0'" json:"total_raised_raw"`
	TotalDeposited              decimal.Decimal `gorm:"column:total_deposited;type:decimal(65,18);not null;default:0.0" json:"total_deposited"`
	TotalDepositedRaw           string          `gorm:"column:total_deposited_raw;size:78;not null;default:'0'" json:"total_deposited_raw"`
	SaleTokenAmountRaw          string          `gorm:"column:sale_token_amount_raw;size:78" json:"sale_token_amount_raw"`
	MinContributionRaw          string          `gorm:"column:min_contribution_raw;size:78" json:"min_contribution_raw"`
	MaxContributionRaw          string          `gorm:"column:max_contribution_raw;size:78" json:"max_contribution_raw"`
	FinalTotalRaisedRaw         string          `gorm:"column:final_total_raised_raw;size:78" json:"final_total_raised_raw"`
	FinalSaleTokenAmountRaw     string          `gorm:"column:final_sale_token_amount_raw;size:78" json:"final_sale_token_amount_raw"`
	ParticipantCount            uint32          `gorm:"column:participant_count;not null;default:0" json:"participant_count"`
	TokenWithdrawn              int             `gorm:"column:token_withdrawn;not null;default:0" json:"token_withdrawn"`
	FundWithdrawn               int             `gorm:"column:fund_withdrawn;not null;default:0" json:"fund_withdrawn"`
	PlatformFeeWithdrawn        int             `gorm:"column:platform_fee_withdrawn;not null;default:0" json:"platform_fee_withdrawn"`
	Status                      string          `gorm:"column:status;size:50;not null;default:''" json:"status"`
	// Status captured by POST /admin/v1/projects/:id/disable so that
	// POST /admin/v1/projects/:id/restore can revive an archived project to its
	// pre-archive lifecycle state instead of guessing a default (e.g. pre_launch).
	PreArchiveStatus string `gorm:"column:pre_archive_status;size:50;not null;default:''" json:"pre_archive_status"`
	// Phase 1 (Sprint A) 展示字段
	Category      string     `gorm:"column:category;size:40;not null;default:''" json:"category"`
	CoverURL      string     `gorm:"column:cover_url;size:500;not null;default:''" json:"cover_url"`
	CoverVideoURL string     `gorm:"column:cover_video_url;size:500;not null;default:''" json:"cover_video_url"`
	LaunchAt      *time.Time `gorm:"column:launch_at" json:"launch_at"`
	IsListed      int        `gorm:"column:is_listed;not null;default:1" json:"is_listed"`
	StoryHTML     string     `gorm:"column:story_html;type:mediumtext" json:"story_html"`
	ResearchHTML  string     `gorm:"column:research_html;type:mediumtext" json:"research_html"`
	RisksHTML     string     `gorm:"column:risks_html;type:mediumtext" json:"risks_html"`
	// 双语并列字段（繁體中文）；前端根据 language pick，空则 fallback 到英语版
	TaglineZh      string `gorm:"column:tagline_zh;size:500;not null;default:''" json:"tagline_zh"`
	StoryHTMLZh    string `gorm:"column:story_html_zh;type:mediumtext" json:"story_html_zh"`
	ResearchHTMLZh string `gorm:"column:research_html_zh;type:mediumtext" json:"research_html_zh"`
	RisksHTMLZh    string `gorm:"column:risks_html_zh;type:mediumtext" json:"risks_html_zh"`
	TeamJSON       string `gorm:"column:team_json;type:json" json:"team_json"`
	TokenomicsJSON string `gorm:"column:tokenomics_json;type:json" json:"tokenomics_json"`
	RoadmapJSON    string `gorm:"column:roadmap_json;type:json" json:"roadmap_json"`
	DocumentsJSON  string `gorm:"column:documents_json;type:json" json:"documents_json"`
	FAQJSON        string `gorm:"column:faq_json;type:json" json:"faq_json"`
	OnchainJSON    string `gorm:"column:onchain_json;type:json" json:"onchain_json"`

	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*Project) TableName() string {
	return "projects"
}

// ProjectUpdate 项目方发帖时间线（Sprint A 引入）
type ProjectUpdate struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID     int64      `gorm:"column:project_id;not null;index" json:"project_id,string"`
	AuthorAdminID uint64     `gorm:"column:author_admin_id;not null;default:0" json:"author_admin_id"`
	Title         string     `gorm:"column:title;size:200;not null;default:''" json:"title"`
	ContentHTML   string     `gorm:"column:content_html;type:mediumtext" json:"content_html"`
	IsPublished   int8       `gorm:"column:is_published;not null;default:0" json:"is_published"`
	PublishedAt   *time.Time `gorm:"column:published_at" json:"published_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ProjectUpdate) TableName() string { return "project_updates" }

// ProjectSubscriber Notify me 订阅（发 Update 时站内信推送）
type ProjectSubscriber struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID int64     `gorm:"column:project_id;not null;index" json:"project_id,string"`
	UID       int       `gorm:"column:uid;not null;index" json:"uid"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*ProjectSubscriber) TableName() string { return "project_subscribers" }

// ProjectWatch Watchlist
type ProjectWatch struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID int64     `gorm:"column:project_id;not null;index" json:"project_id,string"`
	UID       int       `gorm:"column:uid;not null;index" json:"uid"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*ProjectWatch) TableName() string { return "project_watches" }

// ProjectWhitelist Warmup 阶段白名单
type ProjectWhitelist struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID     int64     `gorm:"column:project_id;not null;index" json:"project_id,string"`
	UID           int       `gorm:"column:uid;not null;index" json:"uid"`
	WalletAddress string    `gorm:"column:wallet_address;size:42;not null;default:''" json:"wallet_address"`
	Note          string    `gorm:"column:note;size:200;not null;default:''" json:"note"`
	Approved      int8      `gorm:"column:approved;not null;default:0" json:"approved"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*ProjectWhitelist) TableName() string { return "project_whitelists" }

// UploadFile 通用上传文件元数据（LocalFs Phase 1，Phase 2 切 MinIO）
type UploadFile struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UploaderAdminID uint64    `gorm:"column:uploader_admin_id;not null;default:0" json:"uploader_admin_id"`
	OriginalName    string    `gorm:"column:original_name;size:255;not null;default:''" json:"original_name"`
	StoragePath     string    `gorm:"column:storage_path;size:500;not null;default:''" json:"storage_path"`
	PublicURL       string    `gorm:"column:public_url;size:500;not null;default:''" json:"public_url"`
	Mime            string    `gorm:"column:mime;size:100;not null;default:''" json:"mime"`
	SizeBytes       int64     `gorm:"column:size_bytes;not null;default:0" json:"size_bytes"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*UploadFile) TableName() string { return "upload_files" }

type ProjectTge struct {
	ID              uint64          `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID       int             `gorm:"column:project_id;not null" json:"project_id"`
	Index           int             `gorm:"column:index;not null" json:"index"`
	TgePercent      decimal.Decimal `gorm:"column:tge_percent;type:decimal(5,2);not null;default:0.00" json:"tge_percent"`
	VestingPercent  decimal.Decimal `gorm:"column:vesting_percent;type:decimal(5,2);not null" json:"vesting_percent"`
	VestingInterval uint32          `gorm:"column:vesting_interval;not null" json:"vesting_interval"`
	VestingTime     time.Time       `gorm:"column:vesting_time;not null" json:"vesting_time"`
	VestingDuration int             `gorm:"column:vesting_duration;not null" json:"vesting_duration"`
	IsNotification  int             `gorm:"column:is_notification;not null" json:"is_notification"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ProjectTge) TableName() string {
	return "project_tge"
}

// TransactionRecord mapped from table <transaction_record>
type TransactionRecord struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID uint64    `gorm:"column:project_id" json:"project_id"`
	Type      string    `gorm:"column:type;size:20" json:"type"`
	TxHash    string    `gorm:"column:tx_hash;size:66" json:"tx_hash"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*TransactionRecord) TableName() string {
	return "transaction_record"
}

// ChainTxSubmission is the durable source of truth for relayer transaction
// intents. RawTx contains a signed public transaction, never a private key.
type ChainTxSubmission struct {
	ID                    uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID             uint64     `gorm:"column:project_id;not null" json:"project_id"`
	Action                string     `gorm:"column:action;size:32;not null" json:"action"`
	AttemptNo             uint32     `gorm:"column:attempt_no;not null" json:"attempt_no"`
	ChainID               uint64     `gorm:"column:chain_id;not null" json:"chain_id"`
	SenderAddress         string     `gorm:"column:sender_address;size:42;not null" json:"sender_address"`
	ContractAddress       string     `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	CallData              []byte     `gorm:"column:call_data;type:mediumblob;not null" json:"-"`
	ValueRaw              string     `gorm:"column:value_raw;size:78;not null" json:"value_raw"`
	State                 string     `gorm:"column:state;size:32;not null" json:"state"`
	UnsignedTxDigest      string     `gorm:"column:unsigned_tx_digest;size:66;not null;default:''" json:"unsigned_tx_digest,omitempty"`
	SigningDigest         string     `gorm:"column:signing_digest;size:66;not null;default:''" json:"signing_digest,omitempty"`
	ReplacementGeneration uint32     `gorm:"column:replacement_generation;not null;default:0" json:"replacement_generation"`
	SigningReservedAt     *time.Time `gorm:"column:signing_reserved_at" json:"signing_reserved_at,omitempty"`
	// Read-only on the shared model: only V3 ready->signed writes this field
	// explicitly. Legacy V2 inserts must not require the candidate V3 DDL.
	V3SignedAt            *time.Time `gorm:"column:v3_signed_at;->" json:"-"`
	SigningRecoveryReason string     `gorm:"column:signing_recovery_reason;type:text;not null" json:"signing_recovery_reason,omitempty"`
	Nonce                 *uint64    `gorm:"column:nonce" json:"nonce,omitempty"`
	GasLimit              *uint64    `gorm:"column:gas_limit" json:"gas_limit,omitempty"`
	GasPriceRaw           string     `gorm:"column:gas_price_raw;size:78;not null" json:"gas_price_raw"`
	RawTx                 []byte     `gorm:"column:raw_tx;type:mediumblob" json:"-"`
	TxHash                *string    `gorm:"column:tx_hash;size:66" json:"tx_hash,omitempty"`
	ActiveSlot            *uint8     `gorm:"column:active_slot" json:"-"`
	BroadcastCount        uint32     `gorm:"column:broadcast_count;not null" json:"broadcast_count"`
	LastBroadcastAt       *time.Time `gorm:"column:last_broadcast_at" json:"last_broadcast_at,omitempty"`
	LastError             string     `gorm:"column:last_error;type:text;not null" json:"last_error"`
	ReceiptStatus         *uint64    `gorm:"column:receipt_status" json:"receipt_status,omitempty"`
	ReceiptBlockNumber    *uint64    `gorm:"column:receipt_block_number" json:"receipt_block_number,omitempty"`
	ReceiptBlockHash      string     `gorm:"column:receipt_block_hash;size:66;not null" json:"receipt_block_hash"`
	ReceiptGasUsed        *uint64    `gorm:"column:receipt_gas_used" json:"receipt_gas_used,omitempty"`
	FinalEventRecordID    *uint64    `gorm:"column:final_event_record_id" json:"final_event_record_id,omitempty"`
	FinalizedAt           *time.Time `gorm:"column:finalized_at" json:"finalized_at,omitempty"`
	CancelledByAdminID    *uint64    `gorm:"column:cancelled_by_admin_id" json:"cancelled_by_admin_id,omitempty"`
	CancelReason          string     `gorm:"column:cancel_reason;size:500;not null" json:"cancel_reason,omitempty"`
	CancelledAt           *time.Time `gorm:"column:cancelled_at" json:"cancelled_at,omitempty"`
	AdminCancellable      bool       `gorm:"column:admin_cancellable;->" json:"admin_cancellable"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ChainTxSubmission) TableName() string { return "chain_tx_submission" }

type ChainRelayerNonce struct {
	ChainID            uint64    `gorm:"column:chain_id;primaryKey;not null" json:"chain_id"`
	SenderAddress      string    `gorm:"column:sender_address;primaryKey;size:42;not null" json:"sender_address"`
	NextNonce          uint64    `gorm:"column:next_nonce;not null" json:"next_nonce"`
	ActiveSubmissionID *uint64   `gorm:"column:active_submission_id" json:"active_submission_id,omitempty"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ChainRelayerNonce) TableName() string { return "chain_relayer_nonce" }

// ProjectFile mapped from table <project_files>
type ProjectFile struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID  int       `gorm:"column:project_id;not null;index" json:"project_id"`
	Type       string    `gorm:"column:type;size:50;not null" json:"type"`
	Path       string    `gorm:"column:path;size:500;not null" json:"path"`
	Size       int       `gorm:"column:size;not null" json:"size"`
	MimeType   string    `gorm:"column:mime_type;size:100;not null" json:"mime_type"`
	Width      int       `gorm:"column:width;not null" json:"width"`
	Height     int       `gorm:"column:height;not null" json:"height"`
	UploadedBy string    `gorm:"column:uploaded_by;size:100;not null" json:"uploaded_by"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*ProjectFile) TableName() string {
	return "project_files"
}

// Notification mapped from table <notifications>
type Notification struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID       uint64    `gorm:"column:uid;not null" json:"uid"`
	Type      string    `gorm:"column:type;size:20;not null" json:"type"`
	Title     string    `gorm:"column:title;size:200;not null;default:''" json:"title"`
	Content   string    `gorm:"column:content;size:512;not null;default:''" json:"content"`
	Extra     string    `gorm:"column:extra;type:text;not null" json:"extra"`
	RelatedID *int      `gorm:"column:related_id" json:"related_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
}

func (*Notification) TableName() string {
	return "notifications"
}

// NotificationReadTime mapped from table <notification_read_time>
type NotificationReadTime struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID        uint64    `gorm:"column:uid;not null;uniqueIndex" json:"uid"`
	LastReadAt time.Time `gorm:"column:last_read_at;not null;default:CURRENT_TIMESTAMP(6)" json:"last_read_at"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP(6);autoUpdateTime" json:"updated_at"`
}

func (*NotificationReadTime) TableName() string {
	return "notification_read_time"
}

// CopywritingConfig mapped from table <copywriting_config>
type CopywritingConfig struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Key       string    `gorm:"column:key;size:50;not null;default:''" json:"key"`
	Value     string    `gorm:"column:value;size:255;not null;default:''" json:"value"`
	Language  string    `gorm:"column:language;size:10;not null;default:zh-cn" json:"language"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*CopywritingConfig) TableName() string {
	return "copywriting_config"
}

// News 新闻主表（多语言由 NewsTranslation 表承接）
// CheckInRule 签到阶梯奖励规则
type CheckInRule struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	DayThreshold int       `gorm:"column:day_threshold;not null" json:"day_threshold"`
	BonusPoint   int       `gorm:"column:bonus_point;not null" json:"bonus_point"`
	IsEnable     int8      `gorm:"column:is_enable;not null;default:1" json:"is_enable"`
	Sort         int       `gorm:"column:sort;not null;default:0" json:"sort"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*CheckInRule) TableName() string { return "checkin_rule" }

type HomeBanner struct {
	ID            uint64         `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Title         string         `gorm:"column:title;size:200;not null;default:''" json:"title"`
	Subtitle      string         `gorm:"column:subtitle;size:500;not null;default:''" json:"subtitle"`
	CTALabel      string         `gorm:"column:cta_label;size:64;not null;default:''" json:"cta_label"`
	CTALink       string         `gorm:"column:cta_link;size:500;not null;default:''" json:"cta_link"`
	ImageURL      string         `gorm:"column:image_url;size:500;not null;default:''" json:"image_url"`
	HeroAnimation string         `gorm:"column:hero_animation;size:32;not null;default:'dandelion'" json:"hero_animation"`
	Language      string         `gorm:"column:language;size:8;not null;default:'en'" json:"language"`
	StartAt       *time.Time     `gorm:"column:start_at" json:"start_at"`
	EndAt         *time.Time     `gorm:"column:end_at" json:"end_at"`
	Sort          int            `gorm:"column:sort;not null;default:0" json:"sort"`
	IsEnable      int8           `gorm:"column:is_enable;not null;default:1" json:"is_enable"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (*HomeBanner) TableName() string { return "home_banner" }

type Announcement struct {
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Title     string         `gorm:"column:title;size:200;not null;default:''" json:"title"`
	Content   string         `gorm:"column:content;type:text;not null" json:"content"`
	Type      string         `gorm:"column:type;size:16;not null;default:'popup'" json:"type"`
	Language  string         `gorm:"column:language;size:8;not null;default:'en'" json:"language"`
	Priority  int            `gorm:"column:priority;not null;default:0" json:"priority"`
	StartAt   *time.Time     `gorm:"column:start_at" json:"start_at"`
	EndAt     *time.Time     `gorm:"column:end_at" json:"end_at"`
	IsEnable  int8           `gorm:"column:is_enable;not null;default:1" json:"is_enable"`
	CreatedAt time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (*Announcement) TableName() string { return "announcement" }

type UserWallet struct {
	ID                 uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID                int        `gorm:"column:uid;not null;index" json:"uid"`
	WalletAddress      string     `gorm:"column:wallet_address;size:64;not null" json:"wallet_address"`
	Chain              string     `gorm:"column:chain;size:16;not null;default:'evm'" json:"chain"`
	IsPrimary          int8       `gorm:"column:is_primary;not null;default:0" json:"is_primary"`
	VerifiedAt         *time.Time `gorm:"column:verified_at" json:"verified_at,omitempty"`
	VerificationMethod string     `gorm:"column:verification_method;size:32;not null;default:''" json:"verification_method"`
	BoundAt            time.Time  `gorm:"column:bound_at;not null;default:CURRENT_TIMESTAMP" json:"bound_at"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*UserWallet) TableName() string { return "user_wallet" }

type UserWalletIdentity struct {
	WalletAddress string    `gorm:"column:wallet_address;size:42;primaryKey" json:"wallet_address"`
	UID           int       `gorm:"column:uid;not null;index" json:"uid"`
	Chain         string    `gorm:"column:chain;size:16;not null" json:"chain"`
	ClaimMethod   string    `gorm:"column:claim_method;size:32;not null" json:"claim_method"`
	ClaimedAt     time.Time `gorm:"column:claimed_at;not null" json:"claimed_at"`
}

func (*UserWalletIdentity) TableName() string { return "user_wallet_identity" }

// UserNote admin 内部对某用户的备注（append-only；删除通过独立端点）
type UserNote struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID           int64     `gorm:"column:uid;not null;index" json:"uid"`
	AdminID       uint64    `gorm:"column:admin_id;not null" json:"admin_id"`
	AdminUsername string    `gorm:"column:admin_username;size:64;not null;default:''" json:"admin_username"`
	Note          string    `gorm:"column:note;size:1000;not null;default:''" json:"note"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*UserNote) TableName() string { return "user_note" }

// UserLoginLog C 端每次登录追加一行（wallet / auto / register）
type UserLoginLog struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID        int64     `gorm:"column:uid;not null;index" json:"uid"`
	IP         string    `gorm:"column:ip;size:45;not null;default:''" json:"ip"`
	UserAgent  string    `gorm:"column:user_agent;size:500;not null;default:''" json:"user_agent"`
	LoginType  string    `gorm:"column:login_type;size:20;not null;default:'wallet'" json:"login_type"`
	LoggedInAt time.Time `gorm:"column:logged_in_at;not null;default:CURRENT_TIMESTAMP" json:"logged_in_at"`
}

func (*UserLoginLog) TableName() string { return "user_login_log" }

// TgDailyActivity TG 每日活跃统计（UPSERT：同 chat/user/day 累加消息数）
type TgDailyActivity struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ChatID       int64     `gorm:"column:chat_id;not null" json:"chat_id"`
	ChatTitle    string    `gorm:"column:chat_title;size:200;not null;default:''" json:"chat_title"`
	TgUserID     int64     `gorm:"column:tg_user_id;not null" json:"tg_user_id"`
	TgUsername   string    `gorm:"column:tg_username;size:100;not null;default:''" json:"tg_username"`
	UID          *int64    `gorm:"column:uid" json:"uid"`
	Day          string    `gorm:"column:day;type:date;not null" json:"day"` // "2026-07-01"
	MessageCount int       `gorm:"column:message_count;not null;default:1" json:"message_count"`
	FirstMsgAt   time.Time `gorm:"column:first_msg_at;not null;default:CURRENT_TIMESTAMP" json:"first_msg_at"`
	LastMsgAt    time.Time `gorm:"column:last_msg_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"last_msg_at"`
}

func (*TgDailyActivity) TableName() string { return "tg_daily_activity" }

type News struct {
	ID                uint64         `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Slug              string         `gorm:"column:slug;size:200;not null;default:''" json:"slug"`
	Category          string         `gorm:"column:category;size:50;not null;default:''" json:"category"`
	CoverURL          string         `gorm:"column:cover_url;size:500;not null;default:''" json:"cover_url"`
	SourceURL         string         `gorm:"column:source_url;size:500;not null;default:''" json:"source_url"`
	SourceName        string         `gorm:"column:source_name;size:100;not null;default:''" json:"source_name"`
	IsFeatured        int            `gorm:"column:is_featured;not null;default:0" json:"is_featured"`
	IsImportant       int            `gorm:"column:is_important;not null;default:0" json:"is_important"`
	AlertPriority     int            `gorm:"column:alert_priority;not null;default:0" json:"alert_priority"`
	AlertStartAt      *time.Time     `gorm:"column:alert_start_at" json:"alert_start_at"`
	AlertEndAt        *time.Time     `gorm:"column:alert_end_at" json:"alert_end_at"`
	AlertFrequency    string         `gorm:"column:alert_frequency;size:16;not null;default:'once'" json:"alert_frequency"`
	AlertPages        string         `gorm:"column:alert_pages;size:255;not null;default:'home,ecosystem,launchpad'" json:"alert_pages"`
	AlertDelaySeconds int            `gorm:"column:alert_delay_seconds;not null;default:3" json:"alert_delay_seconds"`
	AlertRevision     int            `gorm:"column:alert_revision;not null;default:1" json:"alert_revision"`
	Sort              int            `gorm:"column:sort;not null;default:0" json:"sort"`
	Status            int            `gorm:"column:status;not null;default:0" json:"status"` // 0草稿 1发布 2下架
	PublishAt         time.Time      `gorm:"column:publish_at;not null;default:CURRENT_TIMESTAMP" json:"publish_at"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (*News) TableName() string { return "news" }

// NewsTranslation 新闻多语言翻译表
type NewsTranslation struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	NewsID     uint64    `gorm:"column:news_id;not null" json:"news_id"`
	Language   string    `gorm:"column:language;size:10;not null" json:"language"`
	Title      string    `gorm:"column:title;size:300;not null;default:''" json:"title"`
	Summary    string    `gorm:"column:summary;size:800;not null;default:''" json:"summary"`
	Content    string    `gorm:"column:content;type:longtext" json:"content"`
	Author     string    `gorm:"column:author;size:100;not null;default:''" json:"author"`
	AuthorRole string    `gorm:"column:author_role;size:150;not null;default:''" json:"author_role"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*NewsTranslation) TableName() string { return "news_translation" }

type UserProject struct {
	ID            uint64          `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID           int             `gorm:"column:uid;not null" json:"uid"`
	ProjectID     int             `gorm:"column:project_id;not null" json:"project_id"`
	PurchaseToken decimal.Decimal `gorm:"column:purchase_token;type:decimal(36,18);not null" json:"purchase_token"`
	NextClaimTime time.Time       `gorm:"column:next_claim_time;not null" json:"next_claim_time"`
	ClaimTime     time.Time       `gorm:"column:claim_time;not null;default:CURRENT_TIMESTAMP" json:"claim_time"`
	ClaimToken    decimal.Decimal `gorm:"column:claim_token;type:decimal(36,18);not null" json:"claim_token"`
	IsRefund      int             `gorm:"column:is_refund;type:int;not null" json:"is_refund"`
	CreatedAt     time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time       `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*UserProject) TableName() string {
	return "user_project"
}

type UserClaimRecord struct {
	ID          uint64          `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID         int             `gorm:"column:uid;not null" json:"uid"`
	ProjectID   int             `gorm:"column:project_id;not null" json:"project_id"`
	ClaimToken  decimal.Decimal `gorm:"column:claim_token;type:decimal(36,18);not null" json:"claim_token"`
	BlockNumber int             `gorm:"column:block_number;not null" json:"block_number"`
	TxHash      string          `gorm:"column:tx_hash;size:100;not null" json:"tx_hash"`
	CreatedAt   time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*UserClaimRecord) TableName() string {
	return "user_claim_record"
}

type Settings struct {
	Key   string `gorm:"column:key" json:"key"`
	Value string `gorm:"column:value" json:"value"`
	Type  string `gorm:"column:type" json:"type"`
}

func (*Settings) TableName() string {
	return "settings"
}

type ChainEventRecord struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Event           string    `gorm:"column:event;size:50;not null" json:"event"`
	BlockNumber     int       `gorm:"column:block_number;uniqueIndex:uk_chain_event_identity,priority:1" json:"block_number"`
	BlockHash       string    `gorm:"column:block_hash;size:66;not null;default:''" json:"block_hash"`
	TxHash          string    `gorm:"column:tx_hash;size:100;not null;uniqueIndex:uk_chain_event_identity,priority:2" json:"tx_hash"`
	LogIndex        uint      `gorm:"column:log_index;uniqueIndex:uk_chain_event_identity,priority:3" json:"log_index"`
	ContractAddress string    `gorm:"column:contract_address;size:42;not null;default:''" json:"contract_address"`
	ContractVersion string    `gorm:"column:contract_version;size:32;not null;default:v1_legacy_unverified" json:"contract_version"`
	Data            string    `gorm:"column:data;type:text;not null" json:"data"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*ChainEventRecord) TableName() string {
	return "chain_event_record"
}

type ChainEventFailure struct {
	ID                uint64                  `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Event             string                  `gorm:"column:event;size:50;not null;default:''" json:"event"`
	BlockNumber       uint64                  `gorm:"column:block_number;not null;uniqueIndex:uk_chain_event_failure_identity,priority:1" json:"block_number"`
	BlockHash         string                  `gorm:"column:block_hash;size:66;not null;default:''" json:"block_hash"`
	TxHash            string                  `gorm:"column:tx_hash;size:100;not null;uniqueIndex:uk_chain_event_failure_identity,priority:2" json:"tx_hash"`
	LogIndex          uint                    `gorm:"column:log_index;not null;uniqueIndex:uk_chain_event_failure_identity,priority:3" json:"log_index"`
	ContractAddress   string                  `gorm:"column:contract_address;size:42;not null;default:''" json:"contract_address"`
	ContractVersion   string                  `gorm:"column:contract_version;size:32;not null;default:v1_legacy_unverified" json:"contract_version"`
	RawLog            string                  `gorm:"column:raw_log;type:longtext;not null" json:"-"`
	ErrorMessage      string                  `gorm:"column:error_message;type:text;not null" json:"error_message"`
	Status            string                  `gorm:"column:status;size:20;not null;index" json:"status"`
	RetryCount        uint                    `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	ConsecutiveCount  uint                    `gorm:"column:consecutive_count;not null;default:0" json:"consecutive_count"`
	ReplayCount       uint                    `gorm:"column:replay_count;not null;default:0" json:"replay_count"`
	FirstFailedAt     time.Time               `gorm:"column:first_failed_at;not null" json:"first_failed_at"`
	LastFailedAt      time.Time               `gorm:"column:last_failed_at;not null" json:"last_failed_at"`
	ReplayRequestedAt *time.Time              `gorm:"column:replay_requested_at" json:"replay_requested_at"`
	AdminReplayable   bool                    `gorm:"column:admin_replayable;->" json:"admin_replayable"`
	ReplayAudits      []ChainEventReplayAudit `gorm:"-" json:"replay_audits,omitempty"`
	ResolvedAt        *time.Time              `gorm:"column:resolved_at" json:"resolved_at"`
	CreatedAt         time.Time               `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time               `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ChainEventFailure) TableName() string { return "chain_event_failure" }

type ChainEventReplayAudit struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	FailureID   uint64    `gorm:"column:failure_id;not null;uniqueIndex:uk_chain_event_replay_no,priority:1" json:"failure_id"`
	ReplayNo    uint      `gorm:"column:replay_no;not null;uniqueIndex:uk_chain_event_replay_no,priority:2" json:"replay_no"`
	AdminID     uint64    `gorm:"column:admin_id;not null" json:"admin_id"`
	Reason      string    `gorm:"column:reason;size:500;not null" json:"reason"`
	RequestedAt time.Time `gorm:"column:requested_at;not null;default:CURRENT_TIMESTAMP(6)" json:"requested_at"`
}

func (*ChainEventReplayAudit) TableName() string { return "chain_event_replay_audit" }

type ChainEventSyncState struct {
	ID                       uint64     `gorm:"column:id;primaryKey" json:"id"`
	Initialized              bool       `gorm:"column:initialized;not null;default:0" json:"initialized"`
	CursorBlock              uint64     `gorm:"column:cursor_block;not null;default:0" json:"cursor_block"`
	CursorBlockHash          string     `gorm:"column:cursor_block_hash;size:66;not null;default:''" json:"cursor_block_hash"`
	LeaseToken               string     `gorm:"column:lease_token;size:64;not null;default:''" json:"-"`
	V2Registered             bool       `gorm:"column:v2_registered;not null;default:0" json:"v2_registered"`
	V2ContractAddress        string     `gorm:"column:v2_contract_address;size:42;not null;default:''" json:"v2_contract_address"`
	V2StartBlock             uint64     `gorm:"column:v2_start_block;not null;default:0" json:"v2_start_block"`
	V2ImplementationAddress  string     `gorm:"column:v2_implementation_address;size:42;not null;default:''" json:"v2_implementation_address"`
	V2ImplementationCodeHash string     `gorm:"column:v2_implementation_code_hash;size:66;not null;default:''" json:"v2_implementation_code_hash"`
	LatestBlock              uint64     `gorm:"column:latest_block;not null;default:0" json:"latest_block"`
	SafeBlock                uint64     `gorm:"column:safe_block;not null;default:0" json:"safe_block"`
	LagBlocks                uint64     `gorm:"column:lag_blocks;not null;default:0" json:"lag_blocks"`
	LastStartedAt            *time.Time `gorm:"column:last_started_at" json:"last_started_at"`
	LastSuccessAt            *time.Time `gorm:"column:last_success_at" json:"last_success_at"`
	LastFailureAt            *time.Time `gorm:"column:last_failure_at" json:"last_failure_at"`
	LastError                string     `gorm:"column:last_error;type:text;not null" json:"last_error"`
	CreatedAt                time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt                time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ChainEventSyncState) TableName() string { return "chain_event_sync_state" }

// ChainEventBlockCheckpoint is the append-only history of committed chain-event
// cursor boundaries. Sparse event rows cannot prove a reorg common ancestor;
// these checkpoints bound a future rewind to a previously verified boundary.
type ChainEventBlockCheckpoint struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ChainID                uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	BlockNumber            uint64    `gorm:"column:block_number;not null" json:"block_number"`
	BlockHash              string    `gorm:"column:block_hash;size:66;not null" json:"block_hash"`
	PreviousCheckpointHash string    `gorm:"column:previous_checkpoint_hash;size:64;not null" json:"previous_checkpoint_hash"`
	CheckpointHash         string    `gorm:"column:checkpoint_hash;size:64;not null" json:"checkpoint_hash"`
	CreatedAt              time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
}

func (*ChainEventBlockCheckpoint) TableName() string { return "chain_event_block_checkpoint" }

// ChainEventReorgIncident freezes a detected post-commit canonicality break.
// Resolution and compensation are deliberately separate, audited operations.
type ChainEventReorgIncident struct {
	ID                 uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	IncidentKey        string     `gorm:"column:incident_key;size:64;not null" json:"incident_key"`
	ChainID            uint64     `gorm:"column:chain_id;not null" json:"chain_id"`
	ContractAddress    string     `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	ContractVersion    string     `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	CursorBlock        uint64     `gorm:"column:cursor_block;not null" json:"cursor_block"`
	ExpectedBlockHash  string     `gorm:"column:expected_block_hash;size:66;not null" json:"expected_block_hash"`
	ObservedBlockHash  string     `gorm:"column:observed_block_hash;size:66;not null" json:"observed_block_hash"`
	CursorCheckpointID uint64     `gorm:"column:cursor_checkpoint_id;not null" json:"cursor_checkpoint_id"`
	V2StartBlock       uint64     `gorm:"column:v2_start_block;not null" json:"v2_start_block"`
	Status             string     `gorm:"column:status;size:32;not null" json:"status"`
	ActiveSlot         *uint8     `gorm:"column:active_slot" json:"-"`
	DetectionCode      string     `gorm:"column:detection_code;size:64;not null" json:"detection_code"`
	DetectedAt         time.Time  `gorm:"column:detected_at;not null;default:CURRENT_TIMESTAMP(6)" json:"detected_at"`
	ResolvedAt         *time.Time `gorm:"column:resolved_at" json:"resolved_at,omitempty"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP(6);autoUpdateTime" json:"updated_at"`
}

func (*ChainEventReorgIncident) TableName() string { return "chain_event_reorg_incident" }

type ChainEventReorgEvent struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	IncidentID        uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	SequenceNo        uint64    `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType         string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	PreviousEventHash string    `gorm:"column:previous_event_hash;size:64;not null" json:"previous_event_hash"`
	EventHash         string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt         time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
}

func (*ChainEventReorgEvent) TableName() string { return "chain_event_reorg_event" }

// ChainEventReorgRecoveryRun freezes one fixed-safe-head recovery scan. It is
// evidence for a later audited compensation step; completion never resolves
// the incident or mutates financial facts by itself.
type ChainEventReorgRecoveryRun struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunUUID                string    `gorm:"column:run_uuid;size:36;not null" json:"run_uuid"`
	IncidentID             uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	AttemptNo              uint      `gorm:"column:attempt_no;not null" json:"attempt_no"`
	ChainID                uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	ContractAddress        string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	LegacyContractAddress  string    `gorm:"column:legacy_contract_address;size:42;not null" json:"legacy_contract_address"`
	LegacySourceRegistered bool      `gorm:"column:legacy_source_registered;not null" json:"legacy_source_registered"`
	ContractVersion        string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	AlgorithmVersion       string    `gorm:"column:algorithm_version;size:48;not null" json:"algorithm_version"`
	BoundaryKind           string    `gorm:"column:boundary_kind;size:32;not null" json:"boundary_kind"`
	BoundaryCheckpointID   *uint64   `gorm:"column:boundary_checkpoint_id" json:"boundary_checkpoint_id,omitempty"`
	BoundaryBlock          uint64    `gorm:"column:boundary_block;not null" json:"boundary_block"`
	BoundaryBlockHash      string    `gorm:"column:boundary_block_hash;size:66;not null" json:"boundary_block_hash"`
	TargetSafeBlock        uint64    `gorm:"column:target_safe_block;not null" json:"target_safe_block"`
	TargetSafeBlockHash    string    `gorm:"column:target_safe_block_hash;size:66;not null" json:"target_safe_block_hash"`
	ConfirmationDepth      uint64    `gorm:"column:confirmation_depth;not null" json:"confirmation_depth"`
	ExpectedBatchCount     uint      `gorm:"column:expected_batch_count;not null" json:"expected_batch_count"`
	ExpectedScannedBlocks  uint64    `gorm:"column:expected_scanned_blocks;not null" json:"expected_scanned_blocks"`
	RequestedAt            time.Time `gorm:"column:requested_at;not null;default:CURRENT_TIMESTAMP(6)" json:"requested_at"`
}

func (*ChainEventReorgRecoveryRun) TableName() string { return "chain_event_reorg_recovery_run" }

type ChainEventReorgRecoveryJob struct {
	RunID               uint64     `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID          uint64     `gorm:"column:incident_id;not null" json:"incident_id"`
	State               string     `gorm:"column:state;size:24;not null" json:"state"`
	ActiveSlot          *uint8     `gorm:"column:active_slot" json:"-"`
	NextBlock           uint64     `gorm:"column:next_block;not null" json:"next_block"`
	NextBatchOrdinal    uint       `gorm:"column:next_batch_ordinal;not null" json:"next_batch_ordinal"`
	NextLogOrdinal      uint64     `gorm:"column:next_log_ordinal;not null" json:"next_log_ordinal"`
	ObservedLogCount    uint64     `gorm:"column:observed_log_count;not null" json:"observed_log_count"`
	BatchEvidenceDigest string     `gorm:"column:batch_evidence_digest;size:64;not null" json:"batch_evidence_digest"`
	LogEvidenceDigest   string     `gorm:"column:log_evidence_digest;size:64;not null" json:"log_evidence_digest"`
	LeaseToken          string     `gorm:"column:lease_token;size:64;not null" json:"-"`
	LeaseExpiresAt      *time.Time `gorm:"column:lease_expires_at" json:"-"`
	FenceEpoch          uint64     `gorm:"column:fence_epoch;not null" json:"fence_epoch"`
	AttemptCount        uint       `gorm:"column:attempt_count;not null" json:"attempt_count"`
	LastErrorCode       string     `gorm:"column:last_error_code;size:64;not null" json:"last_error_code"`
	LastErrorMessage    string     `gorm:"column:last_error_message;size:500;not null" json:"-"`
	CompletedAt         *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ChainEventReorgRecoveryJob) TableName() string { return "chain_event_reorg_recovery_job" }

type ChainEventReorgRecoveryLog struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal           uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	BlockNumber       uint64    `gorm:"column:block_number;not null" json:"block_number"`
	BlockHash         string    `gorm:"column:block_hash;size:66;not null" json:"block_hash"`
	TxHash            string    `gorm:"column:tx_hash;size:66;not null" json:"tx_hash"`
	TxIndex           uint      `gorm:"column:tx_index;not null" json:"tx_index"`
	LogIndex          uint      `gorm:"column:log_index;not null" json:"log_index"`
	ContractAddress   string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	ContractVersion   string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	RawLog            []byte    `gorm:"column:raw_log;type:mediumblob;not null" json:"-"`
	RawLogDigest      string    `gorm:"column:raw_log_digest;size:64;not null" json:"raw_log_digest"`
	PreviousRowDigest string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest         string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	ObservedAt        time.Time `gorm:"column:observed_at;not null" json:"observed_at"`
}

func (*ChainEventReorgRecoveryLog) TableName() string { return "chain_event_reorg_recovery_log" }

type ChainEventReorgRecoveryEvent struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	SequenceNo        uint      `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType         string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	Stage             string    `gorm:"column:stage;size:32;not null" json:"stage"`
	ErrorCode         string    `gorm:"column:error_code;size:64;not null" json:"error_code"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	PreviousEventHash string    `gorm:"column:previous_event_hash;size:64;not null" json:"-"`
	EventHash         string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgRecoveryEvent) TableName() string {
	return "chain_event_reorg_recovery_event"
}

type ChainEventReorgRecoveryBatch struct {
	ID                  uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID               uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal             uint      `gorm:"column:ordinal;not null" json:"ordinal"`
	FromBlock           uint64    `gorm:"column:from_block;not null" json:"from_block"`
	ToBlock             uint64    `gorm:"column:to_block;not null" json:"to_block"`
	FromBlockHash       string    `gorm:"column:from_block_hash;size:66;not null" json:"from_block_hash"`
	ToBlockHash         string    `gorm:"column:to_block_hash;size:66;not null" json:"to_block_hash"`
	FirstLogOrdinal     uint64    `gorm:"column:first_log_ordinal;not null" json:"first_log_ordinal"`
	LogCount            uint64    `gorm:"column:log_count;not null" json:"log_count"`
	BatchLogDigest      string    `gorm:"column:batch_log_digest;size:64;not null" json:"batch_log_digest"`
	PreviousBatchDigest string    `gorm:"column:previous_batch_digest;size:64;not null" json:"-"`
	BatchDigest         string    `gorm:"column:batch_digest;size:64;not null" json:"batch_digest"`
	ObservedAt          time.Time `gorm:"column:observed_at;not null" json:"observed_at"`
}

func (*ChainEventReorgRecoveryBatch) TableName() string { return "chain_event_reorg_recovery_batch" }

type ChainEventReorgRecoveryManifest struct {
	RunID               uint64    `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID          uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	BatchCount          uint      `gorm:"column:batch_count;not null" json:"batch_count"`
	LogCount            uint64    `gorm:"column:log_count;not null" json:"log_count"`
	BatchEvidenceDigest string    `gorm:"column:batch_evidence_digest;size:64;not null" json:"batch_evidence_digest"`
	LogEvidenceDigest   string    `gorm:"column:log_evidence_digest;size:64;not null" json:"log_evidence_digest"`
	ManifestDigest      string    `gorm:"column:manifest_digest;size:64;not null" json:"manifest_digest"`
	CompletedAt         time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*ChainEventReorgRecoveryManifest) TableName() string {
	return "chain_event_reorg_recovery_manifest"
}

// ChainEventReorgRecoveryDefinition anchors the exact MySQL definitions of
// every P3-B1 CHECK and trigger so startup detects post-migration DDL drift.
type ChainEventReorgRecoveryDefinition struct {
	ID               uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"-"`
	ObjectType       string    `gorm:"column:object_type;size:16;not null" json:"-"`
	ObjectName       string    `gorm:"column:object_name;size:128;not null" json:"-"`
	DefinitionDigest string    `gorm:"column:definition_digest;size:64;not null" json:"-"`
	RowDigest        string    `gorm:"column:row_digest;size:64;not null" json:"-"`
	CreatedAt        time.Time `gorm:"column:created_at;not null" json:"-"`
}

func (*ChainEventReorgRecoveryDefinition) TableName() string {
	return "chain_event_reorg_recovery_definition"
}

// ChainEventReorgMaterializationRun freezes the inputs for a P3-B2 semantic
// comparison. This preparation layer never mutates current financial facts or
// resolves the incident; a later audited generation switch consumes its
// manifest.
type ChainEventReorgMaterializationRun struct {
	ID                          uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunUUID                     string    `gorm:"column:run_uuid;size:36;not null" json:"run_uuid"`
	RecoveryRunID               uint64    `gorm:"column:recovery_run_id;not null" json:"recovery_run_id"`
	IncidentID                  uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	AlgorithmVersion            string    `gorm:"column:algorithm_version;size:64;not null" json:"algorithm_version"`
	RecoveryManifestDigest      string    `gorm:"column:recovery_manifest_digest;size:64;not null" json:"recovery_manifest_digest"`
	ChainID                     uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	ContractAddress             string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	LegacyContractAddress       string    `gorm:"column:legacy_contract_address;size:42;not null" json:"legacy_contract_address"`
	LegacySourceRegistered      bool      `gorm:"column:legacy_source_registered;not null" json:"legacy_source_registered"`
	BoundaryBlock               uint64    `gorm:"column:boundary_block;not null" json:"boundary_block"`
	BoundaryBlockHash           string    `gorm:"column:boundary_block_hash;size:66;not null" json:"boundary_block_hash"`
	OldCursorBlock              uint64    `gorm:"column:old_cursor_block;not null" json:"old_cursor_block"`
	OldCursorBlockHash          string    `gorm:"column:old_cursor_block_hash;size:66;not null" json:"old_cursor_block_hash"`
	TargetSafeBlock             uint64    `gorm:"column:target_safe_block;not null" json:"target_safe_block"`
	TargetSafeBlockHash         string    `gorm:"column:target_safe_block_hash;size:66;not null" json:"target_safe_block_hash"`
	ExpectedOldRecordCount      uint64    `gorm:"column:expected_old_record_count;not null" json:"expected_old_record_count"`
	OldRecordSourceDigest       string    `gorm:"column:old_record_source_digest;size:64;not null" json:"old_record_source_digest"`
	ExpectedReplacementLogCount uint64    `gorm:"column:expected_replacement_log_count;not null" json:"expected_replacement_log_count"`
	ReplacementLogSourceDigest  string    `gorm:"column:replacement_log_source_digest;size:64;not null" json:"replacement_log_source_digest"`
	CreatedAt                   time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgMaterializationRun) TableName() string {
	return "chain_event_reorg_materialization_run"
}

type ChainEventReorgMaterializationOldSource struct {
	ID                 uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID              uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal            uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	ChainEventRecordID uint64    `gorm:"column:chain_event_record_id;not null" json:"chain_event_record_id"`
	Event              string    `gorm:"column:event;size:50;not null" json:"event"`
	BlockNumber        uint64    `gorm:"column:block_number;not null" json:"block_number"`
	BlockHash          string    `gorm:"column:block_hash;size:66;not null" json:"block_hash"`
	TxHash             string    `gorm:"column:tx_hash;size:66;not null" json:"tx_hash"`
	LogIndex           uint      `gorm:"column:log_index;not null" json:"log_index"`
	ContractAddress    string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	ContractVersion    string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	EventData          []byte    `gorm:"column:event_data;type:mediumblob;not null" json:"-"`
	EventDataDigest    string    `gorm:"column:event_data_digest;size:64;not null" json:"event_data_digest"`
	PreviousRowDigest  string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest          string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt          time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgMaterializationOldSource) TableName() string {
	return "chain_event_reorg_materialization_old_source"
}

type ChainEventReorgMaterializationJob struct {
	RunID                   uint64     `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID              uint64     `gorm:"column:incident_id;not null" json:"incident_id"`
	State                   string     `gorm:"column:state;size:24;not null" json:"state"`
	ActiveSlot              *uint8     `gorm:"column:active_slot" json:"-"`
	NextBranch              string     `gorm:"column:next_branch;size:24;not null" json:"next_branch"`
	NextOldBlock            uint64     `gorm:"column:next_old_block;not null" json:"next_old_block"`
	NextOldRecordID         uint64     `gorm:"column:next_old_record_id;not null" json:"next_old_record_id"`
	NextReplacementOrdinal  uint64     `gorm:"column:next_replacement_ordinal;not null" json:"next_replacement_ordinal"`
	NextEffectOrdinal       uint64     `gorm:"column:next_effect_ordinal;not null" json:"next_effect_ordinal"`
	NextBranchOrdinal       uint64     `gorm:"column:next_branch_ordinal;not null" json:"next_branch_ordinal"`
	OldEffectCount          uint64     `gorm:"column:old_effect_count;not null" json:"old_effect_count"`
	ReplacementEffectCount  uint64     `gorm:"column:replacement_effect_count;not null" json:"replacement_effect_count"`
	IgnoredLogCount         uint64     `gorm:"column:ignored_log_count;not null" json:"ignored_log_count"`
	OldEffectDigest         string     `gorm:"column:old_effect_digest;size:64;not null" json:"old_effect_digest"`
	ReplacementEffectDigest string     `gorm:"column:replacement_effect_digest;size:64;not null" json:"replacement_effect_digest"`
	LeaseToken              string     `gorm:"column:lease_token;size:64;not null" json:"-"`
	LeaseExpiresAt          *time.Time `gorm:"column:lease_expires_at" json:"-"`
	FenceEpoch              uint64     `gorm:"column:fence_epoch;not null" json:"fence_epoch"`
	AttemptCount            uint       `gorm:"column:attempt_count;not null" json:"attempt_count"`
	LastErrorCode           string     `gorm:"column:last_error_code;size:64;not null" json:"last_error_code"`
	LastErrorMessage        string     `gorm:"column:last_error_message;size:500;not null" json:"-"`
	CompletedAt             *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt               time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ChainEventReorgMaterializationJob) TableName() string {
	return "chain_event_reorg_materialization_job"
}

type ChainEventReorgMaterializationEffect struct {
	ID                   uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal              uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	Branch               string    `gorm:"column:branch;size:24;not null" json:"branch"`
	BranchOrdinal        uint64    `gorm:"column:branch_ordinal;not null" json:"branch_ordinal"`
	SourceTable          string    `gorm:"column:source_table;size:48;not null" json:"source_table"`
	SourceID             uint64    `gorm:"column:source_id;not null" json:"source_id"`
	BlockNumber          uint64    `gorm:"column:block_number;not null" json:"block_number"`
	BlockHash            string    `gorm:"column:block_hash;size:66;not null" json:"block_hash"`
	TxHash               string    `gorm:"column:tx_hash;size:66;not null" json:"tx_hash"`
	LogIndex             uint      `gorm:"column:log_index;not null" json:"log_index"`
	ContractAddress      string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	ContractVersion      string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	EventType            string    `gorm:"column:event_type;size:64;not null" json:"event_type"`
	ProjectID            uint64    `gorm:"column:project_id;not null" json:"project_id"`
	Classification       string    `gorm:"column:classification;size:24;not null" json:"classification"`
	SemanticPayload      []byte    `gorm:"column:semantic_payload;type:mediumblob;not null" json:"-"`
	SemanticDigest       string    `gorm:"column:semantic_digest;size:64;not null" json:"semantic_digest"`
	PreviousEffectDigest string    `gorm:"column:previous_effect_digest;size:64;not null" json:"-"`
	EffectDigest         string    `gorm:"column:effect_digest;size:64;not null" json:"effect_digest"`
	CreatedAt            time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgMaterializationEffect) TableName() string {
	return "chain_event_reorg_materialization_effect"
}

type ChainEventReorgMaterializationBranch struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID         uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Branch        string    `gorm:"column:branch;size:24;not null" json:"branch"`
	EffectCount   uint64    `gorm:"column:effect_count;not null" json:"effect_count"`
	IgnoredCount  uint64    `gorm:"column:ignored_count;not null" json:"ignored_count"`
	EffectDigest  string    `gorm:"column:effect_digest;size:64;not null" json:"effect_digest"`
	SummaryDigest string    `gorm:"column:summary_digest;size:64;not null" json:"summary_digest"`
	CompletedAt   time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*ChainEventReorgMaterializationBranch) TableName() string {
	return "chain_event_reorg_materialization_branch"
}

type ChainEventReorgMaterializationEvent struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	SequenceNo        uint      `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType         string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	Stage             string    `gorm:"column:stage;size:32;not null" json:"stage"`
	ErrorCode         string    `gorm:"column:error_code;size:64;not null" json:"error_code"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	PreviousEventHash string    `gorm:"column:previous_event_hash;size:64;not null" json:"-"`
	EventHash         string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgMaterializationEvent) TableName() string {
	return "chain_event_reorg_materialization_event"
}

type ChainEventReorgMaterializationManifest struct {
	RunID                   uint64    `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID              uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	RecoveryManifestDigest  string    `gorm:"column:recovery_manifest_digest;size:64;not null" json:"recovery_manifest_digest"`
	OldEffectCount          uint64    `gorm:"column:old_effect_count;not null" json:"old_effect_count"`
	ReplacementEffectCount  uint64    `gorm:"column:replacement_effect_count;not null" json:"replacement_effect_count"`
	IgnoredLogCount         uint64    `gorm:"column:ignored_log_count;not null" json:"ignored_log_count"`
	OldEffectDigest         string    `gorm:"column:old_effect_digest;size:64;not null" json:"old_effect_digest"`
	ReplacementEffectDigest string    `gorm:"column:replacement_effect_digest;size:64;not null" json:"replacement_effect_digest"`
	ManifestDigest          string    `gorm:"column:manifest_digest;size:64;not null" json:"manifest_digest"`
	CompletedAt             time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*ChainEventReorgMaterializationManifest) TableName() string {
	return "chain_event_reorg_materialization_manifest"
}

// ChainEventReorgAssessmentRun binds one B2-B0 reversibility assessment to a
// completed B2-A manifest. Assessment completion is evidence only: it never
// authorizes an effective-generation switch or resolves the incident.
type ChainEventReorgAssessmentRun struct {
	ID                            uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunUUID                       string    `gorm:"column:run_uuid;size:36;not null" json:"run_uuid"`
	IncidentID                    uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	MaterializationRunID          uint64    `gorm:"column:materialization_run_id;not null" json:"materialization_run_id"`
	MaterializationManifestDigest string    `gorm:"column:materialization_manifest_digest;size:64;not null" json:"materialization_manifest_digest"`
	AlgorithmVersion              string    `gorm:"column:algorithm_version;size:64;not null" json:"algorithm_version"`
	ChainID                       uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	BoundaryBlock                 uint64    `gorm:"column:boundary_block;not null" json:"boundary_block"`
	BoundaryBlockHash             string    `gorm:"column:boundary_block_hash;size:66;not null" json:"boundary_block_hash"`
	TargetSafeBlock               uint64    `gorm:"column:target_safe_block;not null" json:"target_safe_block"`
	TargetSafeBlockHash           string    `gorm:"column:target_safe_block_hash;size:66;not null" json:"target_safe_block_hash"`
	ExpectedEffectCount           uint64    `gorm:"column:expected_effect_count;not null" json:"expected_effect_count"`
	MaterializationEffectRoot     string    `gorm:"column:materialization_effect_root;size:64;not null" json:"materialization_effect_root"`
	CreatedAt                     time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgAssessmentRun) TableName() string { return "chain_event_reorg_assessment_run" }

type ChainEventReorgAssessmentJob struct {
	RunID             uint64     `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID        uint64     `gorm:"column:incident_id;not null" json:"incident_id"`
	State             string     `gorm:"column:state;size:24;not null" json:"state"`
	ActiveSlot        *uint8     `gorm:"column:active_slot" json:"-"`
	NextEffectOrdinal uint64     `gorm:"column:next_effect_ordinal;not null" json:"next_effect_ordinal"`
	DispositionCount  uint64     `gorm:"column:disposition_count;not null" json:"disposition_count"`
	ExactCount        uint64     `gorm:"column:exact_count;not null" json:"exact_count"`
	ConditionalCount  uint64     `gorm:"column:conditional_count;not null" json:"conditional_count"`
	BlockedCount      uint64     `gorm:"column:blocked_count;not null" json:"blocked_count"`
	NoopCount         uint64     `gorm:"column:noop_count;not null" json:"noop_count"`
	BlockerCount      uint64     `gorm:"column:blocker_count;not null" json:"blocker_count"`
	DispositionDigest string     `gorm:"column:disposition_digest;size:64;not null" json:"disposition_digest"`
	BlockerDigest     string     `gorm:"column:blocker_digest;size:64;not null" json:"blocker_digest"`
	LeaseToken        string     `gorm:"column:lease_token;size:64;not null" json:"-"`
	LeaseExpiresAt    *time.Time `gorm:"column:lease_expires_at" json:"-"`
	FenceEpoch        uint64     `gorm:"column:fence_epoch;not null" json:"-"`
	AttemptCount      uint       `gorm:"column:attempt_count;not null" json:"attempt_count"`
	LastErrorCode     string     `gorm:"column:last_error_code;size:64;not null" json:"last_error_code"`
	LastErrorMessage  string     `gorm:"column:last_error_message;size:500;not null" json:"-"`
	CompletedAt       *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ChainEventReorgAssessmentJob) TableName() string { return "chain_event_reorg_assessment_job" }

type ChainEventReorgAssessmentDisposition struct {
	ID                      uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                   uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal                 uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	MaterializationEffectID uint64    `gorm:"column:materialization_effect_id;not null" json:"-"`
	SourceEffectDigest      string    `gorm:"column:source_effect_digest;size:64;not null" json:"-"`
	Branch                  string    `gorm:"column:branch;size:24;not null" json:"branch"`
	ContractVersion         string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	EventType               string    `gorm:"column:event_type;size:64;not null" json:"event_type"`
	ProjectID               uint64    `gorm:"column:project_id;not null" json:"project_id"`
	Replayability           string    `gorm:"column:replayability;size:24;not null" json:"replayability"`
	ActionKind              string    `gorm:"column:action_kind;size:64;not null" json:"action_kind"`
	BlockerCode             string    `gorm:"column:blocker_code;size:64;not null" json:"blocker_code"`
	PlanPayload             []byte    `gorm:"column:plan_payload;type:mediumblob;not null" json:"-"`
	PlanPayloadDigest       string    `gorm:"column:plan_payload_digest;size:64;not null" json:"plan_payload_digest"`
	PreviousRowDigest       string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest               string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt               time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgAssessmentDisposition) TableName() string {
	return "chain_event_reorg_assessment_disposition"
}

type ChainEventReorgAssessmentBlocker struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal           uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	DispositionID     *uint64   `gorm:"column:disposition_id" json:"disposition_id,omitempty"`
	Scope             string    `gorm:"column:scope;size:32;not null" json:"scope"`
	BlockerCode       string    `gorm:"column:blocker_code;size:64;not null" json:"blocker_code"`
	SafeDetail        string    `gorm:"column:safe_detail;size:500;not null" json:"safe_detail"`
	PreviousRowDigest string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest         string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgAssessmentBlocker) TableName() string {
	return "chain_event_reorg_assessment_blocker"
}

type ChainEventReorgAssessmentEvent struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	SequenceNo        uint      `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType         string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	ErrorCode         string    `gorm:"column:error_code;size:64;not null" json:"error_code"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	PreviousEventHash string    `gorm:"column:previous_event_hash;size:64;not null" json:"-"`
	EventHash         string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgAssessmentEvent) TableName() string {
	return "chain_event_reorg_assessment_event"
}

type ChainEventReorgAssessmentManifest struct {
	RunID                         uint64    `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID                    uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	MaterializationManifestDigest string    `gorm:"column:materialization_manifest_digest;size:64;not null" json:"materialization_manifest_digest"`
	Outcome                       string    `gorm:"column:outcome;size:16;not null" json:"outcome"`
	DispositionCount              uint64    `gorm:"column:disposition_count;not null" json:"disposition_count"`
	ExactCount                    uint64    `gorm:"column:exact_count;not null" json:"exact_count"`
	ConditionalCount              uint64    `gorm:"column:conditional_count;not null" json:"conditional_count"`
	BlockedCount                  uint64    `gorm:"column:blocked_count;not null" json:"blocked_count"`
	NoopCount                     uint64    `gorm:"column:noop_count;not null" json:"noop_count"`
	BlockerCount                  uint64    `gorm:"column:blocker_count;not null" json:"blocker_count"`
	DispositionDigest             string    `gorm:"column:disposition_digest;size:64;not null" json:"disposition_digest"`
	BlockerDigest                 string    `gorm:"column:blocker_digest;size:64;not null" json:"blocker_digest"`
	ManifestDigest                string    `gorm:"column:manifest_digest;size:64;not null" json:"manifest_digest"`
	CompletedAt                   time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*ChainEventReorgAssessmentManifest) TableName() string {
	return "chain_event_reorg_assessment_manifest"
}

// ChainEventReorgPrerequisiteRun binds one B2-B1-A fixed blocker declaration
// to a completed B2-B0 assessment. V1 is evidence-only and cannot make a
// candidate generation ready or authorize an effective-generation switch.
type ChainEventReorgPrerequisiteRun struct {
	ID                       uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunUUID                  string    `gorm:"column:run_uuid;size:36;not null" json:"run_uuid"`
	IncidentID               uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	AssessmentRunID          uint64    `gorm:"column:assessment_run_id;not null" json:"assessment_run_id"`
	AssessmentManifestDigest string    `gorm:"column:assessment_manifest_digest;size:64;not null" json:"assessment_manifest_digest"`
	AlgorithmVersion         string    `gorm:"column:algorithm_version;size:64;not null" json:"algorithm_version"`
	ChainID                  uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	BoundaryBlock            uint64    `gorm:"column:boundary_block;not null" json:"boundary_block"`
	BoundaryBlockHash        string    `gorm:"column:boundary_block_hash;size:66;not null" json:"boundary_block_hash"`
	ExpectedRequirementCount uint64    `gorm:"column:expected_requirement_count;not null" json:"expected_requirement_count"`
	CreatedAt                time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgPrerequisiteRun) TableName() string {
	return "chain_event_reorg_prerequisite_run"
}

type ChainEventReorgPrerequisiteJob struct {
	RunID                  uint64     `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID             uint64     `gorm:"column:incident_id;not null" json:"incident_id"`
	State                  string     `gorm:"column:state;size:24;not null" json:"state"`
	ActiveSlot             *uint8     `gorm:"column:active_slot" json:"-"`
	NextRequirementOrdinal uint64     `gorm:"column:next_requirement_ordinal;not null" json:"next_requirement_ordinal"`
	RequirementCount       uint64     `gorm:"column:requirement_count;not null" json:"requirement_count"`
	UnmetCount             uint64     `gorm:"column:unmet_count;not null" json:"unmet_count"`
	UntrustedCount         uint64     `gorm:"column:untrusted_count;not null" json:"untrusted_count"`
	DeferredCount          uint64     `gorm:"column:deferred_count;not null" json:"deferred_count"`
	ArtifactCount          uint64     `gorm:"column:artifact_count;not null" json:"artifact_count"`
	RequirementDigest      string     `gorm:"column:requirement_digest;size:64;not null" json:"requirement_digest"`
	ArtifactDigest         string     `gorm:"column:artifact_digest;size:64;not null" json:"artifact_digest"`
	LeaseToken             string     `gorm:"column:lease_token;size:64;not null" json:"-"`
	LeaseExpiresAt         *time.Time `gorm:"column:lease_expires_at" json:"-"`
	FenceEpoch             uint64     `gorm:"column:fence_epoch;not null" json:"-"`
	AttemptCount           uint       `gorm:"column:attempt_count;not null" json:"attempt_count"`
	LastErrorCode          string     `gorm:"column:last_error_code;size:64;not null" json:"last_error_code"`
	LastErrorMessage       string     `gorm:"column:last_error_message;size:500;not null" json:"-"`
	CompletedAt            *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt              time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ChainEventReorgPrerequisiteJob) TableName() string {
	return "chain_event_reorg_prerequisite_job"
}

type ChainEventReorgPrerequisiteArtifact struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal           uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	RequirementCode   string    `gorm:"column:requirement_code;size:64;not null" json:"requirement_code"`
	ArtifactKind      string    `gorm:"column:artifact_kind;size:64;not null" json:"artifact_kind"`
	SchemaVersion     string    `gorm:"column:schema_version;size:64;not null" json:"schema_version"`
	SourceKind        string    `gorm:"column:source_kind;size:32;not null" json:"source_kind"`
	SourcePayload     []byte    `gorm:"column:source_payload;type:mediumblob;not null" json:"-"`
	SourceDigest      string    `gorm:"column:source_digest;size:64;not null" json:"source_digest"`
	SafeSummary       string    `gorm:"column:safe_summary;size:500;not null" json:"safe_summary"`
	PreviousRowDigest string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest         string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgPrerequisiteArtifact) TableName() string {
	return "chain_event_reorg_prerequisite_artifact"
}

type ChainEventReorgPrerequisiteRequirement struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal           uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	RequirementCode   string    `gorm:"column:requirement_code;size:64;not null" json:"requirement_code"`
	EvidenceStatus    string    `gorm:"column:evidence_status;size:24;not null" json:"evidence_status"`
	ArtifactID        *uint64   `gorm:"column:artifact_id" json:"artifact_id,omitempty"`
	SafeDetail        string    `gorm:"column:safe_detail;size:500;not null" json:"safe_detail"`
	PreviousRowDigest string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest         string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgPrerequisiteRequirement) TableName() string {
	return "chain_event_reorg_prerequisite_requirement"
}

type ChainEventReorgPrerequisiteEvent struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	SequenceNo        uint      `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType         string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	ErrorCode         string    `gorm:"column:error_code;size:64;not null" json:"error_code"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	PreviousEventHash string    `gorm:"column:previous_event_hash;size:64;not null" json:"-"`
	EventHash         string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgPrerequisiteEvent) TableName() string {
	return "chain_event_reorg_prerequisite_event"
}

type ChainEventReorgPrerequisiteManifest struct {
	RunID                    uint64    `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID               uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	AssessmentManifestDigest string    `gorm:"column:assessment_manifest_digest;size:64;not null" json:"assessment_manifest_digest"`
	Outcome                  string    `gorm:"column:outcome;size:16;not null" json:"outcome"`
	CandidateReady           bool      `gorm:"column:candidate_ready;not null" json:"candidate_ready"`
	RequirementCount         uint64    `gorm:"column:requirement_count;not null" json:"requirement_count"`
	SatisfiedCount           uint64    `gorm:"column:satisfied_count;not null" json:"satisfied_count"`
	UnmetCount               uint64    `gorm:"column:unmet_count;not null" json:"unmet_count"`
	UntrustedCount           uint64    `gorm:"column:untrusted_count;not null" json:"untrusted_count"`
	DeferredCount            uint64    `gorm:"column:deferred_count;not null" json:"deferred_count"`
	ArtifactCount            uint64    `gorm:"column:artifact_count;not null" json:"artifact_count"`
	RequirementDigest        string    `gorm:"column:requirement_digest;size:64;not null" json:"requirement_digest"`
	ArtifactDigest           string    `gorm:"column:artifact_digest;size:64;not null" json:"artifact_digest"`
	EventDigest              string    `gorm:"column:event_digest;size:64;not null" json:"event_digest"`
	ManifestDigest           string    `gorm:"column:manifest_digest;size:64;not null" json:"manifest_digest"`
	CompletedAt              time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*ChainEventReorgPrerequisiteManifest) TableName() string {
	return "chain_event_reorg_prerequisite_manifest"
}

// ChainEventReorgGeneration is one immutable incident-scoped candidate
// attempt. B1-B0 can only seal a BLOCKED foundation; effective state remains a
// separate B2-B2 concern.
type ChainEventReorgGeneration struct {
	ID                            uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	GenerationUUID                string    `gorm:"column:generation_uuid;size:36;not null" json:"generation_uuid"`
	IncidentID                    uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	AttemptNo                     uint      `gorm:"column:attempt_no;not null" json:"attempt_no"`
	GenerationKind                string    `gorm:"column:generation_kind;size:16;not null" json:"generation_kind"`
	PrerequisiteRunID             uint64    `gorm:"column:prerequisite_run_id;not null" json:"prerequisite_run_id"`
	PrerequisiteManifestDigest    string    `gorm:"column:prerequisite_manifest_digest;size:64;not null" json:"prerequisite_manifest_digest"`
	AssessmentRunID               uint64    `gorm:"column:assessment_run_id;not null" json:"assessment_run_id"`
	AssessmentManifestDigest      string    `gorm:"column:assessment_manifest_digest;size:64;not null" json:"assessment_manifest_digest"`
	MaterializationRunID          uint64    `gorm:"column:materialization_run_id;not null" json:"materialization_run_id"`
	MaterializationManifestDigest string    `gorm:"column:materialization_manifest_digest;size:64;not null" json:"materialization_manifest_digest"`
	ChainID                       uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	BoundaryBlock                 uint64    `gorm:"column:boundary_block;not null" json:"boundary_block"`
	BoundaryBlockHash             string    `gorm:"column:boundary_block_hash;size:66;not null" json:"boundary_block_hash"`
	TargetSafeBlock               uint64    `gorm:"column:target_safe_block;not null" json:"target_safe_block"`
	TargetSafeBlockHash           string    `gorm:"column:target_safe_block_hash;size:66;not null" json:"target_safe_block_hash"`
	LegacyContractAddress         string    `gorm:"column:legacy_contract_address;size:42;not null" json:"legacy_contract_address"`
	LegacySourceRegistered        bool      `gorm:"column:legacy_source_registered;not null" json:"legacy_source_registered"`
	V2ContractAddress             string    `gorm:"column:v2_contract_address;size:42;not null" json:"v2_contract_address"`
	OldEffectRoot                 string    `gorm:"column:old_effect_root;size:64;not null" json:"old_effect_root"`
	ReplacementEffectRoot         string    `gorm:"column:replacement_effect_root;size:64;not null" json:"replacement_effect_root"`
	AlgorithmVersion              string    `gorm:"column:algorithm_version;size:64;not null" json:"algorithm_version"`
	SchemaVersion                 string    `gorm:"column:schema_version;size:32;not null" json:"schema_version"`
	DescriptorDigest              string    `gorm:"column:descriptor_digest;size:64;not null" json:"descriptor_digest"`
	CreatedAt                     time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgGeneration) TableName() string { return "chain_event_reorg_generation" }

type ChainEventReorgGenerationJob struct {
	RunID                  uint64     `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID             uint64     `gorm:"column:incident_id;not null" json:"incident_id"`
	State                  string     `gorm:"column:state;size:24;not null" json:"state"`
	Stage                  string     `gorm:"column:stage;size:24;not null" json:"stage"`
	ActiveSlot             *uint8     `gorm:"column:active_slot" json:"-"`
	SenderCount            uint64     `gorm:"column:sender_count;not null" json:"sender_count"`
	SubmissionCount        uint64     `gorm:"column:submission_count;not null" json:"submission_count"`
	SubmissionPayloadBytes uint64     `gorm:"column:submission_payload_bytes;not null" json:"submission_payload_bytes"`
	NonceCount             uint64     `gorm:"column:nonce_count;not null" json:"nonce_count"`
	RequirementCount       uint64     `gorm:"column:requirement_count;not null" json:"requirement_count"`
	UnmetCount             uint64     `gorm:"column:unmet_count;not null" json:"unmet_count"`
	VerifiedCount          uint64     `gorm:"column:verified_count;not null" json:"verified_count"`
	SenderRoot             string     `gorm:"column:sender_root;size:64;not null" json:"sender_root"`
	SubmissionRoot         string     `gorm:"column:submission_root;size:64;not null" json:"submission_root"`
	NonceRoot              string     `gorm:"column:nonce_root;size:64;not null" json:"nonce_root"`
	RequirementRoot        string     `gorm:"column:requirement_root;size:64;not null" json:"requirement_root"`
	LeaseToken             string     `gorm:"column:lease_token;size:64;not null" json:"-"`
	LeaseExpiresAt         *time.Time `gorm:"column:lease_expires_at" json:"-"`
	FenceEpoch             uint64     `gorm:"column:fence_epoch;not null" json:"-"`
	AttemptCount           uint       `gorm:"column:attempt_count;not null" json:"attempt_count"`
	LastErrorCode          string     `gorm:"column:last_error_code;size:64;not null" json:"last_error_code"`
	LastErrorMessage       string     `gorm:"column:last_error_message;size:500;not null" json:"-"`
	CompletedAt            *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt              time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ChainEventReorgGenerationJob) TableName() string { return "chain_event_reorg_generation_job" }

type ChainEventReorgGenerationSender struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                  uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal                uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	ChainID                uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	SenderAddress          string    `gorm:"column:sender_address;size:42;not null" json:"sender_address"`
	SourceContractRoot     string    `gorm:"column:source_contract_root;size:64;not null" json:"source_contract_root"`
	WriterFenceEpoch       uint64    `gorm:"column:writer_fence_epoch;not null" json:"-"`
	WriterLeaseTokenDigest string    `gorm:"column:writer_lease_token_digest;size:64;not null" json:"-"`
	PreviousRowDigest      string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest              string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt              time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgGenerationSender) TableName() string {
	return "chain_event_reorg_generation_sender"
}

type ChainEventReorgGenerationSubmission struct {
	ID                     uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                  uint64     `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal                uint64     `gorm:"column:ordinal;not null" json:"ordinal"`
	SourceSubmissionID     uint64     `gorm:"column:source_submission_id;not null" json:"source_submission_id"`
	ProjectID              uint64     `gorm:"column:project_id;not null" json:"project_id"`
	Action                 string     `gorm:"column:action;size:32;not null" json:"action"`
	SourceAttemptNo        uint32     `gorm:"column:source_attempt_no;not null" json:"source_attempt_no"`
	ChainID                uint64     `gorm:"column:chain_id;not null" json:"chain_id"`
	SenderAddress          string     `gorm:"column:sender_address;size:42;not null" json:"sender_address"`
	ContractAddress        string     `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	CallData               []byte     `gorm:"column:call_data;type:mediumblob;not null" json:"-"`
	CallDataDigest         string     `gorm:"column:call_data_digest;size:64;not null" json:"call_data_digest"`
	ValueRaw               string     `gorm:"column:value_raw;size:78;not null" json:"value_raw"`
	SourceState            string     `gorm:"column:source_state;size:32;not null" json:"source_state"`
	Nonce                  *uint64    `gorm:"column:nonce" json:"nonce,omitempty"`
	GasLimit               *uint64    `gorm:"column:gas_limit" json:"gas_limit,omitempty"`
	GasPriceRaw            string     `gorm:"column:gas_price_raw;size:78;not null" json:"gas_price_raw"`
	RawTx                  []byte     `gorm:"column:raw_tx;type:mediumblob" json:"-"`
	RawTxDigest            string     `gorm:"column:raw_tx_digest;size:64;not null" json:"raw_tx_digest"`
	TxHash                 *string    `gorm:"column:tx_hash;size:66" json:"tx_hash,omitempty"`
	SourceActiveSlot       *uint8     `gorm:"column:source_active_slot" json:"-"`
	BroadcastCount         uint32     `gorm:"column:broadcast_count;not null" json:"broadcast_count"`
	LastBroadcastAt        *time.Time `gorm:"column:last_broadcast_at" json:"last_broadcast_at,omitempty"`
	LastErrorDigest        string     `gorm:"column:last_error_digest;size:64;not null" json:"last_error_digest"`
	ReceiptStatus          *uint64    `gorm:"column:receipt_status" json:"receipt_status,omitempty"`
	ReceiptBlockNumber     *uint64    `gorm:"column:receipt_block_number" json:"receipt_block_number,omitempty"`
	ReceiptBlockHash       string     `gorm:"column:receipt_block_hash;size:66;not null" json:"receipt_block_hash"`
	ReceiptGasUsed         *uint64    `gorm:"column:receipt_gas_used" json:"receipt_gas_used,omitempty"`
	FinalEventRecordID     *uint64    `gorm:"column:final_event_record_id" json:"final_event_record_id,omitempty"`
	FinalizedAt            *time.Time `gorm:"column:finalized_at" json:"finalized_at,omitempty"`
	CancelledByAdminID     *uint64    `gorm:"column:cancelled_by_admin_id" json:"cancelled_by_admin_id,omitempty"`
	CancelReasonDigest     string     `gorm:"column:cancel_reason_digest;size:64;not null" json:"cancel_reason_digest"`
	CancelledAt            *time.Time `gorm:"column:cancelled_at" json:"cancelled_at,omitempty"`
	GenerationDisposition  string     `gorm:"column:generation_disposition;size:24;not null" json:"generation_disposition"`
	SourceSnapshotDigest   string     `gorm:"column:source_snapshot_digest;size:64;not null" json:"source_snapshot_digest"`
	WriterFenceEpoch       uint64     `gorm:"column:writer_fence_epoch;not null" json:"-"`
	WriterLeaseTokenDigest string     `gorm:"column:writer_lease_token_digest;size:64;not null" json:"-"`
	PreviousRowDigest      string     `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest              string     `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	SourceCreatedAt        time.Time  `gorm:"column:source_created_at;not null" json:"source_created_at"`
	SourceUpdatedAt        time.Time  `gorm:"column:source_updated_at;not null" json:"source_updated_at"`
	CreatedAt              time.Time  `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgGenerationSubmission) TableName() string {
	return "chain_event_reorg_generation_submission"
}

type ChainEventReorgGenerationNonce struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                  uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal                uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	ChainID                uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	SenderAddress          string    `gorm:"column:sender_address;size:42;not null" json:"sender_address"`
	NextNonce              uint64    `gorm:"column:next_nonce;not null" json:"next_nonce"`
	ActiveSubmissionID     *uint64   `gorm:"column:active_submission_id" json:"active_submission_id,omitempty"`
	SourceSnapshotDigest   string    `gorm:"column:source_snapshot_digest;size:64;not null" json:"source_snapshot_digest"`
	WriterFenceEpoch       uint64    `gorm:"column:writer_fence_epoch;not null" json:"-"`
	WriterLeaseTokenDigest string    `gorm:"column:writer_lease_token_digest;size:64;not null" json:"-"`
	PreviousRowDigest      string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest              string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	SourceCreatedAt        time.Time `gorm:"column:source_created_at;not null" json:"source_created_at"`
	SourceUpdatedAt        time.Time `gorm:"column:source_updated_at;not null" json:"source_updated_at"`
	CreatedAt              time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgGenerationNonce) TableName() string {
	return "chain_event_reorg_generation_nonce"
}

type ChainEventReorgGenerationRequirement struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                  uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal                uint64    `gorm:"column:ordinal;not null" json:"ordinal"`
	RequirementCode        string    `gorm:"column:requirement_code;size:64;not null" json:"requirement_code"`
	EvidenceStatus         string    `gorm:"column:evidence_status;size:24;not null" json:"evidence_status"`
	SafeDetail             string    `gorm:"column:safe_detail;size:500;not null" json:"safe_detail"`
	WriterFenceEpoch       uint64    `gorm:"column:writer_fence_epoch;not null" json:"-"`
	WriterLeaseTokenDigest string    `gorm:"column:writer_lease_token_digest;size:64;not null" json:"-"`
	PreviousRowDigest      string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	RowDigest              string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt              time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgGenerationRequirement) TableName() string {
	return "chain_event_reorg_generation_requirement"
}

type ChainEventReorgGenerationEvent struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID                  uint64    `gorm:"column:run_id;not null" json:"run_id"`
	SequenceNo             uint      `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType              string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	Stage                  string    `gorm:"column:stage;size:24;not null" json:"stage"`
	ErrorCode              string    `gorm:"column:error_code;size:64;not null" json:"error_code"`
	Message                string    `gorm:"column:message;size:500;not null" json:"message"`
	WriterFenceEpoch       uint64    `gorm:"column:writer_fence_epoch;not null" json:"-"`
	WriterLeaseTokenDigest string    `gorm:"column:writer_lease_token_digest;size:64;not null" json:"-"`
	PreviousEventHash      string    `gorm:"column:previous_event_hash;size:64;not null" json:"-"`
	EventHash              string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt              time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainEventReorgGenerationEvent) TableName() string {
	return "chain_event_reorg_generation_event"
}

type ChainEventReorgGenerationManifest struct {
	RunID                         uint64    `gorm:"column:run_id;primaryKey" json:"run_id"`
	IncidentID                    uint64    `gorm:"column:incident_id;not null" json:"incident_id"`
	DescriptorDigest              string    `gorm:"column:descriptor_digest;size:64;not null" json:"descriptor_digest"`
	PrerequisiteManifestDigest    string    `gorm:"column:prerequisite_manifest_digest;size:64;not null" json:"prerequisite_manifest_digest"`
	AssessmentManifestDigest      string    `gorm:"column:assessment_manifest_digest;size:64;not null" json:"assessment_manifest_digest"`
	MaterializationManifestDigest string    `gorm:"column:materialization_manifest_digest;size:64;not null" json:"materialization_manifest_digest"`
	Outcome                       string    `gorm:"column:outcome;size:16;not null" json:"outcome"`
	CandidateReady                bool      `gorm:"column:candidate_ready;not null" json:"candidate_ready"`
	RequirementCount              uint64    `gorm:"column:requirement_count;not null" json:"requirement_count"`
	UnmetCount                    uint64    `gorm:"column:unmet_count;not null" json:"unmet_count"`
	VerifiedCount                 uint64    `gorm:"column:verified_count;not null" json:"verified_count"`
	SenderCount                   uint64    `gorm:"column:sender_count;not null" json:"sender_count"`
	SubmissionCount               uint64    `gorm:"column:submission_count;not null" json:"submission_count"`
	SubmissionPayloadBytes        uint64    `gorm:"column:submission_payload_bytes;not null" json:"submission_payload_bytes"`
	NonceCount                    uint64    `gorm:"column:nonce_count;not null" json:"nonce_count"`
	CandidateProjectCount         uint64    `gorm:"column:candidate_project_count;not null" json:"candidate_project_count"`
	CandidateLedgerCount          uint64    `gorm:"column:candidate_ledger_count;not null" json:"candidate_ledger_count"`
	CandidateEntitlementCount     uint64    `gorm:"column:candidate_entitlement_count;not null" json:"candidate_entitlement_count"`
	CandidateV1FactCount          uint64    `gorm:"column:candidate_v1_fact_count;not null" json:"candidate_v1_fact_count"`
	CandidateWalletCount          uint64    `gorm:"column:candidate_wallet_count;not null" json:"candidate_wallet_count"`
	SignedDeltaCount              uint64    `gorm:"column:signed_delta_count;not null" json:"signed_delta_count"`
	CheckpointCount               uint64    `gorm:"column:checkpoint_count;not null" json:"checkpoint_count"`
	SenderRoot                    string    `gorm:"column:sender_root;size:64;not null" json:"sender_root"`
	SubmissionRoot                string    `gorm:"column:submission_root;size:64;not null" json:"submission_root"`
	NonceRoot                     string    `gorm:"column:nonce_root;size:64;not null" json:"nonce_root"`
	RequirementRoot               string    `gorm:"column:requirement_root;size:64;not null" json:"requirement_root"`
	CandidateRoot                 string    `gorm:"column:candidate_root;size:64;not null" json:"candidate_root"`
	DeltaRoot                     string    `gorm:"column:delta_root;size:64;not null" json:"delta_root"`
	CheckpointRoot                string    `gorm:"column:checkpoint_root;size:64;not null" json:"checkpoint_root"`
	EventCount                    uint64    `gorm:"column:event_count;not null" json:"event_count"`
	EventRoot                     string    `gorm:"column:event_root;size:64;not null" json:"event_root"`
	WriterFenceEpoch              uint64    `gorm:"column:writer_fence_epoch;not null" json:"-"`
	WriterLeaseTokenDigest        string    `gorm:"column:writer_lease_token_digest;size:64;not null" json:"-"`
	ManifestDigest                string    `gorm:"column:manifest_digest;size:64;not null" json:"manifest_digest"`
	CompletedAt                   time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*ChainEventReorgGenerationManifest) TableName() string {
	return "chain_event_reorg_generation_manifest"
}

// FundLedger is the append-only, raw-unit financial fact reconstructed from one canonical chain log.
// AmountRaw is a base-10 uint256 string; AmountDisplay is derived deterministically from TokenDecimals.
type FundLedger struct {
	ID                  uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ChainID             uint64    `gorm:"column:chain_id;not null;uniqueIndex:uk_fund_ledger_event,priority:1" json:"chain_id"`
	ContractAddress     string    `gorm:"column:contract_address;size:42;not null;index" json:"contract_address"`
	ContractVersion     string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	ProjectID           uint64    `gorm:"column:project_id;not null;index" json:"project_id"`
	WalletAddress       string    `gorm:"column:wallet_address;size:42;not null;default:'';index" json:"wallet_address"`
	CounterpartyAddress string    `gorm:"column:counterparty_address;size:42;not null;default:''" json:"counterparty_address"`
	EntryType           string    `gorm:"column:entry_type;size:32;not null;index" json:"entry_type"`
	AssetRole           string    `gorm:"column:asset_role;size:16;not null" json:"asset_role"`
	TokenSymbol         string    `gorm:"column:token_symbol;size:20;not null;default:''" json:"token_symbol"`
	TokenAddress        string    `gorm:"column:token_address;size:42;not null;index" json:"token_address"`
	TokenDecimals       uint8     `gorm:"column:token_decimals;not null" json:"token_decimals"`
	AmountRaw           string    `gorm:"column:amount_raw;size:78;not null" json:"amount_raw"`
	AmountDisplay       string    `gorm:"column:amount_display;size:160;not null" json:"amount_display"`
	TxHash              string    `gorm:"column:tx_hash;size:66;not null;uniqueIndex:uk_fund_ledger_event,priority:2" json:"tx_hash"`
	LogIndex            uint      `gorm:"column:log_index;not null;uniqueIndex:uk_fund_ledger_event,priority:3" json:"log_index"`
	BlockNumber         uint64    `gorm:"column:block_number;not null;index" json:"block_number"`
	BlockHash           string    `gorm:"column:block_hash;size:66;not null" json:"block_hash"`
	Status              string    `gorm:"column:status;size:20;not null;default:canonical;index" json:"status"`
	EventData           string    `gorm:"column:event_data;type:json;not null" json:"-"`
	CreatedAt           time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP(6);autoUpdateTime" json:"updated_at"`
}

func (*FundLedger) TableName() string { return "fund_ledger" }

// IDOPurchaseEntitlement is the append-only project-token entitlement paired with one V2 Purchased log.
// Raw amounts are canonical base-10 uint256 strings; display values are deterministic decimal renderings.
type IDOPurchaseEntitlement struct {
	ID                    uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	FundLedgerID          uint64    `gorm:"column:fund_ledger_id;not null;uniqueIndex" json:"fund_ledger_id"`
	ChainID               uint64    `gorm:"column:chain_id;not null;uniqueIndex:uk_ido_purchase_entitlement_event,priority:1" json:"chain_id"`
	ContractAddress       string    `gorm:"column:contract_address;size:42;not null;index" json:"contract_address"`
	ContractVersion       string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	ProjectID             uint64    `gorm:"column:project_id;not null;index" json:"project_id"`
	WalletAddress         string    `gorm:"column:wallet_address;size:42;not null;index" json:"wallet_address"`
	PaymentTokenSymbol    string    `gorm:"column:payment_token_symbol;size:20;not null" json:"payment_token_symbol"`
	PaymentTokenAddress   string    `gorm:"column:payment_token_address;size:42;not null" json:"payment_token_address"`
	PaymentTokenDecimals  uint8     `gorm:"column:payment_token_decimals;not null" json:"payment_token_decimals"`
	PaymentAmountRaw      string    `gorm:"column:payment_amount_raw;size:78;not null" json:"payment_amount_raw"`
	PaymentAmountDisplay  string    `gorm:"column:payment_amount_display;size:160;not null" json:"payment_amount_display"`
	ProjectTokenSymbol    string    `gorm:"column:project_token_symbol;size:20;not null" json:"project_token_symbol"`
	ProjectTokenAddress   string    `gorm:"column:project_token_address;size:42;not null" json:"project_token_address"`
	ProjectTokenDecimals  uint8     `gorm:"column:project_token_decimals;not null" json:"project_token_decimals"`
	ProjectTokenAmountRaw string    `gorm:"column:project_token_amount_raw;size:78;not null" json:"project_token_amount_raw"`
	ProjectTokenDisplay   string    `gorm:"column:project_token_amount_display;size:160;not null" json:"project_token_amount_display"`
	TxHash                string    `gorm:"column:tx_hash;size:66;not null;uniqueIndex:uk_ido_purchase_entitlement_event,priority:2" json:"tx_hash"`
	LogIndex              uint      `gorm:"column:log_index;not null;uniqueIndex:uk_ido_purchase_entitlement_event,priority:3" json:"log_index"`
	BlockNumber           uint64    `gorm:"column:block_number;not null;index" json:"block_number"`
	BlockHash             string    `gorm:"column:block_hash;size:66;not null" json:"block_hash"`
	Status                string    `gorm:"column:status;size:20;not null;default:canonical" json:"status"`
	EventData             string    `gorm:"column:event_data;type:json;not null" json:"-"`
	CreatedAt             time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
}

func (*IDOPurchaseEntitlement) TableName() string { return "ido_purchase_entitlement" }

type IDOReconciliationRun struct {
	ID                           uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunUUID                      string    `gorm:"column:run_uuid;size:36;not null" json:"run_uuid"`
	ProjectID                    uint64    `gorm:"column:project_id;not null" json:"project_id"`
	ChainID                      uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	ContractAddress              string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	RequestedByAdminID           uint64    `gorm:"column:requested_by_admin_id;not null" json:"requested_by_admin_id"`
	RequestedRoleCodes           string    `gorm:"column:requested_role_codes;size:500;not null" json:"requested_role_codes"`
	Reason                       string    `gorm:"column:reason;size:500;not null" json:"reason"`
	IdempotencyKey               string    `gorm:"column:idempotency_key;size:100;not null" json:"idempotency_key"`
	AlgorithmVersion             string    `gorm:"column:algorithm_version;size:32;not null" json:"algorithm_version"`
	CanonicalizationVersion      string    `gorm:"column:canonicalization_version;size:32;not null" json:"canonicalization_version"`
	EvidenceBlockNumber          uint64    `gorm:"column:evidence_block_number;not null" json:"evidence_block_number"`
	EvidenceBlockHash            string    `gorm:"column:evidence_block_hash;size:66;not null" json:"evidence_block_hash"`
	ImplementationAddress        string    `gorm:"column:implementation_address;size:42;not null" json:"implementation_address"`
	ImplementationCodeHash       string    `gorm:"column:implementation_code_hash;size:66;not null" json:"implementation_code_hash"`
	ConfirmationDepth            uint64    `gorm:"column:confirmation_depth;not null" json:"confirmation_depth"`
	MaxHealthyLagBlocks          uint64    `gorm:"column:max_healthy_lag_blocks;not null" json:"max_healthy_lag_blocks"`
	ExpectedWalletCount          uint      `gorm:"column:expected_wallet_count;not null" json:"expected_wallet_count"`
	ExpectedCheckCount           uint      `gorm:"column:expected_check_count;not null" json:"expected_check_count"`
	ExpectedWalletIdentityDigest string    `gorm:"column:expected_wallet_identity_digest;size:64;not null" json:"expected_wallet_identity_digest"`
	ExpectedCheckIdentityDigest  string    `gorm:"column:expected_check_identity_digest;size:64;not null" json:"expected_check_identity_digest"`
	SourcePayload                []byte    `gorm:"column:source_payload;type:mediumblob;not null" json:"-"`
	SourceDigest                 string    `gorm:"column:source_digest;size:64;not null" json:"source_digest"`
	RequestedAt                  time.Time `gorm:"column:requested_at;not null;default:CURRENT_TIMESTAMP(6)" json:"requested_at"`
}

func (*IDOReconciliationRun) TableName() string { return "ido_reconciliation_run" }

type IDOReconciliationJob struct {
	RunID                    uint64     `gorm:"column:run_id;primaryKey" json:"run_id"`
	ProjectID                uint64     `gorm:"column:project_id;not null" json:"project_id"`
	State                    string     `gorm:"column:state;size:24;not null" json:"state"`
	Outcome                  *string    `gorm:"column:outcome;size:16" json:"outcome"`
	ActiveSlot               *uint8     `gorm:"column:active_slot" json:"-"`
	AttemptCount             uint       `gorm:"column:attempt_count;not null" json:"attempt_count"`
	ConsecutiveErrorCount    uint       `gorm:"column:consecutive_error_count;not null" json:"consecutive_error_count"`
	NextWalletOrdinal        uint       `gorm:"column:next_wallet_ordinal;not null" json:"next_wallet_ordinal"`
	NextAttemptAt            time.Time  `gorm:"column:next_attempt_at;not null" json:"next_attempt_at"`
	LeaseToken               string     `gorm:"column:lease_token;size:64;not null" json:"-"`
	LeaseExpiresAt           *time.Time `gorm:"column:lease_expires_at" json:"-"`
	LastErrorCode            string     `gorm:"column:last_error_code;size:64;not null" json:"last_error_code"`
	LastErrorMessage         string     `gorm:"column:last_error_message;size:500;not null" json:"last_error_message"`
	CancelRequestedByAdminID *uint64    `gorm:"column:cancel_requested_by_admin_id" json:"cancel_requested_by_admin_id"`
	CancelReason             *string    `gorm:"column:cancel_reason;size:500" json:"cancel_reason"`
	CancelRequestedAt        *time.Time `gorm:"column:cancel_requested_at" json:"cancel_requested_at"`
	StartedAt                *time.Time `gorm:"column:started_at" json:"started_at"`
	CompletedAt              *time.Time `gorm:"column:completed_at" json:"completed_at"`
	UpdatedAt                time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*IDOReconciliationJob) TableName() string { return "ido_reconciliation_job" }

type IDOReconciliationEvent struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	SequenceNo        uint      `gorm:"column:sequence_no;not null" json:"sequence_no"`
	EventType         string    `gorm:"column:event_type;size:32;not null" json:"event_type"`
	Stage             string    `gorm:"column:stage;size:32;not null" json:"stage"`
	ErrorCode         string    `gorm:"column:error_code;size:64;not null" json:"error_code"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	ActorAdminID      *uint64   `gorm:"column:actor_admin_id" json:"actor_admin_id"`
	LeaseTokenHash    string    `gorm:"column:lease_token_hash;size:64;not null" json:"-"`
	PreviousEventHash string    `gorm:"column:previous_event_hash;size:64;not null" json:"previous_event_hash"`
	EventHash         string    `gorm:"column:event_hash;size:64;not null" json:"event_hash"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*IDOReconciliationEvent) TableName() string { return "ido_reconciliation_event" }

type IDOReconciliationWallet struct {
	ID                 uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID              uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal            uint      `gorm:"column:ordinal;not null" json:"ordinal"`
	WalletAddress      string    `gorm:"column:wallet_address;size:42;not null" json:"wallet_address"`
	ExpectedPaymentRaw string    `gorm:"column:expected_payment_raw;size:78;not null" json:"expected_payment_raw"`
	ActualPaymentRaw   string    `gorm:"column:actual_payment_raw;size:78;not null" json:"actual_payment_raw"`
	ExpectedTokensRaw  string    `gorm:"column:expected_tokens_raw;size:78;not null" json:"expected_tokens_raw"`
	ActualTokensRaw    string    `gorm:"column:actual_tokens_raw;size:78;not null" json:"actual_tokens_raw"`
	ExpectedClaimedRaw string    `gorm:"column:expected_claimed_raw;size:78;not null" json:"expected_claimed_raw"`
	ActualClaimedRaw   string    `gorm:"column:actual_claimed_raw;size:78;not null" json:"actual_claimed_raw"`
	ExpectedRefunded   bool      `gorm:"column:expected_refunded;not null" json:"expected_refunded"`
	ActualRefunded     bool      `gorm:"column:actual_refunded;not null" json:"actual_refunded"`
	PreviousRowDigest  string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	IdentityDigest     string    `gorm:"column:identity_digest;size:64;not null" json:"-"`
	RowDigest          string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	ObservedAt         time.Time `gorm:"column:observed_at;not null" json:"observed_at"`
}

func (*IDOReconciliationWallet) TableName() string { return "ido_reconciliation_wallet" }

type IDOReconciliationCheck struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RunID             uint64    `gorm:"column:run_id;not null" json:"run_id"`
	Ordinal           uint      `gorm:"column:ordinal;not null" json:"ordinal"`
	Scope             string    `gorm:"column:scope;size:16;not null" json:"scope"`
	Code              string    `gorm:"column:code;size:160;not null" json:"code"`
	OK                bool      `gorm:"column:ok;not null" json:"ok"`
	ExpectedExact     string    `gorm:"column:expected_exact;size:500;not null" json:"expected_exact"`
	ActualExact       string    `gorm:"column:actual_exact;size:500;not null" json:"actual_exact"`
	Message           string    `gorm:"column:message;size:500;not null" json:"message"`
	PreviousRowDigest string    `gorm:"column:previous_row_digest;size:64;not null" json:"-"`
	IdentityDigest    string    `gorm:"column:identity_digest;size:64;not null" json:"-"`
	RowDigest         string    `gorm:"column:row_digest;size:64;not null" json:"row_digest"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*IDOReconciliationCheck) TableName() string { return "ido_reconciliation_check" }

type IDOReconciliationManifest struct {
	RunID                  uint64    `gorm:"column:run_id;primaryKey" json:"run_id"`
	ProjectID              uint64    `gorm:"column:project_id;not null" json:"project_id"`
	Outcome                string    `gorm:"column:outcome;size:16;not null" json:"outcome"`
	WalletCount            uint      `gorm:"column:wallet_count;not null" json:"wallet_count"`
	CheckCount             uint      `gorm:"column:check_count;not null" json:"check_count"`
	FailedCheckCount       uint      `gorm:"column:failed_check_count;not null" json:"failed_check_count"`
	Payload                []byte    `gorm:"column:payload;type:mediumblob;not null" json:"-"`
	PayloadDigest          string    `gorm:"column:payload_digest;size:64;not null" json:"payload_digest"`
	WalletEvidenceDigest   string    `gorm:"column:wallet_evidence_digest;size:64;not null" json:"wallet_evidence_digest"`
	CheckEvidenceDigest    string    `gorm:"column:check_evidence_digest;size:64;not null" json:"check_evidence_digest"`
	PreviousManifestDigest string    `gorm:"column:previous_manifest_digest;size:64;not null" json:"previous_manifest_digest"`
	ManifestDigest         string    `gorm:"column:manifest_digest;size:64;not null" json:"manifest_digest"`
	CompletedAt            time.Time `gorm:"column:completed_at;not null" json:"completed_at"`
}

func (*IDOReconciliationManifest) TableName() string { return "ido_reconciliation_manifest" }

type UserRefundRecord struct {
	ID          uint64          `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID         int             `gorm:"column:uid;not null" json:"uid"`
	ProjectID   int             `gorm:"column:project_id;not null" json:"project_id"`
	RefundToken decimal.Decimal `gorm:"column:refund_token;type:decimal(36,18);not null" json:"refund_token"`
	BlockNumber int             `gorm:"column:block_number;not null" json:"block_number"`
	TxHash      string          `gorm:"column:tx_hash;size:100;not null" json:"tx_hash"`
	CreatedAt   time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*UserRefundRecord) TableName() string {
	return "user_refund_record"
}

type ApplyProject struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Name            string    `gorm:"column:name;size:100;not null;default:''" json:"name"`
	City            string    `gorm:"column:city;size:100;not null;default:''" json:"city"`
	Email           string    `gorm:"column:email;size:200;not null;default:''" json:"email"`
	Telegram        string    `gorm:"column:telegram;size:100;not null;default:''" json:"telegram"`
	Wechat          string    `gorm:"column:wechat;size:100;not null;default:''" json:"wechat"`
	Major           string    `gorm:"column:major;size:200;not null;default:''" json:"major"`
	Identity        string    `gorm:"column:identity;size:50;not null;default:''" json:"identity"`
	Organization    string    `gorm:"column:organization;size:200;not null;default:''" json:"organization"`
	ProjectName     string    `gorm:"column:project_name;size:200;not null;default:''" json:"project_name"`
	ProjectDesc     string    `gorm:"column:project_desc;size:500;not null;default:''" json:"project_desc"`
	ProjectCategory string    `gorm:"column:project_category;size:50;not null;default:''" json:"project_category"`
	CurrentStep     string    `gorm:"column:current_step;size:50;not null;default:''" json:"current_step"`
	Possessions     string    `gorm:"column:possessions;size:200;not null" json:"possessions"`
	ApplyReason     string    `gorm:"column:apply_reason;type:text;not null" json:"apply_reason"`
	SupportWanted   string    `gorm:"column:support_wanted;size:200;not null;default:''" json:"support_wanted"`
	ProjectLink     string    `gorm:"column:project_link;size:500;not null;default:''" json:"project_link"`
	AdditionalInfo  string    `gorm:"column:additional_info;type:text;not null" json:"additional_info"`
	Status          int8      `gorm:"column:status;not null;default:0" json:"status"`
	WalletAddress   string    `gorm:"column:wallet_address;size:100;not null;default:''" json:"wallet_address"`
	RecordID        string    `gorm:"column:record_id;size:50;not null;default:'';uniqueIndex" json:"record_id"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*ApplyProject) TableName() string {
	return "apply_project"
}

type UserPoint struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID       int       `gorm:"column:uid;not null;default:0;index" json:"uid"`
	Point     int       `gorm:"column:point;not null;default:0" json:"point"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*UserPoint) TableName() string {
	return "user_point"
}

type UserPointFlow struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID         int       `gorm:"column:uid;not null;default:0;index" json:"uid"`
	ChangePoint int       `gorm:"column:change_point;not null;default:0" json:"change_point"`
	Remain      int       `gorm:"column:remain;not null;default:0" json:"remain"`
	Type        string    `gorm:"column:type;size:32;not null;default:''" json:"type"`
	Desc        string    `gorm:"column:desc;size:255;not null;default:''" json:"desc"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
	CreatedAt   time.Time `gorm:"column:createad_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*UserPointFlow) TableName() string {
	return "user_point_flow"
}

type InviteRecord struct {
	ID            int       `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID           int       `gorm:"column:uid;not null;default:0;index" json:"uid"`
	InviterUID    int       `gorm:"column:inviter_uid;not null;default:0;index" json:"inviter_uid"`
	ReceiveAmount int       `gorm:"column:receive_amount;not null;default:0;index" json:"receive_amount"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (*InviteRecord) TableName() string {
	return "invite_record"
}

type InviteUserCode struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID       int       `gorm:"column:uid;not null;index" json:"uid"`
	Code      string    `gorm:"column:code;size:64;not null;default:'';uniqueIndex" json:"code"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (*InviteUserCode) TableName() string {
	return "invite_user_code"
}

type UserCheckInRecord struct {
	ID                    int       `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UID                   int       `gorm:"column:uid;not null;default:0;index" json:"uid"`
	WalletAddress         string    `gorm:"column:wallet_address;size:42;not null;default:'';index" json:"wallet_address"`
	CheckInDate           string    `gorm:"column:check_in_date;size:10;not null;default:'';index" json:"check_in_date"`
	CheckInDay            int64     `gorm:"column:check_in_day;not null;default:0;index" json:"check_in_day"`
	CheckInAt             time.Time `gorm:"column:check_in_at" json:"check_in_at"`
	Point                 int       `gorm:"column:point;not null;default:0" json:"point"`
	TotalCheckInDays      int       `gorm:"column:total_check_in_days;not null;default:0" json:"total_check_in_days"`
	ContinuousCheckInDays int       `gorm:"column:continuous_check_in_days;not null;default:0" json:"continuous_check_in_days"`
	Nonce                 string    `gorm:"column:nonce;size:66;not null;default:'';index" json:"nonce"`
	Deadline              int64     `gorm:"column:deadline;not null;default:0" json:"deadline"`
	Signature             string    `gorm:"column:signature;type:text;not null" json:"signature"`
	TxHash                string    `gorm:"column:tx_hash;size:100;not null;default:''" json:"tx_hash"`
	CreatedAt             time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*UserCheckInRecord) TableName() string {
	return "user_check_in_record"
}

// Task 通用社交任务（X / Telegram）。2026-06-29 schema 重构：单步多平台。
// 旧 follow_url / like_url / retweet_url 列已不再使用（迁移到 task_legacy_v1）。
type Task struct {
	ID             uint64         `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Platform       string         `gorm:"column:platform;size:20;not null;default:'twitter'" json:"platform"`
	TaskKey        string         `gorm:"column:task_key;size:50;not null;default:''" json:"task_key"`
	TargetSlot     uint8          `gorm:"column:target_slot;not null;default:1" json:"target_slot"`
	Title          string         `gorm:"column:title;size:200;not null;default:''" json:"title"`
	Description    string         `gorm:"column:description;size:500;not null;default:''" json:"description"`
	URL            string         `gorm:"column:url;size:500;not null;default:''" json:"url"`
	CTALabel       string         `gorm:"column:cta_label;size:50;not null;default:''" json:"cta_label"`
	VerifyKind     string         `gorm:"column:verify_kind;size:50;not null;default:'manual'" json:"verify_kind"`
	VerifyMeta     string         `gorm:"column:verify_meta;type:text" json:"verify_meta"`
	AmountPoint    int            `gorm:"column:amount_point;not null;default:0" json:"amount_point"`
	Sort           int            `gorm:"column:sort;not null;default:0" json:"sort"`
	IsEnable       int8           `gorm:"column:is_enable;not null;default:1" json:"is_enable"`
	Version        uint32         `gorm:"column:version;not null;default:1" json:"version"`
	TargetRef      string         `gorm:"column:target_ref;size:200;not null;default:''" json:"target_ref"`
	RoundStartedAt time.Time      `gorm:"column:round_started_at;not null;default:CURRENT_TIMESTAMP" json:"round_started_at"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (*Task) TableName() string {
	return "task"
}

type TaskRecord struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	TaskID       int       `gorm:"column:task_id;not null;default:0;uniqueIndex:uk_task_version_uid" json:"task_id"`
	Platform     string    `gorm:"column:platform;size:20;not null;default:'twitter'" json:"platform"`
	TaskVersion  uint32    `gorm:"column:task_version;not null;default:1;uniqueIndex:uk_task_version_uid" json:"task_version"`
	UID          int       `gorm:"column:uid;not null;default:0;uniqueIndex:uk_task_version_uid" json:"uid"`
	AmountPoint  int       `gorm:"column:amount_point;not null" json:"amount_point"`
	VerifyStatus string    `gorm:"column:verify_status;size:32;not null;default:'manual_pass'" json:"verify_status"`
	TargetRef    string    `gorm:"column:target_ref;size:200;not null;default:''" json:"target_ref"`
	TargetURL    string    `gorm:"column:target_url;size:500;not null;default:''" json:"target_url"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updated_at"`
}

func (*TaskRecord) TableName() string {
	return "task_record"
}

// TaskRound preserves every post-bound X task target so old rewards remain
// auditable and a previously used post cannot be recycled for a new reward.
type TaskRound struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	TaskID      uint64    `gorm:"column:task_id;not null;uniqueIndex:uk_task_round;uniqueIndex:uk_task_target" json:"task_id"`
	Version     uint32    `gorm:"column:version;not null;default:1;uniqueIndex:uk_task_round" json:"version"`
	TargetRef   string    `gorm:"column:target_ref;size:200;not null;uniqueIndex:uk_task_target" json:"target_ref"`
	TargetURL   string    `gorm:"column:target_url;size:500;not null;default:''" json:"target_url"`
	AmountPoint int       `gorm:"column:amount_point;not null;default:0" json:"amount_point"`
	StartedAt   time.Time `gorm:"column:started_at;not null;default:CURRENT_TIMESTAMP" json:"started_at"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*TaskRound) TableName() string {
	return "task_round"
}

// TaskTargetHistory audits X post targets across slots and rounds. An operator
// may reuse a post in a different slot; per-task round uniqueness still applies.
type TaskTargetHistory struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Platform    string    `gorm:"column:platform;size:20;not null" json:"platform"`
	TaskKey     string    `gorm:"column:task_key;size:50;not null" json:"task_key"`
	TargetRef   string    `gorm:"column:target_ref;size:200;not null" json:"target_ref"`
	TaskID      uint64    `gorm:"column:task_id;not null" json:"task_id"`
	TaskVersion uint32    `gorm:"column:task_version;not null" json:"task_version"`
	TargetSlot  uint8     `gorm:"column:target_slot;not null" json:"target_slot"`
	TargetURL   string    `gorm:"column:target_url;size:500;not null;default:''" json:"target_url"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*TaskTargetHistory) TableName() string {
	return "task_target_history"
}
