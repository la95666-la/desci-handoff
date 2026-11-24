package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joincare/jc-release/internal/shared"
)

// API wires storage + config into HTTP handlers.
type API struct {
	Store  *Storage
	Cfg    *Config
	Logger *slog.Logger
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, resp shared.APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func ok(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, shared.APIResponse{Status: true, Data: data})
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, shared.APIResponse{Status: false, Code: code, Message: msg})
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("additional json value")
		}
		return err
	}
	return nil
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		parts := strings.Split(v, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// pathParam extracts the segment after prefix, then splits by "/" and returns idx.
// e.g. path="/api/v1/agents/abc/commands/xyz/report", prefix="/api/v1/agents/", idx=0 -> "abc"
func pathParam(r *http.Request, prefix string, idx int) string {
	tail := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(tail, "/")
	if idx >= len(parts) {
		return ""
	}
	return parts[idx]
}

// ---------- auth (session) ----------

func (a *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r, a.Cfg.SessionSecret)
		if u == "" {
			fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
			return
		}
		next(w, r)
	}
}

// requireServiceLogRead accepts either an authenticated admin session or the
// dedicated read-only bearer token. It is intentionally not reused by release
// or command endpoints.
func (a *API) requireServiceLogRead(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if CurrentUser(r, a.Cfg.SessionSecret) != "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		provided := strings.TrimPrefix(auth, "Bearer ")
		if strings.HasPrefix(auth, "Bearer ") && a.Cfg.ReadAPIToken != "" &&
			subtle.ConstantTimeCompare([]byte(provided), []byte(a.Cfg.ReadAPIToken)) == 1 {
			next(w, r)
			return
		}
		fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "admin session or read-only bearer token required")
	}
}

func (a *API) requireAgent(next func(w http.ResponseWriter, r *http.Request, ag shared.Agent)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "bearer token required")
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		ag, err := a.Store.GetAgentByToken(token)
		if err != nil {
			fail(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
			return
		}
		next(w, r, ag)
	}
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
		return
	}
	if r.URL.Path != "/api/v1/auth/login" {
		fail(w, http.StatusNotFound, "NOT_FOUND", "expected /api/v1/auth/login")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "LOGIN_BODY_TOO_LARGE", "login body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "LOGIN_BODY_TOO_LARGE", "login body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "login must contain exactly one json object")
		return
	}
	if err := shared.ValidateLoginFields(body.Username, body.Password); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_LOGIN_FIELDS", err.Error())
		return
	}
	if body.Username != a.Cfg.AdminUsername || !VerifyPassword(a.Cfg.AdminPasswordBcrypt, body.Password) {
		fail(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong username or password")
		return
	}
	SetSessionCookie(w, a.Cfg.SessionSecret, body.Username, a.Cfg.TLS != nil)
	ok(w, map[string]string{"username": body.Username})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	ClearSessionCookie(w, a.Cfg.TLS != nil)
	ok(w, nil)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r, a.Cfg.SessionSecret)
	if u == "" {
		fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "not logged in")
		return
	}
	ok(w, map[string]string{"username": u})
}

// ---------- releases ----------

func (a *API) handleReleases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		comp := shared.Component(r.URL.Query().Get("component"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 20
		}
		rels, err := a.Store.ListReleases(comp, limit)
		if err != nil {
			fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
			return
		}
		ok(w, rels)
	case http.MethodPost:
		a.handleUploadRelease(w, r)
	default:
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
	}
}

func (a *API) handleUploadRelease(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxArtifactBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, http.StatusBadRequest, "BAD_MULTIPART", "invalid upload")
		return
	}
	defer r.MultipartForm.RemoveAll()
	comp := shared.Component(r.FormValue("component"))
	if !shared.ValidComponent(comp) {
		fail(w, http.StatusBadRequest, "INVALID_COMPONENT", "component has no release strategy")
		return
	}
	version := strings.TrimSpace(r.FormValue("version"))
	if version == "" || len(version) > 128 || len(r.FormValue("notes")) > 4096 {
		fail(w, http.StatusBadRequest, "INVALID_RELEASE_FIELDS", "invalid version or notes")
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "MISSING_FILE", "file required")
		return
	}
	defer f.Close()
	if comp == shared.CompJob && r.FormValue("job_admission") != "" {
		fail(w, http.StatusBadRequest, "INVALID_RELEASE_FIELDS", "Job image identity is derived from the uploaded package")
		return
	}
	if comp != shared.CompJob && r.FormValue("job_admission") != "" {
		fail(w, http.StatusBadRequest, "INVALID_RELEASE_FIELDS", "job declaration only permitted for job")
		return
	}
	id := uuid.NewString()
	size, sum, err := a.Store.SaveArtifact(id, f)
	if err != nil {
		fail(w, http.StatusBadRequest, "SAVE_FAILED", err.Error())
		return
	}
	rel := shared.Release{ID: id, Component: comp, Version: version, Filename: header.Filename, Size: size, SHA256: sum, Notes: r.FormValue("notes"), UploadedAt: time.Now().UTC(), UploadedBy: CurrentUser(r, a.Cfg.SessionSecret)}
	if comp == shared.CompJob {
		imageID, err := jobImageID(a.Store.ArtifactPath(id))
		if err != nil {
			os.Remove(a.Store.ArtifactPath(id))
			fail(w, http.StatusBadRequest, "JOB_IMAGE_INVALID", "invalid Job docker-save image package")
			return
		}
		// The Agent enforces monotonic generation from its own durable state.
		// Server upload only describes the artifact, never authorizes deployment.
		rel.JobAdmission, _ = json.Marshal(struct {
			Environment    string `json:"environment"`
			Component      string `json:"component"`
			Generation     uint64 `json:"generation"`
			ArtifactSHA256 string `json:"artifact_sha256"`
			ImageID        string `json:"image_id"`
		}{"prod", "job", 0, sum, imageID})
	}
	if err := a.Store.AddRelease(rel); err != nil {
		os.Remove(a.Store.ArtifactPath(id))
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	a.Logger.Info("release uploaded", "component", comp, "version", version, "size", size)
	ok(w, rel)
}

func (a *API) handleReleaseDetail(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "/api/v1/releases/", 0)
	if !validReleaseID(id) || r.URL.Path != "/api/v1/releases/"+id {
		fail(w, http.StatusBadRequest, "MISSING_ID", "release id required")
		return
	}
	if r.Method != http.MethodDelete {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "DELETE required")
		return
	}
	inUse, err := a.Store.ReleaseInUse(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	if inUse {
		fail(w, http.StatusConflict, "RELEASE_IN_USE", "release is deployed")
		return
	}
	if err := a.Store.DeleteRelease(id); err != nil {
		fail(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	ok(w, nil)
}

// ---------- artifacts ----------

func (a *API) handleArtifact(w http.ResponseWriter, r *http.Request, _ shared.Agent) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET required")
		return
	}
	id := pathParam(r, "/api/v1/artifacts/", 0)
	if !validReleaseID(id) || r.URL.Path != "/api/v1/artifacts/"+id {
		fail(w, http.StatusBadRequest, "BAD_ID", "invalid release id")
		return
	}
	rel, err := a.Store.GetRelease(id)
	if err != nil {
		fail(w, http.StatusNotFound, "NOT_FOUND", "release not found")
		return
	}
	f, err := os.Open(a.Store.ArtifactPath(id))
	if err != nil {
		fail(w, http.StatusNotFound, "ARTIFACT_MISSING", "artifact missing")
		return
	}
	defer f.Close()
	// Never serve an archive different from the manifest identity. The agent
	// independently hashes downloads, but failure here avoids advertising a
	// corrupted or swapped artifact as a usable release.
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxArtifactBytes+1))
	if err != nil || n != rel.Size || hex.EncodeToString(h.Sum(nil)) != rel.SHA256 {
		fail(w, http.StatusConflict, "ARTIFACT_MISMATCH", "stored artifact differs from release identity")
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fail(w, http.StatusInternalServerError, "ARTIFACT_UNAVAILABLE", "artifact cannot be read")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Length", strconv.FormatInt(rel.Size, 10))
	w.Header().Set("X-SHA256", rel.SHA256)
	w.Header().Set("X-Filename", rel.Filename)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// ---------- agents (admin) ----------

func (a *API) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		return
	}
	agents, err := a.Store.ListAgents()
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	// Do not leak agent tokens over admin UI.
	for i := range agents {
		agents[i].Token = ""
	}
	ok(w, agents)
}

func (a *API) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "/api/v1/agents/", 0)
	if id == "" {
		fail(w, http.StatusBadRequest, "MISSING_ID", "agent id required")
		return
	}
	if r.Method != http.MethodDelete {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		return
	}
	if err := a.Store.DeleteAgent(id); err != nil {
		fail(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	ok(w, nil)
}

func (a *API) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		return
	}
	tok, err := randomToken(24)
	if err != nil {
		fail(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
		return
	}
	et := shared.EnrollToken{
		Token:     tok,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := a.Store.AddEnrollToken(et); err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	ok(w, et)
}

// ---------- register (agent) ----------

func (a *API) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
		return
	}
	if r.URL.Path != "/api/v1/register" {
		fail(w, http.StatusNotFound, "NOT_FOUND", "expected /api/v1/register")
		return
	}
	var body struct {
		EnrollToken string `json:"enroll_token"`
		Hostname    string `json:"hostname"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRegisterBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "REGISTER_BODY_TOO_LARGE", "registration body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "REGISTER_BODY_TOO_LARGE", "registration body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "registration must contain exactly one json object")
		return
	}
	if err := shared.ValidateEnrollToken(body.EnrollToken); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_ENROLL_TOKEN", err.Error())
		return
	}
	if err := shared.ValidateHostname(body.Hostname); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_HOSTNAME", err.Error())
		return
	}
	agentToken, err := randomToken(32)
	if err != nil {
		fail(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
		return
	}
	now := time.Now().UTC()
	ag := shared.Agent{
		ID:           uuid.NewString(),
		Hostname:     body.Hostname,
		IP:           clientIP(r),
		Token:        agentToken,
		RegisteredAt: now,
		LastSeenAt:   now,
		Deployed:     map[shared.Component]string{},
	}
	if err := shared.ValidateAgentRegistration(&ag); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_AGENT_REGISTRATION", err.Error())
		return
	}
	if err := a.Store.RegisterAgent(body.EnrollToken, ag); err != nil {
		fail(w, http.StatusUnauthorized, "INVALID_ENROLL", err.Error())
		return
	}
	a.Logger.Info("agent registered", "agent_id", ag.ID, "hostname", ag.Hostname)
	ok(w, map[string]string{"agent_id": ag.ID, "agent_token": agentToken})
}

// ---------- agent bearer endpoints ----------

func (a *API) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request, ag shared.Agent) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
		return
	}
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(tail, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "heartbeat" {
		fail(w, http.StatusNotFound, "NOT_FOUND", "expected /agents/:id/heartbeat")
		return
	}
	if parts[0] != ag.ID {
		fail(w, http.StatusForbidden, "FORBIDDEN", "token/agent mismatch")
		return
	}
	var body struct {
		Deployed map[shared.Component]string `json:"deployed"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxHeartbeatBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "HEARTBEAT_BODY_TOO_LARGE", "heartbeat body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "heartbeat must contain exactly one json object")
		return
	}
	if err := shared.ValidateDeployedMap(body.Deployed); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_HEARTBEAT", err.Error())
		return
	}
	err := a.Store.UpdateAgent(ag.ID, func(x *shared.Agent) error {
		x.LastSeenAt = time.Now().UTC()
		x.IP = clientIP(r)
		if body.Deployed != nil {
			x.Deployed = body.Deployed
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	ok(w, nil)
}

func (a *API) handleAgentManifest(w http.ResponseWriter, r *http.Request, ag shared.Agent) {
	id := pathParam(r, "/api/v1/agents/", 0)
	if id != ag.ID {
		fail(w, http.StatusForbidden, "FORBIDDEN", "token/agent mismatch")
		return
	}
	m, err := a.Store.LatestPerComponent()
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	ok(w, m)
}

// handleAgentRelease returns one exact historical release to its owning agent.
// This is metadata only; artifact bytes still require the separate agent route.
func (a *API) handleAgentRelease(w http.ResponseWriter, r *http.Request, ag shared.Agent) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(tail, "/")
	if r.Method != http.MethodGet || len(parts) != 3 || parts[0] != ag.ID || parts[1] != "releases" || !validReleaseID(parts[2]) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "release not found")
		return
	}
	rel, err := a.Store.GetRelease(parts[2])
	if err != nil || !shared.ValidComponent(rel.Component) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "release not found")
		return
	}
	ok(w, rel)
}

func (a *API) handleAgentPollCommands(w http.ResponseWriter, r *http.Request, ag shared.Agent) {
	id := pathParam(r, "/api/v1/agents/", 0)
	if id != ag.ID {
		fail(w, http.StatusForbidden, "FORBIDDEN", "token/agent mismatch")
		return
	}
	cmds, err := a.Store.ClaimPendingForAgent(ag.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	if cmds == nil {
		cmds = []shared.Command{}
	}
	ok(w, cmds)
}

func (a *API) handleAgentReport(w http.ResponseWriter, r *http.Request, ag shared.Agent) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
		return
	}
	// Exact path: /api/v1/agents/:id/commands/:cmd_id/report
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(tail, "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] != "commands" || parts[2] == "" || parts[3] != "report" {
		fail(w, http.StatusNotFound, "NOT_FOUND", "expected /agents/:id/commands/:cmd_id/report")
		return
	}
	agentID := parts[0]
	cmdID := parts[2]
	if agentID != ag.ID {
		fail(w, http.StatusForbidden, "FORBIDDEN", "token/agent mismatch")
		return
	}
	var body struct {
		Status string `json:"status"`
		Log    string `json:"log"`
		Error  string `json:"error"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAgentReportBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "REPORT_BODY_TOO_LARGE", "agent report body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "REPORT_BODY_TOO_LARGE", "agent report body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "agent report must contain exactly one json object")
		return
	}
	// Force a read at the boundary even when the JSON object itself ended before
	// the limit, so oversized trailing whitespace cannot bypass the body cap.
	probe := make([]byte, 1)
	if _, err := r.Body.Read(probe); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "REPORT_BODY_TOO_LARGE", "agent report body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid agent report body")
		return
	}
	if len([]byte(body.Status)) > shared.MaxCommandStatusBytes {
		fail(w, http.StatusRequestEntityTooLarge, "REPORT_STATUS_TOO_LARGE", "agent report status too large")
		return
	}
	if !shared.ValidTerminalReportStatus(body.Status) {
		fail(w, http.StatusBadRequest, "INVALID_REPORT_STATUS", "status must be success or failed")
		return
	}
	if len([]byte(body.Log)) > shared.MaxCommandLogBytes {
		fail(w, http.StatusRequestEntityTooLarge, "REPORT_LOG_TOO_LARGE", "agent report log exceeds 256 KiB")
		return
	}
	if len([]byte(body.Error)) > shared.MaxCommandErrorBytes {
		fail(w, http.StatusRequestEntityTooLarge, "REPORT_ERROR_TOO_LARGE", "agent report error exceeds 8 KiB")
		return
	}
	now := time.Now().UTC()
	err := a.Store.FinalizeCommand(cmdID, ag.ID, body.Status, body.Log, body.Error, now)
	if err != nil {
		code := "STORAGE_ERROR"
		status := http.StatusConflict
		if err.Error() == "command_belongs_to_different_agent" {
			status = http.StatusForbidden
			code = "FORBIDDEN"
		} else if err.Error() == "command not found" {
			status = http.StatusNotFound
			code = "COMMAND_NOT_FOUND"
		} else if err.Error() == "command_not_in_progress" {
			code = "COMMAND_NOT_IN_PROGRESS"
		}
		fail(w, status, code, err.Error())
		return
	}
	ok(w, nil)
}

// ---------- commands (admin) ----------

func (a *API) handleQueueCommand(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/agents/:agent_id/commands
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(tail, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "commands" {
		fail(w, http.StatusBadRequest, "BAD_PATH", "expected /agents/:id/commands")
		return
	}
	agentID := parts[0]
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		return
	}
	if _, err := a.Store.GetAgent(agentID); err != nil {
		fail(w, http.StatusNotFound, "AGENT_NOT_FOUND", err.Error())
		return
	}
	var body struct {
		Kind      string           `json:"kind"`
		Component shared.Component `json:"component"`
		ReleaseID string           `json:"release_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "command must contain exactly one json object")
		return
	}
	switch body.Kind {
	case shared.CmdUpgrade, shared.CmdRollback, shared.CmdRestart:
	default:
		fail(w, http.StatusBadRequest, "BAD_KIND", "kind must be upgrade|rollback|restart")
		return
	}
	if !shared.ValidComponent(body.Component) {
		fail(w, http.StatusBadRequest, "BAD_COMPONENT", "invalid component")
		return
	}
	if body.Kind == shared.CmdRollback {
		// Local backup selection is not bound to a release ID. Refuse before
		// querying the manifest so a UI cannot claim a target it cannot prove.
		fail(w, http.StatusBadRequest, "ROLLBACK_TARGET_UNVERIFIED", "local backup target cannot be proven by release ID")
		return
	}
	if body.Kind == shared.CmdUpgrade {
		if body.ReleaseID == "" {
			fail(w, http.StatusBadRequest, "MISSING_RELEASE", "release_id required")
			return
		}
		rel, err := a.Store.GetRelease(body.ReleaseID)
		if err != nil {
			fail(w, http.StatusBadRequest, "RELEASE_NOT_FOUND", "release not found")
			return
		}
		if rel.Component != body.Component {
			fail(w, http.StatusConflict, "RELEASE_COMPONENT_MISMATCH", "release component does not match command component")
			return
		}
	}
	cmd := shared.Command{ID: uuid.NewString(), AgentID: agentID, Kind: body.Kind, Component: body.Component, ReleaseID: body.ReleaseID, Status: shared.StatusPending, CreatedAt: time.Now().UTC(), IssuedBy: CurrentUser(r, a.Cfg.SessionSecret)}
	if err := a.Store.AddCommand(cmd); err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	a.Logger.Info("command queued", "agent_id", agentID, "kind", body.Kind, "component", body.Component)
	ok(w, cmd)
}

func (a *API) handleListCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "")
		return
	}
	agentID := r.URL.Query().Get("agent_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	cmds, err := a.Store.ListCommands(agentID, limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	if r.URL.Query().Get("summary") == "true" {
		// History polling must not retransmit every command's full service log.
		// Preserve the legacy full API for existing clients; detail remains exact.
		for i := range cmds {
			cmds[i].Log = ""
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	ok(w, cmds)
}

func (a *API) handleCommandDetail(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "/api/v1/commands/", 0)
	if id == "" {
		fail(w, http.StatusBadRequest, "MISSING_ID", "command id required")
		return
	}
	cmd, err := a.Store.GetCommand(id)
	if err != nil {
		fail(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	ok(w, cmd)
}

// ---------- service logs (admin session or read-only API token) ----------

const (
	maxLoginBodyBytes      int64 = 2048
	maxRegisterBodyBytes   int64 = 2048
	defaultLogLines              = 200
	defaultLogSinceSeconds       = 3600
	maxActiveLogQueries          = 3
)

func (a *API) handleServiceLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		agents, err := a.Store.ListAgents()
		if err != nil {
			fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
			return
		}
		for i := range agents {
			agents[i].Token = ""
		}
		ok(w, map[string]interface{}{
			"agents":  agents,
			"targets": shared.ServiceLogTargets(),
			"defaults": map[string]int{
				"lines":         defaultLogLines,
				"since_seconds": defaultLogSinceSeconds,
			},
		})
	case http.MethodPost:
		a.handleQueueServiceLogs(w, r)
	default:
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET or POST required")
	}
}

func (a *API) handleQueueServiceLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/api/v1/service-logs" {
		if r.Method != http.MethodPost {
			fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
		} else {
			fail(w, http.StatusNotFound, "NOT_FOUND", "expected /api/v1/service-logs")
		}
		return
	}
	var body struct {
		AgentID      string `json:"agent_id"`
		Target       string `json:"target"`
		Lines        int    `json:"lines"`
		SinceSeconds int    `json:"since_seconds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxServiceLogBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "SERVICE_LOG_BODY_TOO_LARGE", "service log request body too large")
			return
		}
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		fail(w, http.StatusBadRequest, "BAD_REQUEST", "service log request must contain exactly one json object")
		return
	}
	body.AgentID = strings.TrimSpace(body.AgentID)
	body.Target = strings.ToLower(strings.TrimSpace(body.Target))
	if _, err := a.Store.GetAgent(body.AgentID); err != nil {
		fail(w, http.StatusNotFound, "AGENT_NOT_FOUND", err.Error())
		return
	}
	if body.Lines == 0 {
		body.Lines = defaultLogLines
	}
	if body.SinceSeconds == 0 {
		body.SinceSeconds = defaultLogSinceSeconds
	}

	commands, err := a.Store.ListCommands(body.AgentID, 100)
	if err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	active := 0
	for _, command := range commands {
		if command.Kind == shared.CmdLogs &&
			(command.Status == shared.StatusPending || command.Status == shared.StatusInProgress) {
			active++
		}
	}
	if active >= maxActiveLogQueries {
		fail(w, http.StatusTooManyRequests, "TOO_MANY_LOG_QUERIES", "wait for an existing log query to finish")
		return
	}

	issuedBy := CurrentUser(r, a.Cfg.SessionSecret)
	if issuedBy == "" {
		issuedBy = "read-api"
	}
	cmd := shared.Command{
		ID:        uuid.NewString(),
		AgentID:   body.AgentID,
		Kind:      shared.CmdLogs,
		Component: shared.Component(body.Target),
		Status:    shared.StatusPending,
		CreatedAt: time.Now().UTC(),
		IssuedBy:  issuedBy,
		LogLines:  body.Lines,
		LogSince:  body.SinceSeconds,
	}
	if err := shared.ValidateServiceLogCommand(&cmd); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_SERVICE_LOG_COMMAND", err.Error())
		return
	}
	if err := a.Store.AddCommand(cmd); err != nil {
		fail(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	a.Logger.Info("service log query queued", "agent_id", body.AgentID, "target", body.Target, "lines", body.Lines)
	ok(w, cmd)
}

func (a *API) handleServiceLogDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET required")
		return
	}
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/service-logs/")
	parts := strings.Split(tail, "/")
	if len(parts) != 1 || parts[0] == "" || r.URL.Path != "/api/v1/service-logs/"+parts[0] {
		fail(w, http.StatusNotFound, "NOT_FOUND", "expected /api/v1/service-logs/:id")
		return
	}
	id := parts[0]
	cmd, err := a.Store.GetCommand(id)
	if err != nil || cmd.Kind != shared.CmdLogs {
		fail(w, http.StatusNotFound, "NOT_FOUND", "service log query not found")
		return
	}
	ok(w, cmd)
}

// ---------- routing under /api/v1/agents/... ----------

// dispatchAgentsSub routes any request under /api/v1/agents/... based on path shape.
func (a *API) dispatchAgentsSub(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(tail, "/")
	if len(parts) == 0 || parts[0] == "" {
		fail(w, http.StatusNotFound, "NOT_FOUND", "")
		return
	}
	agentID := parts[0]
	// bare /agents/:id => admin delete
	if len(parts) == 1 {
		a.requireAdmin(a.handleAgentDetail)(w, r)
		return
	}
	sub := parts[1]
	switch sub {
	case "releases":
		if len(parts) != 3 {
			fail(w, http.StatusNotFound, "NOT_FOUND", "release not found")
			return
		}
		a.requireAgent(a.handleAgentRelease)(w, r)
	case "heartbeat":
		if len(parts) != 2 {
			fail(w, http.StatusNotFound, "NOT_FOUND", "")
			return
		}
		a.requireAgent(a.handleAgentHeartbeat)(w, r)
	case "manifest":
		a.requireAgent(a.handleAgentManifest)(w, r)
	case "commands":
		// /agents/:id/commands => admin queue (POST) — GET is via /api/v1/commands
		// /agents/:id/commands/:cmd_id/report => agent POST, exact shape only.
		if len(parts) == 4 && parts[2] != "" && parts[3] == "report" {
			a.requireAgent(a.handleAgentReport)(w, r)
			return
		}
		if len(parts) == 2 {
			if r.Method == http.MethodGet {
				a.requireAgent(a.handleAgentPollCommands)(w, r)
				return
			}
			a.requireAdmin(a.handleQueueCommand)(w, r)
			return
		}
		fail(w, http.StatusNotFound, "NOT_FOUND", "")
	default:
		_ = agentID
		fail(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("unknown /agents subpath: %s", sub))
	}
}
