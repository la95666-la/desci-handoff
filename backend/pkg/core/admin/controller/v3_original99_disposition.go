package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"joincare_server/pkg/core/service/project"
	"strings"
	"unicode/utf8"
)

// Admin middleware provides authentication, live execute permission and audit.
// Every execution requests exact fixed nonce, target, gas, fee cap and value.
func V3Original99Disposition(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	var req struct {
		Action      string `json:"action"`
		ConfirmHash string `json:"confirm_hash"`
		Reason      string `json:"reason"`
		Nonce       uint64 `json:"nonce"`
		ChainID     uint64 `json:"chain_id"`
		Target      string `json:"target"`
		Gas         uint64 `json:"gas"`
		GasPrice    string `json:"gas_price_raw"`
		Value       string `json:"value_raw"`
	}
	if c.ShouldBindJSON(&req) != nil || utf8.RuneCountInString(strings.TrimSpace(req.Reason)) < 3 || utf8.RuneCountInString(req.Reason) > 500 || req.Nonce != 81 || req.ChainID != 56 || req.Gas != 21000 || req.GasPrice != "100000000" || req.Value != "0" || req.Target != "0xfd712061ee08e070a930ecd45e935ae5d7f5445c" {
		c.JSON(400, gin.H{"status": false, "code": "ORIGINAL99_EXACT_CONFIRMATION_REQUIRED"})
		return
	}
	admin := currentAdminID(c)
	out, err := project.Original99Disposition(c.Request.Context(), req.Action, req.ConfirmHash, func(tx *gorm.DB) error { return requireCurrentProjectExecuteAdmin(tx, admin) })
	if err != nil {
		c.JSON(409, gin.H{"status": false, "code": "ORIGINAL99_DISPOSITION_UNCONFIRMED", "message": "处置结果未确认，请查询原哈希与处置哈希，切勿重复签名或发送。", "data": out})
		return
	}
	c.JSON(200, gin.H{"status": true, "data": out})
}
