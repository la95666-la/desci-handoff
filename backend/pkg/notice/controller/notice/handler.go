package notice

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/model"
	"joincare_server/pkg/notice/service/notice"
)

func Send(c *gin.Context) {
	var req model.NoticeRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	err = notice.Send(req.Uid, req.Event, req.Data)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, nil, nil)
}

func BatchSend(c *gin.Context) {
	var req model.BatchNoticeRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	errUids := notice.BatchSend(req.Uids, req.Event, req.Data)
	if len(errUids) != 0 {
		ret.Error(c, ret.CusError(i18n.ERR_000_00004, fmt.Sprintf("%v", errUids)))
		return
	}
	ret.Success(c, nil, nil)
}
