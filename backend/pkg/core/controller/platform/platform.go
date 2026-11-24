package platform

import (
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/core/service/platform"
)

func Settings(c *gin.Context) {
	resp := platform.Settings()
	ret.Success(c, resp, nil)
}
