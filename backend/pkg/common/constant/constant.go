package constant

// FairLaunchMaterializationUnavailable is the single fail-closed reason used
// while CE-V2 cannot transactionally materialize and reverse every financial
// and governance effect. Read-only Fair Launch surfaces remain available.
const (
	FairLaunchMaterializationEnabled     = false
	FairLaunchMaterializationUnavailable = "fair_launch_materialization_unavailable"
)

// 请求头
const (
	HEADER_JOINCARE_SIGNATURE  = "Jc-Signature"
	HEADER_JOINCARE_SIGN       = "Jc-Sign"
	HEADER_JOINCARE_TIME       = "Jc-Time"
	HEADER_JOINCARE_PERSON     = "Jc-Person"
	HEADER_JOINCARE_REQUEST_ID = "Jc-Request-Id"
	HEADER_JOINCARE_TEST       = "Jc-Test"
	HEADER_JOINCARE_UID        = "Jc-Uid"
	HEADER_JOINCARE_LANGUAGE   = "Jc-Language"
	HEADER_DISPLAY_ERR         = "Jc-Display-err"
)

// 对象存储枚举
const (
	COS_PROGECT_LOGO   = "logo"
	COS_PROGECT_BANNER = "banner"
)

// user枚举
const (
	UserTypeBacker  = "BACKER"  // 投资人
	UserTypeCreator = "CREATOR" // 项目方
	UserTypeAdmin   = "ADMIN"   // 管理员
)

// project枚举
const (
	Precision int32 = 18

	FileTypeLogo   = "logo"
	FileTypeBanner = "banner"

	ProjectStatusSetup                    = "Setup"
	ProjectStatusSet                      = "Set"
	ProjectStatusPending                  = "Pending"
	ProjectStatusActive                   = "Active"
	ProjectStatusSuccessfulPendingRelease = "SuccessfulPendingRelease"
	ProjectStatusSuccessful               = "Successful"
	ProjectStatusFailed                   = "Failed"
	ProjectStatusCanceled                 = "Canceled"
	ProjectStatusEnded                    = "Ended"
	ProjectStatusRefund                   = "Refund"

	// Launchpad content lifecycle statuses (not on-chain lifecycle states)
	ProjectStatusPreLaunch = "pre_launch"
	ProjectStatusWarmup    = "warmup"
	ProjectStatusArchived  = "archived"

	ApplyStatusPending = 0
	ApplyStatusAgree   = 1
	ApplyStatusReject  = 2
)

// chain枚举
const (
	ChainProject   = "project"
	ChainProjectV2 = "project_v2"
	ChainProjectV3 = "project_v3"
	ChainTask      = "task"

	ContractVersionUnassigned         = "unassigned"
	ContractVersionV1Legacy           = "v1_legacy"
	ContractVersionV1LegacyUnverified = "v1_legacy_unverified"
	ContractVersionV2Stablecoin       = "v2_stablecoin"
	// Separate new Proxy for new capped-presale projects; never alias the V2 contract.
	ContractVersionIDOV3Stablecoin = "ido_v3_stablecoin"

	PaymentTokenUSDT = "USDT"
	PaymentTokenUSDC = "USDC"

	TransactionTypeV2Create      = "v2_create"
	TransactionTypeV2Activate    = "v2_activate"
	TransactionTypeProjectEnd    = "project_end"
	TransactionTypeProjectCancel = "project_cancel"
	TransactionTypePlatformFee   = "platform_fee_withdraw"
	// 非项目作用域的全局治理 action：project_id=0。必须走 durable outbox，
	// Paused/Unpaused 事件负责把 submission 推进 applied 并释放 relayer signer fence。
	TransactionTypeGlobalPause   = "global_pause"
	TransactionTypeGlobalUnpause = "global_unpause"

	TransactionTypeFairLaunchCreate                  = "fair_launch_create"
	TransactionTypeFairLaunchActivate                = "fair_launch_activate"
	TransactionTypeFairLaunchEnd                     = "fair_launch_end"
	TransactionTypeFairLaunchCancel                  = "fair_launch_cancel"
	TransactionTypeFairLaunchRelease                 = "fair_launch_release"
	TransactionTypeFairLaunchPauseContributions      = "fair_launch_pause_contributions"
	TransactionTypeFairLaunchPausePayouts            = "fair_launch_pause_payouts"
	TransactionTypeFairLaunchPauseClaims             = "fair_launch_pause_claims"
	TransactionTypeFairLaunchProjectProceedsWithdraw = "fair_launch_proceeds_withdraw"
	TransactionTypeFairLaunchPlatformFeeWithdraw     = "fair_launch_platform_fee_withdraw"

	ChainTxStateIntent           = "intent"
	ChainTxStateReady            = "ready"
	ChainTxStateSigned           = "signed"
	ChainTxStateSubmitted        = "submitted"
	ChainTxStateBroadcastUnknown = "broadcast_unknown"
	ChainTxStateMinedSuccess     = "mined_success"
	ChainTxStateMinedRevert      = "mined_revert"
	ChainTxStateFinalSuccess     = "final_success"
	ChainTxStateFinalRevert      = "final_revert"
	ChainTxStateApplied          = "applied"
	ChainTxStateCancelled        = "cancelled"
	ChainTxStateAdminCancelled   = "admin_cancelled"
	ChainTxStateAmbiguous        = "ambiguous"

	ChainEventFailureRetrying   = "retrying"
	ChainEventFailureDeadLetter = "dead_letter"
	ChainEventFailureResolved   = "resolved"
)

// task/points enums
const (
	TaskCheckInPoint = 10
	TaskInvitePoint  = 50

	InviteDailyCapSettingKey = "invite.daily_cap"
	InviteDailyCapDefault    = 10
	InviteDailyCapMax        = 100000

	// TwitterVerificationDisabledSettingKey controls whether supported X tasks
	// use the external read-only X API. The safe default is strict verification.
	TwitterVerificationDisabledSettingKey = "twitter.verification_disabled"

	PointFlowTypeCheckIn = "CHECK_IN"
	PointFlowTypeInvite  = "INVITE"
	PointFlowTypeTask    = "TASK"
	PointFlowTypeAdjust  = "ADJUST" // admin 手动调整
)
