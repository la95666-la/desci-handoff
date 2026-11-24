package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AdminClaims struct {
	AdminID        int64  `json:"admin_id"`
	Username       string `json:"username"`
	RoleCode       string `json:"role_code"`
	Kind           string `json:"kind"` // "access" / "refresh"
	SessionVersion uint64 `json:"session_version"`
	jwt.RegisteredClaims
}

var (
	accessTTL  = time.Hour * 2      // 短期 access token
	refreshTTL = time.Hour * 24 * 7 // refresh token 7 天
)

// Generate 默认签 access token（保留旧签名兼容）
func Generate(adminID int64, username, roleCode string, sessionVersion uint64, secret string) (string, int64, error) {
	return generate(adminID, username, roleCode, "access", sessionVersion, accessTTL, secret)
}

// GenerateRefresh 签 refresh token
func GenerateRefresh(adminID int64, username, roleCode string, sessionVersion uint64, secret string) (string, int64, error) {
	return generate(adminID, username, roleCode, "refresh", sessionVersion, refreshTTL, secret)
}

func generate(adminID int64, username, roleCode, kind string, sessionVersion uint64, ttl time.Duration, secret string) (string, int64, error) {
	if sessionVersion == 0 {
		return "", 0, errors.New("admin session version must be positive")
	}
	exp := time.Now().Add(ttl)
	claims := AdminClaims{
		AdminID:        adminID,
		Username:       username,
		RoleCode:       roleCode,
		Kind:           kind,
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "JoinCare.Admin",
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", 0, err
	}
	return tok, exp.Unix(), nil
}

func Parse(tokenStr, secret string) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AdminClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithIssuer("JoinCare.Admin"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid && claims.AdminID > 0 && claims.SessionVersion > 0 {
		return claims, nil
	}
	return nil, errors.New("invalid admin token")
}
