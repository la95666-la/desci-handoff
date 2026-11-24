package server

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"strings"
)

// jobImageID reads only bounded metadata from a docker-save gzip. Image ID is
// the digest of the image config, NOT the outer upload checksum. The Agent
// compares this ID with Docker's actual loaded ID before recreating Job.
func jobImageID(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", errors.New("job_image_archive_invalid")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var manifest []byte
	configs := make(map[string]string)
	seen := make(map[string]bool)
	for count := 0; count < 20000; count++ {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil || h == nil || h.Size < 0 || h.Name == "" || strings.HasPrefix(h.Name, "/") || path.Clean(strings.TrimSuffix(h.Name, "/")) != strings.TrimSuffix(h.Name, "/") || strings.HasPrefix(h.Name, "../") || strings.Contains(h.Name, "/../") {
			return "", errors.New("job_image_archive_invalid")
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return "", errors.New("job_image_archive_invalid")
		}
		if seen[h.Name] {
			return "", errors.New("job_image_duplicate_entry")
		}
		seen[h.Name] = true
		if h.Name == "manifest.json" {
			if h.Size > 64*1024 || manifest != nil {
				return "", errors.New("job_image_manifest_invalid")
			}
			manifest, err = io.ReadAll(io.LimitReader(tr, h.Size))
			if err != nil {
				return "", err
			}
		} else if (strings.HasSuffix(h.Name, ".json") || strings.HasPrefix(h.Name, "blobs/sha256/")) && h.Size <= 1024*1024 {
			// Docker config blobs are JSON; hash bounded files without expanding
			// or retaining the binary layers in server memory.
			digest := sha256.New()
			if _, err := io.Copy(digest, tr); err != nil {
				return "", err
			}
			configs[h.Name] = hex.EncodeToString(digest.Sum(nil))
		}
		if count == 19999 {
			return "", errors.New("job_image_archive_too_many_entries")
		}
	}
	var images []struct {
		Config   string   `json:"Config"`
		RepoTags []string `json:"RepoTags"`
		Layers   []string `json:"Layers"`
	}
	if len(manifest) == 0 || json.Unmarshal(manifest, &images) != nil || len(images) != 1 || len(images[0].Layers) == 0 || len(images[0].RepoTags) != 1 || !strings.HasPrefix(images[0].RepoTags[0], "joincare/job:") {
		return "", errors.New("job_image_manifest_invalid")
	}
	for _, layer := range images[0].Layers {
		if !seen[layer] || (!strings.HasSuffix(layer, ".tar") && !strings.HasPrefix(layer, "blobs/sha256/")) {
			return "", errors.New("job_image_layer_missing")
		}
	}
	config := images[0].Config
	if strings.HasPrefix(config, "blobs/sha256/") {
		if strings.TrimPrefix(config, "blobs/sha256/") == configs[config] && len(configs[config]) == 64 {
			return "sha256:" + configs[config], nil
		}
	} else if strings.HasSuffix(config, ".json") && strings.TrimSuffix(config, ".json") == configs[config] && len(configs[config]) == 64 {
		return "sha256:" + configs[config], nil
	}
	return "", errors.New("job_image_config_invalid")
}
