// Command jc-agent runs on production servers, receives commands from jc-release-server,
// and executes upgrade / rollback lifecycles.
//
// Usage:
//
//	jc-agent -config /etc/jc-agent/agent.yaml daemon
//	jc-agent -config /etc/jc-agent/agent.yaml check
//	jc-agent -config /etc/jc-agent/agent.yaml status
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/joincare/jc-release/internal/agent"
	"github.com/joincare/jc-release/internal/strategies"
)

var configPath string

func main() {
	flag.StringVar(&configPath, "config", "/etc/jc-agent/agent.yaml", "path to agent yaml config")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	sub := args[0]

	cfg, err := agent.LoadConfig(configPath)
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		logger.Error("mkdir data_dir failed", "err", err)
		os.Exit(1)
	}

	store := agent.NewStore(cfg.DataDir)
	state, err := store.Load()
	if err != nil {
		logger.Error("load state failed", "err", err)
		os.Exit(1)
	}

	client, err := agent.NewClient(cfg.ServerURL, state.AgentToken)
	if err != nil {
		logger.Error("invalid agent server URL", "err", err)
		os.Exit(1)
	}

	// If not registered yet, do the one-shot enroll → register handshake.
	if state.AgentID == "" {
		if cfg.EnrollToken == "" {
			logger.Error("agent not registered and no enroll_token in config — set enroll_token in yaml and restart")
			os.Exit(1)
		}
		if err := doRegister(cfg, client, store, logger); err != nil {
			logger.Error("register failed", "err", err)
			os.Exit(1)
		}
		// reload state
		state, _ = store.Load()
		client.SetToken(state.AgentToken)
	}

	up := &agent.Upgrader{
		Client: client, Store: store, Registry: strategies.New(cfg.Root),
		DataDir: cfg.DataDir, AgentID: state.AgentID, Logger: logger,
		Root: cfg.Root, JobEnvironment: cfg.JobEnvironment, JobTrustedKeys: cfg.JobTrustedKeys,
		JobReadinessFile: cfg.JobReadinessFile, JobReadinessKeys: cfg.JobReadinessKeys, JobHostKeySHA256: cfg.JobHostKeySHA256,
		JobEvidenceDir: cfg.JobEvidenceDir,
	}

	switch sub {
	case "daemon":
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		p := &agent.Poller{Cfg: cfg, Client: client, Store: store, Upgrader: up, Logger: logger}
		p.Run(ctx, state.AgentID)
	case "check":
		cmdCheck(client, state.AgentID, logger)
	case "status":
		cmdStatus(state, logger)
	case "finalize-failed":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: jc-agent -config <path> finalize-failed <command-id>")
			os.Exit(2)
		}
		cmdFinalizeFailed(client, state.AgentID, args[1], logger)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: jc-agent -config <path> <subcommand>

subcommands:
  daemon   run polling loop (systemd)
  check    print server manifest vs local deployed versions
  status   print local agent identity + state
  finalize-failed <command-id>
           finalize one stranded in-progress command as failed; performs no deployment I/O
`)
}

var commandIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func cmdFinalizeFailed(client *agent.Client, agentID, commandID string, logger *slog.Logger) {
	if !commandIDPattern.MatchString(commandID) {
		logger.Error("invalid command id")
		os.Exit(2)
	}
	const message = "command execution completed but its terminal report was not accepted; manually finalized as failed"
	if err := client.ReportCommand(agentID, commandID, "failed", message+"\n", message); err != nil {
		logger.Error("finalize failed command", "err", err)
		os.Exit(1)
	}
	logger.Info("command finalized as failed", "cmd_id", commandID)
}

func doRegister(cfg *agent.Config, client *agent.Client, store *agent.Store, logger *slog.Logger) error {
	hostname, _ := os.Hostname()
	resp, err := client.Register(cfg.EnrollToken, hostname)
	if err != nil {
		return err
	}
	logger.Info("registered", "agent_id", resp.AgentID, "hostname", hostname)
	if err := store.Update(func(s *agent.State) error {
		s.AgentID = resp.AgentID
		s.AgentToken = resp.AgentToken
		return nil
	}); err != nil {
		return err
	}
	// Clear enroll_token from yaml to prevent reuse
	if err := agent.ClearEnrollToken(configPath, cfg); err != nil {
		logger.Warn("could not clear enroll_token from config", "err", err)
	}
	return nil
}

func cmdCheck(client *agent.Client, agentID string, logger *slog.Logger) {
	m, err := client.GetManifest(agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "manifest fetch failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("组件      服务端最新版本                                   大小")
	fmt.Println("------    ------------------------------------------------ ----------")
	print1 := func(name string, r interface{}) {
		if r == nil {
			fmt.Printf("%-8s  (none)\n", name)
			return
		}
		// type assert to *shared.Release
		type rel = struct {
			Version string
			Size    int64
		}
	}
	_ = print1
	// simpler: reflect on Manifest
	printRel := func(name string, comp string) {
		var v string
		var sz int64
		switch comp {
		case "backend":
			if m.Backend != nil {
				v = m.Backend.Version
				sz = m.Backend.Size
			}
		case "frontend":
			if m.Frontend != nil {
				v = m.Frontend.Version
				sz = m.Frontend.Size
			}
		case "admin":
			if m.Admin != nil {
				v = m.Admin.Version
				sz = m.Admin.Size
			}
		case "tgbot":
			if m.TGBot != nil {
				v = m.TGBot.Version
				sz = m.TGBot.Size
			}
		case "lang":
			if m.Lang != nil {
				v = m.Lang.Version
				sz = m.Lang.Size
			}
		}
		if v == "" {
			fmt.Printf("%-8s  (none)\n", name)
			return
		}
		fmt.Printf("%-8s  %-48s %s\n", name, v, humanBytes(sz))
	}
	for _, c := range []string{"backend", "frontend", "admin", "tgbot", "lang"} {
		printRel(c, c)
	}
}

func cmdStatus(state *agent.State, logger *slog.Logger) {
	fmt.Println("agent_id:    ", state.AgentID)
	fmt.Println("deployed:")
	if len(state.Deployed) == 0 {
		fmt.Println("  (empty)")
		return
	}
	for c, e := range state.Deployed {
		fmt.Printf("  %-8s  release=%s  version=%s  applied=%s\n",
			c, e.ReleaseID[:min(8, len(e.ReleaseID))], e.Version, e.AppliedAt.Format("2006-01-02 15:04:05"))
	}
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(n)/1024/1024)
	}
	return fmt.Sprintf("%.2fGB", float64(n)/1024/1024/1024)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
