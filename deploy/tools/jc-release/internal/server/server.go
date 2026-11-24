package server

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// WebAssets holds the embedded web/ directory. Assigned by the caller (main) via SetWebAssets.
var webAssets fs.FS

// SetWebAssets is called from the main package to inject the embedded UI files.
func SetWebAssets(f embed.FS, sub string) error {
	if sub != "" {
		sf, err := fs.Sub(f, sub)
		if err != nil {
			return err
		}
		webAssets = sf
		return nil
	}
	webAssets = f
	return nil
}

// Server ties config, storage, logger, and http.Server together.
type Server struct {
	cfg    *Config
	store  *Storage
	logger *slog.Logger
	http   *http.Server
}

// New assembles a Server without starting it.
func New(cfg *Config, store *Storage, logger *slog.Logger) *Server {
	api := &API{Store: store, Cfg: cfg, Logger: logger}

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("/api/v1/auth/login", api.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", api.handleLogout)
	mux.HandleFunc("/api/v1/auth/me", api.handleMe)

	// Releases (admin)
	mux.HandleFunc("/api/v1/releases", api.requireAdmin(withUploadDeadline(5*time.Minute, api.handleReleases)))
	mux.HandleFunc("/api/v1/releases/", api.requireAdmin(api.handleReleaseDetail))

	// Enroll (admin)
	mux.HandleFunc("/api/v1/enroll", api.requireAdmin(api.handleEnroll))

	// Register (agent, no bearer — validated via enroll token)
	mux.HandleFunc("/api/v1/register", api.handleRegister)

	// Bare list (admin)
	mux.HandleFunc("/api/v1/agents", api.requireAdmin(api.handleAgents))

	// All other /agents/... paths — dispatched by shape (some admin, some agent-bearer)
	mux.HandleFunc("/api/v1/agents/", api.dispatchAgentsSub)

	// Artifact download (agent bearer)
	mux.HandleFunc("/api/v1/artifacts/", api.requireAgent(api.handleArtifact))

	// Commands (admin)
	mux.HandleFunc("/api/v1/commands", api.requireAdmin(api.handleListCommands))
	mux.HandleFunc("/api/v1/commands/", api.requireAdmin(api.handleCommandDetail))

	// Read-only service logs (admin session or dedicated read-only API token)
	mux.HandleFunc("/api/v1/service-logs", api.requireServiceLogRead(api.handleServiceLogs))
	mux.HandleFunc("/api/v1/service-logs/", api.requireServiceLogRead(api.handleServiceLogDetail))

	// Static web assets (embedded)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveStatic(w, r)
	})

	// Wrap with request logger + panic recover
	handler := recoverMiddleware(logger, requestLog(logger, mux))

	s := &Server{
		cfg:    cfg,
		store:  store,
		logger: logger,
		http: &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           handler,
			ReadHeaderTimeout: 15 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
	return s
}

// Run starts the HTTP(S) server; blocks until ctx is cancelled or fatal error.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server listening", "addr", s.cfg.ListenAddr, "tls", s.cfg.TLS != nil)
		if s.cfg.TLS != nil {
			s.http.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			errCh <- s.http.ListenAndServeTLS(s.cfg.TLS.Cert, s.cfg.TLS.Key)
		} else {
			errCh <- s.http.ListenAndServe()
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ---- middleware ----

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(sr, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sr.code,
				"ms", time.Since(start).Milliseconds())
		}
	})
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic", "err", rec, "path", r.URL.Path)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

// ResponseController needs the underlying network writer to extend only the
// authenticated upload request's deadlines (not those of other API routes).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func withUploadDeadline(limit time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			deadline := time.Now().Add(limit)
			controller := http.NewResponseController(w)
			if err := controller.SetReadDeadline(deadline); err != nil {
				http.Error(w, "upload deadline unavailable", http.StatusServiceUnavailable)
				return
			}
			if err := controller.SetWriteDeadline(deadline); err != nil {
				http.Error(w, "upload deadline unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		next(w, r)
	}
}

func (s *statusRecorder) WriteHeader(c int) {
	s.code = c
	s.ResponseWriter.WriteHeader(c)
}

// ---- static ----

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if webAssets == nil {
		http.Error(w, "web assets not embedded", http.StatusInternalServerError)
		return
	}
	// The admin UI is embedded in the binary. Prevent browsers from retaining an
	// older UI after a self-upgrade replaces and restarts the server binary.
	w.Header().Set("Cache-Control", "no-store")
	// Map "/" to "index.html", "/login" to "login.html"
	p := strings.TrimPrefix(r.URL.Path, "/")
	switch p {
	case "":
		p = "index.html"
	case "login":
		p = "login.html"
	}
	f, err := webAssets.Open(p)
	if err != nil {
		// Fallback: SPA — serve index.html for unknown routes (not /api/*)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		idx, ierr := webAssets.Open("index.html")
		if ierr != nil {
			http.NotFound(w, r)
			return
		}
		defer idx.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		copyFile(w, idx)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if stat.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType(p))
	copyFile(w, f)
}

func copyFile(w http.ResponseWriter, f fs.File) {
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func contentType(p string) string {
	switch {
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(p, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(p, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	}
	return "application/octet-stream"
}
