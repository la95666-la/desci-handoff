package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/service"
	"joincare_server/pkg/core/third/ops"
)

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  false,
			"code":    "ADMIN_LOGIN_BAD_REQUEST",
			"message": err.Error(),
		})
		return
	}

	if err := middleware.CheckAdminLoginLimit(c.Request.Context(), c.ClientIP(), req.Username); err != nil {
		if errors.Is(err, middleware.ErrLoginRateLimited) {
			c.JSON(http.StatusTooManyRequests, gin.H{"status": false, "code": "ADMIN_LOGIN_RATE_LIMITED", "message": "too many login attempts"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_LOGIN_PROTECTION_UNAVAILABLE", "message": "login protection unavailable"})
		}
		return
	}

	result, err := service.Login(req.Username, req.Password, c.ClientIP())
	if err != nil {
		code := "ADMIN_LOGIN_FAILED"
		if err == service.ErrAccountDisabled {
			code = "ADMIN_ACCOUNT_DISABLED"
		}
		// Deduplicate alerts so brute force cannot flood the operations channel.
		if middleware.ShouldSendLoginAlert(c.Request.Context(), req.Username) {
			ops.PostAlert(c.Request.Context(), "WARN", "admin 登录失败",
				fmt.Sprintf("username=%s ip=%s code=%s err=%s", req.Username, c.ClientIP(), code, err.Error()))
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"code":    code,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data":   result,
	})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken POST /admin/v1/refresh
// 注意：路由挂在 authed 组下，所以也校验了当前 access token，避免 refresh token 长期飘在外
func RefreshToken(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_REFRESH_BAD_REQUEST", "message": err.Error()})
		return
	}
	result, err := service.Refresh(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "ADMIN_REFRESH_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": result})
}

func Me(c *gin.Context) {
	adminID, ok := c.Get(middleware.CtxAdminID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"code":    "ADMIN_AUTH_REQUIRED",
			"message": "missing admin context",
		})
		return
	}

	result, err := service.Me(adminID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  false,
			"code":    "ADMIN_ME_FAILED",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data":   result,
	})
}
