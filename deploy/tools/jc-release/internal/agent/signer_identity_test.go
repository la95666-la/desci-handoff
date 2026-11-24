package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joincare/jc-release/internal/activation"
)

func TestPlaintextSignerDerivationAndSignedIdentity(t *testing.T) {
	// Ethereum's published private-key-one address, not a test private key
	// admitted to production. No private material is logged by this test.
	address, err := addressFromPlaintextKey(strings.Repeat("0", 63) + "1")
	if err != nil || !strings.EqualFold(address, "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf") {
		t.Fatalf("secp256k1 address derivation failed: %v", err)
	}
	for _, value := range []string{"0x1234", strings.Repeat("0", 64), strings.Repeat("f", 64), strings.Repeat("z", 64)} {
		if _, err := addressFromPlaintextKey(value); err == nil {
			t.Fatal("invalid scalar accepted")
		}
	}
	root := t.TempDir()
	job := filepath.Join(root, "job-runtime", "config", "secret", "config.yaml")
	core := filepath.Join(root, "config", "secret", "config.yaml")
	config := filepath.Join(root, "job-runtime", "config", "job", "config.yaml")
	for _, path := range []string{job, core, config} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	jobKey := strings.Repeat("1", 64)
	coreKey := strings.Repeat("2", 64)
	jobAddr, _ := addressFromPlaintextKey(jobKey)
	coreAddr, _ := addressFromPlaintextKey(coreKey)
	write(job, "chain:\n  project:\n    jobPrivate: "+jobKey+"\n")
	write(core, "chain:\n  project:\n    private: "+coreKey+"\n")
	contract := "0x0000000000000000000000000000000000000002"
	configBody := "chain:\n  project:\n    chainID: 56\n    v2:\n      contractAddress: " + contract + "\n      coreOperatorAddress: " + coreAddr + "\n"
	write(config, configBody)
	claim := activation.Readiness{JobSignerAddress: jobAddr, CoreOperatorAddress: coreAddr, V2ContractAddress: contract, ChainID: 56}
	if err := checkSeparateJobSigner(root); err != nil {
		t.Fatalf("valid independent Job signer rejected: %v", err)
	}
	if err := checkSignedJobAddresses(root, claim); err != nil {
		t.Fatalf("valid signed identities rejected: %v", err)
	}
	// Core private material need not be mounted or even present on the Job
	// host. Public Core identity is separately evidenced by signed readiness.
	write(core, "chain: {}\n")
	if err := checkSeparateJobSigner(root); err != nil {
		t.Fatalf("Core private incorrectly required at early gate: %v", err)
	}
	if err := checkSignedJobAddresses(root, claim); err != nil {
		t.Fatalf("public Core identity rejected without Core private: %v", err)
	}
	if err := os.Remove(core); err != nil {
		t.Fatal(err)
	}
	if err := checkSignedJobAddresses(root, claim); err != nil {
		t.Fatalf("Core secret file incorrectly required: %v", err)
	}
	claim.JobSignerAddress = coreAddr
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("signed Job address mismatches mounted signer")
	}
	claim.JobSignerAddress = jobAddr
	claim.CoreOperatorAddress = jobAddr
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("signed Core operator mismatches Core signer/config")
	}
	claim.CoreOperatorAddress = coreAddr
	write(config, strings.Replace(configBody, coreAddr, jobAddr, 1))
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("wrong configured operator admitted")
	}
	write(config, configBody)
	claim.V2ContractAddress = "0x0000000000000000000000000000000000000003"
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("wrong signed contract address admitted")
	}
	claim.V2ContractAddress = contract
	claim.ChainID = 97
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("wrong signed chain ID admitted")
	}
	claim.ChainID = 56
	claim.CoreOperatorAddress = "0x" + strings.Repeat("0", 40)
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("zero Core operator admitted")
	}
	claim.CoreOperatorAddress = coreAddr
	claim.CoreOperatorAddress = "0x" + strings.Repeat("g", 40)
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("malformed Core operator admitted")
	}
	claim.CoreOperatorAddress = coreAddr
	write(job, "chain:\n  project:\n    jobPrivate: "+jobKey+"\nCHAIN:\n  PROJECT:\n    JOBPRIVATE: "+coreKey+"\n")
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("case-folded Job signer override admitted")
	}
	write(job, "chain:\n  project:\n    jobPrivate: "+jobKey+"\n")
	write(job, "chain:\n  project:\n    jobPrivate: "+jobKey+"\n    private: "+coreKey+"\n")
	if err := checkSignedJobAddresses(root, claim); err == nil {
		t.Fatal("Core signer leaked into Job secret")
	}
	if err := checkSeparateJobSigner(root); err == nil {
		t.Fatal("early gate admitted Core signer in Job secret")
	}
}
