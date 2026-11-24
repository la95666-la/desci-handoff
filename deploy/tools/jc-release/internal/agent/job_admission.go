package agent

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/joincare/jc-release/internal/activation"
	"github.com/joincare/jc-release/internal/shared"
	"github.com/joincare/jc-release/internal/strategies"
	"golang.org/x/crypto/sha3"
	"gopkg.in/yaml.v3"
)

var jobExecutionMu sync.Mutex

// jobDescriptor is a checksum-bound deployment instruction, not an assertion
// that independent reviewers examined production. The existing Job layout,
// signer separation, Docker image identity and rollback checks still apply.
type jobDescriptor struct {
	Environment    string `json:"environment"`
	Component      string `json:"component"`
	Generation     uint64 `json:"generation"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	ImageID        string `json:"image_id"`
}

func (u *Upgrader) admitJobDescriptor(rel *shared.Release, minGeneration uint64) (uint64, error) {
	var descriptor jobDescriptor
	decoder := json.NewDecoder(bytes.NewReader(rel.JobAdmission))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&descriptor) != nil {
		return 0, errors.New("job_descriptor_invalid")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return 0, errors.New("job_descriptor_invalid")
	}
	canonical, err := json.Marshal(descriptor)
	if err != nil || !bytes.Equal(rel.JobAdmission, canonical) ||
		descriptor.Environment != u.JobEnvironment || descriptor.Component != string(shared.CompJob) ||
		(descriptor.Generation != 0 && descriptor.Generation != minGeneration) || descriptor.ArtifactSHA256 != rel.SHA256 ||
		!validJobDescriptorImage(descriptor.ImageID) {
		return 0, errors.New("job_descriptor_invalid")
	}
	strat, err := u.Registry.Get(shared.CompJob)
	if err != nil {
		return 0, err
	}
	setter, ok := strat.(interface{ SetJobImageID(string) error })
	if !ok {
		return 0, errors.New("job_strategy_unavailable")
	}
	if err := setter.SetJobImageID(descriptor.ImageID); err != nil {
		return 0, err
	}
	return minGeneration, nil
}

func validJobDescriptorImage(image string) bool {
	if !strings.HasPrefix(image, "sha256:") || len(image) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(image, "sha256:"))
	return err == nil && image == strings.ToLower(image)
}

// admitJob validates a checksum-bound image identity before download or Docker mutation.
// The strategy checks the existing rollback target and the loaded image ID.
func (u *Upgrader) admitJob(rel *shared.Release) (uint64, error) {
	if u.Root == "" || u.Store == nil || u.DataDir == "" || u.JobEnvironment == "" {
		return 0, errors.New("job_trust_configuration_unavailable")
	}
	state, err := u.Store.Load()
	if err != nil {
		return 0, err
	}
	if state.JobPending != nil && (u.jobRecovery == nil || !u.jobRecovery.matchesState(state)) {
		return 0, errors.New("job_pending_recovery_required")
	}
	if err := strategies.CheckIDOJobLayout(u.Root); err != nil {
		return 0, err
	}
	if err := checkSeparateJobSigner(u.Root); err != nil {
		return 0, err
	}
	if state.JobGeneration == ^uint64(0) {
		return 0, errors.New("job_generation_exhausted")
	}
	// New releases use an unsigned, canonical checksum/image descriptor.
	// Legacy signed releases remain verifiable without weakening their own
	// readiness contract. In either case layout and signer isolation are checked.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rel.JobAdmission, &fields); err != nil {
		return 0, errors.New("job_descriptor_invalid")
	}
	if _, signed := fields["declaration"]; !signed {
		return u.admitJobDescriptor(rel, state.JobGeneration+1)
	}
	if len(u.JobTrustedKeys) < 3 {
		return 0, errors.New("job_trust_configuration_unavailable")
	}
	keys := make(map[string]ed25519.PublicKey, len(u.JobTrustedKeys))
	seenKeys := map[string]bool{}
	for id, encoded := range u.JobTrustedKeys {
		pub, err := hex.DecodeString(encoded)
		if id == "" || err != nil || len(pub) != ed25519.PublicKeySize || seenKeys[string(pub)] {
			return 0, errors.New("job_trusted_key_invalid")
		}
		seenKeys[string(pub)] = true
		keys[id] = pub
	}
	var envelope activation.ArtifactEnvelope
	if err := json.Unmarshal(rel.JobAdmission, &envelope); err != nil {
		return 0, errors.New("job_admission_invalid")
	}
	var claim activation.ArtifactDeclaration
	if err := json.Unmarshal(envelope.Declaration, &claim); err != nil {
		return 0, errors.New("job_admission_invalid")
	}
	want := activation.ArtifactExpectation{Environment: u.JobEnvironment, Component: string(shared.CompJob), ArtifactSHA256: rel.SHA256,
		ImageID: claim.ImageID, MinGeneration: state.JobGeneration + 1, Now: time.Now().UTC()}
	verified, err := activation.VerifiedDeclaration(rel.JobAdmission, keys, want)
	if err != nil {
		return 0, err
	}
	if err := u.verifyJobReadiness(rel, verified.Generation, verified.ImageID); err != nil {
		return 0, err
	}
	strat, err := u.Registry.Get(shared.CompJob)
	if err != nil {
		return 0, err
	}
	setter, ok := strat.(interface{ SetJobImageID(string) error })
	if !ok {
		return 0, errors.New("job_strategy_unavailable")
	}
	if err := setter.SetJobImageID(verified.ImageID); err != nil {
		return 0, err
	}
	// Docker is inspected at Backup/Apply time; first-deploy failure must
	// terminate in a verified, absent Job rather than a fictitious old image.
	return verified.Generation, nil
}

func (u *Upgrader) verifyJobReadiness(rel *shared.Release, generation uint64, imageID string) error {
	if u.JobReadinessFile == "" || !filepath.IsAbs(u.JobReadinessFile) || u.JobEvidenceDir == "" || !filepath.IsAbs(u.JobEvidenceDir) || u.JobHostKeySHA256 == "" || len(u.JobReadinessKeys) < 3 || u.AgentID == "" {
		return errors.New("job_readiness_unavailable")
	}
	if filepath.Clean(u.JobReadinessFile) != u.JobReadinessFile || strings.HasPrefix(u.JobReadinessFile, u.Root+string(os.PathSeparator)) ||
		strings.HasPrefix(u.JobReadinessFile, u.DataDir+string(os.PathSeparator)) {
		return errors.New("job_readiness_path_unsafe")
	}
	info, err := os.Lstat(u.JobReadinessFile)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return errors.New("job_readiness_file_unsafe")
	}
	if runtime.GOOS == "linux" {
		if err := requireRootAgent(); err != nil {
			return err
		}
		if err := requireTrustedReadinessParents(u.JobReadinessFile); err != nil {
			return errors.New("job_readiness_directory_unsafe")
		}
		if err := requireRootOwnedReadiness(info); err != nil {
			return errors.New("job_readiness_file_unsafe")
		}
		hostKeyFile := u.jobHostKeyFile
		if hostKeyFile == "" {
			hostKeyFile = "/etc/ssh/ssh_host_ed25519_key.pub"
		}
		if err := verifySSHHostKeyFile(hostKeyFile, u.JobHostKeySHA256); err != nil {
			return err
		}
	}
	data, err := readProtectedJobFile(u.JobReadinessFile, 64*1024)
	if err != nil {
		return errors.New("job_readiness_unavailable")
	}
	keys := make(map[string]ed25519.PublicKey, len(u.JobReadinessKeys))
	seen := map[string]bool{}
	for id, encoded := range u.JobReadinessKeys {
		key, err := hex.DecodeString(encoded)
		if id == "" || err != nil || len(key) != ed25519.PublicKeySize || seen[string(key)] || u.JobTrustedKeys[id] != "" {
			return errors.New("job_readiness_keys_invalid")
		}
		for _, artifactKey := range u.JobTrustedKeys {
			if strings.EqualFold(artifactKey, encoded) {
				return errors.New("job_readiness_keys_not_independent")
			}
		}
		seen[string(key)] = true
		keys[id] = key
	}
	claim, err := activation.ParseAndVerifyReadiness(data, keys, activation.ReadinessExpectation{
		Environment: u.JobEnvironment, AgentID: u.AgentID, Root: u.Root,
		HostKeySHA256: u.JobHostKeySHA256, ReleaseSHA256: rel.SHA256, ArtifactImageID: imageID,
		ReleaseID: rel.ID, Generation: generation, Now: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if err := checkSignedJobAddresses(u.Root, claim); err != nil {
		return err
	}
	return u.verifyJobReceipts(claim)
}

func (u *Upgrader) verifyJobReceipts(claim activation.Readiness) error {
	if filepath.Clean(u.JobEvidenceDir) != u.JobEvidenceDir ||
		strings.HasPrefix(u.JobEvidenceDir, u.Root+string(os.PathSeparator)) ||
		strings.HasPrefix(u.JobEvidenceDir, u.DataDir+string(os.PathSeparator)) {
		return errors.New("job_evidence_path_unsafe")
	}
	dir, err := os.Lstat(u.JobEvidenceDir)
	if err != nil || !dir.IsDir() || (runtime.GOOS != "windows" && dir.Mode().Perm()&0022 != 0) {
		return errors.New("job_evidence_directory_unsafe")
	}
	if runtime.GOOS == "linux" {
		if err := requireRootAgent(); err != nil {
			return err
		}
		if err := requireTrustedReadinessParents(u.JobEvidenceDir); err != nil {
			return errors.New("job_evidence_directory_unsafe")
		}
		if err := requireRootOwnedReadiness(dir); err != nil {
			return errors.New("job_evidence_directory_unsafe")
		}
	}
	checks := map[string]string{
		"schema": claim.SchemaEvidenceSHA256, "backup": claim.BackupReceiptSHA256,
		"restore": claim.RestoreEvidenceSHA256, "signer": claim.SignerEvidenceSHA256,
		"chain-rpc": claim.ChainRPCEvidenceSHA256, "permissions": claim.PermissionsEvidenceSHA256,
	}
	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(u.JobEvidenceDir, name+".receipt")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
			return errors.New("job_evidence_receipt_unsafe")
		}
		if runtime.GOOS == "linux" {
			if err := requireRootOwnedReadiness(info); err != nil {
				return errors.New("job_evidence_receipt_unsafe")
			}
		}
		body, err := readProtectedJobFile(path, 1024*1024)
		if err != nil || len(body) == 0 {
			return errors.New("job_evidence_receipt_unavailable")
		}
		digest := sha256.Sum256(body)
		if hex.EncodeToString(digest[:]) != checks[name] {
			return errors.New("job_evidence_digest_mismatch")
		}
		if err := activation.VerifyEvidenceReceipt(body, name, claim, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func checkSeparateJobSigner(root string) error {
	jobData, err := os.ReadFile(filepath.Join(root, "job-runtime", "config", "secret", "config.yaml"))
	if err != nil {
		return errors.New("job_signer_unavailable")
	}
	var job struct {
		Chain struct {
			Project struct {
				JobPrivate string `yaml:"jobPrivate"`
				Private    string `yaml:"private"`
			} `yaml:"project"`
		} `yaml:"chain"`
	}
	if yaml.Unmarshal(jobData, &job) != nil ||
		!canonicalSignerPath(jobData, "chain", "project", "jobPrivate") ||
		job.Chain.Project.Private != "" {
		return errors.New("signer_identity_invalid")
	}
	// The Core signer can be external to this host. Its public identity is
	// independently bound to the signed readiness and signer receipt below.
	if _, err := addressFromPlaintextKey(job.Chain.Project.JobPrivate); err != nil {
		return errors.New("job_signer_identity_unavailable")
	}
	return nil
}

// Verify the Job signer from its isolated private scalar. Bind the public Core
// operator to the signed assertion and canonical Job config without requiring
// the Core private scalar on this host. This does NOT prove who controls Core's
// key or the live on-chain operator(): independent signer and chain receipts
// must substantiate those facts before a readiness assertion is signed.
func checkSignedJobAddresses(root string, claim activation.Readiness) error {
	jobSecret, err := os.ReadFile(filepath.Join(root, "job-runtime", "config", "secret", "config.yaml"))
	if err != nil {
		return errors.New("job_signer_identity_unavailable")
	}
	jobConfig, err := os.ReadFile(filepath.Join(root, "job-runtime", "config", "job", "config.yaml"))
	if err != nil {
		return errors.New("job_core_operator_unavailable")
	}
	var job struct {
		Chain struct {
			Project struct {
				JobPrivate string `yaml:"jobPrivate"`
				Private    string `yaml:"private"`
			} `yaml:"project"`
		} `yaml:"chain"`
	}
	var config struct {
		Chain struct {
			Project struct {
				ChainID uint64 `yaml:"chainID"`
				V2      struct {
					CoreOperatorAddress string `yaml:"coreOperatorAddress"`
					ContractAddress     string `yaml:"contractAddress"`
				} `yaml:"v2"`
			} `yaml:"project"`
		} `yaml:"chain"`
	}
	if yaml.Unmarshal(jobSecret, &job) != nil || yaml.Unmarshal(jobConfig, &config) != nil {
		return errors.New("job_signer_identity_invalid")
	}
	// Viper treats keys case-insensitively. A second differently cased field
	// could change the effective signer even though the typed YAML value above
	// still matches the signed address. Reject ambiguous spellings outright.
	if !canonicalSignerPath(jobSecret, "chain", "project", "jobPrivate") ||
		job.Chain.Project.Private != "" ||
		!canonicalSignerPath(jobConfig, "chain", "project", "v2", "coreOperatorAddress") ||
		!canonicalSignerPath(jobConfig, "chain", "project", "v2", "contractAddress") ||
		!canonicalSignerPath(jobConfig, "chain", "project", "chainID") {
		return errors.New("job_signer_identity_ambiguous")
	}
	jobAddress, err := addressFromPlaintextKey(job.Chain.Project.JobPrivate)
	if err != nil || !strings.EqualFold(jobAddress, claim.JobSignerAddress) {
		return errors.New("job_signer_address_mismatch")
	}
	if !validEthereumAddress(claim.CoreOperatorAddress) ||
		!validEthereumAddress(claim.V2ContractAddress) ||
		!strings.EqualFold(config.Chain.Project.V2.CoreOperatorAddress, claim.CoreOperatorAddress) ||
		!strings.EqualFold(config.Chain.Project.V2.ContractAddress, claim.V2ContractAddress) ||
		config.Chain.Project.ChainID != claim.ChainID ||
		strings.EqualFold(jobAddress, claim.CoreOperatorAddress) {
		return errors.New("core_operator_address_mismatch")
	}
	return nil
}

func validEthereumAddress(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") || strings.EqualFold(address, "0x"+strings.Repeat("0", 40)) {
		return false
	}
	_, err := hex.DecodeString(address[2:])
	return err == nil
}

func canonicalSignerPath(data []byte, path ...string) bool {
	var values map[string]any
	if yaml.Unmarshal(data, &values) != nil {
		return false
	}
	for i, expected := range path {
		matches := 0
		var value any
		for key, candidate := range values {
			if strings.EqualFold(key, expected) {
				if key != expected {
					return false
				}
				matches++
				value = candidate
			}
		}
		if matches != 1 || value == nil {
			return false
		}
		if i == len(path)-1 {
			return true
		}
		var ok bool
		values, ok = value.(map[string]any)
		if !ok {
			return false
		}
	}
	return false
}

func addressFromPlaintextKey(value string) (string, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if len(value) != 64 {
		return "", errors.New("not a plaintext private key")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return "", err
	}
	defer clear(decoded)
	var scalar secp256k1.ModNScalar
	defer scalar.Zero()
	if scalar.SetByteSlice(decoded) || scalar.IsZero() {
		return "", errors.New("invalid secp256k1 scalar")
	}
	key := secp256k1.NewPrivateKey(&scalar)
	defer key.Zero()
	pub := key.PubKey().SerializeUncompressed()
	h := sha3.NewLegacyKeccak256()
	_, _ = h.Write(pub[1:])
	return "0x" + hex.EncodeToString(h.Sum(nil)[12:]), nil
}
