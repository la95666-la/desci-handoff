package tools

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/i18n"
)

func Address(r *http.Request, c *websocket.Conn) string {
	var realIP string
	XForwardedFor := r.Header.Get("X-Forwarded-For")
	if XForwardedFor != "" {
		ips := strings.Split(XForwardedFor, ",")
		if len(ips) > 0 {
			realIP = ips[0]
		}
	}

	if realIP == "" {
		realIP = r.Header.Get("X-Real-IP")
	}

	remoteAddr := c.RemoteAddr().String()
	addrs := strings.Split(remoteAddr, ":")
	var port string
	if l := len(addrs); l > 1 {
		port = addrs[l-1]
	}

	if realIP != "" {
		return fmt.Sprintf("%s:%s", realIP, port)
	}

	return c.RemoteAddr().String()
}

func UID(c *gin.Context) int {
	uid, ok := c.Get(constant.HEADER_JOINCARE_UID)
	if !ok {
		uidStr := c.GetHeader(constant.HEADER_JOINCARE_PERSON)
		uid, _ = strconv.Atoi(uidStr)
		return uid.(int)
	}
	return uid.(int)
}

func Lang(c *gin.Context) string {
	lang := c.GetHeader(constant.HEADER_JOINCARE_LANGUAGE)
	return string(i18n.FindLang(lang))
}

func GetAddr(r *http.Request, c *websocket.Conn) string {
	var realIP string
	XForwardedFor := r.Header.Get("X-Forwarded-For")
	if XForwardedFor != "" {
		ips := strings.Split(XForwardedFor, ",")
		if len(ips) > 0 {
			realIP = ips[0]
		}
	}

	if realIP == "" {
		realIP = r.Header.Get("X-Real-IP")
	}

	remoteAddr := c.RemoteAddr().String()
	addrs := strings.Split(remoteAddr, ":")
	var port string
	if l := len(addrs); l > 1 {
		port = addrs[l-1]
	}

	if realIP != "" {
		return fmt.Sprintf("%s:%s", realIP, port)
	}

	return c.RemoteAddr().String()
}
