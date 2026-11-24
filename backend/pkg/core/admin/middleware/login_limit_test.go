package middleware

import (
	"strings"
	"testing"
)

func TestLoginLimitKeysDoNotExposeUsername(t *testing.T) {
	username := "Security.Admin@joincarelabs.com"
	normalized := strings.ToLower(strings.TrimSpace(username))
	if strings.Contains("ADMLOGIN:U:", normalized) {
		t.Fatal("test precondition invalid")
	}
	// The production helper hashes usernames before building Redis keys. Keep
	// this assertion close to the implementation to prevent accidental PII keys.
	if strings.Contains(loginLimitKeyForTest(normalized), normalized) {
		t.Fatal("Redis rate-limit key exposes the username")
	}
}

func loginLimitKeyForTest(username string) string {
	return hashedLoginIdentity(username)
}
