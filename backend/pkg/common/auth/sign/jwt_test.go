package sign

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestClientJWTRequiresSessionVersionIssuerAndHS256(t *testing.T) {
	secret := "abcdef0123456789abcdef0123456789"
	token, _, err := GenerateToken(9, 4, secret)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(token, secret)
	if err != nil || claims.SessionVersion != 4 {
		t.Fatalf("valid client token rejected: claims=%+v err=%v", claims, err)
	}

	legacy := CustomClaims{UID: 9, RegisteredClaims: jwt.RegisteredClaims{Issuer: "JoinCare.Client", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())}}
	legacyToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString([]byte(secret))
	if _, err := ParseToken(legacyToken, secret); err == nil {
		t.Fatal("legacy client token without session version was accepted")
	}

	wrongIssuer := CustomClaims{UID: 9, SessionVersion: 4, RegisteredClaims: jwt.RegisteredClaims{Issuer: "JoinCare.Admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())}}
	wrongIssuerToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongIssuer).SignedString([]byte(secret))
	if _, err := ParseToken(wrongIssuerToken, secret); err == nil {
		t.Fatal("admin issuer was accepted by client parser")
	}
}
