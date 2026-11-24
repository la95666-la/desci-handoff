package shared

import (
	"errors"
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	MaxLoginUsernameBytes = 128
	MaxLoginPasswordBytes = 512
	MaxEnrollTokenBytes   = 48
	MaxHostnameBytes      = 253
	MaxAgentIDBytes       = 64
	MaxAgentTokenBytes    = 128
	MaxAgentIPBytes       = 64
	MaxIssuedByBytes      = 128
	MaxServiceLogLines    = 2000
	MaxServiceLogSince    = 7 * 24 * 60 * 60
)

var (
	agentIDPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	agentTokenPattern    = regexp.MustCompile(`^[A-Fa-f0-9]{32,128}$`)
	enrollTokenPattern   = regexp.MustCompile(`^[A-Fa-f0-9]{32,128}$`)
	hostLabelPattern     = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	loginUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{0,127}$`)
)

func ValidateLoginFields(username, password string) error {
	if username == "" || len([]byte(username)) > MaxLoginUsernameBytes || !loginUsernamePattern.MatchString(username) {
		return errors.New("invalid_username")
	}
	if password == "" || len([]byte(password)) > MaxLoginPasswordBytes || strings.ContainsRune(password, '\x00') {
		return errors.New("invalid_password")
	}
	return nil
}

func ValidateEnrollToken(token string) error {
	if len([]byte(token)) > MaxEnrollTokenBytes || !enrollTokenPattern.MatchString(token) {
		return errors.New("invalid_enroll_token")
	}
	return nil
}

func ValidateHostname(hostname string) error {
	if hostname == "" || len([]byte(hostname)) > MaxHostnameBytes || strings.TrimSpace(hostname) != hostname || strings.HasSuffix(hostname, ".") {
		return errors.New("invalid_hostname")
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return nil
	}
	for _, label := range strings.Split(hostname, ".") {
		if !hostLabelPattern.MatchString(label) {
			return errors.New("invalid_hostname")
		}
	}
	return nil
}

func ValidateAgentRegistration(agent *Agent) error {
	if agent == nil || len([]byte(agent.ID)) > MaxAgentIDBytes || !agentIDPattern.MatchString(agent.ID) {
		return errors.New("invalid_agent_id")
	}
	if err := ValidateHostname(agent.Hostname); err != nil {
		return err
	}
	if len([]byte(agent.Token)) > MaxAgentTokenBytes || !agentTokenPattern.MatchString(agent.Token) {
		return errors.New("invalid_agent_token")
	}
	if agent.IP == "" || len([]byte(agent.IP)) > MaxAgentIPBytes || net.ParseIP(agent.IP) == nil {
		return errors.New("invalid_agent_ip")
	}
	if agent.RegisteredAt.IsZero() || agent.LastSeenAt.IsZero() || agent.LastSeenAt.Before(agent.RegisteredAt.Add(-time.Second)) {
		return errors.New("invalid_agent_timestamps")
	}
	return ValidateDeployedMap(agent.Deployed)
}

func ValidateServiceLogCommand(cmd *Command) error {
	if cmd == nil || cmd.Kind != CmdLogs {
		return errors.New("invalid_service_log_kind")
	}
	if !ValidServiceLogTarget(string(cmd.Component)) {
		return errors.New("invalid_service_log_target")
	}
	if cmd.ReleaseID != "" {
		return errors.New("service_log_release_id_forbidden")
	}
	if cmd.LogLines < 1 || cmd.LogLines > MaxServiceLogLines {
		return errors.New("invalid_service_log_lines")
	}
	if cmd.LogSince < 1 || cmd.LogSince > MaxServiceLogSince {
		return errors.New("invalid_service_log_since")
	}
	if cmd.ID == "" || len([]byte(cmd.ID)) > MaxAgentIDBytes || !agentIDPattern.MatchString(cmd.ID) {
		return errors.New("invalid_service_log_command_id")
	}
	if cmd.AgentID == "" || len([]byte(cmd.AgentID)) > MaxAgentIDBytes || !agentIDPattern.MatchString(cmd.AgentID) {
		return errors.New("invalid_service_log_agent_id")
	}
	if cmd.IssuedBy == "" || len([]byte(cmd.IssuedBy)) > MaxIssuedByBytes {
		return errors.New("invalid_service_log_issuer")
	}
	if cmd.CreatedAt.IsZero() || cmd.Status != StatusPending || cmd.StartedAt != nil || cmd.FinishedAt != nil || cmd.Log != "" || cmd.Error != "" {
		return errors.New("invalid_service_log_initial_state")
	}
	return nil
}
