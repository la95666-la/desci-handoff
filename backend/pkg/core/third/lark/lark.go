package lark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

// Client 表示一个Lark多维表格客户端
// 每个Client实例对应一个特定的appToken和tableID
type Client struct {
	appID     string
	appSecret string
	appToken  string
	tableID   string
	client    *lark.Client
}

// Manager 管理多个Lark客户端实例
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client // key为appToken+tableID的组合
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// GetManager 返回全局的客户端管理器单例
func GetManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManager = &Manager{
			clients: make(map[string]*Client),
		}
	})
	return defaultManager
}

func newClient(appID, appSecret, appToken, tableID string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		appToken:  appToken,
		tableID:   tableID,
		client:    lark.NewClient(appID, appSecret),
	}
}

func (m *Manager) NewClient(appID, appSecret, appToken, tableID string) *Client {
	key := appToken + ":" + tableID

	// 先尝试读锁获取
	m.mu.RLock()
	if client, exists := m.clients[key]; exists {
		m.mu.RUnlock()
		return client
	}
	m.mu.RUnlock()

	// 需要创建新客户端，使用写锁
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查，防止并发创建
	if client, exists := m.clients[key]; exists {
		return client
	}

	client := newClient(appID, appSecret, appToken, tableID)
	m.clients[key] = client
	return client
}

// GetClient 从管理器获取已存在的客户端
func (m *Manager) GetClient(appToken, tableID string) (*Client, bool) {
	key := appToken + ":" + tableID
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, exists := m.clients[key]
	return client, exists
}

// RemoveClient 从管理器移除客户端
func (m *Manager) RemoveClient(appToken, tableID string) {
	key := appToken + ":" + tableID
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, key)
}

// ListClients 返回所有客户端的键列表
func (m *Manager) ListClients() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.clients))
	for key := range m.clients {
		keys = append(keys, key)
	}
	return keys
}

func (c *Client) CreateLine(line map[string]interface{}) (string, error) {

	req := larkbitable.NewCreateAppTableRecordReqBuilder().
		AppToken(c.appToken).
		TableId(c.tableID).
		UserIdType(`open_id`).
		AppTableRecord(larkbitable.NewAppTableRecordBuilder().Fields(line).Build()).
		Build()
	resp, err := c.client.Bitable.AppTableRecord.Create(context.Background(), req)
	// 处理错误
	if err != nil {
		return "", err
	}

	// 服务端错误处理
	if !resp.Success() {
		return "", fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}
	return *resp.Data.Record.RecordId, nil
}

func (c *Client) CreateLines(lines []map[string]interface{}) error {
	records := make([]*larkbitable.AppTableRecord, 0, len(lines))
	for _, line := range lines {
		records = append(records, larkbitable.NewAppTableRecordBuilder().Fields(line).Build())
	}
	req := larkbitable.NewBatchCreateAppTableRecordReqBuilder().
		AppToken(c.appToken).
		TableId(c.tableID).
		UserIdType(`open_id`).
		Body(larkbitable.NewBatchCreateAppTableRecordReqBodyBuilder().
			Records(records).
			Build()).
		Build()
	resp, err := c.client.Bitable.AppTableRecord.BatchCreate(context.Background(), req)
	// 处理错误
	if err != nil {
		return err
	}

	// 服务端错误处理
	if !resp.Success() {
		return fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}
	return nil
}

func (c *Client) GetTmpDownloadUrl(token []string) (map[string]string, error) {
	req := larkdrive.NewBatchGetTmpDownloadUrlMediaReqBuilder().
		FileTokens(token).
		Build()

	resp, err := c.client.Drive.Media.BatchGetTmpDownloadUrl(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}
	res := make(map[string]string, len(token))
	for _, info := range resp.Data.TmpDownloadUrls {
		res[*info.FileToken] = *info.TmpDownloadUrl
	}
	return res, nil
}

const maxTrustedImageBytes int64 = 10 << 20

// UploadTrustedImage uploads only a backend-generated URL on the configured COS host.
func (c *Client) UploadTrustedImage(image, trustedHost string) (string, error) {
	parsedURL, err := url.Parse(image)
	trustedURL, trustedErr := url.Parse(strings.TrimSpace(trustedHost))
	if err != nil || trustedErr != nil || parsedURL.Scheme != "https" || trustedURL.Hostname() == "" || !strings.EqualFold(parsedURL.Hostname(), trustedURL.Hostname()) || parsedURL.Port() != trustedURL.Port() || parsedURL.User != nil {
		return "", errors.New("Lark image source is not the trusted HTTPS COS host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, parsedURL.Hostname())
	if err != nil || len(addresses) == 0 {
		return "", errors.New("trusted COS host DNS resolution failed")
	}
	for _, address := range addresses {
		ip := address.IP
		if forbiddenEgressIP(ip) {
			return "", errors.New("trusted COS host resolved to a forbidden network address")
		}
	}
	pinnedIPs := append([]net.IPAddr(nil), addresses...)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		_, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return nil, splitErr
		}
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		var lastErr error
		for _, pinned := range pinnedIPs {
			conn, dialErr := dialer.DialContext(dialCtx, network, net.JoinHostPort(pinned.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
	httpClient := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("trusted image returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength < 0 || resp.ContentLength > maxTrustedImageBytes {
		return "", errors.New("trusted image content length is missing or exceeds 10 MiB")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" && contentType != "image/gif" {
		return "", errors.New("trusted image content type is not allowed")
	}

	filePath := parsedURL.Path
	fileName := path.Base(filePath)
	size := int(resp.ContentLength)
	limited := io.LimitReader(resp.Body, maxTrustedImageBytes+1)

	uploadReq := larkdrive.NewUploadAllMediaReqBuilder().
		Body(larkdrive.NewUploadAllMediaReqBodyBuilder().
			FileName(fileName).
			ParentType("bitable_image").
			ParentNode(c.appToken).
			Size(size).
			File(limited).
			Build()).
		Build()

	// 发起请求
	response, err := c.client.Drive.V1.Media.UploadAll(ctx, uploadReq)
	if err != nil {
		return "", err
	}
	return *response.Data.FileToken, nil
}

func forbiddenEgressIP(ip net.IP) bool {
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, raw := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		_, network, _ := net.ParseCIDR(raw)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// DeleteLine 删除一行记录
func (c *Client) DeleteLine(recordID string) error {
	req := larkbitable.NewDeleteAppTableRecordReqBuilder().
		AppToken(c.appToken).
		TableId(c.tableID).
		RecordId(recordID).
		Build()
	resp, err := c.client.Bitable.AppTableRecord.Delete(context.Background(), req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}
	return nil
}

// GetAppToken 返回客户端的appToken
func (c *Client) GetAppToken() string {
	return c.appToken
}

// GetTableID 返回客户端的tableID
func (c *Client) GetTableID() string {
	return c.tableID
}
