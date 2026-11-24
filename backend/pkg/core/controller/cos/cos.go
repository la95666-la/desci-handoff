package cos

import (
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/core/service/cos"
)

func SignedUploadUrl(c *gin.Context) {
	var req SignedUploadUrlRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := cos.SignedUploadUrl(req.UploadType, req.Ext)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}
