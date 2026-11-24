package taskhall

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	taskhallService "joincare_server/pkg/core/service/taskhall"
)

func PointFlows(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	res, err := taskhallService.PointFlows(tools.UID(c), page, pageSize)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func Info(c *gin.Context) {
	res, err := taskhallService.Info(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func CheckInNonce(c *gin.Context) {
	res, err := taskhallService.CheckInNonce(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func CheckIn(c *gin.Context) {
	var req CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ret.Error(c, err)
		return
	}
	res, err := taskhallService.CheckIn(tools.UID(c), req.TxHash)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func ReconcileCheckIn(c *gin.Context) {
	res, err := taskhallService.ReconcileCheckIn(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

type completeTaskRequest struct {
	Platform    string `json:"platform"`
	TaskID      uint64 `json:"task_id"`
	TaskVersion uint32 `json:"task_version"`
	TaskKey     string `json:"task_key"`
	ReplyText   string `json:"reply_text"`
}

func CompleteTask(c *gin.Context) {
	// task_id/version are optional for backward compatibility. New clients send
	// both so a stale tab cannot complete a newly published task round.
	var req completeTaskRequest
	_ = c.ShouldBindJSON(&req)
	if req.Platform == "" {
		req.Platform = "twitter"
	}

	err := taskhallService.CompleteTaskVersioned(
		tools.UID(c), req.Platform, req.TaskID, req.TaskVersion, req.TaskKey, req.ReplyText,
	)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, nil, nil)
}

// CheckTgJoin actively verifies group membership and awards the join task.
func CheckTgJoin(c *gin.Context) {
	res, err := taskhallService.CheckTgJoin(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func XBinding(c *gin.Context) {
	res, err := taskhallService.XBinding(tools.UID(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}
