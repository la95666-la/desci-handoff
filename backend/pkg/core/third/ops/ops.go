// Package ops 运维告警推送（Telegram Bot API 直推运维群）。
// 2026-08-07 A2 口径：运维推送统一走 Telegram，替代原 Lark webhook（已下线）。
package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

// PostAlert 向运维群推一条文本告警。
// 配置：telegram.botToken（复用现有 bot）+ telegram.opsChatID（独立运维群 chat_id）。
// 缺任一配置 → no-op；推送失败 → 仅 warn 日志，不阻塞业务流程。
//
// level 仅作为前缀拼接到消息开头（INFO / WARN / ERROR / CRITICAL）。
//
// 用法：
//
//	ops.PostAlert(ctx, "ERROR", "签到链上失败", fmt.Sprintf("uid=%d err=%v", uid, err))
func PostAlert(ctx context.Context, level, title, body string) {
	token := viper.GetString("telegram.botToken")
	chatID := viper.GetString("telegram.opsChatID")
	if token == "" || chatID == "" {
		return
	}
	if title == "" {
		title = "JoinCare 运维告警"
	}
	text := fmt.Sprintf("[%s] %s\n%s", level, title, body)

	payload, _ := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	api := "https://api.telegram.org/bot" + token + "/sendMessage"

	// 用本地 context，避免外层 req 取消时把告警推送也带挂
	send := func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(c, http.MethodPost, api, bytes.NewReader(payload))
		if err != nil {
			slog.Warn("tg ops alert: new request failed", "err", err)
			return
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("tg ops alert: post failed", "err", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			slog.Warn("tg ops alert: non-2xx", "status", resp.StatusCode)
		}
	}
	// 后台发送，不阻塞调用方
	go send()
}
