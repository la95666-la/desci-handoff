package controller

import (
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"

	projectService "joincare_server/pkg/core/service/project"
)

// Both endpoints are read-only. They neither propose nor execute a Safe
// transaction, and do not change Core's V3 runtime or token policy.
func V3TokenAdmissionStatus(c *gin.Context) {
	status, err := projectService.ReadV3TokenGateStatus()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "V3_TOKEN_GATE_UNAVAILABLE", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": status})
}

func V3ProjectTokenStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	address := c.Param("address")
	if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_TOKEN_INVALID"})
		return
	}
	status, err := projectService.ReadV3ProjectTokenGateStatus(address)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "V3_PROJECT_TOKEN_GATE_UNAVAILABLE", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": status})
}

func V3ProjectTokenReceipt(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	address := c.Param("address")
	if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_TOKEN_INVALID"})
		return
	}
	receipt, err := projectService.VerifyV3ProjectTokenReceipt(address, c.Query("tx_hash"))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "V3_PROJECT_TOKEN_RECEIPT_UNVERIFIED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": receipt})
}

func V3TokenAdmissionReceipt(c *gin.Context) {
	asset := strings.ToLower(c.Param("asset"))
	if asset != "usdt" && asset != "testcat" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_TOKEN_INVALID", "message": "unsupported token"})
		return
	}
	receipt, err := projectService.VerifyV3TokenGateReceipt(asset, c.Query("tx_hash"))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "V3_TOKEN_RECEIPT_UNVERIFIED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": receipt})
}
