package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

func TestCommandSummaryOmitsLogWithoutMutatingStoredDetail(t *testing.T) {
	api := newPublicAPI(t)
	cmd := shared.Command{ID: "summary-fixture", AgentID: "agent", Kind: shared.CmdLogs, LogLines: 200, LogSince: 3600, IssuedBy: "test",
		Component: shared.CompFrontend, Status: shared.StatusPending, CreatedAt: time.Now()}
	if err := api.Store.AddCommand(cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Log = strings.Repeat("private-log-body", 10000)
	if err := api.Store.UpdateCommand(cmd.ID, func(c *shared.Command) error { c.Log = cmd.Log; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, summary := range []bool{true, false} {
		path := "/api/v1/commands?limit=100"
		if summary {
			path += "&summary=true"
		}
		rec := httptest.NewRecorder()
		api.handleListCommands(rec, httptest.NewRequest("GET", path, nil))
		var result struct {
			Data []shared.Command `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Data) != 1 {
			t.Fatal("missing command")
		}
		if summary && (result.Data[0].Log != "" || rec.Body.Len() > 2048) {
			t.Fatal("summary leaked full log")
		}
		if !summary && result.Data[0].Log != cmd.Log {
			t.Fatal("legacy API changed")
		}
	}
	stored, err := api.Store.GetCommand(cmd.ID)
	if err != nil || stored.Log != cmd.Log {
		t.Fatal("summary mutated stored history")
	}
}
