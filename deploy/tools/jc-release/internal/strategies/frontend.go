package strategies

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

const maxStaticArtifactFiles = 20000

type nginxStrategy struct {
	name string
	comp shared.Component
}

func (s *nginxStrategy) Component() shared.Component { return s.comp }

func validateStaticArchive(src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	count := 0
	hasIndex := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		count++
		if count > maxStaticArtifactFiles {
			return fmt.Errorf("archive contains too many entries")
		}
		name := filepath.ToSlash(filepath.Clean(h.Name))
		if name == "." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("unsafe archive path %q", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeRegA, tar.TypeDir:
		default:
			return fmt.Errorf("archive contains unsupported entry %q", h.Name)
		}
		if strings.HasSuffix(name, "dist/index.html") || name == "dist/index.html" {
			hasIndex = true
		}
	}
	if !hasIndex {
		return fmt.Errorf("archive has no dist/index.html")
	}
	return nil
}

func validateStaticBackupArchive(src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	count := 0
	hasIndex := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		count++
		if count > maxStaticArtifactFiles {
			return fmt.Errorf("backup contains too many entries")
		}
		name := filepath.ToSlash(filepath.Clean(h.Name))
		if name != "html" && name != "html/" && !strings.HasPrefix(name, "html/") {
			return fmt.Errorf("unexpected backup path %q", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeRegA, tar.TypeDir:
		default:
			return fmt.Errorf("backup contains unsupported entry %q", h.Name)
		}
		if name == "html/index.html" {
			hasIndex = true
		}
	}
	if !hasIndex {
		return fmt.Errorf("backup has no html/index.html")
	}
	return nil
}

func (s *nginxStrategy) Apply(ctx context.Context, src string, log io.Writer) error {
	if err := validateStaticArchive(src); err != nil {
		return fmt.Errorf("validate static archive: %w", err)
	}
	tmp, err := os.MkdirTemp("", "jc-agent-apply-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err := runCmd(ctx, log, "tar", "--no-same-owner", "--no-same-permissions", "-xzf", src, "-C", tmp); err != nil {
		return err
	}
	distDir, err := findDist(tmp)
	if err != nil {
		return fmt.Errorf("extract layout: %w", err)
	}
	if _, err := runCmd(ctx, log, "docker", "exec", s.name, "sh", "-c", "rm -rf /usr/share/nginx/html"); err != nil {
		return err
	}
	if _, err := runCmd(ctx, log, "docker", "cp", distDir, s.name+":/usr/share/nginx/html"); err != nil {
		return err
	}
	if _, err := runCmd(ctx, log, "docker", "exec", s.name, "nginx", "-s", "reload"); err != nil {
		fmt.Fprintln(log, "[apply] nginx reload failed, falling back to docker restart")
		if _, err := runCmd(ctx, log, "docker", "restart", s.name); err != nil {
			return err
		}
	}
	return nil
}

func (s *nginxStrategy) Backup(ctx context.Context, dstDir string, log io.Writer) (string, error) {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return "", err
	}
	dst := filepath.Join(dstDir, fmt.Sprintf("%s-%s.tar.gz", s.comp, time.Now().UTC().Format("20060102-150405")))
	if _, err := runShell(ctx, log, fmt.Sprintf("docker exec %s tar -C /usr/share/nginx -czf - html > %q", s.name, dst)); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *nginxStrategy) Rollback(ctx context.Context, backup string, log io.Writer) error {
	if backup == "" {
		return ErrNoBackup
	}
	if err := validateStaticBackupArchive(backup); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "jc-agent-rollback-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err := runCmd(ctx, log, "tar", "--no-same-owner", "--no-same-permissions", "-xzf", backup, "-C", tmp); err != nil {
		return err
	}
	restore := filepath.Join(tmp, "html")
	if _, err := os.Stat(filepath.Join(restore, "index.html")); err != nil {
		return fmt.Errorf("backup malformed: %w", err)
	}
	if _, err := runCmd(ctx, log, "docker", "exec", s.name, "sh", "-c", "rm -rf /usr/share/nginx/html"); err != nil {
		return err
	}
	if _, err := runCmd(ctx, log, "docker", "cp", restore, s.name+":/usr/share/nginx/html"); err != nil {
		return err
	}
	if _, err := runCmd(ctx, log, "docker", "exec", s.name, "nginx", "-s", "reload"); err != nil {
		_, err = runCmd(ctx, log, "docker", "restart", s.name)
		return err
	}
	return nil
}

func (s *nginxStrategy) HealthCheck(ctx context.Context) error {
	_, err := runCmd(ctx, io.Discard, "docker", "exec", s.name, "sh", "-c", "curl -sSf --max-time 5 http://localhost/ > /dev/null")
	if err != nil {
		return fmt.Errorf("nginx healthcheck failed: %w", err)
	}
	return nil
}

func findDist(root string) (string, error) {
	for _, dir := range []string{"frontend/dist", "admin/dist", "dist"} {
		p := filepath.Join(root, dir)
		if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
			return p, nil
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		inner := filepath.Join(root, entry.Name(), "dist")
		if _, err := os.Stat(filepath.Join(inner, "index.html")); err == nil {
			return inner, nil
		}
	}
	return "", fmt.Errorf("no dist/ with index.html found under %s", root)
}
