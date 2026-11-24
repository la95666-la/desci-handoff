package twitter

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"testing"
)

func TestGetAuthURLUsesS256PKCE(t *testing.T) {
	NewConfig("client", "secret", "https://joincarelabs.com/oauth/callback")
	state, verifier, rawURL := GetConfig().GetAuthUrl()
	if state == "" || len(verifier) != 64 {
		t.Fatalf("unexpected state/verifier: state=%q verifierLen=%d", state, len(verifier))
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	query := parsed.Query()
	if query.Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE method = %q", query.Get("code_challenge_method"))
	}
	digest := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(digest[:])
	if query.Get("code_challenge") != want {
		t.Fatalf("PKCE challenge mismatch: got=%q want=%q", query.Get("code_challenge"), want)
	}
}
