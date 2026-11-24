package chaintx

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type HTTPGateway struct {
	BaseURL       string
	Client        *http.Client
	Authorization string
}

func (g HTTPGateway) Commit(ctx context.Context, request GatewayCommitRequest) (GatewayJournalEntry, error) {
	return g.call(ctx, http.MethodPost, "/v1/signing/commit", request)
}
func (g HTTPGateway) Journal(ctx context.Context, key GatewayJournalKey) (GatewayJournalEntry, error) {
	return g.call(ctx, http.MethodPost, "/v1/signing/journal", key)
}
func (g HTTPGateway) call(ctx context.Context, method, path string, payload any) (GatewayJournalEntry, error) {
	base, err := url.Parse(strings.TrimSpace(g.BaseURL))
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return GatewayJournalEntry{}, errors.New("gateway requires an absolute HTTPS URL")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return GatewayJournalEntry{}, err
	}
	endpoint := base.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return GatewayJournalEntry{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if g.Authorization != "" {
		req.Header.Set("Authorization", g.Authorization)
	}
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return GatewayJournalEntry{}, err
	}
	defer res.Body.Close()
	limited := io.LimitReader(res.Body, 2<<20)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(limited)
		return GatewayJournalEntry{}, errors.New("gateway rejected request: " + strings.TrimSpace(string(data)))
	}
	data, err := io.ReadAll(limited)
	if err != nil {
		return GatewayJournalEntry{}, err
	}
	var out GatewayJournalEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return GatewayJournalEntry{}, err
	}
	return out, nil
}
