package login

type LoginRequest struct {
	WalletAddress string `json:"walletAddress"`
	Address       string `json:"address"`
	Message       string `json:"message"`
	Signature     string `json:"signature"`
}

type RegisterRequest struct {
	WalletAddress string `json:"walletAddress"`
	Address       string `json:"address"`
	Message       string `json:"message"`
	Signature     string `json:"signature"`
	InviteCode    string `json:"inviteCode"`
}

type NonceRequest struct {
	WalletAddress string `form:"walletAddress"`
	Address       string `form:"address"`
	Action        string `form:"action"`
}

type RegistrationStatusRequest struct {
	WalletAddress string `form:"walletAddress"`
	Address       string `form:"address"`
}

type XLoginRequest struct {
	State string `json:"state"`
	Code  string `json:"code"`
}
