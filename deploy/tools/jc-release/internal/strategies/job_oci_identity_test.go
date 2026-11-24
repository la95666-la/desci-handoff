package strategies

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func jobOCIArchiveFixture(t *testing.T) (string, string, string) {
	return jobManifestArchiveFixture(t, "application/vnd.oci.image.manifest.v1+json")
}

func jobManifestArchiveFixture(t *testing.T, mediaType string) (string, string, string) {
	t.Helper()
	config := []byte(`{"architecture":"amd64","rootfs":{"type":"layers","diff_ids":["sha256:abc"]}}`)
	configID := digestJobBlob(config)
	manifest := []byte(`{"schemaVersion":2,"config":{"digest":"` + configID + `"},"layers":[{"digest":"sha256:abc"}]}`)
	manifestID := digestJobBlob(manifest)
	ref := "joincare/job:reviewed"
	dockerManifest, _ := json.Marshal([]map[string]any{{"Config": "blobs/sha256/" + configID[7:], "RepoTags": []string{ref}, "Layers": []string{"blobs/sha256/" + strings.Repeat("a", 64)}}})
	index, _ := json.Marshal(map[string]any{"manifests": []map[string]string{{"digest": manifestID, "mediaType": mediaType}}})
	path := filepath.Join(t.TempDir(), "job.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range map[string][]byte{
		"manifest.json": dockerManifest, "index.json": index,
		"blobs/sha256/" + configID[7:]:   config,
		"blobs/sha256/" + manifestID[7:]: manifest,
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path, configID, manifestID
}

func TestJobManifestMediaTypeKeepsExactDigestAndTagFences(t *testing.T) {
	for _, mediaType := range []string{"application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json", "application/json", "application/vnd.oci.image.index.v1+json"} {
		archive, configID, manifestID := jobManifestArchiveFixture(t, mediaType)
		got, err := jobOCIManifestID(archive, "joincare/job:reviewed", configID)
		if jobManifestMediaType(mediaType) {
			if err != nil || got != manifestID {
				t.Fatal(mediaType, got, err)
			}
		} else if err == nil {
			t.Fatal("unknown media admitted")
		}
		if _, err := jobOCIManifestID(archive, "joincare/job:other", configID); err == nil {
			t.Fatal("wrong tag admitted")
		}
	}
}

func TestRPCMigrationRealJobArchive(t *testing.T) {
	archive := os.Getenv("JC_RPC_JOB_ARCHIVE")
	if archive == "" {
		t.Skip("set reviewed migration archive")
	}
	got, err := jobOCIManifestID(archive, "joincare/job:release-88e1de2-light-wild-rpc", "sha256:de185f530edb4d58282ee0cd80988e922fdc02995b809843ce2112882506ba05")
	if err != nil || got != "sha256:e05db0d0944eefda70dac93f02164e723ac8a408ebe3947192837512fc4b4673" {
		t.Fatal(got, err)
	}
}

func TestJobOCIManifestIdentityRequiresMatchingConfigAndTag(t *testing.T) {
	archive, configID, manifestID := jobOCIArchiveFixture(t)
	if got, err := jobOCIManifestID(archive, "joincare/job:reviewed", configID); err != nil || got != manifestID {
		t.Fatalf("valid OCI identity: %q %v", got, err)
	}
	for _, testcase := range [][3]string{
		{"wrong tag", "joincare/job:other", configID},
		{"wrong config", "joincare/job:reviewed", "sha256:" + strings.Repeat("b", 64)},
	} {
		t.Run(testcase[0], func(t *testing.T) {
			if _, err := jobOCIManifestID(archive, testcase[1], testcase[2]); err == nil {
				t.Fatal("unbound identity accepted")
			}
		})
	}
}

// Optional real docker-save regression: set the archive path in a local test
// run. This also exercises OCI metadata order, layer sizes and gzip streaming.
func TestJobOCIIdentityRealArchive(t *testing.T) {
	archive := os.Getenv("JC_JOB_OCI_ARCHIVE")
	if archive == "" {
		t.Skip("set JC_JOB_OCI_ARCHIVE to a verified local docker-save gzip")
	}
	const configID = "sha256:b235e599b065205ac6390cafa432bcdc923e021f4fe0eef3245a00417cb8a0d5"
	const manifestID = "sha256:34160e4c3ec8fc81be7ccaa13d8f803675f661015dc4d200932b84218e99f594"
	got, err := jobOCIManifestID(archive, "joincare/job:ido-097e51e-6ba80c3", configID)
	if err != nil || got != manifestID {
		t.Fatalf("real archive OCI identity = %q, %v", got, err)
	}
}

func TestJobApplyUsesVerifiedOCIManifestIdentity(t *testing.T) {
	archive, configID, manifestID := jobOCIArchiveFixture(t)
	for _, testcase := range []struct {
		name, inspected string
		shouldRecreate  bool
	}{
		{"containerd manifest", manifestID, true},
		{"classic config", configID, true},
		{"unrelated digest", "sha256:" + strings.Repeat("f", 64), false},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			inspected, shouldRecreate := testcase.inspected, testcase.shouldRecreate
			root := jobFixture(t)
			s := &jobStrategy{root: root, firstDeploy: true}
			if err := s.SetJobImageID(configID); err != nil {
				t.Fatal(err)
			}
			var recreated bool
			s.command = func(_ context.Context, _ io.Writer, cmd string, args ...string) (string, error) {
				line := cmd + " " + strings.Join(args, " ")
				switch {
				case strings.Contains(line, "container ls"):
					return "", nil
				case strings.Contains(line, "image inspect"):
					return inspected + "\n", nil
				case strings.Contains(line, " config --format json"):
					return resolvedJobOverrideFixture(t, root, s.firstDeployToken, args), nil
				case strings.Contains(line, " up -d "):
					recreated = true
					return "", nil
				}
				t.Fatalf("unexpected command %s", line)
				return "", nil
			}
			s.shell = func(_ context.Context, _ io.Writer, _ string) (string, error) {
				return "Loaded image: joincare/job:reviewed\n", nil
			}
			var log bytes.Buffer
			err := s.Apply(context.Background(), archive, &log)
			if shouldRecreate && err != nil || !shouldRecreate && err == nil || recreated != shouldRecreate {
				t.Fatalf("apply err=%v recreated=%v", err, recreated)
			}
			if shouldRecreate && s.loadedID != inspected {
				t.Fatalf("runtime id=%q want %q", s.loadedID, inspected)
			}
		})
	}
}
