package sign

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"joincare_server/pkg/common/constant"
)

func signedContext(token, requestID, uid, timestamp string) (*gin.Context, string) {
	request := httptest.NewRequest("GET", "/client/test?q=1", nil)
	request.Header.Set(constant.HEADER_JOINCARE_REQUEST_ID, requestID)
	request.Header.Set(constant.HEADER_JOINCARE_SIGN, token)
	request.Header.Set(constant.HEADER_JOINCARE_PERSON, uid)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = request
	material := request.URL.RequestURI() + requestID + token[len(token)-8:] + uid + timestamp
	return c, material
}

func TestVerifySignAcceptsHMACSHA256(t *testing.T) {
	token := "a-valid-token-with-at-least-eight-bytes"
	c, material := signedContext(token, "request-1", "42", "1700000000")
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(material))
	if !verifySignFunc(c, hex.EncodeToString(mac.Sum(nil)), "1700000000") {
		t.Fatal("valid HMAC-SHA256 signature rejected")
	}
}

func TestVerifySignRejectsLegacyMD5(t *testing.T) {
	token := "a-valid-token-with-at-least-eight-bytes"
	c, material := signedContext(token, "request-2", "42", "1700000000")
	digest := md5.Sum([]byte(material)) // test fixture for the retired algorithm
	if verifySignFunc(c, hex.EncodeToString(digest[:]), "1700000000") {
		t.Fatal("legacy MD5 signature was accepted")
	}
}
