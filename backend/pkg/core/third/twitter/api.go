package twitter

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sivchari/gotwtr"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/utils"
)

var twitterAPIBaseURL = "https://api.twitter.com"

const (
	twitterVerifyPageSize = 10
	twitterVerifyMaxPages = 1
)

// TaskNotCompletedError means the read-only X lookup succeeded, but the
// requested action was not present. Service code converts this into the
// localized task-specific "not detected yet" response.
type TaskNotCompletedError struct {
	Action string
}

func (e *TaskNotCompletedError) Error() string {
	return fmt.Sprintf("twitter %s action not detected", e.Action)
}

func (t *Token) Me() (*gotwtr.MeResponse, error) {
	client := gotwtr.New(t.AccessToken)
	params := &gotwtr.MeOption{
		UserFields: []gotwtr.UserField{
			gotwtr.UserFieldID,
			gotwtr.UserFieldProfileImageURL,
			gotwtr.UserFieldName,
			gotwtr.UserFieldUserName,
		},
	}
	me, err := client.Me(context.Background(), params)
	if err != nil {
		return nil, err
	}
	return me, nil
}

func (t *Token) GetUserID(username string) (string, error) {
	query := url.Values{}
	body, err := t.getReadOnly(
		fmt.Sprintf("/2/users/by/username/%s", url.PathEscape(strings.TrimPrefix(strings.TrimSpace(username), "@"))),
		query,
		"user-lookup",
	)
	if err != nil {
		return "", err
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil || strings.TrimSpace(out.Data.ID) == "" {
		return "", ret.CusError(i18n.ERR_001_00020)
	}
	return out.Data.ID, nil
}

func (t *Token) VerifyFollowing(ownerID, homepage string) error {
	username, err := utils.GetLastPathSegment(homepage)
	if err != nil {
		return err
	}
	targetUserID, err := t.GetUserID(username)
	if err != nil {
		return err
	}
	return t.findUserInPagedEndpoint(
		"follow",
		fmt.Sprintf("/2/users/%s/following", url.PathEscape(ownerID)),
		targetUserID,
	)
}

func (t *Token) VerifyLike(userID, tweetPage string) error {
	_, tweetID, err := ParseTweetURL(tweetPage)
	if err != nil {
		return err
	}
	return t.findTweetInPagedEndpoint(
		"like",
		fmt.Sprintf("/2/users/%s/liked_tweets", url.PathEscape(userID)),
		tweetID,
	)
}

func (t *Token) VerifyRetweet(ownerID, tweetPage string) error {
	_, tweetID, err := ParseTweetURL(tweetPage)
	if err != nil {
		return err
	}
	return t.findTweetReferenceInRecentTimeline(ownerID, tweetID, "retweeted", "retweet")
}

// Reply 验证用户是否已手动 reply 目标 tweet。
// 背景：Twitter API 对 App-token 提交的 POST /2/tweets 有 "engagement" 反 spam
// 判定 —— 用户与作者无 mention / follow / like / retweet / reply 关系时，
// 即使 tweet reply_settings=everyone，API 依然会 403 拒绝写入。
// 因此改用只读方案：拉用户最近 timeline，查找 referenced_tweets[type=replied_to].id
// 是否命中目标 tweetID；是否可调用仍取决于生产 X Project 的有效 API 套餐与 credits。
func (t *Token) VerifyReply(userTwitterID, tweetURL string) error {
	_, tweetID, err := ParseTweetURL(tweetURL)
	if err != nil {
		return err
	}
	return t.findTweetReferenceInRecentTimeline(userTwitterID, tweetID, "replied_to", "reply")
}

type twitterPagedUsersResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Meta twitterPaginationMeta `json:"meta"`
}

type twitterPagedTweetsResponse struct {
	Data []struct {
		ID               string `json:"id"`
		ReferencedTweets []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"referenced_tweets"`
	} `json:"data"`
	Meta twitterPaginationMeta `json:"meta"`
}

type twitterPaginationMeta struct {
	NextToken string `json:"next_token"`
}

func (t *Token) findUserInPagedEndpoint(action, endpoint, targetUserID string) error {
	return t.walkPagedEndpoint(action, endpoint, func(body []byte) (bool, string, error) {
		var out twitterPagedUsersResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return false, "", err
		}
		for _, user := range out.Data {
			if user.ID == targetUserID {
				return true, "", nil
			}
		}
		return false, out.Meta.NextToken, nil
	})
}

func (t *Token) findTweetInPagedEndpoint(action, endpoint, targetTweetID string) error {
	return t.walkPagedEndpoint(action, endpoint, func(body []byte) (bool, string, error) {
		var out twitterPagedTweetsResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return false, "", err
		}
		for _, tweet := range out.Data {
			if tweet.ID == targetTweetID {
				return true, "", nil
			}
		}
		return false, out.Meta.NextToken, nil
	})
}

func (t *Token) walkPagedEndpoint(
	action string,
	endpoint string,
	find func(body []byte) (found bool, nextToken string, err error),
) error {
	nextToken := ""
	for page := 0; page < twitterVerifyMaxPages; page++ {
		query := url.Values{}
		query.Set("max_results", fmt.Sprint(twitterVerifyPageSize))
		if nextToken != "" {
			query.Set("pagination_token", nextToken)
		}
		body, err := t.getReadOnly(endpoint, query, action)
		if err != nil {
			return err
		}
		found, next, err := find(body)
		if err != nil {
			return ret.CusError(i18n.ERR_001_00020)
		}
		if found {
			return nil
		}
		if next == "" {
			return &TaskNotCompletedError{Action: action}
		}
		nextToken = next
	}
	return &TaskNotCompletedError{Action: action}
}

func (t *Token) findTweetReferenceInRecentTimeline(userID, targetTweetID, referenceType, action string) error {
	if strings.TrimSpace(userID) == "" {
		return ret.CusError(i18n.ERR_001_00015)
	}
	query := url.Values{}
	query.Set("max_results", fmt.Sprint(twitterVerifyPageSize))
	query.Set("tweet.fields", "referenced_tweets")
	body, err := t.getReadOnly(
		fmt.Sprintf("/2/users/%s/tweets", url.PathEscape(userID)),
		query,
		action,
	)
	if err != nil {
		return err
	}
	var out twitterPagedTweetsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return ret.CusError(i18n.ERR_001_00020)
	}
	for _, tweet := range out.Data {
		for _, ref := range tweet.ReferencedTweets {
			if ref.Type == referenceType && ref.ID == targetTweetID {
				return nil
			}
		}
	}
	return &TaskNotCompletedError{Action: action}
}

func (t *Token) getReadOnly(endpoint string, query url.Values, action string) ([]byte, error) {
	apiURL := strings.TrimRight(twitterAPIBaseURL, "/") + endpoint
	if encoded := query.Encode(); encoded != "" {
		apiURL += "?" + encoded
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, ret.CusError(i18n.ERR_001_00020)
	}
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		warnTwitterf("twitter %s verify request failed err=%v", action, err)
		return nil, ret.CusError(i18n.ERR_001_00020)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ret.CusError(i18n.ERR_001_00014)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		warnTwitterf("twitter %s verify auth failed status=%d body=%s", action, resp.StatusCode, truncateBody(body))
		return nil, ret.CusError(i18n.ERR_001_00015)
	}
	if resp.StatusCode >= 400 {
		warnTwitterf("twitter %s verify failed status=%d body=%s", action, resp.StatusCode, truncateBody(body))
		return nil, ret.CusError(i18n.ERR_001_00020)
	}
	return body, nil
}

func warnTwitterf(format string, args ...any) {
	if logger := log.GetLogger(); logger != nil {
		logger.Warnf(format, args...)
	}
}

func truncateBody(body []byte) string {
	const maxLen = 512
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "...(truncated)"
}

func (t *Token) CheckExpireTime() (bool, error) {
	needRefreshTime := time.Now().Add(10 * time.Minute)
	if needRefreshTime.Unix() > t.ExpireTime.Unix() {
		_, err := t.Refresh()
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (t *Token) Refresh() (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", t.RefreshToken)

	req, _ := http.NewRequest("POST", xConfig.tokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Add Basic Authorization header with client credentials
	req.SetBasicAuth(xConfig.clientID, xConfig.clientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to refresh token: %s", string(body))
	}

	var newToken struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"` // 秒
	}
	err = json.Unmarshal(body, &newToken)
	if err != nil {
		return nil, err
	}
	t.AccessToken = newToken.AccessToken
	t.RefreshToken = newToken.RefreshToken
	t.ExpireTime = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)

	return t, nil
}

func ParseTweetURL(rawURL string) (username, tweetID string, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}

	// 去掉路径首尾的斜杠
	path := strings.Trim(u.Path, "/")
	segments := strings.Split(path, "/")

	// 期望格式: /{username}/status/{tweet_id}
	if len(segments) < 3 || segments[1] != "status" {
		return "", "", fmt.Errorf("unexpected url format: %s", u.Path)
	}

	username = segments[0]
	tweetID = segments[2]
	return username, tweetID, nil
}

// GetPinnedTweetID 用 App-only Bearer Token 查用户的置顶推文 ID
// 官方 endpoint: GET /2/users/by/username/:username?user.fields=pinned_tweet_id
// 返回 pinnedTweetID；用户没有置顶 tweet 时返回 "" nil (无 error)
func GetPinnedTweetID(bearerToken, username string) (string, error) {
	if bearerToken == "" {
		return "", fmt.Errorf("bearer token is empty")
	}
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return "", fmt.Errorf("username is empty")
	}

	apiURL := fmt.Sprintf("https://api.twitter.com/2/users/by/username/%s?user.fields=pinned_tweet_id", url.PathEscape(username))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitter api %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Data struct {
			ID            string `json:"id"`
			Username      string `json:"username"`
			PinnedTweetID string `json:"pinned_tweet_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return out.Data.PinnedTweetID, nil
}

func IsRateLimit(err error) bool {
	var httpErr *gotwtr.HTTPError
	ok := errors.As(err, &httpErr)
	if !ok {
		return false
	}
	if httpErr.Status == "429 Too Many Requests" {
		return true
	}
	return false
}
