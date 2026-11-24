package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/joincare/jc-release/internal/shared"
)

func exactV3RecoveryState() *State {
	return &State{AgentID: "401afd6a-a6ec-4a61-8a57-860a51f8a691", JobGeneration: 39, JobPending: &JobIntent{ReleaseID: v3RecoveryTarget, Generation: 40}, Deployed: map[shared.Component]DeployedEntry{shared.CompJob: {ReleaseID: "2ea068a5-c8ea-4df3-a0be-247f72b73e8a"}}, JobRecovery: &JobRecoveryJournal{CommandID: "fc21cac3-3d2b-4099-949e-fda582b60375", Result: "attempting", PreviousIntent: JobIntent{ReleaseID: "d17301c7-7871-49ed-afe6-4f8afe04efdf", Generation: 40}}}
}
func TestV3RecoveryNeverAcceptsUnrelatedPendingState(t *testing.T) {
	r := &jobV3Recovery{}
	if !r.matchesState(exactV3RecoveryState()) {
		t.Fatal("exact intent rejected")
	}
	for _, mutate := range []func(*State){func(s *State) { s.AgentID = "other" }, func(s *State) { s.JobGeneration++ }, func(s *State) { s.JobPending.ReleaseID = "other" }, func(s *State) { s.JobPending.Generation++ }, func(s *State) { s.JobPending = nil }, func(s *State) { s.JobRecovery = &JobRecoveryJournal{} }, func(s *State) { delete(s.Deployed, shared.CompJob) }} {
		s := exactV3RecoveryState()
		mutate(s)
		if r.matchesState(s) {
			t.Fatal("unrelated recovery admitted")
		}
	}
}
func TestV3RecoveryConfigCASAndNoStateMutation(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	old := []byte("application:\n  jobMode: ido\n")
	next := []byte("application:\n  jobMode: ido-v3\n")
	os.WriteFile(p, old, 0600)
	r := &jobV3Recovery{configPath: p}
	if r.replaceConfig("wrong", next) == nil {
		t.Fatal("bad hash accepted")
	}
	actual, _ := os.ReadFile(p)
	if !bytes.Equal(actual, old) {
		t.Fatal("failed CAS modified config")
	}
	if e := r.replaceConfig(digestV3Recovery(old), next); e != nil {
		t.Fatal(e)
	}
	if e := r.replaceConfig(digestV3Recovery(next), old); e != nil {
		t.Fatal(e)
	}
	actual, _ = os.ReadFile(p)
	if !bytes.Equal(actual, old) {
		t.Fatal("restore failed")
	}
}
func TestV3RecoveryTransitionsIntentWithoutAdvancingGenerationEarly(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failed"}[fail], func(t *testing.T) {
			payload := []byte("reviewed job")
			sum := sha256.Sum256(payload)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
			defer srv.Close()
			client, e := NewTestClientAllowLoopbackHTTP(srv.URL, "token", srv.Client())
			if e != nil {
				t.Fatal(e)
			}
			dir := t.TempDir()
			store := NewStore(dir)
			before := exactV3RecoveryState()
			if e = store.Save(before); e != nil {
				t.Fatal(e)
			}
			recovery := &jobV3Recovery{commandID: "new-command", backupDir: dir}
			strategy := &flowStrategy{failApply: fail, failRollback: fail}
			strategy.onApply = func() {
				s, e := store.Load()
				if e != nil || s.JobGeneration != 39 || s.JobPending == nil || s.JobPending.ReleaseID != v3RecoveryTarget || s.JobPending.Generation != 40 || s.JobRecovery == nil || s.JobRecovery.PreviousIntent != *before.JobPending {
					t.Error("preapply durable intent lost", s, e)
				}
			}
			u := &Upgrader{Client: client, Store: store, DataDir: dir, jobRecovery: recovery}
			rel := &shared.Release{ID: v3RecoveryTarget, Component: shared.CompJob, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload))}
			status, _, _ := u.doUpgrade(context.Background(), shared.Command{ID: "new-command", ReleaseID: v3RecoveryTarget, Component: shared.CompJob}, rel, strategy, new(bytes.Buffer), slog.New(slog.NewTextHandler(io.Discard, nil)), 40)
			s, e := store.Load()
			if e != nil {
				t.Fatal(e)
			}
			if fail {
				if status != shared.StatusFailed || s.JobGeneration != 39 || s.JobPending == nil || s.JobRecovery == nil || s.JobRecovery.Result != "attempting" {
					t.Fatal("failed attempt hid pending", s)
				}
			} else if status != shared.StatusSuccess || s.JobGeneration != 40 || s.JobPending != nil || s.JobRecovery == nil || s.JobRecovery.Result != "success" {
				t.Fatal("success not recorded exactly", s)
			}
			if !fail && reflect.DeepEqual(s.Deployed, before.Deployed) {
				t.Fatal("successful target missing")
			}
		})
	}
}
func TestV3RecoveryRollbackRestoresConfigBeforeImageIO(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, []byte("unrelated"), 0600)
	strategy := &flowStrategy{onRollback: func(context.Context) error { t.Fatal("image IO before config validation"); return nil }}
	s := &jobV3RecoveryStrategy{Strategy: strategy, recovery: &jobV3Recovery{configPath: p}}
	if e := s.Rollback(context.Background(), "backup", io.Discard); e == nil {
		t.Fatal("unrelated config overwritten")
	}
}
