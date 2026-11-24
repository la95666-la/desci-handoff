package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"joincare_server/pkg/core/service/project"
)

func V3TokenSafeProposal(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	asset := strings.ToLower(c.Param("asset"))
	if asset != "usdt" && asset != "testcat" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_TOKEN_INVALID"})
		return
	}
	var signed *project.V3SafeProposal
	if c.Request.Method == http.MethodPost {
		var req project.V3SafeProposal
		if c.ShouldBindJSON(&req) != nil || req.SafeTxHash == "" || req.Sender == "" || req.Signature == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "SAFE_SIGNATURE_REQUIRED"})
			return
		}
		signed = &req
	}
	result, err := project.V3TokenSafeProposal(asset, signed)
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "SAFE_PROPOSAL_UNAVAILABLE", err, "safe_proposal")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": result})
}

func V3ProjectTokenSafeProposal(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	address := c.Param("address")
	if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_TOKEN_INVALID"})
		return
	}
	var signed *project.V3SafeProposal
	if c.Request.Method == http.MethodPost {
		var req project.V3SafeProposal
		if c.ShouldBindJSON(&req) != nil || req.SafeTxHash == "" || req.Sender == "" || req.Signature == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "SAFE_SIGNATURE_REQUIRED"})
			return
		}
		signed = &req
	}
	result, err := project.V3ProjectTokenSafeProposal(address, signed)
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "SAFE_PROPOSAL_UNAVAILABLE", err, "safe_proposal")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": result})
}

func V3PolicySafeProposal(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	id, e1 := strconv.ParseUint(c.Param("id"), 10, 64)
	review, e2 := strconv.ParseUint(c.Param("reviewID"), 10, 64)
	if e1 != nil || e2 != nil || id == 0 || review == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var signed *project.V3SafeProposal
	if c.Request.Method == http.MethodPost {
		var req project.V3SafeProposal
		if c.ShouldBindJSON(&req) != nil || req.SafeTxHash == "" || req.Sender == "" || req.Signature == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "SAFE_SIGNATURE_REQUIRED"})
			return
		}
		signed = &req
	}
	var result project.V3SafeProposal
	var err error
	if !project.ApprovedV3ModeConfigured() {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "SAFE_PROPOSAL_UNAVAILABLE"})
		return
	}
	result, err = project.ApprovedV3PolicySafeProposalContext(c.Request.Context(), id, review, currentAdminID(c), signed)
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "SAFE_PROPOSAL_UNAVAILABLE", err, "safe_proposal")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": result})
}
