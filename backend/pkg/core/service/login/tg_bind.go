package login

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// TgBindLinkResponse C 端 TaskHall 拿来构造"打开 TG bot"按钮的目标 URL。
type TgBindLinkResponse struct {
	URL       string `json:"url"`
	Token     string `json:"token"`
	BotName   string `json:"bot_name"`
	ExpiresAt int64  `json:"expires_at"`
}

// TgBindLink 颁发 deep-link payload：joincare_<uid>_<unix>_<sig>
//   sig = HMAC-SHA256(secret, "joincare_<uid>_<unix>") 的 hex 前 16 字节
//   tgbot 收到 /start <token> 后用同 secret 校验 + 反推 uid
//   5min 过期
func TgBindLink(uid int) (*TgBindLinkResponse, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("invalid uid")
	}
	secret := viper.GetString("telegram.bindSecret")
	if secret == "" {
		return nil, fmt.Errorf("server telegram.bindSecret not configured")
	}
	botName := viper.GetString("telegram.botUsername")
	if botName == "" {
		botName = "joincare_bot"
	}

	now := time.Now()
	payload := fmt.Sprintf("joincare_%d_%d", uid, now.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))[:16]
	token := payload + "_" + sig

	return &TgBindLinkResponse{
		URL:       fmt.Sprintf("https://t.me/%s?start=%s", botName, token),
		Token:     token,
		BotName:   botName,
		ExpiresAt: now.Add(5 * time.Minute).Unix(),
	}, nil
}
