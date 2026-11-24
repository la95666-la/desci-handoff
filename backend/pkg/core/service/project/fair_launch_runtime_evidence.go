package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

// FairLaunchActionGenerations are derived only from locked, persisted trusted
// state. Clients never choose or override these fences.
type FairLaunchActionGenerations struct {
	Runtime        uint64 `json:"runtime_generation"`
	Policy         uint64 `json:"policy_generation"`
	SourceRegistry uint64 `json:"source_registry_generation"`
	Config         uint64 `json:"config_generation"`
}

type FairLaunchRuntimeEvidence struct {
	Runtime     FairLaunchPublicRuntimeIdentity
	Source      map[string]any
	Generations FairLaunchActionGenerations
	Environment string
	SourceID    uint64
	RevisionID  uint64
}

type fairLaunchRegistryBinding struct {
	Environment              string     `gorm:"column:environment"`
	Revision                 uint64     `gorm:"column:revision"`
	PayloadDigest            string     `gorm:"column:payload_digest"`
	CanonicalPayload         string     `gorm:"column:canonical_payload"`
	ChainID                  uint64     `gorm:"column:chain_id"`
	ProxyAddress             string     `gorm:"column:proxy_address"`
	ImplementationAddress    string     `gorm:"column:implementation_address"`
	ImplementationCodeHash   string     `gorm:"column:implementation_code_hash"`
	ABIDigest                string     `gorm:"column:abi_digest"`
	SourceID                 uint64     `gorm:"column:source_id"`
	SourceRevisionID         uint64     `gorm:"column:source_revision_id"`
	SourceRevisionNo         uint32     `gorm:"column:source_revision_no"`
	RuntimeGeneration        uint64     `gorm:"column:runtime_generation"`
	PolicyGeneration         uint64     `gorm:"column:policy_generation"`
	SourceRegistryGeneration uint64     `gorm:"column:source_registry_generation"`
	ConfigGeneration         uint64     `gorm:"column:config_generation"`
	ValidFrom                time.Time  `gorm:"column:valid_from"`
	ValidUntil               *time.Time `gorm:"column:valid_until"`
}

func fairLaunchEnvironment() string {
	return strings.ToLower(strings.TrimSpace(viper.GetString("application.environment")))
}

const jcsMaxSafeInteger uint64 = 9007199254740991

var jcsRegistryEnvironment = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// canonicalRegistryPayload is the reviewed RFC 8785 projection for the exact
// Source Registry binding schema. Keys are emitted in UTF-16 lexicographic
// order (all keys are ASCII), strings are ASCII policy values, and integers are
// restricted to the I-JSON exact range so no uint64 can be rounded by JCS.
func canonicalRegistryPayload(binding fairLaunchRegistryBinding) ([]byte, error) {
	environment := strings.ToLower(strings.TrimSpace(binding.Environment))
	if !jcsRegistryEnvironment.MatchString(environment) {
		return nil, errors.New("source registry environment is not canonical ASCII")
	}
	ints := []uint64{binding.ChainID, binding.ConfigGeneration, binding.PolicyGeneration, binding.Revision, binding.RuntimeGeneration, binding.SourceID, binding.SourceRegistryGeneration, binding.SourceRevisionID, uint64(binding.SourceRevisionNo)}
	for _, value := range ints {
		if value == 0 || value > jcsMaxSafeInteger {
			return nil, errors.New("source registry integer is outside the RFC 8785 I-JSON range")
		}
	}
	values := map[string]any{
		"abi_digest": strings.ToLower(prefixedHex(binding.ABIDigest)), "chain_id": binding.ChainID,
		"config_generation": binding.ConfigGeneration, "environment": environment,
		"implementation_address":   strings.ToLower(prefixedHex(binding.ImplementationAddress)),
		"implementation_code_hash": strings.ToLower(prefixedHex(binding.ImplementationCodeHash)),
		"policy_generation":        binding.PolicyGeneration, "proxy_address": strings.ToLower(prefixedHex(binding.ProxyAddress)),
		"revision": binding.Revision, "runtime_generation": binding.RuntimeGeneration, "source_id": binding.SourceID,
		"source_registry_generation": binding.SourceRegistryGeneration, "source_revision_id": binding.SourceRevisionID,
		"source_revision_no": uint64(binding.SourceRevisionNo),
	}
	keys := []string{"abi_digest", "chain_id", "config_generation", "environment", "implementation_address", "implementation_code_hash", "policy_generation", "proxy_address", "revision", "runtime_generation", "source_id", "source_registry_generation", "source_revision_id", "source_revision_no"}
	var out bytes.Buffer
	out.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		keyJSON, _ := jsonv1.Marshal(key)
		out.Write(keyJSON)
		out.WriteByte(':')
		switch value := values[key].(type) {
		case string:
			valueJSON, err := jsonv1.Marshal(value)
			if err != nil {
				return nil, err
			}
			out.Write(valueJSON)
		case uint64:
			out.WriteString(strconv.FormatUint(value, 10))
		default:
			return nil, errors.New("unsupported Source Registry JCS value")
		}
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

func validateCanonicalRegistryText(raw string, binding fairLaunchRegistryBinding) ([]byte, error) {
	decoder := jsonv1.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var object map[string]jsonv1.RawMessage
	if err := decoder.Decode(&object); err != nil {
		return nil, errors.New("source registry payload is not JSON")
	}
	var trailing any
	if len(object) != 14 {
		return nil, errors.New("source registry payload field set mismatch")
	}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("source registry payload has trailing data")
	}
	expected, err := canonicalRegistryPayload(binding)
	if err != nil {
		return nil, err
	}
	// Parsing first proves an equivalent differently ordered object has the exact
	// field set/types; storage admission still requires the bytes themselves to be JCS.
	if !bytes.Equal([]byte(raw), expected) {
		return nil, errors.New("source registry payload is not RFC 8785 canonical")
	}
	return expected, nil
}

func validateRegistryBinding(binding fairLaunchRegistryBinding, source entity.ChainContractSource, revision entity.ChainContractSourceRevision, r projectruntime.Runtime, environment string) error {
	if binding.Environment != environment || binding.Revision == 0 || binding.ChainID != r.ChainID || binding.SourceID != source.ID || binding.SourceRevisionID != revision.ID || binding.SourceRevisionNo != revision.RevisionNo {
		return errors.New("runtime_stale: source registry binding identity mismatch")
	}
	if !strings.EqualFold(prefixedHex(binding.ProxyAddress), prefixedHex(r.ContractAddress)) ||
		!strings.EqualFold(prefixedHex(binding.ImplementationAddress), prefixedHex(revision.ImplementationAddress)) ||
		!strings.EqualFold(prefixedHex(binding.ImplementationCodeHash), prefixedHex(revision.ImplementationCodeHash)) ||
		!strings.EqualFold(prefixedHex(binding.ABIDigest), prefixedHex(revision.ABIDigest)) {
		return errors.New("runtime_stale: source registry runtime/ABI binding mismatch")
	}
	if binding.RuntimeGeneration == 0 || binding.PolicyGeneration == 0 || binding.SourceRegistryGeneration == 0 || binding.ConfigGeneration == 0 {
		return errors.New("runtime_stale: source registry generations are incomplete")
	}
	canonical, err := validateCanonicalRegistryText(binding.CanonicalPayload, binding)
	if err != nil {
		return fmt.Errorf("runtime_stale: %w", err)
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != strings.TrimPrefix(strings.ToLower(binding.PayloadDigest), "0x") {
		return errors.New("runtime_stale: source registry payload digest mismatch")
	}
	return nil
}

// ReadFairLaunchRuntimeEvidenceTx re-reads and, when lock is true, locks every
// persisted identity/finality/generation row used by an action decision.
func ReadFairLaunchRuntimeEvidenceTx(db *gorm.DB, r projectruntime.Runtime, now time.Time, lock bool) (FairLaunchRuntimeEvidence, error) {
	if !r.Enabled {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_disabled")
	}
	if db == nil {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_unverified")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	environment := fairLaunchEnvironment()
	if environment == "" {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_unverified: application environment unavailable")
	}
	query := func() *gorm.DB {
		if lock {
			return db.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		return db
	}
	var source entity.ChainContractSource
	if err := query().Where("chain_id=? AND contract_address=? AND contract_version=? AND funding_mode=? AND enabled=1 AND retired_at IS NULL", r.ChainID, strings.ToLower(r.ContractAddress), constant.ContractVersionFairLaunchV1, constant.FundingModeFairLaunch).Take(&source).Error; err != nil {
		return FairLaunchRuntimeEvidence{}, fmt.Errorf("runtime_unverified: trusted fair launch source unavailable: %w", err)
	}
	var revision entity.ChainContractSourceRevision
	if err := query().Where("source_id=? AND implementation_address=? AND implementation_code_hash=? AND to_block_number IS NULL AND to_transaction_index IS NULL AND to_tx_log_ordinal_exclusive IS NULL", source.ID, strings.ToLower(prefixedHex(r.ImplementationAddress)), strings.TrimPrefix(strings.ToLower(prefixedHex(r.ImplementationCodeHash)), "0x")).Order("revision_no DESC").Take(&revision).Error; err != nil {
		return FairLaunchRuntimeEvidence{}, fmt.Errorf("runtime_unverified: trusted fair launch source revision unavailable: %w", err)
	}
	var cursor entity.SourceMaterializationCursor
	if err := query().Where("source_id=? AND source_revision_id=? AND status=? AND last_error=''", source.ID, revision.ID, "active").Take(&cursor).Error; err != nil {
		return FairLaunchRuntimeEvidence{}, fmt.Errorf("runtime_stale: trusted fair launch materialization cursor unavailable: %w", err)
	}
	if cursor.MaterializedThroughBlock == 0 || cursor.MaterializedThroughBlockHash == "" || cursor.UpdatedAt.IsZero() || cursor.CursorGeneration == 0 {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_stale: materialization cursor is incomplete")
	}
	var coverage entity.SourceIngestionCoverage
	if err := query().Where("source_id=? AND finalized=1 AND through_block>=?", source.ID, cursor.MaterializedThroughBlock).Order("through_block DESC").Take(&coverage).Error; err != nil {
		return FairLaunchRuntimeEvidence{}, fmt.Errorf("runtime_stale: trusted fair launch finalized coverage unavailable: %w", err)
	}
	var binding fairLaunchRegistryBinding
	proxyAddress, implementationAddress, implementationCodeHash, abiDigest := registrySnapshotLookupIdentity(r, revision)
	if err := query().Table("fair_launch_source_registry_snapshot").Where("environment=? AND chain_id=? AND proxy_address=? AND implementation_address=? AND implementation_code_hash=? AND abi_digest=? AND source_id=? AND source_revision_id=? AND source_revision_no=? AND valid_from<=? AND (valid_until IS NULL OR valid_until>?)", environment, r.ChainID, proxyAddress, implementationAddress, implementationCodeHash, abiDigest, source.ID, revision.ID, revision.RevisionNo, now, now).Order("revision DESC").Take(&binding).Error; err != nil {
		return FairLaunchRuntimeEvidence{}, fmt.Errorf("runtime_stale: exact trusted source registry binding unavailable: %w", err)
	}
	if err := validateRegistryBinding(binding, source, revision, r, environment); err != nil {
		return FairLaunchRuntimeEvidence{}, err
	}
	if coverage.ThroughBlock < cursor.MaterializedThroughBlock {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_stale: finalized coverage trails materialization cursor")
	}
	identity := FairLaunchPublicRuntimeIdentity{
		ChainID: r.ChainID, ProxyAddress: prefixedHex(r.ContractAddress), ImplementationAddress: prefixedHex(revision.ImplementationAddress), ImplementationCodeHash: prefixedHex(revision.ImplementationCodeHash),
		SourceRegistryRevision: binding.Revision, SourceRegistryDigest: prefixedHex(binding.PayloadDigest), ObservedBlockNumber: strconv.FormatUint(cursor.MaterializedThroughBlock, 10), ObservedBlockHash: prefixedHex(cursor.MaterializedThroughBlockHash), ObservedAt: cursor.UpdatedAt.UTC().Format(time.RFC3339), RuntimeVerified: true,
	}
	generations := FairLaunchActionGenerations{Runtime: cursor.CursorGeneration, Policy: binding.PolicyGeneration, SourceRegistry: binding.SourceRegistryGeneration, Config: binding.ConfigGeneration}
	return FairLaunchRuntimeEvidence{Runtime: identity, Generations: generations, Environment: environment, SourceID: source.ID, RevisionID: revision.ID, Source: map[string]any{"finality": "finalized", "stateSource": "canonical_chain_event", "observedAt": identity.ObservedAt, "sourceRevision": revision.RevisionNo, "abiDigest": prefixedHex(revision.ABIDigest)}}, nil
}

func registrySnapshotLookupIdentity(r projectruntime.Runtime, revision entity.ChainContractSourceRevision) (string, string, string, string) {
	return strings.ToLower(prefixedHex(r.ContractAddress)),
		strings.ToLower(prefixedHex(revision.ImplementationAddress)),
		strings.ToLower(prefixedHex(revision.ImplementationCodeHash)),
		strings.ToLower(prefixedHex(revision.ABIDigest))
}
