package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joincare/jc-release/internal/shared"
)

func assertAgentUnchanged(t *testing.T, api *API, before shared.Agent) {
	t.Helper()
	after, err := api.Store.GetAgent(before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.LastSeenAt != before.LastSeenAt || after.IP != before.IP || len(after.Deployed) != len(before.Deployed) {
		t.Fatalf("agent mutated: before=%#v after=%#v", before, after)
	}
}

func heartbeatRequest(method, path string, body []byte, agent shared.Agent) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+agent.Token)
	return req
}

func TestHeartbeatJSONBoundaryAndStorageValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"unknown", `{"deployed":{},"extra":true}`, http.StatusBadRequest},
		{"trailing", `{"deployed":{}} {}`, http.StatusBadRequest},
		{"unknown_component", `{"deployed":{"evil":"release-1"}}`, http.StatusBadRequest},
		{"invalid_release", `{"deployed":{"frontend":"../release"}}`, http.StatusBadRequest},
		{"oversized", `{"deployed":{"frontend":"` + strings.Repeat("x", int(maxHeartbeatBodyBytes)) + `"}}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api, agent := newServiceLogTestAPI(t)
			before, _ := api.Store.GetAgent(agent.ID)
			rec := httptest.NewRecorder()
			api.dispatchAgentsSub(rec, heartbeatRequest(http.MethodPost, "/api/v1/agents/"+agent.ID+"/heartbeat", []byte(tc.body), agent))
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			assertAgentUnchanged(t, api, before)
		})
	}

	api, agent := newServiceLogTestAPI(t)
	before, _ := api.Store.GetAgent(agent.ID)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		api.dispatchAgentsSub(rec, heartbeatRequest(method, "/api/v1/agents/"+agent.ID+"/heartbeat", nil, agent))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method %s status=%d", method, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	api.dispatchAgentsSub(rec, heartbeatRequest(http.MethodPost, "/api/v1/agents/"+agent.ID+"/heartbeat/extra", []byte(`{"deployed":{}}`), agent))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("trailing path status=%d", rec.Code)
	}
	assertAgentUnchanged(t, api, before)

	if err := api.Store.UpdateAgent(agent.ID, func(candidate *shared.Agent) error {
		candidate.Deployed = map[shared.Component]string{"evil": "release-1"}
		return nil
	}); err == nil {
		t.Fatal("storage accepted unknown heartbeat component")
	}
	assertAgentUnchanged(t, api, before)
}

func TestServiceLogRequestJSONBoundaryNoMutation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"unknown", `{"agent_id":"agent-1","target":"frontend","extra":true}`, http.StatusBadRequest},
		{"trailing", `{"agent_id":"agent-1","target":"frontend"} {}`, http.StatusBadRequest},
		{"oversized", `{"agent_id":"agent-1","target":"` + strings.Repeat("x", int(maxServiceLogBodyBytes)) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api, _ := newServiceLogTestAPI(t)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/service-logs", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+testReadToken)
			rec := httptest.NewRecorder()
			api.requireServiceLogRead(api.handleServiceLogs)(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			commands, err := api.Store.ListCommands("", 10)
			if err != nil || len(commands) != 0 {
				t.Fatalf("request mutated queue: %#v err=%v", commands, err)
			}
		})
	}
}

func TestHeartbeatValidKnownComponents(t *testing.T) {
	api, agent := newServiceLogTestAPI(t)
	body, _ := json.Marshal(map[string]any{"deployed": map[string]string{"frontend": "release-abc_1", "admin": "admin.2"}})
	rec := httptest.NewRecorder()
	api.dispatchAgentsSub(rec, heartbeatRequest(http.MethodPost, "/api/v1/agents/"+agent.ID+"/heartbeat", body, agent))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
