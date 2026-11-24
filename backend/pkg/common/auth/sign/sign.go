package sign

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

func Check(c *gin.Context) {
	u := c.Request.URL.Path
	if isWhiteList(u) {
		c.Next()
		return
	}
	if DecodeUID(u, c) == false {
		unauthorized(c)
		return
	}
	if !config.Prod() {
		check := c.GetHeader(constant.HEADER_JOINCARE_TEST)
		if check == "1" {
			c.Next()
			return
		}
	}
	sign := c.GetHeader(constant.HEADER_JOINCARE_SIGNATURE)
	if sign == "" {
		unauthorized(c)
		return
	}

	requestTime := c.GetHeader(constant.HEADER_JOINCARE_TIME)
	ot, err := strconv.Atoi(requestTime)
	if err != nil {
		unauthorized(c)
		return
	}
	if tl := time.Now().Unix() - int64(ot); tl > 600 || tl < -600 {
		unauthorized(c)
		return
	}

	res := verifySignFunc(c, sign, requestTime)
	if res == false {
		unauthorized(c)
		return
	}
	c.Next()
}

func verifySignFunc(c *gin.Context, sign, t string) bool {
	// 签名材料：body + url + requestid + token8 + uid + time（前后端保持一致）
	build := strings.Builder{}
	if c.Request.Method != http.MethodGet {
		buf := new(bytes.Buffer)
		buf.ReadFrom(c.Request.Body)
		c.Request.Body.Close()
		c.Request.Body = io.NopCloser(buf)
		if string(buf.Bytes()) != "\"\"" {
			build.Write(buf.Bytes())
		}
	}
	build.WriteString(c.Request.RequestURI)
	build.WriteString(c.GetHeader(constant.HEADER_JOINCARE_REQUEST_ID))
	token := c.GetHeader(constant.HEADER_JOINCARE_SIGN)
	if token != "" {
		build.WriteString(token[len(token)-8:])
	}
	build.WriteString(c.GetHeader(constant.HEADER_JOINCARE_PERSON))
	build.WriteString(t)

	// 首选 HMAC-SHA256（以完整 JWT 为密钥）——替代弱 MD5
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(build.String()))
	if hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(sign)) {
		return true
	}
	return false
}

func DecodeUID(url string, c *gin.Context) bool {
	signature := c.GetHeader(constant.HEADER_JOINCARE_SIGN)
	if signature == "" && isWithoutSignatureList(url) {
		return false
	}

	claims, err := ParseToken(signature, viper.GetString("jwt.secret"))
	if err != nil {
		preconditionRequired(c)
		return false
	}

	var current entity.User
	if err := db.Mysql().Select("id,is_enable,session_version").Where("id = ? AND is_enable = 1", claims.UID).Take(&current).Error; err != nil || current.SessionVersion != claims.SessionVersion {
		unauthorized(c)
		return false
	}
	c.Set(constant.HEADER_JOINCARE_UID, claims.UID)
	return true
}

func isWhiteList(u string) bool {
	accurate := WhiteAccurateList()
	_, ok := accurate[u]
	if ok {
		return true
	}
	fuzzy := WhiteFuzzyList()
	for _, s := range fuzzy {
		if strings.HasPrefix(u, s) {
			return true
		}
	}
	return false
}

func isWithoutSignatureList(u string) bool {
	accurate := WithoutSignatureList()
	_, ok := accurate[u]
	return ok
}

func WhiteAccurateList() map[string]struct{} {
	return map[string]struct{}{}
}

func WhiteFuzzyList() []string {
	return []string{}
}

func WithoutSignatureList() map[string]struct{} {
	return map[string]struct{}{}
}

func forbidden(c *gin.Context) {
	c.Writer.WriteHeader(http.StatusForbidden)
	c.Abort()
}

func preconditionRequired(c *gin.Context) {
	c.Writer.WriteHeader(http.StatusPreconditionRequired)
	c.Abort()
}

func unauthorized(c *gin.Context) {
	c.Writer.WriteHeader(http.StatusUnauthorized)
	c.Abort()
}
