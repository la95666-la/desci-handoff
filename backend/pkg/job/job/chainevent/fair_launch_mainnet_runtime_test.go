package chainevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
)

func TestStartFairLaunchMainnetRuntimeDefaultsDisabled(t *testing.T) {
	viper.Reset()
	resetFairLaunchMainnetRuntimeStateForTest()
	if err := StartFairLaunchMainnetRuntime(context.Background(), nil); err != nil {
		t.Fatalf("disabled runtime failed: %v", err)
	}
	status := FairLaunchMainnetRuntimeStatusSnapshot()
	if status.Enabled || status.Lifecycle != "disabled" || status.Scanner.Running {
		t.Fatalf("unexpected disabled status: %+v", status)
	}
}

func TestStartFairLaunchMainnetRuntimeEnabledFailsClosedBeforeDial(t *testing.T) {
	viper.Reset()
	resetFairLaunchMainnetRuntimeStateForTest()
	viper.Set("fairLaunchMainnet.scanner.enabled", true)
	viper.Set("fairLaunchMainnet.scanner.exclusiveJobMode", true)
	if err := StartFairLaunchMainnetRuntime(context.Background(), nil); err == nil || err.Error() != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("enabled runtime without database admitted: %v", err)
	}
	status := FairLaunchMainnetRuntimeStatusSnapshot()
	if !status.Enabled || status.Lifecycle != "unavailable" || status.ReasonCode != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("unexpected unavailable status: %+v", status)
	}
}

func TestStartFairLaunchMainnetRuntimeRequiresExclusiveJobMode(t *testing.T) {
	viper.Reset()
	resetFairLaunchMainnetRuntimeStateForTest()
	viper.Set("fairLaunchMainnet.scanner.enabled", true)
	if err := StartFairLaunchMainnetRuntime(context.Background(), nil); err == nil || err.Error() != FairLaunchMainnetRuntimeUnavailable {
		t.Fatalf("non-exclusive runtime admitted: %v", err)
	}
	status := FairLaunchMainnetRuntimeStatusSnapshot()
	if status.Lifecycle != "unavailable" || status.ReasonCode != FairLaunchMainnetRuntimeUnavailable {
		t.Fatalf("unexpected non-exclusive status: %+v", status)
	}
}

func TestLoadFairLaunchMainnetRuntimeManifestChecksumAndUnknownField(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.json")
	raw := testFairLaunchMainnetRuntimeManifestJSON("")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(raw))
	viper.Set("fairLaunchMainnet.scanner.admissionManifestPath", path)
	viper.Set("fairLaunchMainnet.scanner.admissionManifestSHA256", hex.EncodeToString(digest[:]))
	manifest, actual, err := loadFairLaunchMainnetRuntimeManifest()
	if err != nil || actual != hex.EncodeToString(digest[:]) || manifest.Component != "job" || manifest.ProviderIdentity != "provider-a" {
		t.Fatalf("valid manifest rejected: manifest=%+v digest=%s err=%v", manifest, actual, err)
	}
	if err := os.WriteFile(path, []byte(testFairLaunchMainnetRuntimeManifestJSON(",\"unexpected\":true")), 0o600); err != nil {
		t.Fatal(err)
	}
	tampered, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tamperedDigest := sha256.Sum256(tampered)
	viper.Set("fairLaunchMainnet.scanner.admissionManifestSHA256", hex.EncodeToString(tamperedDigest[:]))
	if _, _, err := loadFairLaunchMainnetRuntimeManifest(); err == nil || err.Error() != FairLaunchMainnetRuntimeUnavailable {
		t.Fatalf("unknown field admitted: %v", err)
	}
}

func TestLoadFairLaunchMainnetSecretFiles(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "lease")
	rpcPath := filepath.Join(dir, "rpc")
	if err := os.WriteFile(leasePath, []byte(strings.Repeat("x", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rpcPath, []byte("https://rpc.example.invalid/opaque\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	viper.Set("fairLaunchMainnet.scanner.leaseTokenFile", leasePath)
	viper.Set("fairLaunchMainnet.scanner.rpcEndpointFile", rpcPath)
	lease, err := loadFairLaunchMainnetLeaseToken()
	if err != nil || string(lease) != strings.Repeat("x", 32) {
		t.Fatalf("lease rejected: %v", err)
	}
	endpoint, err := loadFairLaunchMainnetRPCEndpoint()
	if err != nil || string(endpoint) != "https://rpc.example.invalid/opaque" {
		t.Fatalf("endpoint rejected: %v", err)
	}
	zeroFairLaunchSecret(lease)
	zeroFairLaunchSecret(endpoint)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(leasePath, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFairLaunchMainnetLeaseToken(); err == nil {
			t.Fatal("world-readable lease admitted")
		}
	}
}

func TestFairLaunchMainnetGethCallerForwardsArguments(t *testing.T) {
	client := &testFairLaunchMainnetCallContext{}
	caller := fairLaunchMainnetGethCaller{client: client}
	var target string
	if err := caller.Call(context.Background(), "eth_getBlockByNumber", []any{"latest", false}, &target); err != nil {
		t.Fatal(err)
	}
	if client.method != "eth_getBlockByNumber" || len(client.arguments) != 2 || client.arguments[0] != "latest" || client.arguments[1] != false {
		t.Fatalf("unexpected forwarding: %+v", client)
	}
}

type testFairLaunchMainnetCallContext struct {
	method    string
	arguments []interface{}
}

func (client *testFairLaunchMainnetCallContext) CallContext(_ context.Context, _ interface{}, method string, arguments ...interface{}) error {
	client.method = method
	client.arguments = arguments
	return nil
}

func resetFairLaunchMainnetRuntimeStateForTest() {
	fairLaunchMainnetRuntimeState.Lock()
	fairLaunchMainnetRuntimeState.runtime = nil
	fairLaunchMainnetRuntimeState.enabled = false
	fairLaunchMainnetRuntimeState.lifecycle = "disabled"
	fairLaunchMainnetRuntimeState.reason = ""
	fairLaunchMainnetRuntimeState.stoppedAt = nil
	fairLaunchMainnetRuntimeState.Unlock()
}

func testFairLaunchMainnetRuntimeManifestJSON(extra string) string {
	return `{"version":1,"activationId":"12345678-1234-4abc-8def-1234567890ab","component":"job","gitCommit":"` + strings.Repeat("a", 40) + `","artifactSha256":"` + strings.Repeat("b", 64) + `","componentSetDigest":"` + strings.Repeat("c", 64) + `","releaseManifestDigest":"` + strings.Repeat("d", 64) + `","migrationManifestDigest":"` + strings.Repeat("e", 64) + `","activationManifestDigest":"` + strings.Repeat("f", 64) + `","proxyAddress":"0x1111111111111111111111111111111111111111","proxyRuntimeCodeHash":"` + strings.Repeat("1", 64) + `","implementationAddress":"0x2222222222222222222222222222222222222222","implementationCodeHash":"` + strings.Repeat("2", 64) + `","abiDigest":"` + strings.Repeat("3", 64) + `","sourceId":11,"sourceRevisionId":12,"providerIdentity":"provider-a"` + extra + `}`
}
