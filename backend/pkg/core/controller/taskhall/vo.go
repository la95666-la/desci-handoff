package taskhall

// CheckInRequest V2 自付 gas 模型：前端调合约后把 txHash 传给后端结算积分。
type CheckInRequest struct {
	TxHash string `json:"txHash" binding:"required"`
}
