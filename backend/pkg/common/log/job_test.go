package log

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestJobLoggingDefaultsVisibleInContainerAndFile(t *testing.T) {
	for _, level := range []string{"", "typo", " INFO "} {
		t.Run(level, func(t *testing.T) {
			root := t.TempDir()
			var stderr bytes.Buffer
			l := newJobLogger(root, level, 0, &stderr)
			if l.Level != logrus.InfoLevel {
				t.Fatal("default suppresses diagnostics", l.Level)
			}
			l.WithField("project_id", "43368658176704510").Warn("scan_probe")
			if !strings.Contains(stderr.String(), "scan_probe") || !strings.Contains(stderr.String(), `"retention_days":7`) {
				t.Fatal(stderr.String())
			}
			files, _ := filepath.Glob(filepath.Join(root, "logs", "job-*.log"))
			if len(files) != 1 {
				t.Fatal("missing file", files)
			}
			content, e := os.ReadFile(files[0])
			if e != nil || !bytes.Contains(content, []byte("scan_probe")) {
				t.Fatal("missing persisted diagnostic", e)
			}
			if c, ok := l.Out.(*jobLogWriter).file.(interface{ Close() error }); ok {
				_ = c.Close()
			}
		})
	}
}

func TestJobLoggingFallsBackWhenPathIsNotWritableDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-dir")
	if e := os.WriteFile(path, []byte("file"), 0o600); e != nil {
		t.Fatal(e)
	}
	var stderr bytes.Buffer
	l := newJobLogger(path, "error", 7, &stderr)
	l.Error("still_visible")
	if !strings.Contains(stderr.String(), "still_visible") || !strings.Contains(stderr.String(), `"file_configured":false`) || strings.Contains(stderr.String(), path) {
		t.Fatal(stderr.String())
	}
}

type failedJobSink struct{ short bool }

func (w failedJobSink) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("SECRET/private/path")
}

func TestJobRuntimeFileFailureNeverDropsContainerLogs(t *testing.T) {
	for _, short := range []bool{false, true} {
		var stderr bytes.Buffer
		w := &jobLogWriter{stderr: &stderr, file: failedJobSink{short}}
		for i := 0; i < 2; i++ {
			if n, e := w.Write([]byte("scan_failure\n")); e != nil || n != 13 {
				t.Fatal(n, e)
			}
		}
		if strings.Count(stderr.String(), "scan_failure") != 2 || strings.Count(stderr.String(), "log_file_write_failed") != 1 || strings.Contains(stderr.String(), "SECRET") {
			t.Fatal(stderr.String())
		}
	}
}

func TestJobMirroredLogsRedactProviderURLAndCredentials(t *testing.T) {
	var out bytes.Buffer
	l := logrus.New()
	l.SetOutput(&out)
	l.SetFormatter(jobFormatter{base: &logrus.JSONFormatter{}})
	l.WithField("error", errors.New("Post https://rpc.example/SECRET: password=SECRET")).WithField("raw_tx", "SECRET").Warn("failed token=SECRET")
	if strings.Contains(out.String(), "SECRET") || strings.Contains(out.String(), "rpc.example") {
		t.Fatal(out.String())
	}
}
