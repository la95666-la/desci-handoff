package twitter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
)

func withTwitterAPIServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	oldBaseURL := twitterAPIBaseURL
	twitterAPIBaseURL = server.URL
	t.Cleanup(func() {
		twitterAPIBaseURL = oldBaseURL
		server.Close()
	})
}

func customErrorCode(err error) string {
	var custom *ret.CustomizeError
	if errors.As(err, &custom) {
		code, _ := custom.Info()
		return code
	}
	return ""
}

func TestVerifyLikeUsesReadOnlyGetAndFindsTweet(t *testing.T) {
	withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/2/users/user-1/liked_tweets" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("max_results"); got != "10" {
			t.Fatalf("max_results = %q, want 10", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"tweet-2"},{"id":"tweet-1"}],"meta":{}}`))
	})

	err := (&Token{AccessToken: "token"}).VerifyLike("user-1", "https://x.com/joincare/status/tweet-1")
	if err != nil {
		t.Fatalf("VerifyLike() error = %v", err)
	}
}

func TestVerifyFollowingReadsOnlyOnePage(t *testing.T) {
	requests := 0
	withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/2/users/user-1/following" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("max_results"); got != "10" {
			t.Fatalf("max_results = %q, want 10", got)
		}
		if token := r.URL.Query().Get("pagination_token"); token != "" {
			t.Fatalf("pagination_token = %q, want empty", token)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"other"}],"meta":{"next_token":"next-1"}}`))
	})

	err := (&Token{AccessToken: "token"}).findUserInPagedEndpoint("follow", "/2/users/user-1/following", "target")
	var notCompleted *TaskNotCompletedError
	if !errors.As(err, &notCompleted) || notCompleted.Action != "follow" {
		t.Fatalf("error = %v, want follow TaskNotCompletedError", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestVerifyFollowingUsesReadOnlyLookupAndFollowingEndpoints(t *testing.T) {
	var methods []string
	withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.URL.Path {
		case "/2/users/by/username/joincare":
			_, _ = w.Write([]byte(`{"data":{"id":"target"}}`))
		case "/2/users/user-1/following":
			_, _ = w.Write([]byte(`{"data":[{"id":"target"}],"meta":{}}`))
		default:
			http.NotFound(w, r)
		}
	})
	if err := (&Token{AccessToken: "token"}).VerifyFollowing("user-1", "https://x.com/joincare"); err != nil {
		t.Fatalf("VerifyFollowing() error = %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("methods = %v, want two GET requests", methods)
	}
	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("observed write method %s", method)
		}
	}
}

func TestVerifyRetweetAndReplyReadTimelineReferences(t *testing.T) {
	tests := []struct {
		name          string
		referenceType string
		verify        func(*Token) error
	}{
		{
			name:          "retweet",
			referenceType: "retweeted",
			verify: func(token *Token) error {
				return token.VerifyRetweet("user-1", "https://x.com/joincare/status/tweet-1")
			},
		},
		{
			name:          "reply",
			referenceType: "replied_to",
			verify: func(token *Token) error {
				return token.VerifyReply("user-1", "https://x.com/joincare/status/tweet-1")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/2/users/user-1/tweets" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				if r.URL.Query().Get("tweet.fields") != "referenced_tweets" {
					t.Fatalf("missing referenced_tweets query")
				}
				if got := r.URL.Query().Get("max_results"); got != "10" {
					t.Fatalf("max_results = %q, want 10", got)
				}
				_, _ = w.Write([]byte(`{"data":[{"id":"activity","referenced_tweets":[{"type":"` + test.referenceType + `","id":"tweet-1"}]}],"meta":{}}`))
			})
			if err := test.verify(&Token{AccessToken: "token"}); err != nil {
				t.Fatalf("verify error = %v", err)
			}
		})
	}
}

func TestReadOnlyVerificationNotFound(t *testing.T) {
	withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	})

	err := (&Token{AccessToken: "token"}).VerifyLike("user-1", "https://x.com/joincare/status/tweet-1")
	var notCompleted *TaskNotCompletedError
	if !errors.As(err, &notCompleted) || notCompleted.Action != "like" {
		t.Fatalf("error = %v, want like TaskNotCompletedError", err)
	}
}

func TestReadOnlyVerificationMapsXFailures(t *testing.T) {
	tests := []struct {
		status int
		code   string
	}{
		{status: http.StatusUnauthorized, code: i18n.ERR_001_00015},
		{status: http.StatusForbidden, code: i18n.ERR_001_00015},
		{status: http.StatusPaymentRequired, code: i18n.ERR_001_00020},
		{status: http.StatusTooManyRequests, code: i18n.ERR_001_00014},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(`{"detail":"test failure"}`))
			})
			err := (&Token{AccessToken: "token"}).VerifyLike("user-1", "https://x.com/joincare/status/tweet-1")
			if got := customErrorCode(err); got != test.code {
				t.Fatalf("code = %q, want %q; err=%v", got, test.code, err)
			}
		})
	}
}

func TestReadOnlyVerificationNeverBuildsWriteRequests(t *testing.T) {
	var methods []string
	withTwitterAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	})
	token := &Token{AccessToken: "token"}
	_ = token.VerifyLike("user-1", "https://x.com/joincare/status/tweet-1")
	_ = token.VerifyRetweet("user-1", "https://x.com/joincare/status/tweet-1")
	_ = token.VerifyReply("user-1", "https://x.com/joincare/status/tweet-1")
	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("observed write method %s", method)
		}
	}
}

func TestOAuthScopesAreReadOnly(t *testing.T) {
	scopes := strings.Join(xConfig.scopes, " ")
	for _, forbidden := range []string{"follows.write", "like.write", "tweet.write"} {
		if strings.Contains(scopes, forbidden) {
			t.Fatalf("forbidden scope %q present in %q", forbidden, scopes)
		}
	}
	for _, required := range []string{"tweet.read", "users.read", "follows.read", "like.read", "offline.access"} {
		if !strings.Contains(scopes, required) {
			t.Fatalf("required scope %q missing from %q", required, scopes)
		}
	}
}

func TestLegacyTokensRequireReadOnlyReauthorization(t *testing.T) {
	if (&Token{AccessToken: "legacy"}).SupportsReadOnlyVerification() {
		t.Fatal("legacy token without scope version must require reauthorization")
	}
	if !(&Token{AccessToken: "read-only", ScopeVersion: ReadOnlyScopeVersion}).SupportsReadOnlyVerification() {
		t.Fatal("read-only token must be accepted")
	}
}

func TestOAuthAuthorizationURLContainsOnlyReadScopes(t *testing.T) {
	NewConfig("client", "secret", "https://example.com/callback")
	_, _, rawURL := GetConfig().GetAuthUrl()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	scope := parsed.Query().Get("scope")
	if strings.Contains(scope, ".write") {
		t.Fatalf("authorization URL contains write scope: %q", scope)
	}
}
