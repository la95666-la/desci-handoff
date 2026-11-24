package route

import (
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/notice/controller/notice"
)

func Register(router *gin.RouterGroup) {
	noticeV1 := router.Group("/v1")
	{
		noticeV1.POST("/send", notice.Send)
		noticeV1.POST("/batchSend", notice.BatchSend)
	}
}
