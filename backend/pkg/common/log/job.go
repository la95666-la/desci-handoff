package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

// InitJob keeps container stderr authoritative even when rotation setup or a
// later file write fails. Empty/mistyped configuration must not default to
// logrus.PanicLevel (the zero value previously selected by a missing map key).
// Core/Notice Init and their logging contracts are intentionally unchanged.
func InitJob(path, level string, maxDay int) {
	logger = newJobLogger(path, level, maxDay, os.Stderr)
}

func newJobLogger(path, level string, maxDay int, stderr io.Writer) *logrus.Logger {
	l := logrus.New()
	l.SetFormatter(jobFormatter{base: &logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano}})
	parsed, ok := levels[strings.ToUpper(strings.TrimSpace(level))]
	if !ok {
		parsed = logrus.InfoLevel
	}
	l.SetLevel(parsed)
	if maxDay <= 0 {
		maxDay = 7
	}
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	var file io.Writer
	if os.MkdirAll(filepath.Join(path, "logs"), 0o750) == nil {
		writer, err := rotatelogs.New(filepath.Join(path, "logs", "job-%Y%m%d.log"),
			rotatelogs.WithMaxAge(time.Duration(maxDay)*24*time.Hour),
			rotatelogs.WithRotationTime(24*time.Hour))
		if err == nil {
			file = writer
		}
	}
	l.SetOutput(&jobLogWriter{stderr: stderr, file: file})
	// Direct startup diagnostic remains visible even with an explicit ERROR
	// level. Only finite fields; filesystem errors may contain private paths.
	fmt.Fprintf(stderr, "{\"component\":\"job\",\"event\":\"logging_ready\",\"level\":%q,\"retention_days\":%d,\"file_configured\":%t,\"stderr_enabled\":true}\n", parsed.String(), maxDay, file != nil)
	return l
}

var jobURL = regexp.MustCompile(`(?i)https?://[^\s"<>]+`)
var jobCredential = regexp.MustCompile(`(?i)(bearer\s+[^\s"<>]+|(?:password|private[_ ]?key|raw_tx|signature|secret|token)\s*[=:]\s*[^\s,;"<>]+)`)

type jobFormatter struct{ base logrus.Formatter }

func jobRedact(text string) string {
	return jobCredential.ReplaceAllString(jobURL.ReplaceAllString(text, "<redacted-url>"), "<redacted-credential>")
}
func (f jobFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	copy := *entry
	copy.Message = jobRedact(entry.Message)
	copy.Data = make(logrus.Fields, len(entry.Data))
	for key, value := range entry.Data {
		switch key {
		case "password", "private_key", "raw_tx", "signature", "secret", "token", "authorization":
			copy.Data[key] = "<redacted>"
			continue
		}
		switch v := value.(type) {
		case string:
			copy.Data[key] = jobRedact(v)
		case error:
			copy.Data[key] = jobRedact(v.Error())
		default:
			copy.Data[key] = value
		}
	}
	return f.base.Format(&copy)
}

// Unlike MultiWriter, a broken file sink never prevents container delivery.
// logrus serializes writes; once only rate-limits file sink failure notices.
type jobLogWriter struct {
	stderr io.Writer
	file   io.Writer
	once   sync.Once
}

func (w *jobLogWriter) Write(p []byte) (int, error) {
	n, err := w.stderr.Write(p)
	if w.file != nil {
		written, fileErr := w.file.Write(p)
		if fileErr != nil || written != len(p) {
			w.once.Do(func() {
				_, _ = io.WriteString(w.stderr, "{\"component\":\"job\",\"event\":\"log_file_write_failed\",\"stderr_enabled\":true}\n")
			})
		}
	}
	return n, err
}
