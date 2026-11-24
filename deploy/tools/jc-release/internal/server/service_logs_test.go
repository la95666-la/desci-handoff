package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

const testReadToken = "0123456789abcdef0123456789abcdef"

func newServiceLogTestAPI(t *testing.T) (*API, shared.Agent) {
	t.Helper()
	store, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	agent := shared.Agent{
		ID: "agent-1", Hostname: "test-host", Token: "0123456789abcdef0123456789abcdef",
		IP: "127.0.0.1", RegisteredAt: time.Now().UTC(), LastSeenAt: time.Now().UTC(),
		Deployed: map[shared.Component]string{},
	}
	if err := store.AddAgent(agent); err != nil {
		t.Fatalf("AddAgent: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &API{
		Store:  store,
		Cfg:    &Config{SessionSecret: "0123456789abcdef", ReadAPIToken: testReadToken},
		Logger: logger,
	}, agent
}

func TestServiceLogsReadTokenQueuesAndReadsQuery(t *testing.T) {
	api, agent := newServiceLogTestAPI(t)
	body := bytes.NewBufferString(`{"agent_id":"agent-1","target":"frontend","lines":123,"since_seconds":600}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/service-logs", body)
	req.Header.Set("Authorization", "Bearer "+testReadToken)
	rec := httptest.NewRecorder()
	api.requireServiceLogRead(api.handleServiceLogs)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Status bool            `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var command shared.Command
	if err := json.Unmarshal(envelope.Data, &command); err != nil {
		t.Fatalf("decode command: %v", err)
	}
	if command.AgentID != agent.ID || command.Kind != shared.CmdLogs || command.Component != "frontend" {
		t.Fatalf("unexpected command: %#v", command)
	}
	if command.LogLines != 123 || command.LogSince != 600 || command.IssuedBy != "read-api" {
		t.Fatalf("unexpected log options: %#v", command)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/service-logs/"+command.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+testReadToken)
	getRec := httptest.NewRecorder()
	api.requireServiceLogRead(api.handleServiceLogDetail)(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestServiceLogsRejectsUnauthorizedAndInvalidTarget(t *testing.T) {
	api, _ := newServiceLogTestAPI(t)
	unauthorized := httptest.NewRecorder()
	api.requireServiceLogRead(api.handleServiceLogs)(unauthorized,
		httptest.NewRequest(http.MethodGet, "/api/v1/service-logs", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	body := bytes.NewBufferString(`{"agent_id":"agent-1","target":"jc-backend;rm","lines":10,"since_seconds":60}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/service-logs", body)
	req.Header.Set("Authorization", "Bearer "+testReadToken)
	rec := httptest.NewRecorder()
	api.requireServiceLogRead(api.handleServiceLogs)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid target status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServiceLogsHTTPAgentRoundTrip(t *testing.T) {
	api, registeredAgent := newServiceLogTestAPI(t)
	srv := New(api.Cfg, api.Store, api.Logger)
	httpServer := httptest.NewServer(srv.http.Handler)
	defer httpServer.Close()

	requestBody := strings.NewReader(`{"agent_id":"agent-1","target":"admin","lines":80,"since_seconds":300}`)
	req, err := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/service-logs", requestBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testReadToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("queue request: %v", err)
	}
	defer resp.Body.Close()
	var queued struct {
		Status bool           `json:"status"`
		Data   shared.Command `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&queued); err != nil {
		t.Fatalf("decode queued response: %v", err)
	}
	if resp.StatusCode != http.StatusOK || !queued.Status {
		t.Fatalf("queue status=%d response=%#v", resp.StatusCode, queued)
	}

	pollReq, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/api/v1/agents/"+registeredAgent.ID+"/commands", nil)
	pollReq.Header.Set("Authorization", "Bearer "+registeredAgent.Token)
	pollResp, err := http.DefaultClient.Do(pollReq)
	if err != nil {
		t.Fatalf("agent poll: %v", err)
	}
	defer pollResp.Body.Close()
	var polled struct {
		Status bool             `json:"status"`
		Data   []shared.Command `json:"data"`
	}
	if err := json.NewDecoder(pollResp.Body).Decode(&polled); err != nil || len(polled.Data) != 1 || polled.Data[0].ID != queued.Data.ID {
		t.Fatalf("agent received %#v err=%v", polled, err)
	}
	reportBody := strings.NewReader(`{"status":"success","log":"verified-service-log\n","error":""}`)
	reportReq, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/agents/"+registeredAgent.ID+"/commands/"+queued.Data.ID+"/report", reportBody)
	reportReq.Header.Set("Authorization", "Bearer "+registeredAgent.Token)
	reportReq.Header.Set("Content-Type", "application/json")
	reportResp, err := http.DefaultClient.Do(reportReq)
	if err != nil {
		t.Fatalf("agent report: %v", err)
	}
	reportResp.Body.Close()
	if reportResp.StatusCode != http.StatusOK {
		t.Fatalf("agent report status=%d", reportResp.StatusCode)
	}

	resultReq, _ := http.NewRequest(http.MethodGet,
		httpServer.URL+"/api/v1/service-logs/"+queued.Data.ID, nil)
	resultReq.Header.Set("Authorization", "Bearer "+testReadToken)
	resultResp, err := http.DefaultClient.Do(resultReq)
	if err != nil {
		t.Fatalf("result request: %v", err)
	}
	defer resultResp.Body.Close()
	var result struct {
		Status bool           `json:"status"`
		Data   shared.Command `json:"data"`
	}
	if err := json.NewDecoder(resultResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.Status || result.Data.Status != shared.StatusSuccess ||
		result.Data.Log != "verified-service-log\n" {
		t.Fatalf("unexpected result %#v", result)
	}
}
