package route

import (
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/auth/sign"
	"joincare_server/pkg/common/http/middleware"
	"joincare_server/pkg/core/controller/cos"
	"joincare_server/pkg/core/controller/login"
	"joincare_server/pkg/core/controller/notification"
	"joincare_server/pkg/core/controller/platform"
	"joincare_server/pkg/core/controller/project"
	"joincare_server/pkg/core/controller/taskhall"
	"joincare_server/pkg/core/controller/wallet"
)

func Register(router *gin.RouterGroup) {
	RegisterWithoutSign(router)

	signRoute := router.Group("")
	signRoute.Use(sign.Check)
	loginV1 := signRoute.Group("/login/v1")
	{
		loginV1.POST("/xLoginUrl", middleware.RateLimit("xLoginUrl", 20, time.Minute), login.XLoginUrl)
		loginV1.POST("/xLogin", middleware.RateLimit("xLogin", 20, time.Minute), login.XLogin)
		loginV1.POST("/autoLogin", login.AutoLogin)
	}
	auth := signRoute.Group("/auth/v1")
	{
		auth.GET("/profile", login.Profile)
		auth.GET("/tgBindLink", middleware.RateLimit("tgBindLink", 20, time.Minute), login.TgBindLink)
		// 多钱包绑定（Phase 1：无 challenge，直接信 JWT）
		auth.GET("/wallets", wallet.List)
		auth.GET("/wallets/nonce", middleware.RateLimit("walletBindNonce", 20, time.Minute), wallet.BindNonce)
		auth.POST("/wallets", wallet.Bind)
		auth.DELETE("/wallets/:id", wallet.Unbind)
		auth.POST("/wallets/:id/primary", wallet.SetPrimary)
	}
	projectV1 := signRoute.Group("/project/v1")
	{
		projectV1.GET("/ido-v3/:projectID/proof", middleware.RateLimit("idoV3Proof", 10, time.Minute), project.V3WalletProof)
		projectV1.POST("/new", project.NewProject)
		projectV1.GET("/pendingClaimList", project.PendingClaimList)
		projectV1.GET("/purchaseRecord", project.PurchaseRecord)
		projectV1.GET("/claimRecord", project.ClaimRecord)
		projectV1.GET("/refundRecord", project.RefundRecord)
		projectV1.GET("/v2Portfolio", project.V2Portfolio)
		projectV1.GET("/ido-v3/portfolio", project.IDOV3Portfolio)
		projectV1.GET("/fair-launch/:projectID/position", project.FairLaunchPosition)
		projectV1.GET("/fair-launch/portfolio", project.FairLaunchPortfolio)
		projectV1.GET("/myProject", project.MyProject)
		projectV1.GET("/:projectID/investors", project.Investors)
		projectV1.GET("/:projectID/fundingState", project.V2FundingState)
		projectV1.POST("/submitProjectReview", project.SubmitProjectReview)

		// Sprint A: 用户参与行为（Notify me / Watchlist / Whitelist / 综合状态）
		projectV1.POST("/subscribe", project.Subscribe)
		projectV1.DELETE("/subscribe", project.Unsubscribe)
		projectV1.POST("/watch", project.Watch)
		projectV1.DELETE("/watch", project.Unwatch)
		projectV1.POST("/whitelist", project.JoinWhitelist)
		projectV1.GET("/userStatus", project.UserStatus)
	}
	cosV1 := signRoute.Group("/cos/v1")
	{
		cosV1.POST("/signedUploadUrl", cos.SignedUploadUrl)
	}
	noticeV1 := signRoute.Group("/notice/v1")
	{
		noticeV1.GET("/list", notification.List)
		noticeV1.GET("/redPoint", notification.RedPoint)
	}
	platformV1 := signRoute.Group("/platform/v1")
	{
		platformV1.GET("/settings", platform.Settings)
	}
	taskhallV1 := signRoute.Group("/taskhall/v1")
	{
		taskhallV1.GET("/info", taskhall.Info)
		taskhallV1.GET("/pointFlows", taskhall.PointFlows)
		taskhallV1.GET("/checkInNonce", taskhall.CheckInNonce)
		taskhallV1.POST("/checkIn", middleware.RateLimit("checkIn", 10, time.Minute), taskhall.CheckIn)
		taskhallV1.POST("/checkIn/reconcile", middleware.RateLimit("checkInReconcile", 10, time.Minute), taskhall.ReconcileCheckIn)
		taskhallV1.POST("/completeTask", middleware.RateLimit("completeTask", 30, time.Minute), taskhall.CompleteTask)
		taskhallV1.POST("/checkTgJoin", middleware.RateLimit("checkTgJoin", 20, time.Minute), taskhall.CheckTgJoin)
		taskhallV1.GET("/xBinding", taskhall.XBinding)
	}
}

func RegisterWithoutSign(router *gin.RouterGroup) {
	loginV1 := router.Group("/login/v1")
	{
		loginV1.POST("/register", middleware.RateLimit("register", 10, time.Minute), login.Register)
		loginV1.POST("/login", middleware.RateLimit("login", 20, time.Minute), login.Login)
		loginV1.GET("/registrationStatus", middleware.RateLimit("registrationStatus", 30, time.Minute), login.RegistrationStatus)
	}
	auth := router.Group("/auth/v1")
	{
		auth.GET("/nonce", middleware.RateLimit("nonce", 30, time.Minute), login.Nonce)
	}
	projectV1 := router.Group("/project/v1")
	{
		projectV1.GET("/list", project.List)
		projectV1.GET("/detail", project.Detail)

		// Sprint A: 公开 Updates 列表（未登录可查已发布贴）
		projectV1.GET("/updates", project.PublicUpdates)
	}

}
