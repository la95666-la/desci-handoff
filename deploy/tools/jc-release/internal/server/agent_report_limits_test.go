package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

func seedReportCommand(t *testing.T, api *API, agent shared.Agent) shared.Command {
	t.Helper()
	cmd := shared.Command{
		ID: "report-command", AgentID: agent.ID, Kind: shared.CmdLogs,
		Component: shared.CompFrontend, Status: shared.StatusPending,
		CreatedAt: time.Now().UTC(), IssuedBy: "test", LogLines: 10, LogSince: 60,
	}
	if err := api.Store.AddCommand(cmd); err != nil {
		t.Fatalf("AddCommand: %v", err)
	}
	if err := api.Store.UpdateCommand(cmd.ID, func(candidate *shared.Command) error {
		candidate.Status = shared.StatusInProgress
		started := time.Now().UTC()
		candidate.StartedAt = &started
		return nil
	}); err != nil {
		t.Fatalf("start command: %v", err)
	}
	cmd.Status = shared.StatusInProgress
	return cmd
}

func reportRequest(body []byte, agent shared.Agent) *http.Request {
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/agents/"+agent.ID+"/commands/report-command/report", bytes.NewReader(body))
	return req
}

type failOnReadBody struct{}

func (failOnReadBody) Read([]byte) (int, error) { panic("report body read before method rejection") }
func (failOnReadBody) Close() error             { return nil }

var _ io.ReadCloser = failOnReadBody{}

func assertReportUnchanged(t *testing.T, api *API, before shared.Command) {
	t.Helper()
	after, err := api.Store.GetCommand(before.ID)
	if err != nil {
		t.Fatalf("GetCommand: %v", err)
	}
	if after.Status != before.Status || after.Log != before.Log || after.Error != before.Error || after.FinishedAt != nil {
		t.Fatalf("oversized report mutated command: before=%#v after=%#v", before, after)
	}
	entries, err := api.Store.ListCommands(before.AgentID, 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected command state: len=%d err=%v", len(entries), err)
	}
	history, err := os.ReadDir(api.Store.Root + "/logs")
	if err != nil || len(history) != 0 {
		t.Fatalf("oversized report mutated history: entries=%d err=%v", len(history), err)
	}
}

func TestAgentReportRequiresPostAndExactRouteWithoutMutation(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			api, agent := newServiceLogTestAPI(t)
			before := seedReportCommand(t, api, agent)
			req := httptest.NewRequest(method,
				"/api/v1/agents/"+agent.ID+"/commands/"+before.ID+"/report", nil)
			req.Body = failOnReadBody{}
			req.Header.Set("Authorization", "Bearer "+agent.Token)
			rec := httptest.NewRecorder()
			api.dispatchAgentsSub(rec, req)
			if rec.Code != http.StatusMethodNotAllowed || responseCode(t, rec) != "METHOD_NOT_ALLOWED" {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			assertReportUnchanged(t, api, before)
		})
	}

	for _, suffix := range []string{"/", "/extra"} {
		t.Run("trailing_"+strings.TrimPrefix(suffix, "/"), func(t *testing.T) {
			api, agent := newServiceLogTestAPI(t)
			before := seedReportCommand(t, api, agent)
			payload, _ := json.Marshal(map[string]string{"status": shared.StatusSuccess, "log": "must-not-persist"})
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/agents/"+agent.ID+"/commands/"+before.ID+"/report"+suffix, bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+agent.Token)
			rec := httptest.NewRecorder()
			api.dispatchAgentsSub(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			assertReportUnchanged(t, api, before)
		})
	}
}

func TestAgentReportAcceptsSharedCappedLargeOutput(t *testing.T) {
	api, agent := newServiceLogTestAPI(t)
	before := seedReportCommand(t, api, agent)
	logValue, errorValue := shared.CapCommandReport(strings.Repeat("界", shared.MaxCommandLogBytes), strings.Repeat("错", shared.MaxCommandErrorBytes))
	payload, _ := json.Marshal(map[string]string{"status": shared.StatusFailed, "log": logValue, "error": errorValue})
	rec := httptest.NewRecorder()
	api.handleAgentReport(rec, reportRequest(payload, agent), agent)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after, err := api.Store.GetCommand(before.ID)
	if err != nil || after.Log != logValue || after.Error != errorValue || after.Status != shared.StatusFailed || after.FinishedAt == nil {
		t.Fatalf("persisted report mismatch: %#v err=%v", after, err)
	}
}

func TestAgentReportRejectsOversizedFieldsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		body map[string]string
		code string
	}{
		{"log", map[string]string{"status": shared.StatusSuccess, "log": strings.Repeat("l", shared.MaxCommandLogBytes+1)}, "REPORT_LOG_TOO_LARGE"},
		{"error", map[string]string{"status": shared.StatusFailed, "error": strings.Repeat("e", shared.MaxCommandErrorBytes+1)}, "REPORT_ERROR_TOO_LARGE"},
		{"status", map[string]string{"status": strings.Repeat("s", shared.MaxCommandStatusBytes+1)}, "REPORT_STATUS_TOO_LARGE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, agent := newServiceLogTestAPI(t)
			before := seedReportCommand(t, api, agent)
			payload, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			api.handleAgentReport(rec, reportRequest(payload, agent), agent)
			if rec.Code != http.StatusRequestEntityTooLarge || responseCode(t, rec) != tc.code {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			assertReportUnchanged(t, api, before)
		})
	}
}

func TestAgentReportBodyLimitRejectsBeforeMutation(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`{"status":"success","padding":"` + strings.Repeat("x", int(maxAgentReportBodyBytes)) + `"}`),
		[]byte(`{"status":"success"}` + strings.Repeat(" ", int(maxAgentReportBodyBytes))),
	} {
		api, agent := newServiceLogTestAPI(t)
		before := seedReportCommand(t, api, agent)
		rec := httptest.NewRecorder()
		api.handleAgentReport(rec, reportRequest(payload, agent), agent)
		if rec.Code != http.StatusRequestEntityTooLarge || responseCode(t, rec) != "REPORT_BODY_TOO_LARGE" {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertReportUnchanged(t, api, before)
	}
}

func TestAgentReportRejectsInvalidStateAndStatusWithoutMutation(t *testing.T) {
	for _, status := range []string{"", shared.StatusPending, shared.StatusInProgress, shared.StatusRolledBack, "unknown"} {
		t.Run("status_"+status, func(t *testing.T) {
			api, agent := newServiceLogTestAPI(t)
			before := seedReportCommand(t, api, agent)
			payload, _ := json.Marshal(map[string]string{"status": status})
			rec := httptest.NewRecorder()
			api.handleAgentReport(rec, reportRequest(payload, agent), agent)
			if rec.Code != http.StatusBadRequest || responseCode(t, rec) != "INVALID_REPORT_STATUS" {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			assertReportUnchanged(t, api, before)
		})
	}

	t.Run("replay", func(t *testing.T) {
		api, agent := newServiceLogTestAPI(t)
		before := seedReportCommand(t, api, agent)
		payload, _ := json.Marshal(map[string]string{"status": shared.StatusSuccess, "log": "first"})
		first := httptest.NewRecorder()
		api.handleAgentReport(first, reportRequest(payload, agent), agent)
		if first.Code != http.StatusOK {
			t.Fatalf("first report: %d %s", first.Code, first.Body.String())
		}
		replay := httptest.NewRecorder()
		api.handleAgentReport(replay, reportRequest(payload, agent), agent)
		if replay.Code != http.StatusConflict || responseCode(t, replay) != "COMMAND_NOT_IN_PROGRESS" {
			t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
		}
		after, _ := api.Store.GetCommand(before.ID)
		if after.Log != "first" || after.Status != shared.StatusSuccess {
			t.Fatalf("replay mutated command: %#v", after)
		}
	})

	t.Run("pending_command", func(t *testing.T) {
		api, agent := newServiceLogTestAPI(t)
		before := seedReportCommand(t, api, agent)
		if err := api.Store.UpdateCommand(before.ID, func(c *shared.Command) error { c.Status = shared.StatusPending; return nil }); err != nil {
			t.Fatal(err)
		}
		before.Status = shared.StatusPending
		payload, _ := json.Marshal(map[string]string{"status": shared.StatusSuccess})
		rec := httptest.NewRecorder()
		api.handleAgentReport(rec, reportRequest(payload, agent), agent)
		if rec.Code != http.StatusConflict || responseCode(t, rec) != "COMMAND_NOT_IN_PROGRESS" {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		assertReportUnchanged(t, api, before)
	})
}

func TestStorageFinalizeCommandRepeatsOwnershipStateAndStatusChecks(t *testing.T) {
	api, agent := newServiceLogTestAPI(t)
	before := seedReportCommand(t, api, agent)
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, agentID, status, want string
	}{
		{"wrong_owner", "different-agent", shared.StatusSuccess, "command_belongs_to_different_agent"},
		{"invalid_status", agent.ID, shared.StatusPending, "invalid_terminal_report_status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := api.Store.FinalizeCommand(before.ID, tc.agentID, tc.status, "", "", now)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("err=%v want=%s", err, tc.want)
			}
			assertReportUnchanged(t, api, before)
		})
	}
	if err := api.Store.UpdateCommand(before.ID, func(c *shared.Command) error { c.Status = shared.StatusPending; return nil }); err != nil {
		t.Fatal(err)
	}
	before.Status = shared.StatusPending
	if err := api.Store.FinalizeCommand(before.ID, agent.ID, shared.StatusSuccess, "", "", now); err == nil || err.Error() != "command_not_in_progress" {
		t.Fatalf("pending finalize err=%v", err)
	}
	assertReportUnchanged(t, api, before)
}

func TestStorageRejectsOversizedCommandMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
		set  func(*shared.Command)
	}{
		{"log", "command_log_too_large", func(command *shared.Command) { command.Log = strings.Repeat("x", shared.MaxCommandLogBytes+1) }},
		{"error", "command_error_too_large", func(command *shared.Command) { command.Error = strings.Repeat("x", shared.MaxCommandErrorBytes+1) }},
		{"status", "command_status_too_large", func(command *shared.Command) { command.Status = strings.Repeat("x", shared.MaxCommandStatusBytes+1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, agent := newServiceLogTestAPI(t)
			before := seedReportCommand(t, api, agent)
			err := api.Store.UpdateCommand(before.ID, func(command *shared.Command) error {
				tc.set(command)
				return nil
			})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("unexpected storage error: %v", err)
			}
			assertReportUnchanged(t, api, before)
		})
	}
}
