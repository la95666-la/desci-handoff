package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

// Poller is the daemon loop: heartbeat + poll manifest + poll commands + execute.
type Poller struct {
	Cfg      *Config
	Client   *Client
	Store    *Store
	Upgrader *Upgrader
	Logger   *slog.Logger

	agentID        string
	latestManifest *shared.Manifest
	commandTicker  *time.Ticker
	manifestTicker *time.Ticker
}

// Run blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context, agentID string) {
	p.agentID = agentID
	p.commandTicker = time.NewTicker(p.Cfg.PollCommandInterval)
	p.manifestTicker = time.NewTicker(p.Cfg.PollManifestInterval)
	defer p.commandTicker.Stop()
	defer p.manifestTicker.Stop()

	// initial ticks
	p.tickManifest(ctx)
	p.tickCommands(ctx)

	p.Logger.Info("poller running",
		"agent_id", agentID,
		"cmd_interval", p.Cfg.PollCommandInterval,
		"manifest_interval", p.Cfg.PollManifestInterval)

	for {
		select {
		case <-ctx.Done():
			p.Logger.Info("poller shutting down")
			return
		case <-p.commandTicker.C:
			p.tickCommands(ctx)
		case <-p.manifestTicker.C:
			p.tickManifest(ctx)
		}
	}
}

func (p *Poller) tickManifest(ctx context.Context) {
	m, err := p.Client.GetManifest(p.agentID)
	if err != nil {
		p.Logger.Warn("manifest fetch failed", "err", err)
		return
	}
	p.latestManifest = m
	// Also heartbeat with current deployed
	st, err := p.Store.Load()
	if err != nil {
		return
	}
	deployed := map[shared.Component]string{}
	for c, e := range st.Deployed {
		deployed[c] = e.ReleaseID
	}
	if err := p.Client.Heartbeat(p.agentID, deployed); err != nil {
		p.Logger.Warn("heartbeat failed", "err", err)
	}
}

func (p *Poller) tickCommands(ctx context.Context) {
	cmds, err := p.Client.PollCommands(p.agentID)
	if err != nil {
		p.Logger.Warn("command poll failed", "err", err)
		return
	}
	for _, cmd := range cmds {
		p.Logger.Info("executing command", "cmd_id", cmd.ID, "kind", cmd.Kind, "component", cmd.Component)
		var rel *shared.Release
		if cmd.Kind == shared.CmdUpgrade && cmd.ReleaseID != "" {
			// Try cache first, else refresh manifest
			if p.latestManifest != nil {
				rel = p.Upgrader.LookupReleaseForComponent(cmd.ReleaseID, cmd.Component, p.latestManifest)
			}
			if rel == nil {
				// Refresh once to see if a newer version is in manifest
				if m, err := p.Client.GetManifest(p.agentID); err == nil {
					p.latestManifest = m
					rel = p.Upgrader.LookupReleaseForComponent(cmd.ReleaseID, cmd.Component, m)
				}
			}
			if rel == nil {
				if exact, err := p.Client.GetRelease(p.agentID, cmd.ReleaseID); err == nil && exact.Component == cmd.Component {
					rel = exact
				}
			}
		}

		status, logStr, errMsg := p.Upgrader.ExecuteCommand(ctx, cmd, rel)
		logStr, errMsg = shared.CapCommandReport(logStr, errMsg)

		if err := p.reportCommand(ctx, cmd.ID, status, logStr, errMsg); err != nil {
			p.Logger.Error("report command failed", "cmd_id", cmd.ID, "err", err)
		} else {
			p.Logger.Info("command finished", "cmd_id", cmd.ID, "status", status)
		}
	}
}

func (p *Poller) reportCommand(ctx context.Context, cmdID, status, logStr, errMsg string) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := p.Client.ReportCommand(p.agentID, cmdID, status, logStr, errMsg); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}
	return fmt.Errorf("report failed after 3 attempts: %w", lastErr)
}
