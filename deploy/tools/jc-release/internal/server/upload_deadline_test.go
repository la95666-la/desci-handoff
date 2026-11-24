package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	readDeadline  time.Time
	writeDeadline time.Time
}

func (w *deadlineWriter) SetReadDeadline(t time.Time) error {
	w.readDeadline = t
	return nil
}

func (w *deadlineWriter) SetWriteDeadline(t time.Time) error {
	w.writeDeadline = t
	return nil
}

func TestUploadDeadlineOnlyExtendsPostAndUnwrapsLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	called := 0
	handler := requestLog(logger, withUploadDeadline(5*time.Minute, func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		writer := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		start := time.Now()
		handler.ServeHTTP(writer, httptest.NewRequest(method, "/api/v1/releases", nil))
		if writer.Code != http.StatusNoContent {
			t.Fatalf("%s status=%d", method, writer.Code)
		}
		if method == http.MethodGet {
			if !writer.readDeadline.IsZero() || !writer.writeDeadline.IsZero() {
				t.Fatal("GET deadlines changed")
			}
		} else {
			for _, deadline := range []time.Time{writer.readDeadline, writer.writeDeadline} {
				if deadline.Before(start.Add(5*time.Minute)) || deadline.After(time.Now().Add(5*time.Minute)) {
					t.Fatalf("POST deadline not 5 minutes: %v", deadline)
				}
			}
		}
	}
	if called != 2 {
		t.Fatalf("handler called %d times", called)
	}
}

func TestUploadDeadlineFailsClosedIfWriterCannotSetDeadlines(t *testing.T) {
	called := false
	rec := httptest.NewRecorder()
	withUploadDeadline(5*time.Minute, func(http.ResponseWriter, *http.Request) { called = true })(rec, httptest.NewRequest(http.MethodPost, "/api/v1/releases", nil))
	if called || rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unprotected upload handler called=%t status=%d", called, rec.Code)
	}
}
