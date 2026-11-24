package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/activation"
	"github.com/joincare/jc-release/internal/shared"
	"github.com/joincare/jc-release/internal/strategies"
)

type flowStrategy struct {
	calls        []string
	failApply    bool
	onApply      func()
	failRollback bool
	onRollback   func(context.Context) error
}

func (*flowStrategy) Component() shared.Component { return shared.CompFrontend }
func (s *flowStrategy) Backup(_ context.Context, dir string, _ io.Writer) (string, error) {
	s.calls = append(s.calls, "backup")
	path := filepath.Join(dir, "old.tar.gz")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte("previous"), 0600)
}
func (s *flowStrategy) Apply(_ context.Context, src string, _ io.Writer) error {
	s.calls = append(s.calls, "apply")
	if s.onApply != nil {
		s.onApply()
	}
	if _, err := os.Stat(src); err != nil {
		return err
	}
	if s.failApply {
		return fmt.Errorf("simulated apply failure")
	}
	return nil
}
func (s *flowStrategy) Rollback(ctx context.Context, backup string, _ io.Writer) error {
	s.calls = append(s.calls, "rollback")
	if s.onRollback != nil {
		return s.onRollback(ctx)
	}
	if s.failRollback {
		return fmt.Errorf("simulated rollback failure")
	}
	_, err := os.Stat(backup)
	return err
}

func TestCanceledCommandStillAttemptsBoundedVerifiedRollback(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Update(func(st *State) error {
		st.JobPending = &JobIntent{ReleaseID: "canceled-job", Generation: 9}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	strategy := &flowStrategy{onRollback: func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, ok := ctx.Deadline(); !ok {
			return fmt.Errorf("unbounded recovery context")
		}
		return nil
	}}
	u := &Upgrader{Store: store}
	cmd := shared.Command{Component: shared.CompJob, ReleaseID: "canceled-job"}
	status, _, _ := u.rollbackOrFail(ctx, cmd, strategy, "job-absent", new(bytes.Buffer), slog.New(slog.NewTextHandler(io.Discard, nil)), "healthcheck", context.Canceled)
	state, err := store.Load()
	if err != nil || status != shared.StatusFailed || state.JobPending != nil {
		t.Fatalf("verified rollback after canceled command failed: %s %+v %v", status, state, err)
	}
	if err := store.Update(func(st *State) error {
		st.JobPending = &JobIntent{ReleaseID: "canceled-job", Generation: 9}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	strategy.onRollback = func(ctx context.Context) error { return fmt.Errorf("recovery unavailable") }
	status, _, _ = u.rollbackOrFail(ctx, cmd, strategy, "job-absent", new(bytes.Buffer), slog.New(slog.NewTextHandler(io.Discard, nil)), "healthcheck", context.Canceled)
	state, err = store.Load()
	if err != nil || status != shared.StatusFailed || state.JobPending == nil {
		t.Fatalf("unverified rollback cleared journal: %s %+v %v", status, state, err)
	}
}

func TestJobIntentSurvivesFailedRollbackAndClearsOnlyAfterVerifiedRecovery(t *testing.T) {
	for _, outcome := range []string{"success", "failed rollback", "verified rollback"} {
		t.Run(outcome, func(t *testing.T) {
			payload := []byte("job image archive")
			sum := sha256.Sum256(payload)
			rel := &shared.Release{ID: "job-1", Component: shared.CompJob, Version: "1", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload))}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }))
			defer srv.Close()
			client, err := NewTestClientAllowLoopbackHTTP(srv.URL, "token", srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			strategy := &flowStrategy{failApply: outcome != "success", failRollback: outcome == "failed rollback"}
			store := NewStore(dir)
			strategy.onApply = func() {
				state, err := store.Load()
				if err != nil || state.JobPending == nil || state.JobPending.ReleaseID != rel.ID || state.JobPending.Generation != 4 {
					t.Errorf("container mutation without durable job intent: %+v %v", state, err)
				}
			}
			u := &Upgrader{Client: client, Store: store, DataDir: dir}
			cmd := shared.Command{ID: "job-command", Kind: shared.CmdUpgrade, Component: shared.CompJob, ReleaseID: rel.ID}
			status, _, _ := u.doUpgrade(context.Background(), cmd, rel, strategy, new(bytes.Buffer), slog.New(slog.NewTextHandler(io.Discard, nil)), 4)
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if outcome == "success" {
				if status != shared.StatusSuccess || state.JobPending != nil || state.JobGeneration != 4 {
					t.Fatalf("success state: %+v %s", state, status)
				}
			} else if (state.JobPending != nil) != (outcome == "failed rollback") || state.JobGeneration != 0 || status != shared.StatusFailed {
				t.Fatalf("failure state: %+v %s", state, status)
			}
		})
	}
}

type signedJobStrategy struct {
	image    string
	backups  int
	onBackup func()
	backupOK bool
}

func (*signedJobStrategy) Component() shared.Component        { return shared.CompJob }
func (s *signedJobStrategy) SetJobImageID(image string) error { s.image = image; return nil }
func (s *signedJobStrategy) Backup(context.Context, string, io.Writer) (string, error) {
	s.backups++
	if s.onBackup != nil {
		s.onBackup()
	}
	if s.backupOK {
		return "job-absent", nil
	}
	return "", fmt.Errorf("stop before Docker")
}
func (*signedJobStrategy) Apply(context.Context, string, io.Writer) error { panic("unexpected apply") }
func (*signedJobStrategy) Rollback(context.Context, string, io.Writer) error {
	panic("unexpected rollback")
}
func (*signedJobStrategy) HealthCheck(context.Context) error { panic("unexpected health") }

func TestSignedJobAdmissionReachesBackupAndPendingRecoveryBlocksRetry(t *testing.T) {
	if runtime.GOOS == "linux" {
		if requireRootAgent() != nil {
			t.Skip("positive fixture needs root-owned production-style receipt tree")
		}
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"job", "common", "secret"} {
		if err := os.MkdirAll(filepath.Join(dir, "job-runtime", "config", part), 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"job-runtime/config/job/config.yaml":    "application:\n  environment: prod\n  jobMode: ido\nchain:\n  project:\n    chainID: 56\n    url: https://rpc.example.invalid\n    v1:\n      enabled: 'false'\n    v2:\n      contractAddress: 0x0000000000000000000000000000000000000002\n      coreOperatorAddress: TEST_CORE_ADDRESS\nfairLaunchMainnet:\n  scanner:\n    enabled: false\n    exclusiveJobMode: false\n",
		"job-runtime/config/common/config.yaml": "application: {}\n",
		"job-runtime/config/secret/config.yaml": "chain:\n  project:\n    jobPrivate: " + strings.Repeat("1", 64) + "\n",
		"config/secret/config.yaml":             "chain:\n  project:\n    private: " + strings.Repeat("2", 64) + "\n",
		"docker-compose.yml":                    "services:\n  job:\n    container_name: jc-job\n    image: joincare/job:reviewed\n    entrypoint: ['/app/joincare-job', '--path', '/app/config/']\n    volumes: ['./job-runtime/config:/app/config:ro']\n    healthcheck:\n      test: ['CMD', 'curl', '--fail', 'http://127.0.0.1:13007/health/job-ready']\n",
	}
	jobSigner, err := addressFromPlaintextKey(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	coreOperator, err := addressFromPlaintextKey(strings.Repeat("2", 64))
	if err != nil {
		t.Fatal(err)
	}
	files["job-runtime/config/job/config.yaml"] = strings.Replace(files["job-runtime/config/job/config.yaml"], "TEST_CORE_ADDRESS", coreOperator, 1)
	for path, body := range files {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	keys := map[string]string{}
	private := map[string]ed25519.PrivateKey{}
	for _, id := range []string{"one", "two", "three"} {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		keys[id] = hex.EncodeToString(pub)
		private[id] = priv
	}
	image := "sha256:" + strings.Repeat("a", 64)
	payload := []byte("archive")
	digest := sha256.Sum256(payload)
	claim := activation.ArtifactDeclaration{Domain: "joincare.release.artifact.v1", Environment: "mainnet", Component: "job", Generation: 1, ArtifactSHA256: hex.EncodeToString(digest[:]), ImageID: image, IssuedAt: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	body, err := json.Marshal(claim)
	if err != nil {
		t.Fatal(err)
	}
	signed := sha256.Sum256(append([]byte("joincare.release.artifact.v1\n"), body...))
	env := activation.ArtifactEnvelope{Declaration: body}
	for _, id := range []string{"one", "two"} {
		env.Signatures = append(env.Signatures, activation.ArtifactSignature{KeyID: id, Signature: hex.EncodeToString(ed25519.Sign(private[id], signed[:]))})
	}
	admission, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	rel := &shared.Release{ID: "job-1", Component: shared.CompJob, Version: "1", Size: int64(len(payload)), SHA256: hex.EncodeToString(digest[:]), JobAdmission: admission}
	readyKeys := map[string]string{}
	evidenceDir := t.TempDir()
	if err := os.Chmod(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	receiptDigests := map[string]string{}
	receiptBodies := map[string][]byte{}
	receiptUUID := "78dbed8b-77a9-11f1-ac1c-06eab0296b63"
	issued := time.Now().UTC().Add(-time.Minute)
	hostFile := filepath.Join(t.TempDir(), "ssh_host_ed25519_key.pub")
	if err := os.Chmod(filepath.Dir(hostFile), 0700); err != nil {
		t.Fatal(err)
	}
	hostKey := []byte("isolated-test-host-key-not-a-production-key")
	hostDigest := sha256.Sum256(hostKey)
	hostFingerprint := "SHA256:" + base64.RawStdEncoding.EncodeToString(hostDigest[:])
	if err := os.WriteFile(hostFile, []byte("ssh-ed25519 "+base64.StdEncoding.EncodeToString(hostKey)+" local-fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"schema", "backup", "restore", "signer", "chain-rpc", "permissions"} {
		receipt := activation.EvidenceReceipt{Domain: activation.EvidenceReceiptDomain, Kind: name, Result: "verified", Environment: "mainnet", AgentID: "app-agent", HostKeySHA256: hostFingerprint, ReleaseID: rel.ID, Generation: 1, DatabaseServerUUID: receiptUUID, MigrationSHA256: activation.SigningMetadataMigrationSHA256, ChainID: 56, EvidenceReferenceSHA256: strings.Repeat("4", 64), ObservedAt: issued}
		body, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		receiptBodies[name] = body
		if err := os.WriteFile(filepath.Join(evidenceDir, name+".receipt"), body, 0600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		receiptDigests[name] = hex.EncodeToString(digest[:])
	}
	readyPrivate := map[string]ed25519.PrivateKey{}
	for _, id := range []string{"four", "five", "six"} {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		readyKeys[id], readyPrivate[id] = hex.EncodeToString(pub), priv
	}
	ready := activation.Readiness{
		Domain: activation.ReadinessDomain, Environment: "mainnet", AgentID: "app-agent", Root: dir,
		HostKeySHA256: hostFingerprint, ReleaseSHA256: rel.SHA256, ReleaseID: rel.ID, Generation: 1,
		ArtifactImageID: image,
		MigrationSHA256: activation.SigningMetadataMigrationSHA256, DatabaseServerUUID: receiptUUID,
		SchemaEvidenceSHA256: receiptDigests["schema"], BackupReceiptSHA256: receiptDigests["backup"],
		RestoreEvidenceSHA256: receiptDigests["restore"], SignerEvidenceSHA256: receiptDigests["signer"],
		ChainRPCEvidenceSHA256: receiptDigests["chain-rpc"], PermissionsEvidenceSHA256: receiptDigests["permissions"],
		CoreOperatorAddress: coreOperator, JobSignerAddress: jobSigner, V2ContractAddress: "0x0000000000000000000000000000000000000002", ChainID: 56,
		IssuedAt: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	readyBody, err := json.Marshal(ready)
	if err != nil {
		t.Fatal(err)
	}
	readyDigest := sha256.Sum256(append([]byte(activation.ReadinessDomain+"\n"), readyBody...))
	readyEnvelope := activation.ArtifactEnvelope{Declaration: readyBody}
	for _, id := range []string{"four", "five"} {
		readyEnvelope.Signatures = append(readyEnvelope.Signatures, activation.ArtifactSignature{KeyID: id, Signature: hex.EncodeToString(ed25519.Sign(readyPrivate[id], readyDigest[:]))})
	}
	readyData, err := json.Marshal(readyEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	readyParent := t.TempDir()
	if err := os.Chmod(readyParent, 0700); err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(readyParent, "job-readiness.json")
	if err := os.WriteFile(readyPath, readyData, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	client, err := NewTestClientAllowLoopbackHTTP(server.URL, "token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	strat := &signedJobStrategy{}
	store := NewStore(dir)
	u := &Upgrader{Root: dir, DataDir: dir, Store: store, Client: client, JobEnvironment: "mainnet", JobTrustedKeys: keys, Registry: strategies.Registry{shared.CompJob: strat}, AgentID: "app-agent", JobReadinessFile: readyPath, JobEvidenceDir: evidenceDir, JobReadinessKeys: readyKeys, JobHostKeySHA256: ready.HostKeySHA256, jobHostKeyFile: hostFile}
	cmd := shared.Command{ID: "cmd", Kind: shared.CmdUpgrade, Component: shared.CompJob, ReleaseID: rel.ID}
	for _, test := range []struct {
		name      string
		breakGate func() func()
	}{
		{"missing readiness", func() func() { u.JobReadinessFile = ""; return func() { u.JobReadinessFile = readyPath } }},
		{"wrong host", func() func() {
			u.JobHostKeySHA256 = "SHA256:other"
			return func() { u.JobHostKeySHA256 = ready.HostKeySHA256 }
		}},
		{"wrong agent", func() func() { u.AgentID = "control-agent"; return func() { u.AgentID = "app-agent" } }},
		{"signed signer differs from local config", func() func() {
			path := filepath.Join(dir, "job-runtime", "config", "secret", "config.yaml")
			_ = os.WriteFile(path, []byte("chain:\n  project:\n    jobPrivate: "+strings.Repeat("3", 64)+"\n"), 0600)
			return func() { _ = os.WriteFile(path, []byte(files["job-runtime/config/secret/config.yaml"]), 0600) }
		}},
		{"configured operator differs from signed address", func() func() {
			path := filepath.Join(dir, "job-runtime", "config", "job", "config.yaml")
			_ = os.WriteFile(path, []byte(strings.Replace(files["job-runtime/config/job/config.yaml"], coreOperator, jobSigner, 1)), 0600)
			return func() { _ = os.WriteFile(path, []byte(files["job-runtime/config/job/config.yaml"]), 0600) }
		}},
		{"missing receipt", func() func() { u.JobEvidenceDir = t.TempDir(); return func() { u.JobEvidenceDir = evidenceDir } }},
		{"wrong receipt digest", func() func() {
			path := filepath.Join(evidenceDir, "schema.receipt")
			_ = os.WriteFile(path, []byte("tampered"), 0600)
			return func() { _ = os.WriteFile(path, receiptBodies["schema"], 0600) }
		}},
		{"artifact approver reused as evidence signer", func() func() {
			old := u.JobReadinessKeys["four"]
			u.JobReadinessKeys["four"] = keys["one"]
			return func() { u.JobReadinessKeys["four"] = old }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			restore := test.breakGate()
			defer restore()
			status, _, msg := u.ExecuteCommand(context.Background(), cmd, rel)
			if status != shared.StatusFailed || !strings.Contains(msg, "job_runtime_admission_unavailable") || strat.backups != 0 {
				t.Fatalf("not fail closed: %s %s backups=%d", status, msg, strat.backups)
			}
		})
	}
	status, _, msg := u.ExecuteCommand(context.Background(), cmd, rel)
	if status != shared.StatusFailed || !strings.Contains(msg, "backup:") || strat.backups != 1 || strat.image != image {
		t.Fatalf("signed admission did not reach backup: %s %s %+v", status, msg, strat)
	}
	// The first gate alone is insufficient: an assertion can expire or a
	// receipt change while a large archive is downloaded/backed up.
	strat.backupOK = true
	strat.onBackup = func() {
		if err := os.WriteFile(filepath.Join(evidenceDir, "schema.receipt"), []byte("changed after admission"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	status, _, msg = u.ExecuteCommand(context.Background(), cmd, rel)
	if status != shared.StatusFailed || !strings.Contains(msg, "job_readiness_changed_before_apply") {
		t.Fatalf("pre-apply drift not rejected: %s %s", status, msg)
	}
	beforePending, err := store.Load()
	if err != nil || beforePending.JobPending != nil {
		t.Fatalf("drift produced pending mutation: %+v %v", beforePending, err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "schema.receipt"), receiptBodies["schema"], 0600); err != nil {
		t.Fatal(err)
	}
	strat.onBackup = func() {
		path := filepath.Join(dir, "job-runtime", "config", "job", "config.yaml")
		if err := os.WriteFile(path, []byte(strings.Replace(files["job-runtime/config/job/config.yaml"], coreOperator, jobSigner, 1)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	status, _, msg = u.ExecuteCommand(context.Background(), cmd, rel)
	if status != shared.StatusFailed || !strings.Contains(msg, "job_readiness_changed_before_apply") {
		t.Fatalf("changed operator reached apply: %s %s", status, msg)
	}
	if state, err := store.Load(); err != nil || state.JobPending != nil {
		t.Fatalf("operator drift produced pending mutation: %+v %v", state, err)
	}
	strat.backupOK, strat.onBackup = false, nil
	if err := store.Update(func(st *State) error { st.JobPending = &JobIntent{ReleaseID: rel.ID, Generation: 1}; return nil }); err != nil {
		t.Fatal(err)
	}
	status, _, msg = u.ExecuteCommand(context.Background(), cmd, rel)
	if status != shared.StatusFailed || msg != "job_pending_recovery_required" || strat.backups != 3 {
		t.Fatalf("pending job mutated: %s %s %d", status, msg, strat.backups)
	}
}
func (s *flowStrategy) HealthCheck(context.Context) error {
	s.calls = append(s.calls, "health")
	return nil
}

func TestReleaseAgentDownloadBackupApplyAndFailureRollback(t *testing.T) {
	for _, failApply := range []bool{false, true} {
		t.Run(fmt.Sprint(failApply), func(t *testing.T) {
			payload := []byte("artifact")
			sum := sha256.Sum256(payload)
			rel := &shared.Release{ID: "historical-1", Component: shared.CompFrontend, Version: "v1", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Errorf("missing token")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/api/v1/artifacts/historical-1":
					w.Write(payload)
				default:
					t.Errorf("unexpected route %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			client, err := NewTestClientAllowLoopbackHTTP(srv.URL, "token", srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			strategy := &flowStrategy{failApply: failApply}
			u := &Upgrader{Client: client, Store: NewStore(root), Registry: strategies.Registry{shared.CompFrontend: strategy}, DataDir: root, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			status, _, msg := u.ExecuteCommand(context.Background(), shared.Command{ID: "cmd", Kind: shared.CmdUpgrade, Component: shared.CompFrontend, ReleaseID: rel.ID}, rel)
			if failApply {
				if status != shared.StatusFailed || !strings.Contains(msg, "rolled back") || strings.Join(strategy.calls, ",") != "backup,apply,rollback,health" {
					t.Fatalf("failure status=%s msg=%s calls=%v", status, msg, strategy.calls)
				}
			} else if status != shared.StatusSuccess || strings.Join(strategy.calls, ",") != "backup,apply,health" {
				t.Fatalf("success status=%s msg=%s calls=%v", status, msg, strategy.calls)
			}
		})
	}
}
