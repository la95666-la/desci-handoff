package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

func TestAdminJWTRequiresSessionVersionIssuerAndHS256(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, _, err := Generate(7, "admin", "approver", 3, secret)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Parse(token, secret)
	if err != nil || claims.SessionVersion != 3 {
		t.Fatalf("valid admin token rejected: claims=%+v err=%v", claims, err)
	}

	oldClaims := AdminClaims{AdminID: 7, Username: "admin", RoleCode: "approver", Kind: "access", RegisteredClaims: gojwt.RegisteredClaims{Issuer: "JoinCare.Admin", ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: gojwt.NewNumericDate(time.Now())}}
	oldToken, _ := gojwt.NewWithClaims(gojwt.SigningMethodHS256, oldClaims).SignedString([]byte(secret))
	if _, err := Parse(oldToken, secret); err == nil {
		t.Fatal("legacy admin token without session version was accepted")
	}

	wrongAlgorithm, _ := gojwt.NewWithClaims(gojwt.SigningMethodHS384, AdminClaims{AdminID: 7, SessionVersion: 3, RegisteredClaims: gojwt.RegisteredClaims{Issuer: "JoinCare.Admin", ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: gojwt.NewNumericDate(time.Now())}}).SignedString([]byte(secret))
	if _, err := Parse(wrongAlgorithm, secret); err == nil {
		t.Fatal("non-HS256 admin token was accepted")
	}
}
