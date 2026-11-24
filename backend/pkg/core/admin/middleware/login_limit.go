package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"joincare_server/pkg/core/db"
)

var (
	ErrLoginRateLimited  = errors.New("admin login rate limited")
	ErrLoginLimitBackend = errors.New("admin login protection unavailable")
)

const loginLimitWindow = time.Minute

func hashedLoginIdentity(username string) string {
	normalized := strings.ToLower(strings.TrimSpace(username))
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:])
}

func CheckAdminLoginLimit(ctx context.Context, clientIP, username string) error {
	if db.Redis() == nil {
		return ErrLoginLimitBackend
	}
	userKey := hashedLoginIdentity(username)
	dimensions := []struct {
		key   string
		limit int64
	}{
		{key: fmt.Sprintf("ADMLOGIN:IP:%s", clientIP), limit: 30},
		{key: fmt.Sprintf("ADMLOGIN:U:%s", userKey), limit: 10},
		{key: fmt.Sprintf("ADMLOGIN:IU:%s:%s", clientIP, userKey), limit: 5},
	}
	for _, dimension := range dimensions {
		count, err := db.Redis().FixedWindowIncrement(ctx, dimension.key, loginLimitWindow)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrLoginLimitBackend, err)
		}
		if count > dimension.limit {
			return ErrLoginRateLimited
		}
	}
	return nil
}

func ShouldSendLoginAlert(ctx context.Context, username string) bool {
	if db.Redis() == nil {
		return false
	}
	return db.Redis().SetNX(ctx, "ADMLOGIN:ALERT:"+hashedLoginIdentity(username), "1", time.Minute)
}
