package chainevent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/constant"
)

const FairLaunchMainnetRuntimeUnavailable = "fair_launch_mainnet_runtime_unavailable"

type FairLaunchMainnetRuntimeManifest struct {
	Version                  uint32 `json:"version"`
	ActivationID             string `json:"activationId"`
	Component                string `json:"component"`
	GitCommit                string `json:"gitCommit"`
	ArtifactSHA256           string `json:"artifactSha256"`
	ComponentSetDigest       string `json:"componentSetDigest"`
	ReleaseManifestDigest    string `json:"releaseManifestDigest"`
	MigrationManifestDigest  string `json:"migrationManifestDigest"`
	ActivationManifestDigest string `json:"activationManifestDigest"`
	ProxyAddress             string `json:"proxyAddress"`
	ProxyRuntimeCodeHash     string `json:"proxyRuntimeCodeHash"`
	ImplementationAddress    string `json:"implementationAddress"`
	ImplementationCodeHash   string `json:"implementationCodeHash"`
	ABIDigest                string `json:"abiDigest"`
	SourceID                 uint64 `json:"sourceId"`
	SourceRevisionID         uint64 `json:"sourceRevisionId"`
	ProviderIdentity         string `json:"providerIdentity"`
}

type FairLaunchMainnetRuntimeStatus struct {
	Enabled          bool                           `json:"enabled"`
	Lifecycle        string                         `json:"lifecycle"`
	ReasonCode       string                         `json:"reason_code,omitempty"`
	ActivationID     string                         `json:"activation_id,omitempty"`
	ProviderIdentity string                         `json:"provider_identity,omitempty"`
	ManifestSHA256   string                         `json:"manifest_sha256,omitempty"`
	StartedAt        *time.Time                     `json:"started_at,omitempty"`
	StoppedAt        *time.Time                     `json:"stopped_at,omitempty"`
	Scanner          FairLaunchMainnetScannerStatus `json:"scanner"`
}

type fairLaunchMainnetRuntime struct {
	manifestDigest string
	manifest       FairLaunchMainnetRuntimeManifest
	rpc            *gethrpc.Client
	scanner        *FairLaunchMainnetScanner
	startedAt      time.Time
}

var fairLaunchMainnetRuntimeState = struct {
	sync.Mutex
	runtime   *fairLaunchMainnetRuntime
	enabled   bool
	lifecycle string
	reason    string
	stoppedAt *time.Time
}{lifecycle: "disabled"}

type fairLaunchMainnetRPCCallContext interface {
	CallContext(context.Context, interface{}, string, ...interface{}) error
}

type fairLaunchMainnetGethCaller struct {
	client fairLaunchMainnetRPCCallContext
}

var fairLaunchMainnetRPCDial = func(ctx context.Context, endpoint string) (*gethrpc.Client, error) {
	return chainrpc.DialRPC(ctx, endpoint)
}

func (caller fairLaunchMainnetGethCaller) Call(ctx context.Context, method string, params any, target any) error {
	if caller.client == nil || ctx == nil || target == nil || method == "" {
		return errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	arguments, ok := params.([]any)
	if !ok {
		return errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	return caller.client.CallContext(ctx, target, method, arguments...)
}

func StartFairLaunchMainnetRuntime(parent context.Context, database *gorm.DB) error {
	enabled := viper.GetBool("fairLaunchMainnet.scanner.enabled")
	fairLaunchMainnetRuntimeState.Lock()
	if fairLaunchMainnetRuntimeState.runtime != nil || fairLaunchMainnetRuntimeState.lifecycle == "starting" {
		fairLaunchMainnetRuntimeState.Unlock()
		return errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	fairLaunchMainnetRuntimeState.enabled = enabled
	fairLaunchMainnetRuntimeState.reason = ""
	fairLaunchMainnetRuntimeState.stoppedAt = nil
	if !enabled {
		fairLaunchMainnetRuntimeState.lifecycle = "disabled"
		fairLaunchMainnetRuntimeState.Unlock()
		return nil
	}
	if !viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode") {
		fairLaunchMainnetRuntimeState.lifecycle = "unavailable"
		fairLaunchMainnetRuntimeState.reason = FairLaunchMainnetRuntimeUnavailable
		fairLaunchMainnetRuntimeState.Unlock()
		return errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	// The archive scanner itself cannot materialize or reverse financial
	// effects. Reject direct callers too, not only the production job starter.
	if !constant.FairLaunchMaterializationEnabled {
		fairLaunchMainnetRuntimeState.lifecycle = "unavailable"
		fairLaunchMainnetRuntimeState.reason = constant.FairLaunchMaterializationUnavailable
		fairLaunchMainnetRuntimeState.Unlock()
		return errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	fairLaunchMainnetRuntimeState.lifecycle = "starting"
	fairLaunchMainnetRuntimeState.Unlock()

	runtime, err := buildFairLaunchMainnetRuntime(parent, database, time.Now().UTC())
	if err != nil {
		setFairLaunchMainnetRuntimeFailure()
		return errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	fairLaunchMainnetRuntimeState.Lock()
	fairLaunchMainnetRuntimeState.runtime = runtime
	fairLaunchMainnetRuntimeState.lifecycle = "running"
	fairLaunchMainnetRuntimeState.Unlock()
	return nil
}

func buildFairLaunchMainnetRuntime(parent context.Context, database *gorm.DB, now time.Time) (*fairLaunchMainnetRuntime, error) {
	if parent == nil || database == nil || now.IsZero() {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	manifest, manifestDigest, err := loadFairLaunchMainnetRuntimeManifest()
	if err != nil {
		return nil, err
	}
	leaseToken, err := loadFairLaunchMainnetLeaseToken()
	if err != nil {
		return nil, err
	}
	defer zeroFairLaunchSecret(leaseToken)
	request := manifest.activationRequest(leaseToken)
	var capability FairLaunchMainnetCapability
	err = database.Transaction(func(tx *gorm.DB) error {
		var inner error
		capability, inner = LoadFairLaunchMainnetCapabilityTx(tx, request, now)
		return inner
	})
	zeroFairLaunchSecret(request.LeaseToken)
	if err != nil {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	rpcEndpoint, err := loadFairLaunchMainnetRPCEndpoint()
	if err != nil {
		return nil, err
	}
	defer zeroFairLaunchSecret(rpcEndpoint)
	dialCtx, cancelDial := context.WithTimeout(parent, 10*time.Second)
	client, err := fairLaunchMainnetRPCDial(dialCtx, string(rpcEndpoint))
	cancelDial()
	if err != nil {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	boundRPC, err := NewFairLaunchMainnetRPC(manifest.ProviderIdentity, fairLaunchMainnetGethCaller{client: client})
	if err != nil {
		client.Close()
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	registry, err := BindFairLaunchMainnetSourceRegistry(capability, boundRPC, now)
	if err != nil {
		client.Close()
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	statusCtx, cancelStatus := context.WithTimeout(parent, 45*time.Second)
	archiveStatus, err := LoadFairLaunchMainnetArchiveStatus(statusCtx, registry, database, now)
	cancelStatus()
	if err != nil || archiveStatus.CursorStatus != "active" || !archiveStatus.CursorCanonical || archiveStatus.DeadLetterCount != 0 {
		client.Close()
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	config := fairLaunchMainnetScannerConfigFromViper()
	metrics := NewFairLaunchMainnetMetrics()
	scanner, err := NewFairLaunchMainnetScannerWithMetrics(config, database, []FairLaunchMainnetSourceRegistry{registry}, metrics, now)
	if err != nil {
		client.Close()
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	if err := scanner.Start(parent); err != nil {
		client.Close()
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	return &fairLaunchMainnetRuntime{manifestDigest: manifestDigest, manifest: manifest, rpc: client, scanner: scanner, startedAt: now}, nil
}

func StopFairLaunchMainnetRuntime(ctx context.Context) error {
	if ctx == nil {
		return errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	fairLaunchMainnetRuntimeState.Lock()
	runtime := fairLaunchMainnetRuntimeState.runtime
	if runtime == nil {
		now := time.Now().UTC()
		fairLaunchMainnetRuntimeState.stoppedAt = &now
		fairLaunchMainnetRuntimeState.Unlock()
		return nil
	}
	fairLaunchMainnetRuntimeState.lifecycle = "stopping"
	fairLaunchMainnetRuntimeState.Unlock()
	err := runtime.scanner.Stop(ctx)
	runtime.rpc.Close()
	now := time.Now().UTC()
	fairLaunchMainnetRuntimeState.Lock()
	fairLaunchMainnetRuntimeState.runtime = nil
	fairLaunchMainnetRuntimeState.lifecycle = "stopped"
	fairLaunchMainnetRuntimeState.stoppedAt = &now
	fairLaunchMainnetRuntimeState.Unlock()
	return err
}

func FairLaunchMainnetRuntimeStatusSnapshot() FairLaunchMainnetRuntimeStatus {
	fairLaunchMainnetRuntimeState.Lock()
	runtime := fairLaunchMainnetRuntimeState.runtime
	status := FairLaunchMainnetRuntimeStatus{Enabled: fairLaunchMainnetRuntimeState.enabled, Lifecycle: fairLaunchMainnetRuntimeState.lifecycle, ReasonCode: fairLaunchMainnetRuntimeState.reason, StoppedAt: copyTimePointer(fairLaunchMainnetRuntimeState.stoppedAt)}
	if runtime != nil {
		status.ActivationID = runtime.manifest.ActivationID
		status.ProviderIdentity = runtime.manifest.ProviderIdentity
		status.ManifestSHA256 = runtime.manifestDigest
		status.StartedAt = timePointer(runtime.startedAt)
	}
	fairLaunchMainnetRuntimeState.Unlock()
	if runtime != nil {
		status.Scanner = runtime.scanner.Status()
	}
	return status
}

func setFairLaunchMainnetRuntimeFailure() {
	fairLaunchMainnetRuntimeState.Lock()
	fairLaunchMainnetRuntimeState.runtime = nil
	fairLaunchMainnetRuntimeState.lifecycle = "unavailable"
	fairLaunchMainnetRuntimeState.reason = FairLaunchMainnetRuntimeUnavailable
	fairLaunchMainnetRuntimeState.Unlock()
}

func loadFairLaunchMainnetRuntimeManifest() (FairLaunchMainnetRuntimeManifest, string, error) {
	raw, err := readFairLaunchRuntimeFile(strings.TrimSpace(viper.GetString("fairLaunchMainnet.scanner.admissionManifestPath")), 64*1024, false)
	if err != nil {
		return FairLaunchMainnetRuntimeManifest{}, "", err
	}
	digest := sha256.Sum256(raw)
	digestHex := hex.EncodeToString(digest[:])
	expected := strings.TrimSpace(viper.GetString("fairLaunchMainnet.scanner.admissionManifestSHA256"))
	if !validMainnetNonzeroDigest(expected) || digestHex != expected {
		return FairLaunchMainnetRuntimeManifest{}, "", errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest FairLaunchMainnetRuntimeManifest
	if err := decoder.Decode(&manifest); err != nil {
		return FairLaunchMainnetRuntimeManifest{}, "", errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return FairLaunchMainnetRuntimeManifest{}, "", errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	if manifest.Version != 1 || manifest.Component != "job" || !fairLaunchMainnetProviderIdentityPattern.MatchString(manifest.ProviderIdentity) {
		return FairLaunchMainnetRuntimeManifest{}, "", errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	return manifest, digestHex, nil
}

func loadFairLaunchMainnetLeaseToken() ([]byte, error) {
	value, err := readFairLaunchRuntimeFile(strings.TrimSpace(viper.GetString("fairLaunchMainnet.scanner.leaseTokenFile")), 512, true)
	if err != nil {
		return nil, err
	}
	value = bytes.TrimSuffix(value, []byte("\n"))
	value = bytes.TrimSuffix(value, []byte("\r"))
	if len(value) < 32 || len(value) > 256 || bytes.IndexByte(value, 0) >= 0 || bytes.IndexByte(value, '\n') >= 0 || bytes.IndexByte(value, '\r') >= 0 {
		zeroFairLaunchSecret(value)
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	return value, nil
}

func loadFairLaunchMainnetRPCEndpoint() ([]byte, error) {
	value, err := readFairLaunchRuntimeFile(strings.TrimSpace(viper.GetString("fairLaunchMainnet.scanner.rpcEndpointFile")), 4096, true)
	if err != nil {
		return nil, err
	}
	value = bytes.TrimSpace(value)
	parsed, err := url.Parse(string(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != "https" && parsed.Scheme != "wss" || bytes.IndexAny(value, "\x00\r\n\t ") >= 0 {
		zeroFairLaunchSecret(value)
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	return value, nil
}

func (manifest FairLaunchMainnetRuntimeManifest) activationRequest(leaseToken []byte) FairLaunchMainnetActivationRequest {
	return FairLaunchMainnetActivationRequest{ActivationID: manifest.ActivationID, Component: manifest.Component, GitCommit: manifest.GitCommit, ArtifactSHA256: manifest.ArtifactSHA256, ComponentSetDigest: manifest.ComponentSetDigest, ReleaseManifestDigest: manifest.ReleaseManifestDigest, MigrationManifestDigest: manifest.MigrationManifestDigest, ActivationManifestDigest: manifest.ActivationManifestDigest, ProxyAddress: manifest.ProxyAddress, ProxyRuntimeCodeHash: manifest.ProxyRuntimeCodeHash, ImplementationAddress: manifest.ImplementationAddress, ImplementationCodeHash: manifest.ImplementationCodeHash, ABIDigest: manifest.ABIDigest, SourceID: manifest.SourceID, SourceRevisionID: manifest.SourceRevisionID, LeaseToken: append([]byte(nil), leaseToken...)}
}

func fairLaunchMainnetScannerConfigFromViper() FairLaunchMainnetScannerConfig {
	config := DefaultFairLaunchMainnetScannerConfig()
	config.Enabled = true
	for key, target := range map[string]*time.Duration{
		"fairLaunchMainnet.scanner.pollInterval": &config.PollInterval,
		"fairLaunchMainnet.scanner.retryInitial": &config.RetryInitial,
		"fairLaunchMainnet.scanner.retryMaximum": &config.RetryMaximum,
		"fairLaunchMainnet.scanner.runTimeout":   &config.RunTimeout,
	} {
		if viper.IsSet(key) {
			*target = viper.GetDuration(key)
		}
	}
	if viper.IsSet("fairLaunchMainnet.scanner.maxLagBlocks") {
		config.MaxLagBlocks = viper.GetUint64("fairLaunchMainnet.scanner.maxLagBlocks")
	}
	return config
}

func zeroFairLaunchSecret(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
