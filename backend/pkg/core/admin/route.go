package admin

import (
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/core/admin/controller"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/service/approval"
)

// init 注册 4 眼审批的所有 handler
func init() {
	approval.RegisterAll()
}

// Register mounts /admin/v1/* routes onto the provided router group.
// Mount point in cmd/core/main.go:  admin.Register(httpRouter.Group("/admin"))
func Register(r *gin.RouterGroup) {
	v1 := r.Group("/v1")

	// 公开端点（不需要 Bearer）
	v1.POST("/login", controller.Login)

	// 受保护端点（需要 Bearer admin token）+ 写操作自动审计
	authed := v1.Group("")
	authed.Use(middleware.Auth)
	authed.Use(middleware.Audit) // 写操作落 admin_audit_log（哈希链）
	{
		authed.GET("/me", controller.Me)
		// 自助改密（个人中心场景，无需 rbac 写权限）
		authed.POST("/self/password", controller.ChangeSelfPassword)

		// M7.1 用户管理（Phase 1 只读）
		authed.GET("/users", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.ListUsers)
		authed.GET("/users/:id", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.GetUser)
		authed.DELETE("/users/:id", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.DeleteUser)

		// M8 i18n / 文案管理（原始 by-id 接口保留）
		authed.GET("/copywriting", middleware.RequirePerm(middleware.ModI18n, middleware.ActRead), controller.ListCopywriting)
		authed.POST("/copywriting", middleware.RequirePerm(middleware.ModI18n, middleware.ActWrite), controller.CreateCopywriting)
		authed.PUT("/copywriting/:id", middleware.RequirePerm(middleware.ModI18n, middleware.ActWrite), controller.UpdateCopywriting)
		authed.DELETE("/copywriting/:id", middleware.RequirePerm(middleware.ModI18n, middleware.ActWrite), controller.DeleteCopywriting)
		// 双语聚合接口（每个 key 一行 en + tw）
		authed.GET("/copywriting/bilingual", middleware.RequirePerm(middleware.ModI18n, middleware.ActRead), controller.ListCopywritingBilingual)
		authed.GET("/copywriting/prefixes", middleware.RequirePerm(middleware.ModI18n, middleware.ActRead), controller.ListCopywritingPrefixes)
		authed.POST("/copywriting/bilingual", middleware.RequirePerm(middleware.ModI18n, middleware.ActWrite), controller.UpsertCopywritingBilingual)
		authed.DELETE("/copywriting/by-key", middleware.RequirePerm(middleware.ModI18n, middleware.ActWrite), controller.DeleteCopywritingByKey)

		// F3 新闻管理
		authed.GET("/news", middleware.RequirePerm(middleware.ModNews, middleware.ActRead), controller.ListNews)
		authed.GET("/news/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActRead), controller.GetNews)
		authed.POST("/news", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.CreateNews)
		authed.PUT("/news/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.UpdateNews)
		authed.DELETE("/news/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.DeleteNews)

		// F4 任务管理
		authed.GET("/tasks", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.ListTasks)
		authed.POST("/tasks/sync-pinned-tweet", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.SyncPinnedTweet)
		authed.POST("/tasks", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.CreateTask)
		authed.PUT("/tasks/:id", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.UpdateTask)
		authed.DELETE("/tasks/:id", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.DeleteTask)
		authed.GET("/invite-settings", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.ListInviteSettings)
		authed.PUT("/invite-settings", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.UpdateInviteSettings)
		authed.GET("/twitter-verification-settings", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.ListTwitterVerificationSettings)
		authed.PUT("/twitter-verification-settings", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.UpdateTwitterVerificationSettings)

		// F5 项目申请管理（Phase 1：单人通过/驳回，不含链上/4-eye）
		authed.GET("/applications", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListApplications)
		authed.GET("/applications/:id", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetApplication)
		// V3 Owner Safe handoff: read-only two-provider preflight and original
		// receipt verification. No Safe signature or transaction broadcast here.
		authed.GET("/ido-v3/token-admission", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3TokenAdmissionStatus)
		authed.GET("/ido-v3/token-admission/:asset/receipt", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3TokenAdmissionReceipt)
		authed.GET("/ido-v3/token-admission/:asset/safe-proposal", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3TokenSafeProposal)
		authed.POST("/ido-v3/token-admission/:asset/safe-proposal", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.V3TokenSafeProposal)
		authed.GET("/ido-v3/project-tokens/:address", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3ProjectTokenStatus)
		authed.GET("/ido-v3/project-tokens/:address/receipt", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3ProjectTokenReceipt)
		authed.GET("/ido-v3/project-tokens/:address/safe-proposal", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3ProjectTokenSafeProposal)
		authed.POST("/ido-v3/project-tokens/:address/safe-proposal", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.V3ProjectTokenSafeProposal)
		authed.POST("/applications/:id/clone", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.CloneApprovedApplication)
		authed.POST("/applications/:id/approve", middleware.RequirePerm(middleware.ModProject, middleware.ActApprove), controller.ApproveApplication)
		authed.POST("/applications/:id/reject", middleware.RequirePerm(middleware.ModProject, middleware.ActApprove), controller.RejectApplication)

		// admin 首页聚合统计（真实数据）
		authed.GET("/dashboard/overview", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.DashboardOverview)

		// Launchpad Sprint A: 项目内容 patch + Updates CRUD + 通用上传
		authed.GET("/projects", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListProjects)
		authed.GET("/projects/:id/tracking", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetProjectTracking)
		authed.POST("/projects/:id/tracking", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.ChangeProjectTracking)
		authed.PUT("/projects/:id/listing-visibility", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.UpdateProjectListingVisibility)
		authed.GET("/projects/:id/fund-ledger", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListProjectFundLedger)
		authed.GET("/projects/:id/purchase-entitlements", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListProjectPurchaseEntitlements)
		authed.GET("/projects/:id/wallet-purchase", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetWalletPurchaseDetail)
		authed.GET("/projects/:id/fund-reconciliation", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetProjectFundReconciliation)
		authed.GET("/projects/:id/onchain-reconciliation", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetProjectOnchainReconciliation)
		authed.POST("/projects/:id/onchain-reconciliations", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.CreateProjectOnchainReconciliation)
		authed.GET("/projects/:id/onchain-reconciliations", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListProjectIDOReconciliations)
		authed.GET("/onchain-reconciliations/:runId", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetIDOReconciliation)
		authed.GET("/onchain-reconciliations/:runId/wallets", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListIDOReconciliationWallets)
		authed.POST("/onchain-reconciliations/:runId/cancel", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.CancelIDOReconciliation)
		authed.POST("/projects/:id/create-onchain", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.CreateProjectOnChain)
		authed.POST("/projects/:id/activate-onchain", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.ActivateProjectOnChain)
		authed.POST("/projects/:id/end", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.EndProjectOnChain)
		authed.POST("/projects/:id/cancel", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.CancelProjectOnChain)
		authed.POST("/projects/:id/withdraw-platform-fee", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.WithdrawPlatformFee)
		authed.POST("/ido/pause", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.PauseIDOV2)
		authed.POST("/ido/unpause", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.UnpauseIDOV2)
		authed.GET("/ido/pause-state", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.IDOPausedState)
		authed.GET("/chain-events/health", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ChainEventHealth)
		authed.GET("/chain-events/failures", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListChainEventFailures)
		authed.GET("/chain-events/failures/:id", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetChainEventFailure)
		authed.POST("/chain-events/failures/:id/replay", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.ReplayChainEventFailure)
		authed.GET("/chain-transactions/summary", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ChainTransactionSummary)
		authed.GET("/ido-v3/sender-readiness", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3SenderReadiness)
		authed.GET("/chain-transactions", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListChainTransactions)
		authed.GET("/chain-transactions/:id", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetChainTransaction)
		authed.POST("/chain-transactions/:id/cancel", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.CancelChainTransaction)
		authed.POST("/chain-transactions/:id/v3-original/preflight", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.V3OriginalPreflight)
		authed.POST("/chain-transactions/:id/v3-original/broadcast", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.V3OriginalBroadcast)
		authed.POST("/ido-v3/original99-disposition", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.V3Original99Disposition)
		authed.POST("/projects/:id/disable", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.DisableProject)
		authed.POST("/projects/:id/restore", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.RestoreProject)
		authed.DELETE("/projects/:id", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.DeleteProject)
		authed.GET("/projects/:id/content", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetProjectContent)
		authed.PUT("/projects/:id/content", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.UpdateProjectContent)
		authed.PUT("/projects/:id/min-purchase", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.UpdateProjectMinPurchase)
		// 链上参数（Set 阶段专用表单）：preview 为只读校验+换算预览；submit 走四眼审批
		authed.GET("/projects/:id/onchain-params", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetProjectOnchainParams)
		authed.POST("/projects/:id/onchain-params/preview", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.PreviewProjectOnchainParams)
		authed.POST("/ido-v3/runtime-registration", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.SubmitV3RuntimeRegistration)
		authed.POST("/projects/:id/onchain-params", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.SubmitProjectOnchainParams)
		// Diagnostic-only proposal. This route does not create an approval,
		// authorize a Safe proposal, or serve a wallet proof.
		authed.POST("/projects/:id/ido-v3/policy/preview", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.PreviewV3InitialPolicy)
		authed.GET("/projects/:id/ido-v3/policy/status", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetV3PolicyStatus)
		authed.POST("/projects/:id/ido-v3/policy/reviews", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.StartV3PolicyReview)
		authed.GET("/projects/:id/ido-v3/policy/reviews", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListV3PolicyReviews)
		authed.GET("/projects/:id/ido-v3/policy/reviews/:reviewID", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ReadV3PolicyReview)
		authed.GET("/projects/:id/ido-v3/policy/reviews/:reviewID/safe-handoff", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetV3PolicySafeHandoff)
		authed.GET("/projects/:id/ido-v3/policy/reviews/:reviewID/safe-proposal", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.V3PolicySafeProposal)
		authed.POST("/projects/:id/ido-v3/policy/reviews/:reviewID/safe-proposal", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.V3PolicySafeProposal)
		authed.POST("/projects/:id/ido-v3/policy/reviews/:reviewID/approve", middleware.RequirePerm(middleware.ModProject, middleware.ActApprove), controller.ApproveV3PolicyReview)
		authed.POST("/projects/:id/ido-v3/policy/reviews/:reviewID/observe", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.ObserveV3PolicyExecution)

		fair := authed.Group("/projects/:id/fair-launch")
		fair.GET("/capabilities", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.FairLaunchCapabilities)
		fair.GET("/onchain-params", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.GetFairLaunchSet)
		fair.POST("/onchain-params/preview", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.PreviewFairLaunchSet)
		fair.POST("/onchain-params", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.SubmitFairLaunchSet)
		for _, lifecycle := range []string{"create-onchain", "activate-onchain", "end", "cancel", "release", "withdraw-project-proceeds", "withdraw-platform-fee", "withdraw-failed-project-tokens"} {
			fair.POST("/"+lifecycle+"/preview", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.FairLaunchActionPreview)
			fair.POST("/"+lifecycle, middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.SubmitFairLaunchAction)
		}
		fair.POST("/pause/:action/preview", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.FairLaunchActionPreview)
		fair.POST("/pause/:action", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.SubmitFairLaunchAction)
		fair.POST("/governance/:action/preview", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.FairLaunchActionPreview)
		fair.POST("/governance/:action", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.SubmitFairLaunchAction)
		fair.GET("/actions/execution-queue", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.FairLaunchExecutionQueue)
		fair.POST("/actions/:requestID/execute", middleware.RequirePerm(middleware.ModProject, middleware.ActExecute), controller.ExecuteFairLaunchAction)

		authed.GET("/projects/:id/updates", middleware.RequirePerm(middleware.ModProject, middleware.ActRead), controller.ListProjectUpdates)
		authed.POST("/projects/:id/updates", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.CreateProjectUpdate)
		authed.PUT("/project-updates/:updateId", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.UpdateProjectUpdate)
		authed.DELETE("/project-updates/:updateId", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.DeleteProjectUpdate)
		authed.POST("/upload", middleware.RequirePerm(middleware.ModProject, middleware.ActWrite), controller.AdminUpload)

		// Telegram bot 集成（joincare-tgbot 服务调）
		authed.POST("/tg/bind", middleware.RequirePerm(middleware.ModService, middleware.ActExecute), controller.BindTG)
		authed.GET("/tg/resolve", middleware.RequirePerm(middleware.ModService, middleware.ActRead), controller.ResolveTG)
		authed.POST("/tg/task-complete", middleware.RequirePerm(middleware.ModService, middleware.ActExecute), controller.CompleteTGTask)
		authed.POST("/tg/activity", middleware.RequirePerm(middleware.ModService, middleware.ActExecute), controller.ReportTgActivity)

		// TG 活跃监控（admin 视图）
		authed.GET("/tg-activity/daily", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.TgDailyStats)
		authed.GET("/tg-activity/detail", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.TgActivityDetail)
		authed.GET("/tg-activity/chats", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.TgChats)
		authed.GET("/tg-activity/user-summary", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.TgUserActivity)

		// TG 配置（admin 编辑；tgbot 拉 effective 值使用同一 JWT）
		authed.GET("/tg-settings", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.ListTgSettings)
		authed.PUT("/tg-settings", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.UpdateTgSettings)
		authed.GET("/tg-settings/effective", middleware.RequirePerm(middleware.ModService, middleware.ActRead), controller.GetEffectiveTgSettings)

		// 签到阶梯奖励规则（改动等同于资金规则变更，写操作要求 task:write 权限）
		authed.GET("/checkin-rules", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.ListCheckInRules)
		authed.POST("/checkin-rules", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.CreateCheckInRule)
		authed.PUT("/checkin-rules/:id", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.UpdateCheckInRule)
		authed.DELETE("/checkin-rules/:id", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.DeleteCheckInRule)

		// 首页 Banner（含 3D 动效选择）
		authed.GET("/banners", middleware.RequirePerm(middleware.ModNews, middleware.ActRead), controller.ListBanners)
		authed.POST("/banners", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.CreateBanner)
		authed.PUT("/banners/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.UpdateBanner)
		authed.DELETE("/banners/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.DeleteBanner)

		// 系统公告
		authed.GET("/announcements", middleware.RequirePerm(middleware.ModNews, middleware.ActRead), controller.ListAnnouncements)
		authed.POST("/announcements", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.CreateAnnouncement)
		authed.PUT("/announcements/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.UpdateAnnouncement)
		authed.DELETE("/announcements/:id", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.DeleteAnnouncement)

		// Token 续期（不需要额外权限，只要持有效 access token 即可）
		authed.POST("/refresh", controller.RefreshToken)

		// 操作审计日志（自动落）+ 哈希链核对
		authed.GET("/audit-logs", middleware.RequirePerm("audit", middleware.ActRead), controller.ListAuditLogs)
		authed.GET("/audit-logs/verify-chain", middleware.RequirePerm("audit", middleware.ActRead), controller.VerifyAuditChain)

		// Admin 用户 CRUD（rbac:write 保护）
		authed.GET("/admin-users", middleware.RequirePerm("rbac", middleware.ActRead), controller.ListAdminUsers)
		authed.POST("/admin-users", middleware.RequirePerm("rbac", middleware.ActWrite), controller.CreateAdminUser)
		authed.PUT("/admin-users/:id", middleware.RequirePerm("rbac", middleware.ActWrite), controller.UpdateAdminUser)
		authed.POST("/admin-users/:id/password", middleware.RequirePerm("rbac", middleware.ActWrite), controller.ChangeAdminUserPassword)
		authed.POST("/admin-users/:id/roles", middleware.RequirePerm("rbac", middleware.ActWrite), controller.SetAdminUserRoles)
		authed.DELETE("/admin-users/:id", middleware.RequirePerm("rbac", middleware.ActWrite), controller.DeleteAdminUser)

		// 角色 + 权限矩阵
		authed.GET("/roles", middleware.RequirePerm("rbac", middleware.ActRead), controller.ListRoles)
		authed.GET("/roles/:code/permissions", middleware.RequirePerm("rbac", middleware.ActRead), controller.GetRolePermissions)
		authed.PUT("/roles/:code/permissions", middleware.RequirePerm("rbac", middleware.ActWrite), controller.SetRolePermissions)

		// 当前 admin 自己的权限矩阵（用于前端菜单/按钮显隐）
		authed.GET("/my/permissions", controller.MyPermissions)

		// 4 眼审批工单
		authed.GET("/approvals", middleware.RequirePerm(middleware.ModApproval, middleware.ActRead), controller.ListApprovals)
		authed.GET("/approvals/ops", middleware.RequirePerm(middleware.ModApproval, middleware.ActRead), controller.ListApprovalOps)
		authed.GET("/approvals/:id", middleware.RequirePerm(middleware.ModApproval, middleware.ActRead), controller.GetApproval)
		authed.POST("/approvals/:id/approve", middleware.RequirePerm(middleware.ModApproval, middleware.ActRead), controller.ApproveApproval)
		authed.POST("/approvals/:id/reject", middleware.RequirePerm(middleware.ModApproval, middleware.ActRead), controller.RejectApproval)
		authed.POST("/approvals/:id/cancel", middleware.RequirePerm(middleware.ModApproval, middleware.ActRead), controller.CancelApproval)

		// 回收站（软删除恢复；30d 内可恢复，超期由 cron 硬删）
		authed.GET("/trash", middleware.RequirePerm("audit", middleware.ActRead), controller.TrashList)
		authed.POST("/news/:id/restore", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.RestoreNews)
		authed.POST("/announcements/:id/restore", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.RestoreAnnouncement)
		authed.POST("/banners/:id/restore", middleware.RequirePerm(middleware.ModNews, middleware.ActWrite), controller.RestoreBanner)
		authed.POST("/tasks/:id/restore", middleware.RequirePerm(middleware.ModTask, middleware.ActWrite), controller.RestoreTask)
		authed.POST("/admin-users/:id/restore", middleware.RequirePerm("rbac", middleware.ActWrite), controller.RestoreAdminUser)

		// 用户管理写操作（启停 + 积分调整 + 解绑第三方账号）
		authed.POST("/users/:id/enable", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.EnableUser)
		authed.POST("/users/:id/disable", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.DisableUser)
		authed.POST("/users/:id/adjust-point", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.AdjustUserPoint)
		authed.POST("/users/:id/unbind-twitter", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.UnbindTwitter)
		authed.POST("/users/:id/unbind-telegram", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.UnbindTelegram)

		// 全用户登录日志（独立菜单页）
		authed.GET("/login-logs", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.ListAllLoginLogs)

		// 全用户积分流水（任务/签到/邀请/手工调分）
		// 手工加/扣分复用 /users/:id/adjust-point（|Δ|≥1000 走 4 眼审批）
		authed.GET("/point-flows", middleware.RequirePerm(middleware.ModTask, middleware.ActRead), controller.ListPointFlows)

		// 用户详情：备注 / 登录记录 / 操作时间线 / 汇总
		authed.GET("/users/:id/summary", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.UserSummary)
		authed.GET("/users/:id/notes", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.ListUserNotes)
		authed.POST("/users/:id/notes", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.CreateUserNote)
		authed.DELETE("/users/notes/:noteId", middleware.RequirePerm(middleware.ModUser, middleware.ActWrite), controller.DeleteUserNote)
		authed.GET("/users/:id/login-logs", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.ListUserLoginLogs)
		authed.GET("/users/:id/activity", middleware.RequirePerm(middleware.ModUser, middleware.ActRead), controller.ListUserActivity)
	}
}

// RegisterClient mounts C 端公开接口到 /client 下（不在 /admin 域，由 main.go 单独调用）
func RegisterClient(r *gin.RouterGroup) {
	news := r.Group("/news/v1")
	{
		news.GET("/list", controller.PublicNewsList)
		news.GET("/important-alert", controller.PublicImportantNewsAlert)
		news.GET("/:id", controller.PublicNewsDetail)
	}
	cw := r.Group("/copywriting/v1")
	{
		cw.GET("/list", controller.PublicCopywriting)
	}
	taskhall := r.Group("/taskhall/v1")
	{
		taskhall.GET("/tasks", controller.PublicTaskList)
	}
	banner := r.Group("/banner/v1")
	{
		banner.GET("/current", controller.PublicCurrentBanner)
	}
	stats := r.Group("/stats/v1")
	{
		stats.GET("/overview", controller.PublicStatsOverview)
		stats.POST("/visit", controller.RecordPageVisit)
	}
	announcement := r.Group("/announcement/v1")
	{
		announcement.GET("/current", controller.PublicAnnouncementCurrent)
	}
}
