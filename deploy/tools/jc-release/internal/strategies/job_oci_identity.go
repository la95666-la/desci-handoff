package strategies

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// jobOCIManifestID binds containerd's manifest-based image ID to the config
// digest already admitted by the server. The archive itself was SHA256-checked
// by the Agent before Apply; this function also hashes the referenced blobs.
func jobOCIManifestID(archive, loadedRef, configID string) (string, error) {
	invalid := errors.New("job_oci_identity_invalid")
	if !validImageID(configID) || !strings.HasPrefix(loadedRef, "joincare/job:") {
		return "", invalid
	}
	f, err := os.Open(archive)
	if err != nil {
		return "", invalid
	}
	defer f.Close()
	var stream io.Reader = f
	header := make([]byte, 2)
	if _, err := f.ReadAt(header, 0); err != nil {
		return "", invalid
	}
	if header[0] == 0x1f && header[1] == 0x8b {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", invalid
		}
		defer gz.Close()
		stream = gz
	}
	tr := tar.NewReader(stream)
	var index, dockerManifest []byte
	blobs := make(map[string][]byte)
	var metadataBytes int64
	complete := false
	for count := 0; count < 20000; count++ {
		h, err := tr.Next()
		if err == io.EOF {
			complete = true
			break
		}
		if err == nil && h != nil && h.Typeflag == tar.TypeDir && path.Clean(strings.TrimSuffix(h.Name, "/")) == strings.TrimSuffix(h.Name, "/") && !strings.HasPrefix(h.Name, "/") {
			continue
		}
		if err != nil || h == nil || h.Name == "" || path.Clean(h.Name) != h.Name || strings.HasPrefix(h.Name, "/") || strings.HasPrefix(h.Name, "../") ||
			(h.Name == "index.json" || h.Name == "manifest.json" || strings.HasPrefix(h.Name, "blobs/sha256/")) && h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return "", fmt.Errorf("%w: invalid tar entry %q type %d", invalid, h.Name, h.Typeflag)
		}
		if h.Name != "index.json" && h.Name != "manifest.json" && !strings.HasPrefix(h.Name, "blobs/sha256/") {
			continue
		}
		if strings.HasPrefix(h.Name, "blobs/sha256/") && !validImageID("sha256:"+strings.TrimPrefix(h.Name, "blobs/sha256/")) {
			return "", fmt.Errorf("%w: invalid blob path %q", invalid, h.Name)
		}
		if h.Name == "index.json" || h.Name == "manifest.json" || h.Name == "blobs/sha256/"+configID[7:] || h.Size <= 65536 && strings.HasPrefix(h.Name, "blobs/sha256/") {
			if h.Size < 1 || h.Size > 1024*1024 || h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return "", fmt.Errorf("%w: invalid metadata size %q %d", invalid, h.Name, h.Size)
			}
			metadataBytes += h.Size
			if metadataBytes > 4*1024*1024 {
				return "", fmt.Errorf("%w: metadata limit", invalid)
			}
			data, err := io.ReadAll(io.LimitReader(tr, h.Size))
			if err != nil {
				return "", fmt.Errorf("%w: metadata read %q", invalid, h.Name)
			}
			switch h.Name {
			case "index.json":
				if index != nil {
					return "", fmt.Errorf("%w: duplicate index", invalid)
				}
				index = data
			case "manifest.json":
				if dockerManifest != nil {
					return "", fmt.Errorf("%w: duplicate docker manifest", invalid)
				}
				dockerManifest = data
			default:
				if blobs[h.Name] != nil {
					return "", fmt.Errorf("%w: duplicate metadata blob", invalid)
				}
				blobs[h.Name] = data
			}
		}
	}
	if !complete {
		return "", fmt.Errorf("%w: incomplete archive", invalid)
	}
	var dockerImages []struct {
		Config   string   `json:"Config"`
		RepoTags []string `json:"RepoTags"`
	}
	var idx struct {
		Manifests []struct {
			Digest    string `json:"digest"`
			MediaType string `json:"mediaType"`
		} `json:"manifests"`
	}
	if json.Unmarshal(dockerManifest, &dockerImages) != nil || len(dockerImages) != 1 ||
		dockerImages[0].Config != "blobs/sha256/"+configID[7:] || len(dockerImages[0].RepoTags) != 1 || dockerImages[0].RepoTags[0] != loadedRef ||
		json.Unmarshal(index, &idx) != nil || len(idx.Manifests) != 1 || !jobManifestMediaType(idx.Manifests[0].MediaType) ||
		!validImageID(idx.Manifests[0].Digest) {
		return "", fmt.Errorf("%w: manifest/index mismatch", invalid)
	}
	config := blobs[dockerImages[0].Config]
	manifestID := idx.Manifests[0].Digest
	manifest := blobs["blobs/sha256/"+manifestID[7:]]
	if len(config) == 0 || len(manifest) == 0 || digestJobBlob(config) != configID || digestJobBlob(manifest) != manifestID {
		return "", fmt.Errorf("%w: missing or mismatched config/manifest blob: config=%d manifest=%d", invalid, len(config), len(manifest))
	}
	var image struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Config        struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Layers []json.RawMessage `json:"layers"`
	}
	if json.Unmarshal(manifest, &image) != nil || image.SchemaVersion != 2 ||
		(image.MediaType != "" && image.MediaType != idx.Manifests[0].MediaType) || image.Config.Digest != configID || len(image.Layers) == 0 {
		return "", invalid
	}
	return manifestID, nil
}

func jobManifestMediaType(value string) bool {
	return value == "application/vnd.oci.image.manifest.v1+json" || value == "application/vnd.docker.distribution.manifest.v2+json"
}

func digestJobBlob(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
