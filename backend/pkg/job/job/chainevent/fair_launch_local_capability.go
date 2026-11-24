package chainevent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchLocalE2ECapabilityRequired            = "fair_launch_local_e2e_capability_required"
	FairLaunchLocalE2ETransportAdmissionUnavailable = "fair_launch_local_e2e_transport_admission_unavailable"
	FairLaunchLocalE2EAdmissionRejected             = "fair_launch_local_e2e_admission_rejected"
	FairLaunchLocalE2EForwardUnavailable            = "fair_launch_local_e2e_forward_materialization_unavailable"
)

const (
	fairLaunchLocalAuthority        = "TEST_ONLY_NOT_PRODUCTION_ADMITTED"
	fairLaunchLocalBundleSchema     = "fair-launch-local-e2e-backend-admission-bundle-v1"
	fairLaunchLocalBootstrapSchema  = "fair-launch-local-e2e-backend-bootstrap-v1"
	fairLaunchLocalRuntimeSchema    = "fair-launch-local-e2e-contract-runtime-admission-v1"
	fairLaunchLocalManifestSchema   = "fair-launch-local-e2e-deployment-v1"
	fairLaunchLocalTrustSchema      = "fair-launch-local-e2e-public-trust-root-v1"
	fairLaunchLocalPinSchema        = "fair-launch-local-e2e-external-trust-pin-v1"
	fairLaunchLocalMaxBundleBytes   = 512 << 10
	fairLaunchLocalMaxLifetime      = 24 * time.Hour
	fairLaunchLocalContractVersion  = "fair_launch_v1"
	fairLaunchLocalFundingMode      = "fair_launch"
	fairLaunchLocalExpectedChainID  = uint64(31337)
	fairLaunchLocalExpectedDatabase = "jc_fl_e2e_"
	// This is the exact information_schema.COLUMNS digest visible to the
	// accepted table-scoped local_job role for Deploy schema generation 12.
	// The full root-visible schema digest remains independently bound by the
	// immutable Deploy/bootstrap authorities and persisted run row.
	fairLaunchLocalJobSchemaDigest = "252885e48b32c26c91bd4b3c5215b15521b4ca1210c87f49611d388d1f3277b2"
)

var (
	fairLaunchLocalRunPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	fairLaunchLocalAddressPattern  = regexp.MustCompile(`^0x[0-9a-f]{40}$`)
	fairLaunchLocalHashPattern     = regexp.MustCompile(`^0x[0-9a-f]{64}$`)
	fairLaunchLocalHexBytesPattern = regexp.MustCompile(`^0x(?:[0-9a-f]{2})*$`)
	fairLaunchLocalDigestPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	fairLaunchLocalKeyIDPattern    = regexp.MustCompile(`^local-e2e-[0-9a-f]{24}$`)
)

// FairLaunchLocalE2EAdmissionBundle is accepted only on stdin by the dedicated
// LOCAL-ONLY command. Secret-bearing MySQL profile data must never be supplied
// through argv, ordinary environment variables, logs, evidence, or HTTP.
type FairLaunchLocalE2EAdmissionBundle struct {
	SchemaVersion            string `json:"schemaVersion"`
	RunID                    string `json:"runId"`
	ContractRuntimeAdmission string `json:"contractRuntimeAdmission"`
	DeploymentManifest       string `json:"deploymentManifest"`
	ExternalTrustPin         string `json:"externalTrustPin"`
	PublicTrustRoot          string `json:"publicTrustRoot"`
	BackendBootstrap         string `json:"backendBootstrap"`
	DatabaseSchemaEvidence   string `json:"databaseSchemaEvidence"`
	SchemaReadiness          string `json:"schemaReadiness"`
	InfrastructureEvidence   string `json:"infrastructureEvidence"`
	ContainerAuthority       string `json:"containerAuthority"`
	ExpectedJobGrants        string `json:"expectedJobGrants"`
	ActualJobGrants          string `json:"actualJobGrants"`
	ExpectedIngestGrants     string `json:"expectedIngestGrants"`
	ActualIngestGrants       string `json:"actualIngestGrants"`
	MySQLJobProfile          string `json:"mysqlJobProfile"`
	MySQLIngestProfile       string `json:"mysqlIngestProfile"`
}

type FairLaunchLocalE2EAdmission struct {
	Authority              string    `json:"authority"`
	RunID                  string    `json:"runId"`
	RunRowID               uint64    `json:"runRowId"`
	ChainID                uint64    `json:"chainId"`
	RPCURL                 string    `json:"rpcUrl"`
	GenesisHash            string    `json:"genesisHash"`
	DatabaseName           string    `json:"database"`
	MySQLServerUUID        string    `json:"mysqlServerUuid"`
	SchemaDigest           string    `json:"schemaDigest"`
	GrantPolicyDigest      string    `json:"grantPolicyDigest"`
	ManifestSHA256         string    `json:"manifestSha256"`
	RuntimeAdmissionSHA256 string    `json:"runtimeAdmissionSha256"`
	SourceRegistryDigest   string    `json:"sourceRegistryDigest"`
	SourceID               uint64    `json:"sourceId"`
	SourceRevisionID       uint64    `json:"sourceRevisionId"`
	SourceBindingID        uint64    `json:"sourceBindingId"`
	SignedTipBlock         uint64    `json:"signedTipBlock"`
	SignedTipHash          string    `json:"signedTipHash"`
	ExpiresAt              time.Time `json:"expiresAt"`
	Status                 string    `json:"status"`
}

// FairLaunchLocalE2ECapability is opaque outside this package. It is minted
// only after exact Contract, Deploy, MySQL identity, schema, privilege, source,
// project and token bindings have all been re-verified.
type FairLaunchLocalE2ECapability struct {
	seal *fairLaunchLocalE2ESeal
}

type fairLaunchLocalE2ESeal struct {
	admission       FairLaunchLocalE2EAdmission
	lease           string
	fence           uint64
	rpc             fairLaunchLocalRPCCaller
	contractAddress string
	manifest        fairLaunchManifestPayload
}

func (capability FairLaunchLocalE2ECapability) valid(now time.Time) bool {
	return capability.seal != nil && !now.IsZero() && now.Before(capability.seal.admission.ExpiresAt)
}

func (capability FairLaunchLocalE2ECapability) Admission() (FairLaunchLocalE2EAdmission, error) {
	if capability.seal == nil {
		return FairLaunchLocalE2EAdmission{}, errors.New(FairLaunchLocalE2ECapabilityRequired)
	}
	return capability.seal.admission, nil
}

// NewFairLaunchLocalE2EWrite fixes every security-sensitive repository field
// from the opaque capability. Callers provide only an already decoded frozen
// event and the archive observation digests.
func NewFairLaunchLocalE2EWrite(capability FairLaunchLocalE2ECapability, event *commonchain.FairLaunchTypedEvent, txLogOrdinal, transactionIndex uint32, providerIdentity, topicsDataDigest, rawLogDigest string) (FairLaunchCEWrite, error) {
	if capability.seal == nil || event == nil || providerIdentity == "" || !fairLaunchLocalDigestPattern.MatchString(stripCE(topicsDataDigest)) || !fairLaunchLocalDigestPattern.MatchString(stripCE(rawLogDigest)) {
		return FairLaunchCEWrite{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	return FairLaunchCEWrite{Event: event, Context: FairLaunchMaterializationContext{ChainID: fairLaunchLocalExpectedChainID, TxLogOrdinal: txLogOrdinal, SourceRevisionID: capability.seal.admission.SourceRevisionID, EffectVersion: FairLaunchEffectManifestVersion, Canonical: true, Finalized: true}, ProviderIdentity: providerIdentity, TopicsDataDigest: stripCE(topicsDataDigest), RawLogDigest: stripCE(rawLogDigest), TransactionIndex: transactionIndex, SourceID: capability.seal.admission.SourceID}, nil
}

func DecodeFairLaunchLocalE2EAdmissionBundle(reader io.Reader) (FairLaunchLocalE2EAdmissionBundle, error) {
	if reader == nil {
		return FairLaunchLocalE2EAdmissionBundle{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	limited := io.LimitReader(reader, fairLaunchLocalMaxBundleBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) == 0 || len(raw) > fairLaunchLocalMaxBundleBytes {
		return FairLaunchLocalE2EAdmissionBundle{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var bundle FairLaunchLocalE2EAdmissionBundle
	if err := jsonv2.Unmarshal(raw, &bundle, jsonv2.RejectUnknownMembers(true)); err != nil {
		return FairLaunchLocalE2EAdmissionBundle{}, fmt.Errorf("%s: bundle JSON invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	canonical, err := jsonv2.Marshal(bundle)
	if err != nil || !bytes.Equal(bytes.TrimSpace(raw), canonical) {
		return FairLaunchLocalE2EAdmissionBundle{}, fmt.Errorf("%s: bundle is not canonical JSON", FairLaunchLocalE2EAdmissionRejected)
	}
	if bundle.SchemaVersion != fairLaunchLocalBundleSchema || !fairLaunchLocalRunPattern.MatchString(bundle.RunID) {
		return FairLaunchLocalE2EAdmissionBundle{}, fmt.Errorf("%s: bundle identity invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	return bundle, nil
}

type fairLaunchRuntimeAuthority struct {
	Authority      string `json:"authority"`
	ChainID        uint64 `json:"chainId"`
	ContractRoot   string `json:"contractStateRoot"`
	CreatedAt      string `json:"createdAt"`
	GenesisHash    string `json:"genesisHash"`
	ManifestPath   string `json:"manifestPath"`
	ManifestSHA256 string `json:"manifestSha256"`
	PayloadSHA256  string `json:"payloadSha256"`
	ProjectID      string `json:"projectId"`
	RPCURL         string `json:"rpcUrl"`
	RunID          string `json:"runId"`
	SchemaVersion  string `json:"schemaVersion"`
	TrustPinPath   string `json:"trustPinPath"`
	TrustPinSHA256 string `json:"trustPinSha256"`
}

type fairLaunchJWK struct {
	CRV string `json:"crv"`
	KTY string `json:"kty"`
	X   string `json:"x"`
}

type fairLaunchTrustRoot struct {
	SchemaVersion          string        `json:"schemaVersion"`
	Authority              string        `json:"authority"`
	RunID                  string        `json:"runId"`
	RPCURL                 string        `json:"rpcUrl"`
	ChainID                uint64        `json:"chainId"`
	GenesisHash            string        `json:"genesisHash"`
	RootAuthoritySHA256    string        `json:"rootAuthoritySha256"`
	ProcessAuthoritySHA256 string        `json:"processAuthoritySha256"`
	KeyID                  string        `json:"keyId"`
	PublicKeyJWK           fairLaunchJWK `json:"publicKeyJwk"`
	CreatedAt              string        `json:"createdAt"`
}

type fairLaunchTrustPin struct {
	SchemaVersion     string `json:"schemaVersion"`
	Authority         string `json:"authority"`
	RunID             string `json:"runId"`
	RPCURL            string `json:"rpcUrl"`
	ChainID           uint64 `json:"chainId"`
	GenesisHash       string `json:"genesisHash"`
	KeyID             string `json:"keyId"`
	TrustRecordSHA256 string `json:"trustRecordSha256"`
	PublicKeySHA256   string `json:"publicKeySha256"`
	TrustCreatedAt    string `json:"trustCreatedAt"`
	CreatedAt         string `json:"createdAt"`
}

type fairLaunchManifestSignature struct {
	Algorithm     string `json:"algorithm"`
	KeyID         string `json:"keyId"`
	PayloadSHA256 string `json:"payloadSha256"`
	Value         string `json:"value"`
}

type fairLaunchEmbeddedTrust struct {
	Authority    string        `json:"authority"`
	KeyID        string        `json:"keyId"`
	PublicKeyJWK fairLaunchJWK `json:"publicKeyJwk"`
}

type fairLaunchCodeIdentity struct {
	Address         string `json:"address"`
	RuntimeCodeHash string `json:"runtimeCodeHash"`
}

type fairLaunchTokenIdentity struct {
	Address         string `json:"address"`
	Decimals        uint8  `json:"decimals"`
	RuntimeCodeHash string `json:"runtimeCodeHash"`
	Symbol          string `json:"symbol"`
	Key             string `json:"key,omitzero"`
}

type fairLaunchReceiptIdentity struct {
	BlockHash        string  `json:"blockHash"`
	BlockNumber      string  `json:"blockNumber"`
	BlockTimestamp   string  `json:"blockTimestamp"`
	ContractAddress  *string `json:"contractAddress"`
	From             string  `json:"from"`
	InputHash        string  `json:"inputHash"`
	To               *string `json:"to"`
	TransactionIndex uint32  `json:"transactionIndex"`
	TxHash           string  `json:"txHash"`
	ValueRaw         string  `json:"valueRaw"`
}

type fairLaunchManifestProject struct {
	Creation                fairLaunchReceiptIdentity `json:"creation"`
	CreationEventLogIndex   uint32                    `json:"creationEventLogIndex"`
	EndTime                 string                    `json:"endTime"`
	InitialStatus           string                    `json:"initialStatus"`
	Key                     string                    `json:"key"`
	MaxContributionRaw      string                    `json:"maxContributionRaw"`
	MinContributionRaw      string                    `json:"minContributionRaw"`
	PaymentToken            string                    `json:"paymentToken"`
	PlatformFeeBPS          uint16                    `json:"platformFeeBps"`
	PlatformFeeReceiver     string                    `json:"platformFeeReceiver"`
	PristineStorage         map[string]any            `json:"pristineStorage"`
	ProjectID               string                    `json:"projectId"`
	ProjectOwner            string                    `json:"projectOwner"`
	ProjectProceedsReceiver string                    `json:"projectProceedsReceiver"`
	ProjectToken            string                    `json:"projectToken"`
	SaleTokenAmountRaw      string                    `json:"saleTokenAmountRaw"`
	SoftCapRaw              string                    `json:"softCapRaw"`
	StartTime               string                    `json:"startTime"`
	StatusCode              uint8                     `json:"statusCode"`
	TerminalReasonCode      uint8                     `json:"terminalReasonCode"`
	TotalTokensDepositedRaw string                    `json:"totalTokensDepositedRaw"`
}

type fairLaunchManifestPayload struct {
	Assets struct {
		PaymentToken  fairLaunchTokenIdentity   `json:"paymentToken"`
		ProjectTokens []fairLaunchTokenIdentity `json:"projectTokens"`
		Faucet        struct {
			NativeGasStrategy string `json:"nativeGasStrategy"`
			StoresPrivateKeys bool   `json:"storesPrivateKeys"`
			Strategy          string `json:"strategy"`
		} `json:"faucet"`
	} `json:"assets"`
	Authority  string                            `json:"authority"`
	Contracts  map[string]fairLaunchCodeIdentity `json:"contracts"`
	CreatedAt  string                            `json:"createdAt"`
	Deployment struct {
		Observation fairLaunchReceiptIdentity `json:"observation"`
		Proxy       fairLaunchReceiptIdentity `json:"proxy"`
	} `json:"deployment"`
	ExpiresAt string `json:"expiresAt"`
	Network   struct {
		ChainID     uint64 `json:"chainId"`
		GenesisHash string `json:"genesisHash"`
		Name        string `json:"name"`
		RPCURL      string `json:"rpcUrl"`
	} `json:"network"`
	Projects []fairLaunchManifestProject `json:"projects"`
	Roles    map[string]string           `json:"roles"`
	RunID    string                      `json:"runId"`
	Safes    struct {
		Owners      []string `json:"owners"`
		PlatformFee struct {
			Address         string `json:"address"`
			RuntimeCodeHash string `json:"runtimeCodeHash"`
			Threshold       uint8  `json:"threshold"`
		} `json:"platformFee"`
		Proceeds struct {
			Address         string `json:"address"`
			RuntimeCodeHash string `json:"runtimeCodeHash"`
			Threshold       uint8  `json:"threshold"`
		} `json:"proceeds"`
	} `json:"safes"`
	SchemaVersion string `json:"schemaVersion"`
	Source        struct {
		AggregateABIDigest  string `json:"aggregateAbiDigest"`
		Commit              string `json:"commit"`
		Dirty               bool   `json:"dirty"`
		FullSetDigest       string `json:"fullSetDigest"`
		SelectorMapDigest   string `json:"selectorMapDigest"`
		SourceTreeDigest    string `json:"sourceTreeDigest"`
		StorageLayoutDigest string `json:"storageLayoutDigest"`
	} `json:"source"`
}

type fairLaunchManifestEnvelope struct {
	Payload       fairLaunchManifestPayload   `json:"payload"`
	Signature     fairLaunchManifestSignature `json:"signature"`
	TestTrustRoot fairLaunchEmbeddedTrust     `json:"testTrustRoot"`
}

type fairLaunchBootstrapEvidence struct {
	Authority              string   `json:"authority"`
	ChainID                uint64   `json:"chainId"`
	CreatedAt              string   `json:"createdAt"`
	Database               string   `json:"database"`
	ExpiresAt              string   `json:"expiresAt"`
	GenesisHash            string   `json:"genesisHash"`
	GrantPolicyDigest      string   `json:"grantPolicyDigest"`
	ManifestSHA256         string   `json:"manifestSha256"`
	ProjectIDs             []uint64 `json:"projectIds"`
	RPCURL                 string   `json:"rpcUrl"`
	RunID                  string   `json:"runId"`
	RunRowID               uint64   `json:"runRowId"`
	RuntimeAdmissionSHA256 string   `json:"runtimeAdmissionSha256"`
	SchemaDigest           string   `json:"schemaDigest"`
	SchemaVersion          string   `json:"schemaVersion"`
	SignedTip              struct {
		BlockHash        string `json:"blockHash"`
		BlockNumber      string `json:"blockNumber"`
		TransactionIndex uint32 `json:"transactionIndex"`
	} `json:"signedTip"`
	SourceBindingID      uint64 `json:"sourceBindingId"`
	SourceID             uint64 `json:"sourceId"`
	SourceRegistryDigest string `json:"sourceRegistryDigest"`
	SourceRevisionID     uint64 `json:"sourceRevisionId"`
	Status               string `json:"status"`
}

type fairLaunchVerifiedArtifacts struct {
	authority      fairLaunchRuntimeAuthority
	manifest       fairLaunchManifestEnvelope
	bootstrap      fairLaunchBootstrapEvidence
	expiresAt      time.Time
	runtimeDigest  string
	manifestDigest string
	databaseName   string
	mysqlUUID      string
	mysqlVersion   string
	schemaDigest   string
	grantDigest    string
	containerID    string
	ownerDigest    string
}

type fairLaunchDatabaseEvidence struct {
	Authority               string `json:"authority"`
	RunID                   string `json:"runId"`
	MySQLVersion            string `json:"mysqlVersion"`
	MySQLServerUUID         string `json:"mysqlServerUuid"`
	ContainerID             string `json:"containerId"`
	MySQLImageID            string `json:"mysqlImageId"`
	Endpoint                string `json:"endpoint"`
	Database                string `json:"database"`
	TransportDigest         string `json:"transportDigest"`
	BaselineDigest          string `json:"baselineDigest"`
	MigrationManifestDigest string `json:"migrationManifestDigest"`
	SchemaDigest            string `json:"schemaDigest"`
	GrantPolicyDigest       string `json:"grantPolicyDigest"`
	OwnerLabelDigest        string `json:"ownerLabelDigest"`
	Status                  string `json:"status"`
}

type fairLaunchSchemaReadinessEvidence struct {
	Authority               string            `json:"authority"`
	RunID                   string            `json:"runId"`
	TransportDigest         string            `json:"transportDigest"`
	MySQLImageID            string            `json:"mysqlImageId"`
	InfrastructureDigest    string            `json:"infrastructureDigest"`
	SchemaEvidenceDigest    string            `json:"schemaEvidenceDigest"`
	ProfileDigests          map[string]string `json:"profileDigests"`
	BaselineDigest          string            `json:"baselineDigest"`
	MigrationManifestDigest string            `json:"migrationManifestDigest"`
	SchemaDigest            string            `json:"schemaDigest"`
	GrantPolicyDigest       string            `json:"grantPolicyDigest"`
	OwnerLabelDigest        string            `json:"ownerLabelDigest"`
	Status                  string            `json:"status"`
	ReadinessDigest         string            `json:"readinessDigest"`
}

type fairLaunchInfrastructureMySQL struct {
	Endpoint              string `json:"endpoint"`
	ContainerID           string `json:"containerId"`
	ContainerPID          string `json:"containerPid"`
	ContainerStartedAt    string `json:"containerStartedAt"`
	ImageRef              string `json:"imageRef"`
	ImageID               string `json:"imageId"`
	NetworkMode           string `json:"networkMode"`
	PublishedPortMappings bool   `json:"publishedPortMappings"`
	Database              string `json:"database"`
	ServerUUID            string `json:"serverUuid"`
	Version               string `json:"version"`
	ReservationDigest     string `json:"reservationDigest"`
	TransportDigest       string `json:"transportDigest"`
}

type fairLaunchInfrastructureEvidence struct {
	Authority string                        `json:"authority"`
	RunID     string                        `json:"runId"`
	Status    string                        `json:"status"`
	MySQL     fairLaunchInfrastructureMySQL `json:"mysql"`
}

type fairLaunchContainerAuthority struct {
	Domain           string         `json:"domain"`
	Version          uint64         `json:"version"`
	RunID            string         `json:"runId"`
	Component        string         `json:"component"`
	RunDir           string         `json:"runDir"`
	RunDevice        uint64         `json:"runDevice"`
	RunInode         uint64         `json:"runInode"`
	RunUID           uint64         `json:"runUid"`
	RunMode          uint32         `json:"runMode"`
	EvidenceDir      string         `json:"evidenceDir"`
	EvidenceDevice   uint64         `json:"evidenceDevice"`
	EvidenceInode    uint64         `json:"evidenceInode"`
	EvidenceUID      uint64         `json:"evidenceUid"`
	EvidenceMode     uint32         `json:"evidenceMode"`
	RecordPath       string         `json:"recordPath"`
	ContainerID      string         `json:"containerId"`
	ProjectionDigest string         `json:"projectionDigest"`
	Projection       map[string]any `json:"projection"`
}

func strictUnmarshalLocal(raw string, target any, label string) error {
	if raw == "" || !strings.HasSuffix(raw, "\n") {
		return fmt.Errorf("%s: %s bytes invalid", FairLaunchLocalE2EAdmissionRejected, label)
	}
	if err := jsonv2.Unmarshal([]byte(raw), target, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("%s: %s JSON invalid", FairLaunchLocalE2EAdmissionRejected, label)
	}
	return nil
}

func canonicalLocalJSON(value any) ([]byte, error) {
	raw, err := jsonv2.Marshal(value)
	if err != nil {
		return nil, err
	}
	canonical := jsontext.Value(raw)
	if err := canonical.Canonicalize(); err != nil {
		return nil, err
	}
	return canonical, nil
}

func localSHA256(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func verifyCanonicalLocalFile(raw string, value any, label string) error {
	canonical := jsontext.Value(append([]byte(nil), strings.TrimSuffix(raw, "\n")...))
	if err := canonical.Canonicalize(); err != nil || raw != string(canonical)+"\n" {
		return fmt.Errorf("%s: %s is not canonical JSON", FairLaunchLocalE2EAdmissionRejected, label)
	}
	return nil
}

func parseFairLaunchLocalTime(value, label string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %s timestamp invalid", FairLaunchLocalE2EAdmissionRejected, label)
	}
	return parsed.UTC(), nil
}

func verifyFairLaunchLocalArtifacts(ctx context.Context, bundle FairLaunchLocalE2EAdmissionBundle, now time.Time, rpc fairLaunchLocalRPCCaller) (fairLaunchVerifiedArtifacts, error) {
	if ctx == nil || now.IsZero() || rpc == nil || bundle.SchemaVersion != fairLaunchLocalBundleSchema || !fairLaunchLocalRunPattern.MatchString(bundle.RunID) {
		return fairLaunchVerifiedArtifacts{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var authority fairLaunchRuntimeAuthority
	if err := strictUnmarshalLocal(bundle.ContractRuntimeAdmission, &authority, "Contract runtime admission"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if authority.SchemaVersion != fairLaunchLocalRuntimeSchema || authority.Authority != fairLaunchLocalAuthority || authority.RunID != bundle.RunID || authority.ChainID != fairLaunchLocalExpectedChainID || authority.ProjectID != "102" || !fairLaunchLocalHashPattern.MatchString(authority.GenesisHash) || !fairLaunchLocalDigestPattern.MatchString(authority.ManifestSHA256) || !fairLaunchLocalDigestPattern.MatchString(authority.TrustPinSHA256) || !fairLaunchLocalDigestPattern.MatchString(authority.PayloadSHA256) {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: Contract runtime authority mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if _, err := exactFairLaunchLocalLoopbackURL(authority.RPCURL); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if localSHA256([]byte(bundle.DeploymentManifest)) != authority.ManifestSHA256 || localSHA256([]byte(bundle.ExternalTrustPin)) != authority.TrustPinSHA256 {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: manifest/pin digest mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	var envelope fairLaunchManifestEnvelope
	var pin fairLaunchTrustPin
	var trust fairLaunchTrustRoot
	if err := strictUnmarshalLocal(bundle.DeploymentManifest, &envelope, "deployment manifest"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if err := strictUnmarshalLocal(bundle.ExternalTrustPin, &pin, "external trust pin"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if err := strictUnmarshalLocal(bundle.PublicTrustRoot, &trust, "public trust root"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if err := verifyCanonicalLocalFile(bundle.DeploymentManifest, envelope, "deployment manifest"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if err := verifyCanonicalLocalFile(bundle.ExternalTrustPin, pin, "external trust pin"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if trust.SchemaVersion != fairLaunchLocalTrustSchema || pin.SchemaVersion != fairLaunchLocalPinSchema || trust.Authority != fairLaunchLocalAuthority || pin.Authority != fairLaunchLocalAuthority || trust.RunID != bundle.RunID || pin.RunID != bundle.RunID || trust.RPCURL != authority.RPCURL || pin.RPCURL != authority.RPCURL || trust.ChainID != fairLaunchLocalExpectedChainID || pin.ChainID != fairLaunchLocalExpectedChainID || trust.GenesisHash != authority.GenesisHash || pin.GenesisHash != authority.GenesisHash || trust.KeyID != pin.KeyID || !fairLaunchLocalKeyIDPattern.MatchString(trust.KeyID) {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: trust root/pin identity mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	trustCanonical, err := canonicalLocalJSON(trust)
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	jwkCanonical, err := canonicalLocalJSON(trust.PublicKeyJWK)
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if pin.TrustRecordSHA256 != localSHA256(trustCanonical) || pin.PublicKeySHA256 != localSHA256(jwkCanonical) || pin.TrustCreatedAt != trust.CreatedAt || envelope.TestTrustRoot.Authority != fairLaunchLocalAuthority || envelope.TestTrustRoot.KeyID != trust.KeyID || envelope.TestTrustRoot.PublicKeyJWK != trust.PublicKeyJWK {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: external trust projection mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	publicKey, keyID, err := fairLaunchLocalEd25519Key(trust.PublicKeyJWK)
	if err != nil || keyID != trust.KeyID || envelope.Signature.Algorithm != "ed25519" || envelope.Signature.KeyID != trust.KeyID || envelope.Signature.PayloadSHA256 != authority.PayloadSHA256 {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: manifest signature identity mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	payloadCanonical, err := canonicalLocalJSON(envelope.Payload)
	if err != nil || localSHA256(payloadCanonical) != envelope.Signature.PayloadSHA256 {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: manifest payload digest mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature.Value)
	if err != nil || !ed25519.Verify(publicKey, payloadCanonical, signature) {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: manifest Ed25519 verification failed", FairLaunchLocalE2EAdmissionRejected)
	}
	createdAt, err := parseFairLaunchLocalTime(envelope.Payload.CreatedAt, "manifest createdAt")
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	expiresAt, err := parseFairLaunchLocalTime(envelope.Payload.ExpiresAt, "manifest expiresAt")
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	trustAt, err := parseFairLaunchLocalTime(trust.CreatedAt, "trust createdAt")
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	pinAt, err := parseFairLaunchLocalTime(pin.CreatedAt, "pin createdAt")
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if createdAt.Before(pinAt) || pinAt.Before(trustAt) || expiresAt.Sub(createdAt) > fairLaunchLocalMaxLifetime || !now.Before(expiresAt) || now.Before(createdAt) {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: manifest validity interval invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := validateFairLaunchLocalManifest(bundle.RunID, authority, envelope.Payload); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if err := verifyFairLaunchLocalRPC(ctx, rpc, authority, envelope.Payload); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	var bootstrap fairLaunchBootstrapEvidence
	if err := strictUnmarshalLocal(bundle.BackendBootstrap, &bootstrap, "Backend bootstrap evidence"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if err := verifyCanonicalLocalFile(bundle.BackendBootstrap, bootstrap, "Backend bootstrap evidence"); err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	bootstrapExpires, err := parseFairLaunchLocalTime(bootstrap.ExpiresAt, "bootstrap expiresAt")
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	if bootstrap.SchemaVersion != fairLaunchLocalBootstrapSchema || bootstrap.Authority != fairLaunchLocalAuthority || bootstrap.Status != "BOOTSTRAPPED_NOT_PRODUCTION_ADMITTED" || bootstrap.RunID != bundle.RunID || bootstrap.ChainID != fairLaunchLocalExpectedChainID || bootstrap.RPCURL != authority.RPCURL || bootstrap.GenesisHash != authority.GenesisHash || bootstrap.ManifestSHA256 != authority.ManifestSHA256 || bootstrap.RuntimeAdmissionSHA256 != localSHA256([]byte(bundle.ContractRuntimeAdmission)) || !bootstrapExpires.Equal(expiresAt) || bootstrap.SignedTip.BlockNumber != envelope.Payload.Deployment.Observation.BlockNumber || bootstrap.SignedTip.BlockHash != envelope.Payload.Deployment.Observation.BlockHash || bootstrap.SignedTip.TransactionIndex != envelope.Payload.Deployment.Observation.TransactionIndex || len(bootstrap.ProjectIDs) != 3 || bootstrap.ProjectIDs[0] != 101 || bootstrap.ProjectIDs[1] != 102 || bootstrap.ProjectIDs[2] != 103 || bootstrap.RunRowID == 0 || bootstrap.SourceID == 0 || bootstrap.SourceRevisionID == 0 || bootstrap.SourceBindingID == 0 || !fairLaunchLocalDigestPattern.MatchString(bootstrap.SchemaDigest) || !fairLaunchLocalDigestPattern.MatchString(bootstrap.GrantPolicyDigest) || !fairLaunchLocalDigestPattern.MatchString(bootstrap.SourceRegistryDigest) {
		return fairLaunchVerifiedArtifacts{}, fmt.Errorf("%s: Backend bootstrap cross-binding mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	databaseName, mysqlUUID, mysqlVersion, containerID, ownerDigest, err := verifyFairLaunchDeployEvidence(bundle, bootstrap)
	if err != nil {
		return fairLaunchVerifiedArtifacts{}, err
	}
	return fairLaunchVerifiedArtifacts{authority: authority, manifest: envelope, bootstrap: bootstrap, expiresAt: expiresAt, runtimeDigest: bootstrap.RuntimeAdmissionSHA256, manifestDigest: authority.ManifestSHA256, databaseName: databaseName, mysqlUUID: mysqlUUID, mysqlVersion: mysqlVersion, schemaDigest: bootstrap.SchemaDigest, grantDigest: bootstrap.GrantPolicyDigest, containerID: containerID, ownerDigest: ownerDigest}, nil
}

func fairLaunchLocalEd25519Key(jwk fairLaunchJWK) (ed25519.PublicKey, string, error) {
	if jwk.KTY != "OKP" || jwk.CRV != "Ed25519" || jwk.X == "" {
		return nil, "", errors.New("invalid Ed25519 JWK")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(jwk.X)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, "", errors.New("invalid Ed25519 JWK material")
	}
	key := ed25519.PublicKey(raw)
	spki, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, "", err
	}
	return key, "local-e2e-" + localSHA256(spki)[:24], nil
}

func exactFairLaunchLocalLoopbackURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil || parsed.String() != value {
		return nil, fmt.Errorf("%s: RPC URL is not exact loopback", FairLaunchLocalE2EAdmissionRejected)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1024 || port > 65535 {
		return nil, fmt.Errorf("%s: RPC port invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	return parsed, nil
}

func validateFairLaunchLocalManifest(runID string, authority fairLaunchRuntimeAuthority, payload fairLaunchManifestPayload) error {
	if payload.SchemaVersion != fairLaunchLocalManifestSchema || payload.Authority != fairLaunchLocalAuthority || payload.RunID != runID || payload.Network.ChainID != fairLaunchLocalExpectedChainID || payload.Network.Name != "hardhat-local-persistent" || payload.Network.RPCURL != authority.RPCURL || payload.Network.GenesisHash != authority.GenesisHash {
		return fmt.Errorf("%s: manifest network identity mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	requiredContracts := []string{"adminFacet", "coreFacet", "dispatcher", "fundsFacet", "logicLibrary", "proxy", "releaseFacet", "runtimeLibrary", "viewsFacet"}
	keys := make([]string, 0, len(payload.Contracts))
	for key := range payload.Contracts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if strings.Join(keys, "|") != strings.Join(requiredContracts, "|") {
		return fmt.Errorf("%s: manifest Contract set mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	for _, row := range payload.Contracts {
		if !fairLaunchLocalAddressPattern.MatchString(row.Address) || !fairLaunchLocalHashPattern.MatchString(row.RuntimeCodeHash) {
			return fmt.Errorf("%s: manifest Contract identity invalid", FairLaunchLocalE2EAdmissionRejected)
		}
	}
	if payload.Assets.PaymentToken.Symbol != "tUSDT" || payload.Assets.PaymentToken.Decimals != 6 || len(payload.Assets.ProjectTokens) != 3 || len(payload.Projects) != 3 || payload.Assets.Faucet.Strategy != "unrestricted-token-mint" || payload.Assets.Faucet.NativeGasStrategy != "frontend-local-hardhat_setBalance" || payload.Assets.Faucet.StoresPrivateKeys {
		return fmt.Errorf("%s: manifest local asset policy mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	tokens := map[string]fairLaunchTokenIdentity{}
	for _, token := range payload.Assets.ProjectTokens {
		if token.Decimals != 18 || !fairLaunchLocalAddressPattern.MatchString(token.Address) || !fairLaunchLocalHashPattern.MatchString(token.RuntimeCodeHash) {
			return fmt.Errorf("%s: project token identity invalid", FairLaunchLocalE2EAdmissionRejected)
		}
		tokens[token.Key] = token
	}
	expected := map[string]string{"failed": "101", "successful": "102", "cancelled": "103"}
	projects := map[string]fairLaunchManifestProject{}
	for _, project := range payload.Projects {
		projects[project.Key] = project
	}
	if len(tokens) != 3 || len(projects) != 3 {
		return fmt.Errorf("%s: duplicate project/token keys", FairLaunchLocalE2EAdmissionRejected)
	}
	proxy := payload.Contracts["proxy"].Address
	for key, projectID := range expected {
		project, exists := projects[key]
		token, tokenExists := tokens[key]
		if !exists || !tokenExists || project.ProjectID != projectID || project.ProjectToken != token.Address || project.PaymentToken != payload.Assets.PaymentToken.Address || project.Creation.To == nil || *project.Creation.To != proxy || project.Creation.From != payload.Roles["operator"] || project.PlatformFeeBPS != 250 || project.PlatformFeeReceiver != payload.Safes.PlatformFee.Address || project.ProjectProceedsReceiver != payload.Safes.Proceeds.Address || project.TotalTokensDepositedRaw != project.SaleTokenAmountRaw || decimalString(project.PristineStorage["projectId"]) != projectID || decimalString(project.PristineStorage["totalTokensDeposited"]) != project.TotalTokensDepositedRaw {
			return fmt.Errorf("%s: project %s binding mismatch", FairLaunchLocalE2EAdmissionRejected, key)
		}
	}
	if payload.Deployment.Observation.From != payload.Roles["browserSeedAccount"] || payload.Deployment.Observation.To == nil || *payload.Deployment.Observation.To != payload.Roles["browserSeedAccount"] || payload.Deployment.Observation.ValueRaw != "0" || payload.Deployment.Observation.InputHash != crypto.Keccak256Hash(nil).Hex() {
		return fmt.Errorf("%s: signed observation identity mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	return nil
}

func decimalString(value any) string {
	switch number := value.(type) {
	case string:
		return number
	case float64:
		return strconv.FormatFloat(number, 'f', -1, 64)
	default:
		return ""
	}
}

type fairLaunchLocalRPCCaller interface {
	Call(context.Context, string, any, any) error
}

func verifyFairLaunchLocalLiveBlock(rpc fairLaunchLocalRPCCaller, ctx context.Context, blockNumber uint64, expectedHash string) error {
	if rpc == nil || !fairLaunchLocalHashPattern.MatchString(expectedHash) {
		return fmt.Errorf("%s: live RPC block authority invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	blockTag := fmt.Sprintf("0x%x", blockNumber)
	var block fairLaunchRPCBlock
	if err := rpc.Call(ctx, "eth_getBlockByNumber", []any{blockTag, false}, &block); err != nil || block.Number != blockTag || block.Hash != expectedHash {
		return fmt.Errorf("%s: live canonical block mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if blockNumber <= math.MaxUint64-2 {
		var successor fairLaunchRPCBlock
		if err := rpc.Call(ctx, "eth_getBlockByNumber", []any{fmt.Sprintf("0x%x", blockNumber+1), false}, &successor); err == nil && successor.Number != "" && successor.ParentHash != expectedHash {
			return fmt.Errorf("%s: live canonical successor mismatch", FairLaunchLocalE2EAdmissionRejected)
		}
	}
	return nil
}

type fairLaunchLocalHTTPRPC struct {
	url    string
	client *http.Client
}

func (rpc fairLaunchLocalHTTPRPC) Call(ctx context.Context, method string, params any, target any) error {
	requestBody, err := jsonv2.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      uint64 `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rpc.url, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	request.Header.Set("content-type", "application/json")
	response, err := rpc.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil || response.StatusCode != http.StatusOK {
		return errors.New("RPC response unavailable")
	}
	var envelope struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      uint64         `json:"id"`
		Result  jsontext.Value `json:"result"`
		Error   jsontext.Value `json:"error,omitzero"`
	}
	if err := jsonv2.Unmarshal(raw, &envelope); err != nil || envelope.ID != 1 || len(envelope.Error) != 0 || len(envelope.Result) == 0 {
		return errors.New("RPC response invalid")
	}
	return jsonv2.Unmarshal(envelope.Result, target)
}

type fairLaunchRPCBlock struct {
	Hash       string `json:"hash"`
	Number     string `json:"number"`
	ParentHash string `json:"parentHash"`
	Timestamp  string `json:"timestamp"`
}

func verifyFairLaunchLocalRPC(ctx context.Context, rpc fairLaunchLocalRPCCaller, authority fairLaunchRuntimeAuthority, payload fairLaunchManifestPayload) error {
	var chainID string
	if err := rpc.Call(ctx, "eth_chainId", []any{}, &chainID); err != nil || chainID != "0x7a69" {
		return fmt.Errorf("%s: live RPC chain ID mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	var genesis, latest fairLaunchRPCBlock
	if err := rpc.Call(ctx, "eth_getBlockByNumber", []any{"0x0", false}, &genesis); err != nil || genesis.Hash != authority.GenesisHash {
		return fmt.Errorf("%s: live RPC genesis mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := rpc.Call(ctx, "eth_getBlockByNumber", []any{"latest", false}, &latest); err != nil {
		return fmt.Errorf("%s: live RPC tip unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	observation := payload.Deployment.Observation
	wantBlock, err := strconv.ParseUint(observation.BlockNumber, 10, 64)
	if err != nil || latest.Hash != observation.BlockHash || latest.Number != fmt.Sprintf("0x%x", wantBlock) {
		return fmt.Errorf("%s: signed pristine tip mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	wantTimestamp, err := strconv.ParseUint(observation.BlockTimestamp, 10, 64)
	if err != nil || latest.Timestamp != fmt.Sprintf("0x%x", wantTimestamp) {
		return fmt.Errorf("%s: signed pristine tip timestamp mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	codeRows := []fairLaunchCodeIdentity{}
	for _, row := range payload.Contracts {
		codeRows = append(codeRows, row)
	}
	codeRows = append(codeRows, fairLaunchCodeIdentity{Address: payload.Assets.PaymentToken.Address, RuntimeCodeHash: payload.Assets.PaymentToken.RuntimeCodeHash})
	for _, row := range payload.Assets.ProjectTokens {
		codeRows = append(codeRows, fairLaunchCodeIdentity{Address: row.Address, RuntimeCodeHash: row.RuntimeCodeHash})
	}
	codeRows = append(codeRows, fairLaunchCodeIdentity{Address: payload.Safes.Proceeds.Address, RuntimeCodeHash: payload.Safes.Proceeds.RuntimeCodeHash}, fairLaunchCodeIdentity{Address: payload.Safes.PlatformFee.Address, RuntimeCodeHash: payload.Safes.PlatformFee.RuntimeCodeHash})
	for _, row := range codeRows {
		var code string
		if err := rpc.Call(ctx, "eth_getCode", []any{row.Address, "latest"}, &code); err != nil || code == "0x" || strings.ToLower(crypto.Keccak256Hash(common.FromHex(code)).Hex()) != row.RuntimeCodeHash {
			return fmt.Errorf("%s: live code identity mismatch", FairLaunchLocalE2EAdmissionRejected)
		}
	}
	return nil
}

func verifyFairLaunchDeployEvidence(bundle FairLaunchLocalE2EAdmissionBundle, bootstrap fairLaunchBootstrapEvidence) (string, string, string, string, string, error) {
	var database fairLaunchDatabaseEvidence
	var readiness fairLaunchSchemaReadinessEvidence
	var infrastructure fairLaunchInfrastructureEvidence
	var container fairLaunchContainerAuthority
	for _, item := range []struct {
		raw    string
		target any
		label  string
	}{{bundle.DatabaseSchemaEvidence, &database, "database schema evidence"}, {bundle.SchemaReadiness, &readiness, "schema readiness"}, {bundle.InfrastructureEvidence, &infrastructure, "infrastructure evidence"}, {bundle.ContainerAuthority, &container, "container authority"}} {
		if err := strictUnmarshalLocal(item.raw, item.target, item.label); err != nil {
			return "", "", "", "", "", err
		}
	}
	if bundle.ExpectedJobGrants == "" || bundle.ExpectedJobGrants != bundle.ActualJobGrants || bundle.ExpectedIngestGrants == "" || bundle.ExpectedIngestGrants != bundle.ActualIngestGrants {
		return "", "", "", "", "", fmt.Errorf("%s: expected/actual local role grants differ", FairLaunchLocalE2EAdmissionRejected)
	}
	databaseName := database.Database
	mysqlUUID := database.MySQLServerUUID
	mysqlVersion := database.MySQLVersion
	containerID := database.ContainerID
	ownerDigest := database.OwnerLabelDigest
	expectedDatabase := fairLaunchLocalExpectedDatabase + strings.ReplaceAll(bootstrap.RunID, "-", "")
	profiles := readiness.ProfileDigests
	if database.Authority != fairLaunchLocalAuthority || database.Status != "SCHEMA_STAGED_NOT_RUNTIME_ADMITTED" || database.RunID != bootstrap.RunID || database.SchemaDigest != bootstrap.SchemaDigest || database.GrantPolicyDigest != bootstrap.GrantPolicyDigest || readiness.Authority != fairLaunchLocalAuthority || readiness.Status != "SCHEMA_READY_NOT_RUNTIME_ADMITTED" || readiness.RunID != bootstrap.RunID || readiness.SchemaDigest != bootstrap.SchemaDigest || readiness.GrantPolicyDigest != bootstrap.GrantPolicyDigest || readiness.SchemaEvidenceDigest != localSHA256([]byte(bundle.DatabaseSchemaEvidence)) || readiness.InfrastructureDigest != localSHA256([]byte(bundle.InfrastructureEvidence)) || readiness.TransportDigest != database.TransportDigest || readiness.MySQLImageID != database.MySQLImageID || readiness.BaselineDigest != database.BaselineDigest || readiness.MigrationManifestDigest != database.MigrationManifestDigest || readiness.OwnerLabelDigest != ownerDigest || len(profiles) != 4 || profiles["job"] != localSHA256([]byte(bundle.MySQLJobProfile)) || profiles["ingest"] != localSHA256([]byte(bundle.MySQLIngestProfile)) || !fairLaunchLocalDigestPattern.MatchString(profiles["runtime"]) || !fairLaunchLocalDigestPattern.MatchString(profiles["admin"]) || infrastructure.Authority != fairLaunchLocalAuthority || infrastructure.Status != "INFRASTRUCTURE_STARTED_NOT_ADMITTED" || infrastructure.RunID != bootstrap.RunID || infrastructure.MySQL.Endpoint != database.Endpoint || infrastructure.MySQL.ContainerID != containerID || infrastructure.MySQL.ImageID != database.MySQLImageID || infrastructure.MySQL.Database != databaseName || infrastructure.MySQL.ServerUUID != mysqlUUID || infrastructure.MySQL.Version != mysqlVersion || infrastructure.MySQL.TransportDigest != database.TransportDigest || infrastructure.MySQL.NetworkMode != "host" || infrastructure.MySQL.PublishedPortMappings || container.RunID != bootstrap.RunID || container.ContainerID != containerID || container.Component != "mysql" || container.Domain != "JC_FL_TEST_CONTAINER_CLEANUP_V3" || container.Version != 3 || bootstrap.Database != databaseName || databaseName != expectedDatabase || !fairLaunchLocalDigestPattern.MatchString(ownerDigest) {
		return "", "", "", "", "", fmt.Errorf("%s: Deploy evidence binding mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	return databaseName, mysqlUUID, mysqlVersion, containerID, ownerDigest, nil
}

type fairLaunchLocalMySQLProfile struct {
	Host     string
	Port     uint16
	Database string
	User     string
	Password string
}

func parseFairLaunchLocalMySQLProfile(raw, runID, expectedUser string) (fairLaunchLocalMySQLProfile, error) {
	if raw == "" || strings.ContainsRune(raw, '\x00') || !strings.HasSuffix(raw, "\n") {
		return fairLaunchLocalMySQLProfile{}, fmt.Errorf("%s: MySQL profile bytes invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	expected := map[string]string{"host": "127.0.0.1", "protocol": "tcp", "user": expectedUser, "ssl-mode": "DISABLED", "get-server-public-key": "TRUE", "connect-timeout": "5"}
	values := map[string]string{}
	seenHeader, seenRun := false, false
	for _, line := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		if line == "[client]" && !seenHeader {
			seenHeader = true
			continue
		}
		if line == "# local-only-run-id="+runID && !seenRun {
			seenRun = true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || value == "" {
			return fairLaunchLocalMySQLProfile{}, fmt.Errorf("%s: MySQL profile syntax invalid", FairLaunchLocalE2EAdmissionRejected)
		}
		if _, duplicate := values[key]; duplicate {
			return fairLaunchLocalMySQLProfile{}, fmt.Errorf("%s: MySQL profile duplicate option", FairLaunchLocalE2EAdmissionRejected)
		}
		values[key] = value
	}
	allowed := map[string]bool{"host": true, "port": true, "protocol": true, "database": true, "user": true, "password": true, "ssl-mode": true, "get-server-public-key": true, "connect-timeout": true}
	if !seenHeader || !seenRun || len(values) != len(allowed) {
		return fairLaunchLocalMySQLProfile{}, fmt.Errorf("%s: MySQL profile field set invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	for key, value := range values {
		if !allowed[key] || (expected[key] != "" && value != expected[key]) {
			return fairLaunchLocalMySQLProfile{}, fmt.Errorf("%s: MySQL profile option invalid", FairLaunchLocalE2EAdmissionRejected)
		}
	}
	port, err := strconv.ParseUint(values["port"], 10, 16)
	if err != nil || port < 1024 || !strings.HasPrefix(values["database"], fairLaunchLocalExpectedDatabase) {
		return fairLaunchLocalMySQLProfile{}, fmt.Errorf("%s: MySQL profile endpoint invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	return fairLaunchLocalMySQLProfile{Host: values["host"], Port: uint16(port), Database: values["database"], User: values["user"], Password: values["password"]}, nil
}

// AdmitFairLaunchLocalE2E is the only non-test capability constructor. It opens
// exactly the run-bound local_job pool from a stdin profile and re-verifies the
// live DB before returning an opaque capability.
func AdmitFairLaunchLocalE2E(ctx context.Context, bundle FairLaunchLocalE2EAdmissionBundle, now time.Time) (FairLaunchLocalE2ECapability, *gorm.DB, error) {
	if ctx == nil {
		return FairLaunchLocalE2ECapability{}, nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rpc := fairLaunchLocalHTTPRPC{url: bundleRunRPCURL(bundle.ContractRuntimeAdmission), client: &http.Client{Timeout: 5 * time.Second}}
	artifacts, err := verifyFairLaunchLocalArtifacts(ctx, bundle, now.UTC(), rpc)
	if err != nil {
		return FairLaunchLocalE2ECapability{}, nil, err
	}
	profile, err := parseFairLaunchLocalMySQLProfile(bundle.MySQLJobProfile, bundle.RunID, "local_job")
	if err != nil || profile.Database != artifacts.databaseName {
		return FairLaunchLocalE2ECapability{}, nil, fmt.Errorf("%s: MySQL profile database mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	database, sqlDB, err := openFairLaunchLocalMySQL(profile)
	if err != nil {
		return FairLaunchLocalE2ECapability{}, nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = sqlDB.Close()
		}
	}()
	if err := sqlDB.PingContext(ctx); err != nil {
		return FairLaunchLocalE2ECapability{}, nil, fmt.Errorf("%s: MySQL connection rejected", FairLaunchLocalE2EAdmissionRejected)
	}
	lease, fence, err := verifyFairLaunchLocalDatabase(ctx, database.WithContext(ctx), bundle, artifacts, now.UTC())
	if err != nil {
		return FairLaunchLocalE2ECapability{}, nil, err
	}
	admission := FairLaunchLocalE2EAdmission{Authority: fairLaunchLocalAuthority, RunID: bundle.RunID, RunRowID: artifacts.bootstrap.RunRowID, ChainID: fairLaunchLocalExpectedChainID, RPCURL: artifacts.authority.RPCURL, GenesisHash: artifacts.authority.GenesisHash, DatabaseName: artifacts.databaseName, MySQLServerUUID: artifacts.mysqlUUID, SchemaDigest: artifacts.schemaDigest, GrantPolicyDigest: artifacts.grantDigest, ManifestSHA256: artifacts.manifestDigest, RuntimeAdmissionSHA256: artifacts.runtimeDigest, SourceRegistryDigest: artifacts.bootstrap.SourceRegistryDigest, SourceID: artifacts.bootstrap.SourceID, SourceRevisionID: artifacts.bootstrap.SourceRevisionID, SourceBindingID: artifacts.bootstrap.SourceBindingID, SignedTipBlock: mustLocalUint(artifacts.bootstrap.SignedTip.BlockNumber), SignedTipHash: artifacts.bootstrap.SignedTip.BlockHash, ExpiresAt: artifacts.expiresAt, Status: "BACKEND_LOCAL_ONLY_ADMITTED"}
	closeOnError = false
	return FairLaunchLocalE2ECapability{seal: &fairLaunchLocalE2ESeal{admission: admission, lease: lease, fence: fence, rpc: rpc, contractAddress: artifacts.manifest.Payload.Contracts["proxy"].Address, manifest: artifacts.manifest.Payload}}, database, nil
}

func openFairLaunchLocalMySQL(profile fairLaunchLocalMySQLProfile) (*gorm.DB, *sql.DB, error) {
	config := drivermysql.NewConfig()
	config.User = profile.User
	config.Passwd = profile.Password
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%d", profile.Host, profile.Port)
	config.DBName = profile.Database
	config.ParseTime = true
	config.Loc = time.UTC
	config.Timeout = 5 * time.Second
	config.ReadTimeout = 20 * time.Second
	config.WriteTimeout = 20 * time.Second
	config.TLSConfig = "false"
	config.AllowNativePasswords = true
	config.AllowCleartextPasswords = false
	profile.Password = ""
	sqlDB, err := sql.Open("mysql", config.FormatDSN())
	config.Passwd = ""
	if err != nil {
		return nil, nil, fmt.Errorf("%s: MySQL pool unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NowFunc: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("%s: GORM pool unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	return database, sqlDB, nil
}

// OpenFairLaunchLocalE2EIngest opens the separately least-privileged archive
// role after the main capability is minted. It never shares a transaction with
// financial application.
func OpenFairLaunchLocalE2EIngest(ctx context.Context, capability FairLaunchLocalE2ECapability, bundle FairLaunchLocalE2EAdmissionBundle, now time.Time) (*gorm.DB, error) {
	if ctx == nil || !capability.valid(now) || capability.seal.admission.RunID != bundle.RunID {
		return nil, errors.New(FairLaunchLocalE2ECapabilityRequired)
	}
	profile, err := parseFairLaunchLocalMySQLProfile(bundle.MySQLIngestProfile, bundle.RunID, "local_ingest")
	if err != nil || profile.Database != capability.seal.admission.DatabaseName {
		return nil, fmt.Errorf("%s: local_ingest profile mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	database, sqlDB, err := openFairLaunchLocalMySQL(profile)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("%s: local_ingest connection rejected", FairLaunchLocalE2EAdmissionRejected)
	}
	var identity fairLaunchDBIdentity
	if err := database.Raw("SELECT CURRENT_USER() AS current_user_value,DATABASE() AS database_name,LOWER(@@server_uuid) AS server_uuid,VERSION() AS mysql_version,@@FOREIGN_KEY_CHECKS AS foreign_keys").Scan(&identity).Error; err != nil || identity.CurrentUser != "local_ingest@%" || identity.Database != capability.seal.admission.DatabaseName || identity.ServerUUID != capability.seal.admission.MySQLServerUUID || identity.ForeignKeys != 1 {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("%s: local_ingest live identity mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := verifyFairLaunchGrantText(database, capability.seal.admission.DatabaseName, "local_ingest", bundle.ExpectedIngestGrants); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return database, nil
}

func bundleRunRPCURL(raw string) string {
	var authority fairLaunchRuntimeAuthority
	if strictUnmarshalLocal(raw, &authority, "Contract runtime admission") != nil {
		return "http://127.0.0.1:1"
	}
	return authority.RPCURL
}

func mustLocalUint(value string) uint64 {
	parsed, _ := strconv.ParseUint(value, 10, 64)
	return parsed
}

type fairLaunchDBIdentity struct {
	CurrentUser string `gorm:"column:current_user_value"`
	Database    string `gorm:"column:database_name"`
	ServerUUID  string `gorm:"column:server_uuid"`
	Version     string `gorm:"column:mysql_version"`
	ForeignKeys uint8  `gorm:"column:foreign_keys"`
}

func verifyFairLaunchLocalDatabase(ctx context.Context, database *gorm.DB, bundle FairLaunchLocalE2EAdmissionBundle, artifacts fairLaunchVerifiedArtifacts, now time.Time) (string, uint64, error) {
	if database == nil {
		return "", 0, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var identity fairLaunchDBIdentity
	if err := database.Raw("SELECT CURRENT_USER() AS current_user_value,DATABASE() AS database_name,LOWER(@@server_uuid) AS server_uuid,VERSION() AS mysql_version,@@FOREIGN_KEY_CHECKS AS foreign_keys").Scan(&identity).Error; err != nil || identity.CurrentUser != "local_job@%" || identity.Database != artifacts.databaseName || identity.ServerUUID != artifacts.mysqlUUID || identity.Version != artifacts.mysqlVersion || identity.ForeignKeys != 1 {
		return "", 0, fmt.Errorf("%s: MySQL live identity mismatch user=%q database=%q uuid=%q version=%q fk=%d", FairLaunchLocalE2EAdmissionRejected, identity.CurrentUser, identity.Database, identity.ServerUUID, identity.Version, identity.ForeignKeys)
	}
	var schemaDigest string
	if err := database.Exec("SET SESSION group_concat_max_len=16777216").Error; err != nil || database.Raw("SELECT LOWER(SHA2(GROUP_CONCAT(CONCAT_WS('|',TABLE_NAME,COLUMN_NAME,ORDINAL_POSITION,COLUMN_TYPE,IS_NULLABLE,COALESCE(COLUMN_DEFAULT,'<NULL>'),COALESCE(COLLATION_NAME,'<NULL>'),EXTRA) ORDER BY BINARY TABLE_NAME,ORDINAL_POSITION SEPARATOR '\n'),256)) AS schema_digest FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()").Scan(&schemaDigest).Error != nil || schemaDigest != fairLaunchLocalJobSchemaDigest {
		return "", 0, fmt.Errorf("%s: local_job-visible live schema digest mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := verifyFairLaunchLocalPrivileges(database, artifacts.databaseName); err != nil {
		return "", 0, err
	}
	var run entity.FairLaunchLocalE2ERun
	if err := database.Where("id=? AND run_id=?", artifacts.bootstrap.RunRowID, bundle.RunID).Take(&run).Error; err != nil || run.Environment != "local-e2e" || run.State != "verified" || run.ChainID != fairLaunchLocalExpectedChainID || run.DatabaseName != artifacts.databaseName || run.MySQLServerUUID != artifacts.mysqlUUID || run.MySQLVersion != artifacts.mysqlVersion || run.ContainerID != artifacts.containerID || run.OwnerLabelDigest != artifacts.ownerDigest || run.SchemaGeneration != 12 || run.SchemaDigest != artifacts.schemaDigest || run.GrantDigest != artifacts.grantDigest || run.SourceRegistryDigest != artifacts.bootstrap.SourceRegistryDigest || run.HardhatGenesisHash != artifacts.authority.GenesisHash || run.ChainManifestDigest != artifacts.authority.PayloadSHA256 || run.DeploymentManifestDigest != artifacts.authority.ManifestSHA256 || run.RuntimeAdmissionDigest != artifacts.runtimeDigest || !run.ExpiresAt.Equal(artifacts.expiresAt) || !now.Before(run.ExpiresAt) || run.ClosedAt != nil {
		return "", 0, fmt.Errorf("%s: persisted run authority mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	var source entity.ChainContractSource
	if err := database.Where("id=? AND chain_id=31337 AND contract_address=? AND contract_version=? AND funding_mode=? AND enabled=1 AND retired_at IS NULL", artifacts.bootstrap.SourceID, artifacts.manifest.Payload.Contracts["proxy"].Address, fairLaunchLocalContractVersion, fairLaunchLocalFundingMode).Take(&source).Error; err != nil {
		return "", 0, fmt.Errorf("%s: source binding unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	var revision entity.ChainContractSourceRevision
	if err := database.Where("id=? AND source_id=? AND revision_no=1 AND implementation_address=? AND implementation_code_hash=? AND abi_digest=? AND dependency_manifest_digest=? AND to_block_number IS NULL", artifacts.bootstrap.SourceRevisionID, source.ID, artifacts.manifest.Payload.Contracts["dispatcher"].Address, strings.TrimPrefix(artifacts.manifest.Payload.Contracts["dispatcher"].RuntimeCodeHash, "0x"), strings.TrimPrefix(artifacts.manifest.Payload.Source.AggregateABIDigest, "0x"), strings.TrimPrefix(artifacts.manifest.Payload.Source.FullSetDigest, "0x")).Take(&revision).Error; err != nil {
		return "", 0, fmt.Errorf("%s: source revision binding unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	var sourceBinding entity.FairLaunchSourceRevisionBindingV3
	if err := database.Where("id=? AND run_row_id=? AND source_id=? AND source_revision_id=? AND chain_id=31337", artifacts.bootstrap.SourceBindingID, run.ID, source.ID, revision.ID).Take(&sourceBinding).Error; err != nil {
		return "", 0, fmt.Errorf("%s: source revision run binding unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	var projectCount, tokenCount, observationCount int64
	if database.Model(&entity.FairLaunchProjectBindingV3{}).Where("run_row_id=? AND source_binding_id=? AND source_id=? AND source_revision_id=?", run.ID, sourceBinding.ID, source.ID, revision.ID).Count(&projectCount).Error != nil || projectCount != 3 || database.Model(&entity.FairLaunchSourceTokenBindingV3{}).Where("run_row_id=? AND source_id=? AND contract_address=?", run.ID, source.ID, source.ContractAddress).Count(&tokenCount).Error != nil || tokenCount != 4 || database.Model(&entity.FairLaunchLocalE2EObservation{}).Where("run_row_id=?", run.ID).Count(&observationCount).Error != nil || observationCount != 11 {
		return "", 0, fmt.Errorf("%s: persisted binding cardinality mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	var cursor entity.SourceMaterializationCursor
	if err := database.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_id=? AND source_revision_id=? AND status='active' AND last_error=''", source.ID, revision.ID).Take(&cursor).Error; err != nil || cursor.LeaseToken == "" || cursor.FenceEpoch == 0 || cursor.CursorGeneration == 0 || cursor.MaterializedThroughBlock != mustLocalUint(artifacts.bootstrap.SignedTip.BlockNumber) || cursor.MaterializedThroughBlockHash != artifacts.bootstrap.SignedTip.BlockHash {
		return "", 0, fmt.Errorf("%s: materialization cursor mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	return cursor.LeaseToken, cursor.FenceEpoch, nil
}

type fairLaunchPrivilegeRow struct {
	TableName     string `gorm:"column:TABLE_NAME"`
	PrivilegeType string `gorm:"column:PRIVILEGE_TYPE"`
	Grantable     string `gorm:"column:IS_GRANTABLE"`
}

func verifyFairLaunchLocalPrivileges(database *gorm.DB, databaseName string) error {
	var rows []fairLaunchPrivilegeRow
	if err := database.Raw("SELECT TABLE_NAME,PRIVILEGE_TYPE,IS_GRANTABLE FROM information_schema.TABLE_PRIVILEGES WHERE GRANTEE=? AND TABLE_SCHEMA=? ORDER BY BINARY TABLE_NAME,BINARY PRIVILEGE_TYPE", "'local_job'@'%'", databaseName).Scan(&rows).Error; err != nil {
		return fmt.Errorf("%s: local_job grants unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	observed := map[string][]string{}
	for _, row := range rows {
		if row.Grantable != "NO" {
			return fmt.Errorf("%s: local_job has grant option", FairLaunchLocalE2EAdmissionRejected)
		}
		observed[row.TableName] = append(observed[row.TableName], row.PrivilegeType)
	}
	if len(observed) != len(fairLaunchLocalE2ERequiredPrivileges) {
		return fmt.Errorf("%s: local_job table grant cardinality mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	for table, expected := range fairLaunchLocalE2ERequiredPrivileges {
		actual := append([]string(nil), observed[table]...)
		sort.Strings(actual)
		want := append([]string(nil), expected...)
		sort.Strings(want)
		if strings.Join(actual, "|") != strings.Join(want, "|") {
			return fmt.Errorf("%s: local_job grant mismatch for %s", FairLaunchLocalE2EAdmissionRejected, table)
		}
	}
	var broad int64
	if err := database.Raw("SELECT (SELECT COUNT(*) FROM information_schema.SCHEMA_PRIVILEGES WHERE GRANTEE=?)+(SELECT COUNT(*) FROM information_schema.USER_PRIVILEGES WHERE GRANTEE=? AND PRIVILEGE_TYPE<>'USAGE')", "'local_job'@'%'", "'local_job'@'%'").Scan(&broad).Error; err != nil || broad != 0 {
		return fmt.Errorf("%s: local_job broad privilege detected", FairLaunchLocalE2EAdmissionRejected)
	}
	return nil
}

func verifyFairLaunchGrantText(database *gorm.DB, databaseName, user, expectedText string) error {
	if database == nil || expectedText == "" || strings.ContainsRune(expectedText, '\x00') {
		return fmt.Errorf("%s: %s grant authority unavailable", FairLaunchLocalE2EAdmissionRejected, user)
	}
	pattern := regexp.MustCompile(`^GRANT ([A-Z, ]+) ON ` + "`" + regexp.QuoteMeta(databaseName) + "`\\.`([a-z0-9_]+)` TO '" + regexp.QuoteMeta(user) + `'@'%'$`)
	expected := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(expectedText, "\n"), "\n") {
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			return fmt.Errorf("%s: %s grant authority syntax invalid", FairLaunchLocalE2EAdmissionRejected, user)
		}
		for _, privilege := range strings.Split(match[1], ", ") {
			expected[match[2]] = append(expected[match[2]], privilege)
		}
	}
	var rows []fairLaunchPrivilegeRow
	grantee := "'" + user + "'@'%'"
	if err := database.Raw("SELECT TABLE_NAME,PRIVILEGE_TYPE,IS_GRANTABLE FROM information_schema.TABLE_PRIVILEGES WHERE GRANTEE=? AND TABLE_SCHEMA=? ORDER BY BINARY TABLE_NAME,BINARY PRIVILEGE_TYPE", grantee, databaseName).Scan(&rows).Error; err != nil {
		return fmt.Errorf("%s: %s live grants unavailable", FairLaunchLocalE2EAdmissionRejected, user)
	}
	observed := map[string][]string{}
	for _, row := range rows {
		if row.Grantable != "NO" {
			return fmt.Errorf("%s: %s has grant option", FairLaunchLocalE2EAdmissionRejected, user)
		}
		observed[row.TableName] = append(observed[row.TableName], row.PrivilegeType)
	}
	if len(expected) != len(observed) {
		return fmt.Errorf("%s: %s grant table cardinality mismatch", FairLaunchLocalE2EAdmissionRejected, user)
	}
	for table, want := range expected {
		actual := append([]string(nil), observed[table]...)
		sort.Strings(actual)
		sort.Strings(want)
		if strings.Join(actual, "|") != strings.Join(want, "|") {
			return fmt.Errorf("%s: %s grant mismatch", FairLaunchLocalE2EAdmissionRejected, user)
		}
	}
	var broad int64
	if err := database.Raw("SELECT (SELECT COUNT(*) FROM information_schema.SCHEMA_PRIVILEGES WHERE GRANTEE=?)+(SELECT COUNT(*) FROM information_schema.USER_PRIVILEGES WHERE GRANTEE=? AND PRIVILEGE_TYPE<>'USAGE')", grantee, grantee).Scan(&broad).Error; err != nil || broad != 0 {
		return fmt.Errorf("%s: %s broad privilege detected", FairLaunchLocalE2EAdmissionRejected, user)
	}
	return nil
}

// ApplyFairLaunchCEV2LocalTx remains distinct from the production repository.
// The capability and live run row are checked before the V3 forward writer.
func ApplyFairLaunchCEV2LocalTx(capability FairLaunchLocalE2ECapability, tx *gorm.DB, in FairLaunchCEWrite, now time.Time) (FairLaunchCEWriteResult, error) {
	if !capability.valid(now) {
		return FairLaunchCEWriteResult{}, errors.New(FairLaunchLocalE2ECapabilityRequired)
	}
	if tx == nil {
		return FairLaunchCEWriteResult{}, errors.New(FairLaunchLocalE2EForwardUnavailable)
	}
	var run entity.FairLaunchLocalE2ERun
	admission := capability.seal.admission
	// The run fingerprint is trigger-enforced immutable and local_job deliberately
	// has SELECT-only authority on it. FOR UPDATE would incorrectly require wider
	// privilege without adding a meaningful serialization guarantee.
	if err := tx.Where("id=? AND run_id=? AND state='verified' AND closed_at IS NULL AND expires_at>?", admission.RunRowID, admission.RunID, now.UTC()).Take(&run).Error; err != nil || run.SchemaDigest != admission.SchemaDigest || run.GrantDigest != admission.GrantPolicyDigest || run.RuntimeAdmissionDigest != admission.RuntimeAdmissionSHA256 {
		return FairLaunchCEWriteResult{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	return applyFairLaunchV3ForwardTx(capability, tx, in, now.UTC())
}
