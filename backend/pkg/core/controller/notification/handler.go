package notification

import (
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	"joincare_server/pkg/core/service/notification"
)

func List(c *gin.Context) {
	var req ListRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := notification.List(tools.UID(c), req.Cursor, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func RedPoint(c *gin.Context) {
	res := notification.RedPoint(tools.UID(c))
	ret.Success(c, res, nil)
}
