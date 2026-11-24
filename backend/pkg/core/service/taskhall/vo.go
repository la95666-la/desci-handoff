package taskhall

import (
	"time"
)

const (
	checkInNonceTTL = 10 * time.Minute

	taskStatusAvailable = "AVAILABLE"
	taskStatusCompleted = "COMPLETED"
	taskStatusDisabled  = "DISABLED"
)

type CheckInNonceResponse struct {
	WalletAddress     string         `json:"walletAddress"`
	Day               int64          `json:"day"`
	CheckInDate       string         `json:"checkInDate"`
	Nonce             string         `json:"nonce"`
	Deadline          int64          `json:"deadline"`
	ChainID           string         `json:"chainId"`
	VerifyingContract string         `json:"verifyingContract"`
	TypedData         map[string]any `json:"typedData"`
}

type CheckInResponse struct {
	TxHash                string `json:"txHash"`
	Point                 int    `json:"point"`
	TotalPoint            int    `json:"totalPoint"`
	CheckInDate           string `json:"checkInDate"`
	ChainStatus           string `json:"chainStatus"`
	TotalCheckInDays      int    `json:"totalCheckInDays"`
	ContinuousCheckInDays int    `json:"continuousCheckInDays"`
}

type checkInNonceRecord struct {
	UID               int    `json:"uid"`
	WalletAddress     string `json:"walletAddress"`
	CheckInDate       string `json:"checkInDate"`
	Day               int64  `json:"day"`
	Nonce             string `json:"nonce"`
	Deadline          int64  `json:"deadline"`
	ChainID           int64  `json:"chainId"`
	VerifyingContract string `json:"verifyingContract"`
}

type TwitterTaskInfo struct {
	Status           string   `json:"status"`
	CompletedKeys    []string `json:"completedKeys"`    // 兼容旧客户端：用户已完成的 X 任务 task_key 列表
	CompletedTaskIDs []uint64 `json:"completedTaskIds"` // 精确完成态：支持同一 task_key 的两个帖子槽位
}

type TelegramTaskInfo struct {
	Status          string   `json:"status"`
	CompletedKeys   []string `json:"completedKeys"`   // 用户已完成的 TG 任务 task_key 列表
	ChatEarnedPoint int      `json:"chatEarnedPoint"` // TG chat activity 累计已发放积分
}

type InfoResponse struct {
	PointOverview      PointOverview        `json:"pointOverview"`
	CheckInTask        CheckInTaskInfo      `json:"checkInTask"`
	TwitterTask        TwitterTaskInfo      `json:"twitterTask"`
	TelegramTask       TelegramTaskInfo     `json:"telegramTask"`
	InviteTask         InviteTaskInfo       `json:"inviteTask"`
	CheckInLeaderboard []CheckInLeaderboard `json:"checkInLeaderboard"`
}

// XBindingResponse 用户 X 绑定状态 —— 前端据此决定是否弹绑定弹窗
type XBindingResponse struct {
	IsBound            bool   `json:"isBound"`
	TwitterID          string `json:"twitterId"`
	TwitterHandle      string `json:"twitterHandle"`
	TwitterDisplayName string `json:"twitterDisplayName"`
	TwitterAvatar      string `json:"twitterAvatar"`
	// TokenValid: user 绑过 X，但 access_token 已过期或 refresh 失败 —— 需要重新走 OAuth
	TokenValid bool `json:"tokenValid"`
}

type PointOverview struct {
	WalletAddress         string `json:"walletAddress"`
	TotalPoint            int    `json:"totalPoint"`
	TotalCheckInDays      int    `json:"totalCheckInDays"`
	ContinuousCheckInDays int    `json:"continuousCheckInDays"`
}

// PointFlowItem is a current user's point acquisition record. UID is
// intentionally omitted so the client endpoint cannot expose another user.
type PointFlowItem struct {
	ID          int       `json:"id" gorm:"column:id"`
	ChangePoint int       `json:"changePoint" gorm:"column:change_point"`
	Remain      int       `json:"remain" gorm:"column:remain"`
	Type        string    `json:"type" gorm:"column:type"`
	Desc        string    `json:"desc" gorm:"column:desc"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
}

type PointFlowListResponse struct {
	Items      []PointFlowItem `json:"items"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	TotalPoint int             `json:"totalPoint"`
}

type CheckInTaskInfo struct {
	Status string `json:"status"`
	// CheckedDates 本月（UTC）已签到的号（1..31），用于前端日历高亮
	CheckedDates []int `json:"checkedDates"`
}

type InviteTaskInfo struct {
	InviteCode            string `json:"inviteCode"`
	RewardPoint           int    `json:"rewardPoint"`
	SuccessfulInviteCount int    `json:"successfulInviteCount"`
	EarnedPoint           int    `json:"earnedPoint"`
}

type CheckInLeaderboard struct {
	Rank          int    `json:"rank"`
	WalletAddress string `json:"walletAddress"`
	CheckInCount  int    `json:"checkInCount"`
	IsCurrentUser bool   `json:"isCurrentUser"`
}
