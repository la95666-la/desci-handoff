package chainevent

import (
	"bytes"
	"context"
	"crypto/sha256"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestFairLaunchLocalCapabilityZeroValueFailsClosedBeforeDatabase(t *testing.T) {
	_, err := ApplyFairLaunchCEV2LocalTx(FairLaunchLocalE2ECapability{}, nil, FairLaunchCEWrite{}, time.Now())
	if err == nil || err.Error() != FairLaunchLocalE2ECapabilityRequired {
		t.Fatalf("zero local capability escaped: %v", err)
	}
}

func TestFairLaunchLocalAdmissionRejectsUnavailableTransportBeforeDatabase(t *testing.T) {
	_, err := ApplyFairLaunchCEV2LocalTx(fairLaunchLocalE2ETestCapability(), nil, FairLaunchCEWrite{}, time.Now())
	if err == nil || err.Error() != FairLaunchLocalE2EForwardUnavailable {
		t.Fatalf("local V3 forward path did not fail closed: %v", err)
	}
}

func TestFairLaunchLocalTransportErrorIsDistinctFromProductionError(t *testing.T) {
	if FairLaunchLocalE2ETransportAdmissionUnavailable == "fair_launch_materialization_unavailable" {
		t.Fatal("local transport blocker must not alias the production shutdown error")
	}
}

func TestDecodeFairLaunchLocalE2EAdmissionBundleCanonicalMatrix(t *testing.T) {
	bundle := fairLaunchLocalBundleFixture()
	raw, err := jsonv2.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFairLaunchLocalE2EAdmissionBundle(bytes.NewReader(raw))
	if err != nil || decoded.RunID != bundle.RunID {
		t.Fatalf("canonical bundle rejected: decoded=%#v err=%v", decoded, err)
	}
	cases := map[string][]byte{
		"unknown member":   append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"unknown":true}`)...),
		"duplicate member": append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"runId":"123e4567-e89b-42d3-a456-426614174000"}`)...),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFairLaunchLocalE2EAdmissionBundle(bytes.NewReader(candidate)); err == nil {
				t.Fatal("invalid bundle was accepted")
			}
		})
	}
	bundle.SchemaVersion = "wrong"
	raw, _ = jsonv2.Marshal(bundle)
	if _, err := DecodeFairLaunchLocalE2EAdmissionBundle(bytes.NewReader(raw)); err == nil {
		t.Fatal("wrong bundle schema was accepted")
	}
	bundle = fairLaunchLocalBundleFixture()
	bundle.RunID = "not-a-run"
	raw, _ = jsonv2.Marshal(bundle)
	if _, err := DecodeFairLaunchLocalE2EAdmissionBundle(bytes.NewReader(raw)); err == nil {
		t.Fatal("wrong bundle run identity was accepted")
	}
	if _, err := DecodeFairLaunchLocalE2EAdmissionBundle(strings.NewReader(strings.Repeat("x", fairLaunchLocalMaxBundleBytes+1))); err == nil {
		t.Fatal("oversized bundle was accepted")
	}
}

func TestStrictUnmarshalLocalRejectsDuplicateUnknownAndNonFileBytes(t *testing.T) {
	type record struct {
		Value string `json:"value"`
	}
	for name, raw := range map[string]string{
		"missing newline":  `{"value":"ok"}`,
		"unknown member":   "{\"value\":\"ok\",\"unknown\":true}\n",
		"duplicate member": "{\"value\":\"ok\",\"value\":\"other\"}\n",
	} {
		t.Run(name, func(t *testing.T) {
			var out record
			if err := strictUnmarshalLocal(raw, &out, "test record"); err == nil {
				t.Fatal("invalid strict JSON file was accepted")
			}
		})
	}
}

func TestBundleRunRPCURLFailsClosedOnStrictAuthorityBytes(t *testing.T) {
	authority := fairLaunchRuntimeAuthority{RPCURL: "http://127.0.0.1:18545"}
	raw := testCanonicalLocalFile(t, authority)
	if got := bundleRunRPCURL(raw); got != authority.RPCURL {
		t.Fatalf("canonical authority RPC=%q", got)
	}
	for name, candidate := range map[string]string{
		"duplicate":  strings.TrimSuffix(raw, "}\n") + ",\"rpcUrl\":\"http://127.0.0.1:18545\"}\n",
		"unknown":    strings.TrimSuffix(raw, "}\n") + ",\"unknown\":true}\n",
		"no newline": strings.TrimSuffix(raw, "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := bundleRunRPCURL(candidate); got != "http://127.0.0.1:1" {
				t.Fatalf("invalid authority selected RPC %q", got)
			}
		})
	}
}

func TestParseFairLaunchLocalMySQLProfileNegativeMatrix(t *testing.T) {
	runID := "123e4567-e89b-42d3-a456-426614174000"
	valid := "[client]\n# local-only-run-id=" + runID + "\nhost=127.0.0.1\nport=24491\nprotocol=tcp\ndatabase=jc_fl_e2e_0123456789abcdef0123456789abcdef\nuser=local_job\npassword=secret\nssl-mode=DISABLED\nget-server-public-key=TRUE\nconnect-timeout=5\n"
	profile, err := parseFairLaunchLocalMySQLProfile(valid, runID, "local_job")
	if err != nil || profile.Host != "127.0.0.1" || profile.Port != 24491 || profile.User != "local_job" {
		t.Fatalf("valid profile rejected: %#v err=%v", profile, err)
	}
	cases := map[string]string{
		"duplicate":       valid + "host=127.0.0.1\n",
		"unknown":         strings.Replace(valid, "connect-timeout=5\n", "connect-timeout=5\nunknown=1\n", 1),
		"remote host":     strings.Replace(valid, "host=127.0.0.1", "host=localhost", 1),
		"wrong database":  strings.Replace(valid, "database=jc_fl_e2e_0123456789abcdef0123456789abcdef", "database=joincare", 1),
		"wrong user":      strings.Replace(valid, "user=local_job", "user=root", 1),
		"wrong run":       strings.Replace(valid, runID, "123e4567-e89b-42d3-a456-426614174001", 1),
		"small port":      strings.Replace(valid, "port=24491", "port=80", 1),
		"missing newline": strings.TrimSuffix(valid, "\n"),
		"nul byte":        valid + "\x00",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFairLaunchLocalMySQLProfile(raw, runID, "local_job"); err == nil {
				t.Fatal("invalid profile was accepted")
			}
		})
	}
}

func TestFairLaunchLocalLoopbackAndExpiryBoundaries(t *testing.T) {
	if _, err := exactFairLaunchLocalLoopbackURL("http://127.0.0.1:18545"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"http://localhost:18545", "http://[::1]:18545", "http://0.0.0.0:18545", "https://127.0.0.1:18545", "http://127.0.0.1:80", "http://127.0.0.1:18545/"} {
		if _, err := exactFairLaunchLocalLoopbackURL(value); err == nil {
			t.Fatalf("non-exact loopback URL accepted: %s", value)
		}
	}
	now := time.Now().UTC()
	capability := FairLaunchLocalE2ECapability{seal: &fairLaunchLocalE2ESeal{admission: FairLaunchLocalE2EAdmission{ExpiresAt: now.Add(time.Second)}}}
	if !capability.valid(now) || capability.valid(now.Add(time.Second)) || capability.valid(now.Add(2*time.Second)) {
		t.Fatal("capability expiry boundary is not fail closed")
	}
}

func TestVerifyFairLaunchLocalRPCNegativeMatrix(t *testing.T) {
	genesisHash := testFairLaunchLocalHash("genesis")
	tipHash := testFairLaunchLocalHash("tip")
	authority := fairLaunchRuntimeAuthority{GenesisHash: genesisHash}
	payload := fairLaunchManifestPayload{}
	payload.Deployment.Observation.BlockNumber = "39"
	payload.Deployment.Observation.BlockHash = tipHash
	payload.Deployment.Observation.BlockTimestamp = "1700000000"
	codeHash := strings.ToLower(crypto.Keccak256Hash(common.FromHex("0x6000")).Hex())
	payload.Assets.PaymentToken.RuntimeCodeHash = codeHash
	payload.Safes.Proceeds.RuntimeCodeHash = codeHash
	payload.Safes.PlatformFee.RuntimeCodeHash = codeHash
	valid := map[string]any{
		"eth_chainId":                 "0x7a69",
		"eth_getBlockByNumber:0x0":    fairLaunchRPCBlock{Hash: genesisHash, Number: "0x0", Timestamp: "0x0"},
		"eth_getBlockByNumber:latest": fairLaunchRPCBlock{Hash: tipHash, Number: "0x27", Timestamp: "0x6553f100"},
	}
	valid["eth_getCode"] = "0x6000"
	if err := verifyFairLaunchLocalRPC(context.Background(), testFairLaunchLocalRPC{responses: valid}, authority, payload); err != nil {
		t.Fatalf("valid RPC identity rejected: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"chain id": func(rows map[string]any) { rows["eth_chainId"] = "0x1" },
		"genesis": func(rows map[string]any) {
			rows["eth_getBlockByNumber:0x0"] = fairLaunchRPCBlock{Hash: testFairLaunchLocalHash("other-genesis"), Number: "0x0", Timestamp: "0x0"}
		},
		"tip hash": func(rows map[string]any) {
			rows["eth_getBlockByNumber:latest"] = fairLaunchRPCBlock{Hash: testFairLaunchLocalHash("other-tip"), Number: "0x27", Timestamp: "0x6553f100"}
		},
		"tip number": func(rows map[string]any) {
			rows["eth_getBlockByNumber:latest"] = fairLaunchRPCBlock{Hash: tipHash, Number: "0x28", Timestamp: "0x6553f100"}
		},
		"tip timestamp": func(rows map[string]any) {
			rows["eth_getBlockByNumber:latest"] = fairLaunchRPCBlock{Hash: tipHash, Number: "0x27", Timestamp: "0x6553f101"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			rows := make(map[string]any, len(valid))
			for key, value := range valid {
				rows[key] = value
			}
			mutate(rows)
			if err := verifyFairLaunchLocalRPC(context.Background(), testFairLaunchLocalRPC{responses: rows}, authority, payload); err == nil {
				t.Fatal("RPC mismatch was accepted")
			}
		})
	}
}

func TestVerifyFairLaunchDeployEvidenceNegativeMatrix(t *testing.T) {
	runID := "123e4567-e89b-42d3-a456-426614174000"
	schemaDigest := strings.Repeat("a", 64)
	grantDigest := strings.Repeat("b", 64)
	ownerDigest := strings.Repeat("c", 64)
	containerID := strings.Repeat("d", 64)
	databaseName := fairLaunchLocalExpectedDatabase + strings.ReplaceAll(runID, "-", "")
	database := map[string]any{"authority": fairLaunchLocalAuthority, "runId": runID, "mysqlVersion": "8.4.0", "mysqlServerUuid": "01234567-89ab-cdef-0123-456789abcdef", "containerId": containerID, "mysqlImageId": "sha256:" + strings.Repeat("1", 64), "endpoint": "127.0.0.1:24491", "database": databaseName, "transportDigest": strings.Repeat("2", 64), "baselineDigest": strings.Repeat("3", 64), "migrationManifestDigest": strings.Repeat("4", 64), "schemaDigest": schemaDigest, "grantPolicyDigest": grantDigest, "ownerLabelDigest": ownerDigest, "status": "SCHEMA_STAGED_NOT_RUNTIME_ADMITTED"}
	infrastructure := map[string]any{"authority": fairLaunchLocalAuthority, "runId": runID, "status": "INFRASTRUCTURE_STARTED_NOT_ADMITTED", "mysql": map[string]any{"endpoint": "127.0.0.1:24491", "containerId": containerID, "containerPid": "100", "containerStartedAt": "2026-09-13T00:00:00Z", "imageRef": "mysql@sha256:" + strings.Repeat("1", 64), "imageId": "sha256:" + strings.Repeat("1", 64), "networkMode": "host", "publishedPortMappings": false, "database": databaseName, "serverUuid": "01234567-89ab-cdef-0123-456789abcdef", "version": "8.4.0", "reservationDigest": strings.Repeat("a", 64), "transportDigest": strings.Repeat("2", 64)}}
	container := map[string]any{"domain": "JC_FL_TEST_CONTAINER_CLEANUP_V3", "version": 3, "runId": runID, "component": "mysql", "runDir": "/tmp/run", "runDevice": 1, "runInode": 2, "runUid": 1000, "runMode": 448, "evidenceDir": "/tmp/run/evidence", "evidenceDevice": 1, "evidenceInode": 3, "evidenceUid": 1000, "evidenceMode": 448, "recordPath": "/tmp/run/evidence/docker-created.source.json", "containerId": containerID, "projectionDigest": strings.Repeat("f", 64), "projection": map[string]any{}}
	bundle := fairLaunchLocalBundleFixture()
	bundle.RunID = runID
	bundle.DatabaseSchemaEvidence = testCanonicalLocalFile(t, database)
	bundle.InfrastructureEvidence = testCanonicalLocalFile(t, infrastructure)
	bundle.ContainerAuthority = testCanonicalLocalFile(t, container)
	bundle.ExpectedJobGrants = "job\n"
	bundle.ActualJobGrants = "job\n"
	bundle.ExpectedIngestGrants = "ingest\n"
	bundle.ActualIngestGrants = "ingest\n"
	bundle.MySQLJobProfile = "job-profile\n"
	bundle.MySQLIngestProfile = "ingest-profile\n"
	readiness := map[string]any{"authority": fairLaunchLocalAuthority, "runId": runID, "transportDigest": strings.Repeat("2", 64), "mysqlImageId": "sha256:" + strings.Repeat("1", 64), "infrastructureDigest": localSHA256([]byte(bundle.InfrastructureEvidence)), "schemaEvidenceDigest": localSHA256([]byte(bundle.DatabaseSchemaEvidence)), "profileDigests": map[string]any{"runtime": strings.Repeat("7", 64), "admin": strings.Repeat("8", 64), "job": localSHA256([]byte(bundle.MySQLJobProfile)), "ingest": localSHA256([]byte(bundle.MySQLIngestProfile))}, "baselineDigest": strings.Repeat("3", 64), "migrationManifestDigest": strings.Repeat("4", 64), "schemaDigest": schemaDigest, "grantPolicyDigest": grantDigest, "ownerLabelDigest": ownerDigest, "status": "SCHEMA_READY_NOT_RUNTIME_ADMITTED", "readinessDigest": strings.Repeat("e", 64)}
	bundle.SchemaReadiness = testCanonicalLocalFile(t, readiness)
	bootstrap := fairLaunchBootstrapEvidence{RunID: runID, Database: database["database"].(string), SchemaDigest: schemaDigest, GrantPolicyDigest: grantDigest}
	if _, _, _, _, _, err := verifyFairLaunchDeployEvidence(bundle, bootstrap); err != nil {
		t.Fatalf("valid Deploy evidence rejected: %v", err)
	}
	for name, mutate := range map[string]func(*FairLaunchLocalE2EAdmissionBundle){
		"job grant mismatch":    func(candidate *FairLaunchLocalE2EAdmissionBundle) { candidate.ActualJobGrants = "other\n" },
		"ingest grant mismatch": func(candidate *FairLaunchLocalE2EAdmissionBundle) { candidate.ActualIngestGrants = "other\n" },
		"schema mismatch": func(candidate *FairLaunchLocalE2EAdmissionBundle) {
			row := cloneFairLaunchLocalMap(database)
			row["schemaDigest"] = strings.Repeat("e", 64)
			candidate.DatabaseSchemaEvidence = testCanonicalLocalFile(t, row)
		},
		"unknown evidence member": func(candidate *FairLaunchLocalE2EAdmissionBundle) {
			candidate.DatabaseSchemaEvidence = strings.TrimSuffix(candidate.DatabaseSchemaEvidence, "}\n") + ",\"unknown\":true}\n"
		},
		"duplicate evidence member": func(candidate *FairLaunchLocalE2EAdmissionBundle) {
			candidate.DatabaseSchemaEvidence = strings.TrimSuffix(candidate.DatabaseSchemaEvidence, "}\n") + ",\"runId\":\"" + runID + "\"}\n"
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := bundle
			mutate(&candidate)
			if _, _, _, _, _, err := verifyFairLaunchDeployEvidence(candidate, bootstrap); err == nil {
				t.Fatal("Deploy evidence mismatch was accepted")
			}
		})
	}
}

type testFairLaunchLocalRPC struct {
	responses map[string]any
}

func (rpc testFairLaunchLocalRPC) Call(_ context.Context, method string, params any, target any) error {
	key := method
	if method == "eth_getBlockByNumber" {
		values, ok := params.([]any)
		if !ok || len(values) == 0 {
			return errors.New("invalid params")
		}
		key += ":" + fmt.Sprint(values[0])
	}
	value, ok := rpc.responses[key]
	if !ok {
		return errors.New("missing response")
	}
	raw, err := jsonv2.Marshal(value)
	if err != nil {
		return err
	}
	return jsonv2.Unmarshal(raw, target)
}

func testFairLaunchLocalHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "0x" + fmt.Sprintf("%x", digest[:])
}

func testCanonicalLocalFile(t *testing.T, value any) string {
	t.Helper()
	raw, err := canonicalLocalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw) + "\n"
}

func cloneFairLaunchLocalMap(value map[string]any) map[string]any {
	clone := make(map[string]any, len(value))
	for key, item := range value {
		clone[key] = item
	}
	return clone
}

func fairLaunchLocalBundleFixture() FairLaunchLocalE2EAdmissionBundle {
	return FairLaunchLocalE2EAdmissionBundle{
		SchemaVersion:            fairLaunchLocalBundleSchema,
		RunID:                    "123e4567-e89b-42d3-a456-426614174000",
		ContractRuntimeAdmission: "runtime\n",
		DeploymentManifest:       "manifest\n",
		ExternalTrustPin:         "pin\n",
		PublicTrustRoot:          "trust\n",
		BackendBootstrap:         "bootstrap\n",
		DatabaseSchemaEvidence:   "database\n",
		SchemaReadiness:          "readiness\n",
		InfrastructureEvidence:   "infrastructure\n",
		ContainerAuthority:       "container\n",
		ExpectedJobGrants:        "job\n",
		ActualJobGrants:          "job\n",
		ExpectedIngestGrants:     "ingest\n",
		ActualIngestGrants:       "ingest\n",
		MySQLJobProfile:          "job-profile\n",
		MySQLIngestProfile:       "ingest-profile\n",
	}
}

func fairLaunchLocalE2ETestCapability() FairLaunchLocalE2ECapability {
	return FairLaunchLocalE2ECapability{seal: &fairLaunchLocalE2ESeal{admission: FairLaunchLocalE2EAdmission{ExpiresAt: time.Now().Add(time.Hour)}, rpc: testUnavailableLocalRPC{}}}
}

type testUnavailableLocalRPC struct{}

func (testUnavailableLocalRPC) Call(context.Context, string, any, any) error {
	return errors.New("unavailable")
}
