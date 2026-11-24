package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joincare/jc-release/internal/shared"
	"github.com/joincare/jc-release/internal/strategies"
)

func TestJobDescriptorBindsChecksumImageAndGeneration(t *testing.T) {
	image := "sha256:" + strings.Repeat("a", 64)
	checksum := strings.Repeat("b", 64)
	s := &signedJobStrategy{}
	u := &Upgrader{JobEnvironment: "prod", Registry: strategies.Registry{shared.CompJob: s}}
	rel := &shared.Release{Component: shared.CompJob, SHA256: checksum}
	base := jobDescriptor{"prod", "job", 1, checksum, image}
	encode := func(d jobDescriptor) { rel.JobAdmission, _ = json.Marshal(d) }
	encode(base)
	if generation, err := u.admitJobDescriptor(rel, 1); err != nil || generation != 1 || s.image != image {
		t.Fatalf("valid descriptor: generation=%d image=%q err=%v", generation, s.image, err)
	}
	base.Generation = 0 // server-derived descriptor leaves generation to Agent
	encode(base)
	if generation, err := u.admitJobDescriptor(rel, 5); err != nil || generation != 5 {
		t.Fatalf("auto-generation: %d %v", generation, err)
	}
	base.Generation = 1
	for name, change := range map[string]func(*jobDescriptor){
		"environment": func(d *jobDescriptor) { d.Environment = "test" },
		"component":   func(d *jobDescriptor) { d.Component = "backend" },
		"generation":  func(d *jobDescriptor) { d.Generation = 2 },
		"checksum":    func(d *jobDescriptor) { d.ArtifactSHA256 = strings.Repeat("c", 64) },
		"image":       func(d *jobDescriptor) { d.ImageID = "joincare/job:latest" },
	} {
		t.Run(name, func(t *testing.T) {
			d := base
			change(&d)
			encode(d)
			s.image = ""
			if _, err := u.admitJobDescriptor(rel, 1); err == nil || s.image != "" {
				t.Fatalf("invalid descriptor reached strategy: %v %q", err, s.image)
			}
		})
	}
	rel.JobAdmission = []byte(`{"environment":"prod","component":"job","generation":1,"artifact_sha256":"` + checksum + `","image_id":"` + image + `","extra":true}`)
	if _, err := u.admitJobDescriptor(rel, 1); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestUnsignedDescriptorStillRequiresIsolatedLayoutAndSigner(t *testing.T) {
	root := t.TempDir()
	for _, part := range []string{"job", "common", "secret"} {
		if err := os.MkdirAll(filepath.Join(root, "job-runtime", "config", part), 0700); err != nil {
			t.Fatal(err)
		}
	}
	jobSigner, err := addressFromPlaintextKey(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	coreOperator, err := addressFromPlaintextKey(strings.Repeat("2", 64))
	if err != nil {
		t.Fatal(err)
	}
	_ = jobSigner
	files := map[string]string{
		"job-runtime/config/job/config.yaml":    "application:\n  environment: prod\n  jobMode: ido\nchain:\n  project:\n    chainID: 56\n    url: https://rpc.example.invalid\n    v1:\n      enabled: 'false'\n    v2:\n      contractAddress: 0x0000000000000000000000000000000000000002\n      coreOperatorAddress: " + coreOperator + "\nfairLaunchMainnet:\n  scanner:\n    enabled: false\n    exclusiveJobMode: false\n",
		"job-runtime/config/common/config.yaml": "application: {}\n",
		"job-runtime/config/secret/config.yaml": "chain:\n  project:\n    jobPrivate: " + strings.Repeat("1", 64) + "\n",
		"docker-compose.yml":                    "services:\n  job:\n    container_name: jc-job\n    image: joincare/job:reviewed\n    entrypoint: ['/app/joincare-job', '--path', '/app/config/']\n    volumes: ['./job-runtime/config:/app/config:ro']\n    healthcheck:\n      test: ['CMD', 'curl', '--fail', 'http://127.0.0.1:13007/health/job-ready']\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := &signedJobStrategy{}
	u := &Upgrader{Root: root, DataDir: root, Store: NewStore(root), JobEnvironment: "prod", Registry: strategies.Registry{shared.CompJob: s}}
	rel := &shared.Release{Component: shared.CompJob, SHA256: strings.Repeat("b", 64)}
	rel.JobAdmission, _ = json.Marshal(jobDescriptor{"prod", "job", 1, rel.SHA256, "sha256:" + strings.Repeat("a", 64)})
	if generation, err := u.admitJob(rel); err != nil || generation != 1 {
		t.Fatalf("admit: %d %v", generation, err)
	}
	if err := os.WriteFile(filepath.Join(root, "job-runtime/config/secret/config.yaml"), []byte(files["job-runtime/config/secret/config.yaml"]+"    private: "+strings.Repeat("2", 64)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := u.admitJob(rel); err == nil {
		t.Fatal("Core private key in Job secret accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "job-runtime/config/secret/config.yaml"), []byte(files["job-runtime/config/secret/config.yaml"]), 0600); err != nil {
		t.Fatal(err)
	}
	if err := u.Store.Update(func(st *State) error { st.JobPending = &JobIntent{ReleaseID: "previous", Generation: 1}; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := u.admitJob(rel); err == nil {
		t.Fatal("pending recovery accepted")
	}
}
