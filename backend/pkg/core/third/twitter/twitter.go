package twitter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/oauth2"

	"joincare_server/pkg/common/utils"
)

func NewConfig(clientID, clientSecret, redirectURL string) {
	xConfig.clientID = clientID
	xConfig.clientSecret = clientSecret
	xConfig.redirectURL = redirectURL
	xConfig.oAuthConfig = &oauth2.Config{
		ClientID:     xConfig.clientID,
		ClientSecret: xConfig.clientSecret,
		RedirectURL:  xConfig.redirectURL,
		Scopes:       xConfig.scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  xConfig.authURL,
			TokenURL: xConfig.tokenURL,
		},
	}
}

func GetConfig() *Config {
	return xConfig
}

func (c *Config) GetAuthUrl() (string, string, string) {
	state := utils.NewUUID32()
	// UUIDs use crypto/rand; concatenating two 32-byte hex forms yields a
	// high-entropy RFC 7636 verifier without relying on math/rand.
	codeVerifier := utils.NewUUID32() + utils.NewUUID32()
	digest := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(digest[:])
	authURL := c.oAuthConfig.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.AccessTypeOffline,
	)
	return state, codeVerifier, authURL
}

func (c *Config) GetToken(code, codeVerifier string) (*Token, error) {
	token, err := c.oAuthConfig.Exchange(
		context.Background(),
		code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, err
	}
	return &Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpireTime:   token.Expiry,
		ScopeVersion: ReadOnlyScopeVersion,
	}, nil
}
