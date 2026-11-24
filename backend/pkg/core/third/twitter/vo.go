package twitter

import (
	"sync"
	"time"

	"golang.org/x/oauth2"
)

type Config struct {
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
	authURL      string
	tokenURL     string
	oAuthConfig  *oauth2.Config
	sync.Once
}

type Token struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpireTime   time.Time `json:"expireTime"`
	ScopeVersion int       `json:"scopeVersion"`
}

const ReadOnlyScopeVersion = 2

func (t *Token) SupportsReadOnlyVerification() bool {
	return t != nil && t.ScopeVersion >= ReadOnlyScopeVersion
}

var xConfig = &Config{
	scopes: []string{
		"tweet.read",
		"users.read",
		"follows.read",
		"like.read",
		"offline.access",
	},
	authURL:  "https://x.com/i/oauth2/authorize",
	tokenURL: "https://api.x.com/2/oauth2/token",
}
