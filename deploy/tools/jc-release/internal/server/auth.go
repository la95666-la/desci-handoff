package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie   = "jcr_session"
	sessionLifetime = 7 * 24 * time.Hour
)

// VerifyPassword returns true if password matches the bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SignSession produces a cookie value binding username to an expiry, HMAC-signed with secret.
func SignSession(secret, username string, expiresAt time.Time) string {
	body := fmt.Sprintf("%s|%d", username, expiresAt.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(body)) + "." + sig
}

// VerifySession returns the username encoded in cookie if signature+expiry are valid.
func VerifySession(secret, cookie string) (string, error) {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return "", errors.New("malformed session")
	}
	bodyBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("bad session body")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("bad session sig")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(expected, sigBytes) {
		return "", errors.New("invalid signature")
	}
	segs := strings.SplitN(string(bodyBytes), "|", 2)
	if len(segs) != 2 {
		return "", errors.New("bad session payload")
	}
	exp, err := strconv.ParseInt(segs[1], 10, 64)
	if err != nil {
		return "", errors.New("bad expiry")
	}
	if time.Now().After(time.Unix(exp, 0)) {
		return "", errors.New("session expired")
	}
	return segs[0], nil
}

// SetSessionCookie writes a signed session cookie into w.
func SetSessionCookie(w http.ResponseWriter, secret, username string, tlsOn bool) {
	exp := time.Now().Add(sessionLifetime)
	v := SignSession(secret, username, exp)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    v,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   tlsOn,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, tlsOn bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   tlsOn,
		SameSite: http.SameSiteLaxMode,
	})
}

// CurrentUser returns the username in the request's session cookie, or "" if none/invalid.
func CurrentUser(r *http.Request, secret string) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	u, err := VerifySession(secret, c.Value)
	if err != nil {
		return ""
	}
	return u
}
