package shared

import (
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

// Component is one of the deployable pieces of the JoinCare stack.
type Component string

const (
	CompBackend  Component = "backend"
	CompFrontend Component = "frontend"
	CompAdmin    Component = "admin"
	CompTGBot    Component = "tgbot"
	CompLang     Component = "lang"
	// CompJob is restricted to the isolated IDO Job upgrade path.
	CompJob Component = "job"
)

// AllComponents returns the fixed list of components supported by jc-release.
func AllComponents() []Component {
	return []Component{CompBackend, CompFrontend, CompAdmin, CompTGBot, CompLang, CompJob}
}

// KnownComponent reports whether c belongs to the fixed component vocabulary.
func KnownComponent(c Component) bool {
	switch c {
	case CompBackend, CompFrontend, CompAdmin, CompTGBot, CompLang, CompJob:
		return true
	}
	return false
}

// ValidComponent reports the components with implemented, rollback-capable
// strategies. This is a capability check, not a decision to deploy.
func ValidComponent(c Component) bool {
	switch c {
	case CompBackend, CompFrontend, CompAdmin, CompJob:
		return true
	default:
		return false
	}
}

const (
	MaxHeartbeatComponents   = 6
	MaxHeartbeatComponentLen = 16
	MaxReleaseIDBytes        = 64
)

var releaseIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidateDeployedMap bounds the retained heartbeat control-plane state. Known
// historical components may be reported read-only, but arbitrary keys and values
// are never persisted.
func ValidateDeployedMap(deployed map[Component]string) error {
	if len(deployed) > MaxHeartbeatComponents {
		return errors.New("heartbeat_deployed_count_exceeded")
	}
	for component, releaseID := range deployed {
		if len(component) == 0 || len(component) > MaxHeartbeatComponentLen || !KnownComponent(component) {
			return errors.New("heartbeat_unknown_component")
		}
		if len(releaseID) == 0 || len(releaseID) > MaxReleaseIDBytes || !releaseIDPattern.MatchString(releaseID) {
			return errors.New("heartbeat_invalid_release_id")
		}
	}
	return nil
}

// Release describes an uploaded artifact for a component.
type Release struct {
	ID         string    `json:"id"`
	Component  Component `json:"component"`
	Version    string    `json:"version"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
	Notes      string    `json:"notes"`
	UploadedAt time.Time `json:"uploaded_at"`
	UploadedBy string    `json:"uploaded_by"`
	// JobAdmission is server-derived from the Job image archive, or a legacy signed declaration.
	// Other components must never carry it.
	JobAdmission json.RawMessage `json:"job_admission,omitempty"`
}

// Agent describes a registered deployment agent (one per production host).
type Agent struct {
	ID           string               `json:"id"`
	Hostname     string               `json:"hostname"`
	IP           string               `json:"ip"`
	Token        string               `json:"token"`
	RegisteredAt time.Time            `json:"registered_at"`
	LastSeenAt   time.Time            `json:"last_seen_at"`
	Deployed     map[Component]string `json:"deployed"`
}

// Command is an admin-issued instruction for an agent to execute.
type Command struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agent_id"`
	Kind       string     `json:"kind"`
	Component  Component  `json:"component"`
	ReleaseID  string     `json:"release_id,omitempty"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Log        string     `json:"log,omitempty"`
	Error      string     `json:"error,omitempty"`
	IssuedBy   string     `json:"issued_by"`
	LogLines   int        `json:"log_lines,omitempty"`
	LogSince   int        `json:"log_since_seconds,omitempty"`
}

// Command kinds and statuses.
const (
	CmdUpgrade  = "upgrade"
	CmdRollback = "rollback"
	CmdRestart  = "restart"
	CmdLogs     = "logs"

	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"
)

// Service log targets are fixed server-side so a log query can never become
// an arbitrary container lookup or remote shell command.
var serviceLogTargets = []string{"frontend", "admin", "agent"}

func ServiceLogTargets() []string {
	return append([]string(nil), serviceLogTargets...)
}

func ValidServiceLogTarget(target string) bool {
	for _, allowed := range serviceLogTargets {
		if target == allowed {
			return true
		}
	}
	return false
}

// EnrollToken is a one-time token for onboarding an agent.
type EnrollToken struct {
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

// LogEntry is a rotated command record persisted in logs/history-YYYYMM.jsonl.
type LogEntry struct {
	Command  Command   `json:"command"`
	Rotated  time.Time `json:"rotated_at"`
	AgentTag string    `json:"agent_tag,omitempty"`
}

// APIResponse is the wire format for all JSON API responses.
type APIResponse struct {
	Status  bool        `json:"status"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Manifest is the response payload of GET /agents/:id/manifest.
type Manifest struct {
	Backend  *Release `json:"backend,omitempty"`
	Frontend *Release `json:"frontend,omitempty"`
	Admin    *Release `json:"admin,omitempty"`
	TGBot    *Release `json:"tgbot,omitempty"`
	Lang     *Release `json:"lang,omitempty"`
	Job      *Release `json:"job,omitempty"`
}

// Set assigns rel to the field named by c. Nil rel clears the field.
func (m *Manifest) Set(c Component, rel *Release) {
	switch c {
	case CompBackend:
		m.Backend = rel
	case CompFrontend:
		m.Frontend = rel
	case CompAdmin:
		m.Admin = rel
	case CompTGBot:
		m.TGBot = rel
	case CompLang:
		m.Lang = rel
	case CompJob:
		m.Job = rel
	}
}

// Get returns the release for c (may be nil).
func (m *Manifest) Get(c Component) *Release {
	switch c {
	case CompBackend:
		return m.Backend
	case CompFrontend:
		return m.Frontend
	case CompAdmin:
		return m.Admin
	case CompTGBot:
		return m.TGBot
	case CompLang:
		return m.Lang
	case CompJob:
		return m.Job
	}
	return nil
}
