package constant

const (
	NoticeLangCn = "zh-cn"
	NoticeLangTw = "zh-tw"
	NoticeLangEn = "en-us"
)

const (
	PurchaseSuccessful   = "PurchaseSuccessful"   // 认购成功
	TokensReleased       = "TokensReleased"       // 代币释放可领取
	ClaimSuccessful      = "ClaimSuccessful"      // 代币领取成功
	ProjectFundingFailed = "ProjectFundingFailed" // 项目募资失败，可申请退款
	RefundSuccessful     = "RefundSuccessful"     // 退款成功
	CheckInSuccessful    = "CheckInSuccessful"    // 签到成功（Phase 1 业务通知）
	TaskCompleted        = "TaskCompleted"        // 任务完成（Phase 1）
	NewsPublished        = "NewsPublished"        // 新闻发布（Phase 1 全员广播）
	// 注意：notifications.type 列 size=20，type 常量不超 20 字符
	ProjectUpdatePublished = "ProjectUpdate" // 项目 Update 发布（Sprint C，推给 subscribers）
)

const (
	ReadPoint = "readPoint"
)
