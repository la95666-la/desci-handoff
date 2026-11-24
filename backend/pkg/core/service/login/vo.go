package login

import "time"

type LoginResponse struct {
	Uid                  int    `json:"uid"`
	Address              string `json:"address"`
	Type                 string `json:"type"`
	Signature            string `json:"signature"`
	SignatureNearExpired int    `json:"signatureNearExpired"`
	InviteCode           string `json:"inviteCode"`
	Point                int    `json:"point"`
	Registered           bool   `json:"registered"`
}

type NonceResponse struct {
	WalletAddress string `json:"walletAddress"`
	Nonce         string `json:"nonce"`
	Message       string `json:"message"`
	ExpiredAt     int64  `json:"expiredAt"`
}

type RegistrationStatusResponse struct {
	WalletAddress string `json:"walletAddress"`
	Registered    bool   `json:"registered"`
	Uid           int    `json:"uid"`
	InviteCode    string `json:"inviteCode"`
}

type ProfileResponse struct {
	Uid           int    `json:"uid"`
	WalletAddress string `json:"walletAddress"`
	Type          string `json:"type"`
	InviteCode    string `json:"inviteCode"`
	Point         int    `json:"point"`
}

const (
	authActionLogin      = "login"
	authActionRegister   = "register"
	authActionWalletBind = "wallet_bind"
	authNonceTTL         = 10 * time.Minute
)

type authNonceRecord struct {
	WalletAddress  string `json:"walletAddress"`
	PrimaryWallet  string `json:"primaryWallet,omitempty"`
	Action         string `json:"action"`
	ExpiredAt      int64  `json:"expiredAt"`
	UID            int    `json:"uid,omitempty"`
	SessionVersion uint64 `json:"sessionVersion,omitempty"`
}

type authMessage struct {
	Domain    string
	Action    string
	Nonce     string
	ExpiredAt time.Time
}
type XLoginUrlResponse struct {
	Url string `json:"url"`
}
