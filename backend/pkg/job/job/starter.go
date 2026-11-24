package job

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/job/chainevent"
	"joincare_server/pkg/job/job/idoreconciliation"
)

var crontab = cron.New(cron.WithSeconds())
var immediateScanner sync.WaitGroup
var scannerStop context.CancelFunc

type starter struct{}

// Production needs two separately admitted processes: the ordinary IDO worker
// and the Fair Launch exclusive scanner/materializer. Never silently drop
// either workload because both toggles happened to be set in one process.
func ValidateRuntimeMode(environment, mode, v1Enabled string, scannerEnabled, exclusive bool) error {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "ido", "ido-v3":
		if scannerEnabled || exclusive {
			return fmt.Errorf("ido worker cannot enable Fair Launch scanner or exclusive mode")
		}
		if environment == "prod" && strings.TrimSpace(strings.ToLower(v1Enabled)) != "false" {
			return fmt.Errorf("production IDO worker requires explicit chain.project.v1.enabled=false for V2-only lifecycle")
		}
	case "fair-launch":
		if !scannerEnabled || !exclusive {
			return fmt.Errorf("fair-launch worker requires scanner and exclusive mode")
		}
		if !constant.FairLaunchMaterializationEnabled {
			return fmt.Errorf("fair-launch worker requires reviewed CE-V2 materialization: %s", constant.FairLaunchMaterializationUnavailable)
		}
	default:
		if environment == "prod" || mode != "" {
			return fmt.Errorf("production job requires explicit application.jobMode: ido or fair-launch")
		}
		// Non-production legacy configs retain their existing behavior.
	}
	return nil
}

func (v starter) Chain() {
	ctx, cancel := context.WithCancel(context.Background())
	scannerStop = cancel
	if viper.GetString("application.jobMode") == "ido-v3" {
		// Never register the legacy scanner, reconciliation, auto-end/TGE or
		// transaction recovery jobs in the explicitly V3-only process.
		if approvedV3Source != nil {
			approvedV3Source.Context = ctx
			approvedV3Source.Add(crontab)
			immediateScanner.Add(1)
			go func() { defer immediateScanner.Done(); approvedV3Source.Run() }()
		}
		return
	}
	chainEventJob := &chainevent.GetChainEventJob{}
	chainEventJob.SetRunContext(ctx)
	chainEventJob.Add(crontab)
	chainevent.NewReorgRecoveryScanJob().Add(crontab)
	chainevent.NewReorgMaterializationJob().Add(crontab)
	chainevent.NewReorgAssessmentJob().Add(crontab)
	chainevent.NewReorgPrerequisiteJob().Add(crontab)
	chainevent.NewReorgGenerationJob().Add(crontab)
	chainevent.NewChainTxRecoveryJob().Add(crontab)
	chainevent.NewProjectEndJob().Add(crontab)
	chainevent.NewProjectTgeJob().Add(crontab)
	idoreconciliation.NewWorker().Add(crontab)
	// A separately signed V3 source runs beside (never inside) the legacy/V2
	// event address list. StartAll validated admission before cron registration.
	if v3Source != nil {
		v3Source.Context = ctx
		v3Source.Add(crontab)
		immediateScanner.Add(1)
		go func() {
			defer immediateScanner.Done()
			v3Source.Run()
		}()
	}
	if approvedV3Source != nil {
		approvedV3Source.Context = ctx
		approvedV3Source.Add(crontab)
		immediateScanner.Add(1)
		go func() {
			defer immediateScanner.Done()
			approvedV3Source.Run()
		}()
	}
	// HTTP health 已先启动；立即追块与 cron 共用同一实例锁，避免长追块重入。
	immediateScanner.Add(1)
	go func() {
		defer immediateScanner.Done()
		chainEventJob.Run()
	}()
}

func StopAll() context.Context {
	if scannerStop != nil {
		scannerStop()
	}
	cronStopped := crontab.Stop()
	stopped, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer stopCancel()
		_ = chainevent.StopFairLaunchMainnetRuntime(stopCtx)
		<-cronStopped.Done()
		// cron.Stop only tracks scheduled entries, not Chain's immediate catch-up.
		// Keep DB/RPC open until that scanner returns or the outer shutdown
		// deadline makes the process fail closed.
		immediateScanner.Wait()
	}()
	return stopped
}

var v3Source *chainevent.V3ScannerJob
var approvedV3Source *chainevent.ApprovedV3ScannerJob

func StartAll() error {
	if err := ValidateRuntimeMode(viper.GetString("application.environment"), viper.GetString("application.jobMode"), viper.GetString("chain.project.v1.enabled"),
		viper.GetBool("fairLaunchMainnet.scanner.enabled"), viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode")); err != nil {
		return err
	}
	if err := chainevent.StartFairLaunchMainnetRuntime(context.Background(), db.Mysql()); err != nil {
		return err
	}
	if viper.GetBool("fairLaunchMainnet.scanner.enabled") && viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode") {
		if viper.GetBool("idoV3Scanner.enabled") || viper.GetBool("idoV3Approved.enabled") {
			return fmt.Errorf("V3 scanner cannot run in Fair Launch exclusive mode")
		}
		return nil
	}
	var err error
	if viper.GetBool("idoV3Approved.enabled") {
		v3Source = nil
		approvedV3Source, err = configuredApprovedV3Scanner()
	} else {
		approvedV3Source = nil
		v3Source = nil
		if viper.GetBool("idoV3Scanner.enabled") {
			return fmt.Errorf("legacy per-project V3 scanner release admission retired")
		}
	}
	if err != nil {
		return err
	}
	if viper.GetString("application.jobMode") == "ido-v3" && (approvedV3Source == nil || approvedV3Source.Stream.Mode != "stream") {
		return fmt.Errorf("V3-only Job requires approved stream scanner")
	}
	if viper.GetString("application.jobMode") == "ido-v3" && !idov3whitelist.ProjectFreshnessEnabled() {
		return fmt.Errorf("V3-only Job requires project freshness")
	}
	rt := reflect.ValueOf(starter{})
	for i := 0; i < rt.NumMethod(); i++ {
		rt.Method(i).Call(nil)
	}

	crontab.Start()
	return nil
}

// Only one V3 scanner source may own a deployment. An incomplete schema or
// trust-root configuration stops startup rather than silently starting V2 or
// the old single-project scanner as a fallback.
func configuredApprovedV3Scanner() (*chainevent.ApprovedV3ScannerJob, error) {
	if !viper.GetBool("idoV3Approved.enabled") || viper.GetBool("idoV3Scanner.enabled") ||
		!viper.GetBool("idoV3TokenAdmission.enabled") ||
		!viper.GetBool("idoV3Execution.enabled") ||
		!viper.GetBool("idoV3Lifecycle.enabled") ||
		!viper.GetBool("idoV3PolicyPreview.enabled") ||
		!viper.GetBool("idoV3PolicyReview.enabled") ||
		!viper.GetBool("idoV3PolicyExecution.enabled") ||
		!viper.GetBool("idoV3PolicyProof.enabled") ||
		!viper.GetBool("idoV3PolicyJournal.enabled") || db.Mysql() == nil || !chain.V3ShadowConfigured() ||
		!chain.MatchesV3Shadow(56, 128805696,
			common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e"),
			common.HexToAddress("0x887fcc6e54fc8229919166ad90b16c19b4992560"),
			common.HexToAddress("0xc46659e67543aaaec553f7f3fc8f004b7dd286ef"),
			common.HexToHash("0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8"),
			common.HexToHash("0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590")) {
		return nil, fmt.Errorf("V3 approved scanner configuration unavailable")
	}
	url := strings.TrimSpace(viper.GetString("idoV3TokenAdmission.primaryRPC"))
	owner := strings.TrimSpace(viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash"))
	governor := strings.TrimSpace(viper.GetString("idoV3TokenAdmission.governorSafeCodeHash"))
	if chainevent.ValidateApprovedV3ScannerConfiguration(db.Mysql(), url, owner, governor) != nil ||
		idov3whitelist.ApprovedV3SchemaReady(db.Mysql()) != nil ||
		common.HexToAddress(viper.GetString("chain.project.v2.contractAddress")) == common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e") {
		return nil, fmt.Errorf("V3 approved scanner prerequisites unavailable")
	}
	ownerHash, governorHash, err := idov3whitelist.ParseApprovedV3SafeCodeHashes(owner, governor)
	if err != nil {
		return nil, fmt.Errorf("V3 approved scanner Safe identity unavailable")
	}
	stream, err := chainevent.ConfiguredV3ProxyStream(db.Mysql())
	if err != nil {
		return nil, fmt.Errorf("V3 proxy stream configuration unavailable")
	}
	return &chainevent.ApprovedV3ScannerJob{Database: db.Mysql(), RPCURL: url,
		OwnerHash: ownerHash, GovernorHash: governorHash, Stream: stream}, nil
}

func configuredV3Scanner(ctx context.Context) (*chainevent.V3ScannerJob, error) {
	const prefix = "idoV3Scanner."
	if !viper.GetBool(prefix + "enabled") {
		return nil, nil
	}
	if !chain.V3ShadowConfigured() || db.Mysql() == nil {
		return nil, fmt.Errorf("V3 scanner runtime or database unavailable")
	}
	key, err := base64.StdEncoding.Strict().DecodeString(viper.GetString(prefix + "pinnedPublicKey"))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("V3 scanner pinned verifier unavailable")
	}
	admission, err := chainevent.ParseV3ScanAdmission(ed25519.PublicKey(key), viper.GetString(prefix+"manifest"), viper.GetString(prefix+"signature"))
	if err != nil {
		return nil, err
	}
	if viper.GetString("chain.project.url") == "" {
		return nil, fmt.Errorf("V3 scanner RPC unavailable")
	}
	job := &chainevent.V3ScannerJob{Database: db.Mysql(), RPCURL: viper.GetString("chain.project.url"), Admission: admission, Context: ctx}
	operator := strings.TrimSpace(viper.GetString(prefix + "operatorAddress"))
	if !common.IsHexAddress(operator) || common.HexToAddress(operator) == (common.Address{}) {
		return nil, fmt.Errorf("V3 scanner pinned creation operator unavailable")
	}
	job.Operator = common.HexToAddress(operator)
	// The pre-lock bridge needs the distinct signed Set review, not just the
	// post-creation scan manifest. Keep the verifier release-pinned in Job.
	setKey, keyErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Set.pinnedPublicKey"))
	setSig, sigErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Set.signature"))
	var setManifest idov3whitelist.SetManifest
	decoder := json.NewDecoder(strings.NewReader(viper.GetString("idoV3Set.manifest")))
	decoder.DisallowUnknownFields()
	var extra any
	if keyErr == nil && sigErr == nil && len(setKey) == ed25519.PublicKeySize && len(setSig) == ed25519.SignatureSize &&
		decoder.Decode(&setManifest) == nil && decoder.Decode(&extra) == io.EOF {
		job.SetKey, job.SetManifest, job.SetSignature = ed25519.PublicKey(setKey), setManifest, setSig
	}
	client := chain.GetClient()
	m := admission.Manifest
	if client == nil || !job.DistinctFrom(client.ContractAddress, chain.V2ContractAddress()) ||
		!chain.MatchesV3Shadow(m.ChainID, m.StartBlock, common.HexToAddress(m.ProxyAddress), common.HexToAddress(m.Implementation),
			common.HexToAddress(m.Governor), common.HexToHash(m.ProxyCodeHash), common.HexToHash(m.ImplementationCodeHash)) {
		return nil, fmt.Errorf("V3 scanner runtime identity unavailable or overlaps existing source")
	}
	// Both pre-lock and locked projects require the original immutable Set
	// identity. The lifecycle receipt worker cannot reconcile its original
	// sender or governance from a scanner signature alone. Historical Set
	// verification is identity evidence, never permission for a fresh create.
	reviewTime := time.Time{}
	if job.SetManifest.ExpiresAt > 1 {
		reviewTime = time.Unix(job.SetManifest.ExpiresAt-1, 0)
	}
	if idov3whitelist.VerifySignedSetManifest(job.SetKey, job.SetManifest, job.SetSignature,
		common.HexToHash(job.SetManifest.RequestDigest), reviewTime) != nil ||
		idov3whitelist.CheckCreationSetCommitment(ctx, db.Mysql(), job.SetManifest, admission.Manifest) != nil ||
		common.HexToAddress(job.SetManifest.Operator) != job.Operator {
		return nil, fmt.Errorf("V3 scanner Set continuity unavailable")
	}
	// This is a startup fence only. Each tick rechecks signature, expiration,
	// exact DB join and fixed-block on-chain runtime before reading any event.
	if _, err := idov3whitelist.AdmitSignedProject(ctx, db.Mysql(), admission.PublicKey, admission.Manifest, admission.Signature, time.Now()); err != nil {
		if _, createErr := idov3whitelist.AdmitSignedCreation(ctx, db.Mysql(), admission.PublicKey, admission.Manifest, admission.Signature, time.Now()); createErr != nil {
			return nil, fmt.Errorf("V3 scanner signed admission unavailable")
		}
	}
	return job, nil
}
