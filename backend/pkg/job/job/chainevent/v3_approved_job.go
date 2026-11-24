package chainevent

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/model"
)

// ApprovedV3ScannerJob is separate from the legacy signed single-project
// source. It holds no signing key and enumerates all bindings afresh per tick.
// Startup must refuse to register both V3 sources at once.
type ApprovedV3ScannerJob struct {
	Database                *gorm.DB
	RPCURL                  string
	OwnerHash, GovernorHash common.Hash
	Context                 context.Context
	Stream                  V3ProxyStreamConfig
	mu                      sync.Mutex
	after                   uint64 // fair next-tick priority; never a persisted cursor
	producerBudget          v3ProducerBudget
	producerMu              sync.Mutex
	retries                 v3RetryState
	audits                  v3PeriodicAudit
	priority                v3PrioritySchedule
	freshness               v3FreshnessSession
	senderMaintenance       v3SenderMaintenanceState
	receiptsNext            time.Time // protected by mu, shared with project writer
}

func (j *ApprovedV3ScannerJob) Add(crontab *cron.Cron) {
	// Attempt catch-up every five seconds. A tick still runs for at most 50s;
	// skip overlapping triggers instead of running two writers for one cursor.
	crontab.AddJob("*/5 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
	crontab.AddJob("*/5 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(cron.FuncJob(j.runSenderMaintenance)))
	crontab.AddJob("*/5 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(cron.FuncJob(j.runPriorityReceipts)))
	if j.Stream.ExternalProducer && j.Stream.Mode != "off" && j.Stream.Mode != "" {
		crontab.AddJob("*/5 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(cron.FuncJob(j.runProxyProducer)))
	}
}

var _ model.Add = (*ApprovedV3ScannerJob)(nil)

func (j *ApprovedV3ScannerJob) Run() {
	if j == nil || !j.mu.TryLock() {
		return
	}
	defer j.mu.Unlock()
	parent := j.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 50*time.Second)
	defer cancel()
	j.runPriorityReceiptsLocked(ctx)
	if err := j.run(ctx); err != nil {
		// Never print DB diagnostics, provider URL or its credential.
		log.GetLogger().Warn("V3 approved multi-project scanner tick unavailable")
	}
}

func (j *ApprovedV3ScannerJob) run(ctx context.Context) error {
	invalid := errors.New("V3 approved scanner unavailable")
	if j == nil || ctx == nil || j.Database == nil ||
		idov3whitelist.ValidateApprovedV3RPCEndpoint(j.RPCURL) != nil ||
		j.OwnerHash == (common.Hash{}) || j.GovernorHash == (common.Hash{}) {
		return invalid
	}
	if j.Stream.Mode != "stream" {
		j.audits.clear()
		j.freshness = v3FreshnessSession{}
	}
	var err error
	if j.Stream.Mode == "stream" {
		ctx, err = j.prepareProjectFreshness(ctx)
		if err != nil {
			return err
		}
	}
	// Close the previous full-tick proof window before attempting to reach the
	// provider. An RPC outage must not leave yesterday's healthy marker usable
	// for another 90 seconds while the next tick has already started.
	generation, err := idov3whitelist.BeginApprovedV3Scan(ctx, j.Database)
	if err != nil {
		cleanup, cancel := freshnessCleanupContext(ctx)
		defer cancel()
		_ = markV3FreshSource(cleanup, j.Database, productionV3StreamScope(), v3SharedConfirmed{height: 1}, 1, false)
		return invalid
	}
	client, err := chainrpc.DialScanner(ctx, j.RPCURL)
	if err != nil {
		cleanup, cancel := freshnessCleanupContext(ctx)
		defer cancel()
		_ = markV3FreshSource(cleanup, j.Database, productionV3StreamScope(), v3SharedConfirmed{height: 1}, 1, false)
		return invalid
	}
	defer client.Close()
	started := time.Now()
	ctx = context.WithValue(ctx, v3RetryKey{}, &j.retries)
	if j.Stream.Mode == "stream" {
		ctx = context.WithValue(ctx, v3PriorityKey{}, &j.priority)
		// RPC endpoint/DB ownership changes invalidate cached historical
		// evidence without exposing credentials to a log or cache key string.
		source := sha256.Sum256([]byte(fmt.Sprintf("%p\x00%s", j.Database, j.RPCURL)))
		ctx = context.WithValue(ctx, v3AuditContextKey{}, &v3AuditTurn{cache: &j.audits, source: source})
	}
	var result approvedV3ScanResult
	var scanErr error
	if j.Stream.Mode != "" && j.Stream.Mode != "off" {
		result, scanErr = runV3ProxyStreamWithBudget(ctx, j.Database, client, j.Stream, j.OwnerHash, j.GovernorHash, &j.after, &j.producerBudget)
	} else {
		result, scanErr = scanApprovedV3Fair(ctx, j.Database, client, j.OwnerHash, j.GovernorHash, &j.after)
	}
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(map[string]interface{}{
			"attempted": result.Attempted, "completed": result.Completed,
			"failed": result.Failed, "total": result.Total,
			"not_started": result.Skipped, "deferred": result.Deferred,
			"elapsed_ms":         time.Since(started).Milliseconds(),
			"deadline":           errors.Is(ctx.Err(), context.DeadlineExceeded),
			"shared_first_reads": result.SharedReads,
			"source_first_reads": result.SourceReads,
		}).Info("V3 approved scanner tick coverage")
	}
	if scanErr != nil {
		return invalid
	}
	if idov3whitelist.FinishApprovedV3Scan(ctx, j.Database, generation) != nil {
		return invalid
	}
	return nil
}

// ValidateApprovedV3ScannerConfiguration performs an isolated deployment
// identity check before registering a scheduled job. The commercial URL is
// never returned to logs, errors or a caller-visible endpoint.
func ValidateApprovedV3ScannerConfiguration(database *gorm.DB, url, ownerHash, governorHash string) error {
	if database == nil || idov3whitelist.ValidateApprovedV3RPCEndpoint(url) != nil ||
		strings.TrimSpace(url) != url {
		return errors.New("V3 approved scanner configuration unavailable")
	}
	if _, _, err := idov3whitelist.ParseApprovedV3SafeCodeHashes(ownerHash, governorHash); err != nil {
		return errors.New("V3 approved scanner Safe identity unavailable")
	}
	return nil
}
