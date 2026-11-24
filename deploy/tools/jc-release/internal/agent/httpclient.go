package agent

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

// Client talks to jc-release-server on behalf of the agent.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	validated, err := validateServerURL(baseURL)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   5 * time.Minute,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("redirects are not permitted")
			},
		}
	}
	return &Client{baseURL: validated, token: token, http: httpClient}, nil
}

// NewClient creates a production client. The URL must be validated HTTPS and
// certificate verification cannot be disabled.
func NewClient(baseURL, token string) (*Client, error) {
	return newClient(baseURL, token, nil)
}

// SetToken updates the bearer token used for future calls.
func (c *Client) SetToken(t string) { c.token = t }

type wrapped struct {
	Status  bool            `json:"status"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (c *Client) do(method, path string, body interface{}, out interface{}) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server %d: %s", resp.StatusCode, string(data))
	}
	var w wrapped
	if err := json.Unmarshal(data, &w); err != nil {
		return fmt.Errorf("decode response: %w · body=%s", err, string(data))
	}
	if !w.Status {
		return fmt.Errorf("api %s: %s", w.Code, w.Message)
	}
	if out != nil && len(w.Data) > 0 {
		return json.Unmarshal(w.Data, out)
	}
	return nil
}

// RegisterResp is the payload of POST /register.
type RegisterResp struct {
	AgentID    string `json:"agent_id"`
	AgentToken string `json:"agent_token"`
}

// Register performs the one-shot enroll → permanent token exchange.
func (c *Client) Register(enrollToken, hostname string) (*RegisterResp, error) {
	// no bearer token yet
	c.token = ""
	var out RegisterResp
	err := c.do("POST", "/api/v1/register", map[string]string{
		"enroll_token": enrollToken,
		"hostname":     hostname,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Heartbeat reports current deployed versions to server.
func (c *Client) Heartbeat(agentID string, deployed map[shared.Component]string) error {
	return c.do("POST", "/api/v1/agents/"+agentID+"/heartbeat",
		map[string]interface{}{"deployed": deployed}, nil)
}

// GetManifest fetches the latest release per component.
func (c *Client) GetManifest(agentID string) (*shared.Manifest, error) {
	var m shared.Manifest
	if err := c.do("GET", "/api/v1/agents/"+agentID+"/manifest", nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

var releaseIDForURL = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,63}$`)

// GetRelease resolves an exact historical release from the agent-authenticated
// endpoint. A latest-per-component manifest alone cannot represent an older
// release selected by the operator.
func (c *Client) GetRelease(agentID, releaseID string) (*shared.Release, error) {
	if !releaseIDForURL.MatchString(releaseID) {
		return nil, fmt.Errorf("invalid release ID")
	}
	var rel shared.Release
	if err := c.do(http.MethodGet, "/api/v1/agents/"+agentID+"/releases/"+releaseID, nil, &rel); err != nil {
		return nil, err
	}
	if rel.ID != releaseID {
		return nil, fmt.Errorf("release identity mismatch")
	}
	return &rel, nil
}

// PollCommands returns pending commands, atomically transitioning them to in_progress on the server.
func (c *Client) PollCommands(agentID string) ([]shared.Command, error) {
	var cmds []shared.Command
	if err := c.do("GET", "/api/v1/agents/"+agentID+"/commands", nil, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

// ReportCommand finalizes a command with status + log + error.
func (c *Client) ReportCommand(agentID, cmdID, status, log, errMsg string) error {
	if !shared.ValidTerminalReportStatus(status) {
		return fmt.Errorf("invalid terminal report status %q", status)
	}
	log, errMsg = shared.CapCommandReport(log, errMsg)
	return c.do("POST", "/api/v1/agents/"+agentID+"/commands/"+cmdID+"/report",
		map[string]string{"status": status, "log": log, "error": errMsg}, nil)
}

// DownloadArtifact streams one component artifact to a same-directory
// temporary file, verifies its exact digest, and renames it atomically.
func (c *Client) DownloadArtifact(releaseID, expectedSHA256, dstPath string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/artifacts/"+releaseID, nil)
	if err != nil {
		return 0, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("download %d: %s", resp.StatusCode, string(body))
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
		return 0, err
	}
	tmp := dstPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		_ = os.Remove(tmp)
		f, err = os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return 0, err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return 0, copyErr
		}
		return 0, closeErr
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expectedSHA256 {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("sha256 mismatch: got=%s expected=%s", got, expectedSHA256)
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}
