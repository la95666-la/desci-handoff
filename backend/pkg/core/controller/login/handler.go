package login

import (
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	"joincare_server/pkg/core/service/login"
	"joincare_server/pkg/core/service/loginlog"
)

func Login(c *gin.Context) {
	var req LoginRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := login.Login(req.WalletAddress, req.Address, req.Message, req.Signature, c.ClientIP())
	if err != nil {
		ret.Error(c, err)
		return
	}
	loginlog.Record(int64(res.Uid), c.ClientIP(), c.Request.UserAgent(), loginlog.TypeWallet)
	ret.Success(c, res, nil)
}

func Nonce(c *gin.Context) {
	var req NonceRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := login.Nonce(req.WalletAddress, req.Address, req.Action)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func Register(c *gin.Context) {
	var req RegisterRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := login.Register(req.WalletAddress, req.Address, req.Message, req.Signature, req.InviteCode, c.ClientIP())
	if err != nil {
		ret.Error(c, err)
		return
	}
	loginlog.Record(int64(res.Uid), c.ClientIP(), c.Request.UserAgent(), loginlog.TypeRegister)
	ret.Success(c, res, nil)
}

func RegistrationStatus(c *gin.Context) {
	var req RegistrationStatusRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := login.RegistrationStatus(req.WalletAddress, req.Address)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func Profile(c *gin.Context) {
	res, err := login.Profile(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func AutoLogin(c *gin.Context) {
	uid := tools.UID(c)
	res, err := login.AutoLogin(uid, c.ClientIP())
	if err != nil {
		ret.Error(c, err)
		return
	}
	loginlog.Record(int64(uid), c.ClientIP(), c.Request.UserAgent(), loginlog.TypeAuto)
	ret.Success(c, res, nil)
}

// TgBindLink 颁发 TG bot deep-link URL（需登录态）
//   GET /client/auth/v1/tgBindLink
func TgBindLink(c *gin.Context) {
	res, err := login.TgBindLink(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func XLoginUrl(c *gin.Context) {
	res, err := login.XLoginUrl()
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func XLogin(c *gin.Context) {
	var req XLoginRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	err = login.XLogin(tools.UID(c), req.State, req.Code)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, nil, nil)
}
