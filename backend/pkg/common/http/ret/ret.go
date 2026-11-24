package ret

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/i18n"
)

var format = `{{%d}}`
var serverErr = CusError(i18n.ERR_000_00001)

type Result struct {
	Status  bool   `json:"status"`
	Data    any    `json:"data"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

type CustomizeError struct {
	Code      string
	ErrorMsgs []any
}

func (e *CustomizeError) Error() string {
	return fmt.Sprintf("code:%s", e.Code)
}

func (e *CustomizeError) Info() (string, []any) {
	return e.Code, e.ErrorMsgs
}

func CusError(code string, msgs ...any) *CustomizeError {
	msgErr := &CustomizeError{Code: code}
	msgErr.ErrorMsgs = msgs
	return msgErr
}

func DecorationError(err error, code string, msgs ...any) *CustomizeError {
	var cusErr *CustomizeError
	ok := errors.As(err, &cusErr)
	if !ok {
		return CusError(code, msgs)
	}
	return cusErr
}

func Success(c *gin.Context, data any, err error) {
	if data == nil {
		data = map[string]interface{}{}
	}
	res := &Result{Status: true, Data: data}
	if err != nil {
		lang := c.GetHeader(constant.HEADER_JOINCARE_LANGUAGE)
		res.msg(c, lang, err)
	}
	c.JSON(http.StatusOK, res)
}

func Error(c *gin.Context, err error) {
	res := &Result{Status: false}
	lang := c.GetHeader(constant.HEADER_JOINCARE_LANGUAGE)
	res.msg(c, lang, err)
	c.AbortWithStatusJSON(http.StatusOK, res)
}

func (v *Result) msg(c *gin.Context, lang string, err error) {
	if err == nil {
		return
	}
	lang = string(i18n.FindLang(lang))
	var cusErr *CustomizeError
	ok := errors.As(err, &cusErr)
	if !ok {
		_, exists := c.Get(constant.HEADER_DISPLAY_ERR)
		if viper.GetString("application.environment") != "prod" || exists {
			v.Message = err.Error()
			v.Code, _ = serverErr.Info()
			return
		} else {
			cusErr = serverErr
		}
	}

	code, msgs := cusErr.Info()
	msgStr := i18n.Localize(i18n.I18nLang(lang), code, msgs)
	v.Message = msgStr
	v.Code = code
}
