package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

func commandState(t *testing.T, api *API) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(api.Store.Root, commandsFile))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestServiceLogDetailRequiresExactSingleSegmentBeforeLookup(t *testing.T) {
	api, _ := newServiceLogTestAPI(t)
	before := commandState(t, api)
	for _, path := range []string{"/api/v1/service-logs/", "/api/v1/service-logs/id/", "/api/v1/service-logs/id/extra"} {
		rec := httptest.NewRecorder()
		api.handleServiceLogDetail(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		if after := commandState(t, api); string(before) != string(after) {
			t.Fatalf("%s mutated command state", path)
		}
	}
}

func TestStorageRejectsInvalidServiceLogCommandsWithoutMutation(t *testing.T) {
	api, agent := newServiceLogTestAPI(t)
	base := shared.Command{ID: "log-1", AgentID: agent.ID, Kind: shared.CmdLogs, Component: shared.CompFrontend,
		Status: shared.StatusPending, CreatedAt: time.Now(), IssuedBy: "test", LogLines: 10, LogSince: 60}
	cases := []func(*shared.Command){
		func(c *shared.Command) { c.Kind = shared.CmdRestart },
		func(c *shared.Command) { c.Component = shared.CompBackend },
		func(c *shared.Command) { c.Component = "../../root" },
		func(c *shared.Command) { c.LogLines = 0 },
		func(c *shared.Command) { c.LogLines = shared.MaxServiceLogLines + 1 },
		func(c *shared.Command) { c.LogSince = 0 },
		func(c *shared.Command) { c.LogSince = shared.MaxServiceLogSince + 1 },
		func(c *shared.Command) { c.ReleaseID = "release-1" },
		func(c *shared.Command) { c.Status = shared.StatusInProgress },
	}
	for i, mutate := range cases {
		cmd := base
		cmd.ID = "invalid-" + string(rune('a'+i))
		mutate(&cmd)
		before := commandState(t, api)
		if err := api.Store.AddCommand(cmd); err == nil {
			t.Fatalf("case %d accepted: %#v", i, cmd)
		}
		if after := commandState(t, api); string(before) != string(after) {
			t.Fatalf("case %d mutated command state", i)
		}
	}
	var data commandsData
	if err := json.Unmarshal(commandState(t, api), &data); err != nil || len(data.Commands) != 0 {
		t.Fatal("invalid command persisted")
	}
}

func TestServiceLogAPIUsesSharedValidator(t *testing.T) {
	for _, body := range []string{
		`{"agent_id":"agent-1","target":"backend","lines":10,"since_seconds":60}`,
		`{"agent_id":"agent-1","target":"frontend","lines":2001,"since_seconds":60}`,
		`{"agent_id":"agent-1","target":"frontend","lines":10,"since_seconds":604801}`,
	} {
		api, _ := newServiceLogTestAPI(t)
		before := commandState(t, api)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/service-logs", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testReadToken)
		rec := httptest.NewRecorder()
		api.requireServiceLogRead(api.handleServiceLogs)(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if after := commandState(t, api); string(before) != string(after) {
			t.Fatal("invalid API command mutated state")
		}
	}
}
