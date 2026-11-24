package sign

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UID            int    `json:"uid"`
	SessionVersion uint64 `json:"session_version"`
	jwt.RegisteredClaims
}

var tokenExpireDuration = time.Hour * 24 * 7

func GenerateToken(uid int, sessionVersion uint64, jwtSecret string) (string, int64, error) {
	if uid <= 0 || sessionVersion == 0 {
		return "", 0, errors.New("invalid user session identity")
	}
	signatureNearExpired := time.Now().Add(tokenExpireDuration)
	claims := CustomClaims{
		UID:            uid,
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(signatureNearExpired), // 过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()),           // 签发时间
			NotBefore: jwt.NewNumericDate(time.Now()),           // 生效时间
			Issuer:    "JoinCare.Client",                        // 签发者
		},
	}
	// 使用HS256算法创建token
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret))
	if err != nil {
		return "", signatureNearExpired.Unix(), err
	}
	// 使用密钥签名token
	return token, signatureNearExpired.Unix(), nil
}

// ParseToken 解析JWT Token
func ParseToken(tokenString string, jwtSecret string) (*CustomClaims, error) {
	// 解析token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	}, jwt.WithIssuer("JoinCare.Client"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())

	if err != nil {
		return nil, err
	}

	// 验证claims
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid && claims.UID > 0 && claims.SessionVersion > 0 {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
