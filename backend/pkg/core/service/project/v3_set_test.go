package project

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func testV3SetManifest() idov3whitelist.SetManifest {
	a := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	h := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	return idov3whitelist.SetManifest{ChainID: 31337, ProjectID: 19, FundingMode: constant.FundingModeCappedPresale, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		ProxyAddress: a("1"), Implementation: a("2"), OwnerSafe: a("3"), Operator: a("9"), Governor: a("4"), ProjectToken: a("5"),
		ProjectOwner: a("6"), PaymentToken: a("7"), FeeReceiver: a("8"), PaymentSymbol: "USDT", PaymentDecimals: 18,
		FeeRate: 200, StartBlock: 90, Confirmations: 3, ExpiresAt: time.Now().Add(time.Hour).Unix(),
		ProxyCodeHash: h("a"), ImplementationCodeHash: h("b"), OwnerCodeHash: h("c"), GovernorCodeHash: h("d"),
		RequestDigest: h("e")}
}

func TestV3SetAdmissionConfigRejectsMissingAndUnknownFields(t *testing.T) {
	const prefix = "idoV3Set."
	for _, key := range []string{"pinnedPublicKey", "signature", "manifest"} {
		old := viper.Get(keyWithPrefix(prefix, key))
		k := keyWithPrefix(prefix, key)
		t.Cleanup(func() { viper.Set(k, old) })
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	viper.Set(prefix+"pinnedPublicKey", base64.StdEncoding.EncodeToString(pub))
	viper.Set(prefix+"signature", base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte("not a manifest"))))
	viper.Set(prefix+"manifest", `{"chain_id":31337,"unexpected":true}`)
	if _, _, _, err := pinnedV3SetAdmission(); err == nil {
		t.Fatal("unknown manifest field accepted")
	}
	viper.Set(prefix+"manifest", `{"chain_id":31337} {}`)
	if _, _, _, err := pinnedV3SetAdmission(); err == nil {
		t.Fatal("concatenated manifests accepted")
	}
	viper.Set(prefix+"manifest", `{"chain_id":31337}`)
	if _, _, _, err := pinnedV3SetAdmission(); err != nil {
		t.Fatal(err)
	}
	viper.Set(prefix+"pinnedPublicKey", "invalid")
	if _, _, _, err := pinnedV3SetAdmission(); err == nil {
		t.Fatal("missing verifier accepted")
	}
}

func keyWithPrefix(prefix, key string) string { return prefix + key }

func TestV3SetDigestBindsVersionAndBusinessTerms(t *testing.T) {
	req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin, PaymentToken: "USDT", TokenPrice: "1"}
	got, err := v3SetDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(req)
	if got != sha256.Sum256(encoded) {
		t.Fatal("noncanonical digest")
	}
	req.ContractVersion = constant.ContractVersionV2Stablecoin
	other, _ := v3SetDigest(req)
	if got == other {
		t.Fatal("V2 version escaped request digest")
	}
	req.ContractVersion = constant.ContractVersionIDOV3Stablecoin
	req.TokenPrice = "2"
	other, _ = v3SetDigest(req)
	if got == other {
		t.Fatal("business parameter escaped request digest")
	}
}

func TestV3SetRequiresIndependentFeatureGateBeforeDBOrRPC(t *testing.T) {
	old := viper.Get("idoV3Set.enabled")
	t.Cleanup(func() { viper.Set("idoV3Set.enabled", old) })
	viper.Set("idoV3Set.enabled", false)
	if _, _, err := resolveAdmittedV3Set(19, OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin}); err == nil || err.Error() != "V3 Set release admission unavailable" {
		t.Fatalf("V3 Set escaped default-off release gate: %v", err)
	}
}

func TestPersistAdmittedV3SetRequiresLockedSetupAndAtomicBinding(t *testing.T) {
	m := testV3SetManifest()
	req := OnchainParamsRequest{ContractVersion: m.ContractVersion, PaymentToken: m.PaymentSymbol, TokenAddress: m.ProjectToken, OwnerAddress: m.ProjectOwner}
	digest, _ := v3SetDigest(req)
	m.RequestDigest = digest.Hex()
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	resolved := &resolvedOnchainParams{Trusted: trustedPaymentToken{Symbol: m.PaymentSymbol, ChainID: m.ChainID, Contract: m.ProxyAddress,
		Address: m.PaymentToken, Decimals: int(m.PaymentDecimals), FeeReceiver: m.FeeReceiver, SuccessFeeRate: m.FeeRate},
		Req: req, TokenDecimals: 18, PriceRaw: "1", HardRaw: "10", SoftRaw: "5", MaxRaw: "8",
		Start: start, End: start.Add(time.Hour), TGERows: []entity.ProjectTge{{ProjectID: 19, Index: 1, VestingTime: start.Add(2 * time.Hour), VestingPercent: decimal.NewFromInt(100)}}}
	encoded, err := v3SetCreateCalldata(19, resolved, m)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest, err := v3ManifestDigest(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, version, status, mode                            string
		locked, expectWrite, bindingFailure, commitmentFailure bool
	}{
		{name: "exact", version: constant.ContractVersionUnassigned, status: constant.ProjectStatusSetup, mode: constant.FundingModeCappedPresale, expectWrite: true},
		{name: "binding failure rolls back", version: constant.ContractVersionUnassigned, status: constant.ProjectStatusSetup, mode: constant.FundingModeCappedPresale, expectWrite: true, bindingFailure: true},
		{name: "commitment failure rolls back", version: constant.ContractVersionUnassigned, status: constant.ProjectStatusSetup, mode: constant.FundingModeCappedPresale, expectWrite: true, commitmentFailure: true},
		{name: "V2 project", version: constant.ContractVersionV2Stablecoin, status: constant.ProjectStatusSetup, mode: constant.FundingModeCappedPresale},
		{name: "locked project", version: constant.ContractVersionUnassigned, status: constant.ProjectStatusSetup, mode: constant.FundingModeCappedPresale, locked: true},
		{name: "active status", version: constant.ContractVersionUnassigned, status: constant.ProjectStatusActive, mode: constant.FundingModeCappedPresale},
		{name: "foreign mode", version: constant.ContractVersionUnassigned, status: constant.ProjectStatusSetup, mode: constant.FundingModeFairLaunch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `projects` WHERE project_id = .*").WithArgs(int64(19), 1).
				WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "onchain_locked", "funding_mode", "contract_version", "chain_id", "contract_address"}).
					AddRow(19, tc.status, tc.locked, tc.mode, tc.version, 0, ""))
			if tc.expectWrite {
				mock.ExpectExec("UPDATE `projects` SET .*contract_version.* WHERE .*project_id.*").WillReturnResult(sqlmock.NewResult(0, 1))
				binding := mock.ExpectExec("INSERT INTO ido_v3_project_binding .* VALUES .*ido_v3_stablecoin.*").WithArgs(int64(19), m.ChainID, m.ProxyAddress)
				if tc.bindingFailure {
					binding.WillReturnError(errors.New("sensitive SQL diagnostics"))
				} else {
					binding.WillReturnResult(sqlmock.NewResult(0, 1))
					commitment := mock.ExpectExec("INSERT INTO ido_v3_set_create_commitment").WithArgs(int64(19), m.ChainID, m.ProxyAddress, m.RequestDigest, manifestDigest.Hex(), crypto.Keccak256Hash(encoded).Hex(), m.Operator)
					if tc.commitmentFailure {
						commitment.WillReturnError(errors.New("sensitive commitment diagnostics"))
					} else {
						commitment.WillReturnResult(sqlmock.NewResult(0, 1))
						mock.ExpectExec("DELETE FROM `project_tge` WHERE project_id = .*").WithArgs(int64(19)).WillReturnResult(sqlmock.NewResult(0, 0))
						mock.ExpectExec("INSERT INTO `project_tge` .* VALUES .*").WillReturnResult(sqlmock.NewResult(1, 1))
					}
				}
			}
			mock.ExpectRollback() // Caller owns rollback/commit, including after success.
			tx := database.Begin()
			_, err := persistAdmittedV3SetTx(tx, 19, req, resolved, m)
			if (err == nil) != (tc.expectWrite && !tc.bindingFailure && !tc.commitmentFailure) || (err != nil && strings.Contains(err.Error(), "sensitive")) {
				t.Fatalf("Set write result: %v", err)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPersistV3SetRejectsAlteredReviewDigestBeforeTransactionRead(t *testing.T) {
	m := testV3SetManifest()
	req := OnchainParamsRequest{ContractVersion: m.ContractVersion, PaymentToken: m.PaymentSymbol,
		TokenAddress: m.ProjectToken, OwnerAddress: m.ProjectOwner}
	res := &resolvedOnchainParams{Trusted: trustedPaymentToken{ChainID: m.ChainID, Contract: m.ProxyAddress,
		Address: m.PaymentToken, Decimals: int(m.PaymentDecimals), FeeReceiver: m.FeeReceiver, SuccessFeeRate: m.FeeRate},
		End: time.Now(), TGERows: []entity.ProjectTge{{VestingTime: time.Now().Add(time.Hour)}}}
	database, mock := v2AdmissionMock(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	tx := database.Begin()
	if _, err := persistAdmittedV3SetTx(tx, int64(m.ProjectID), req, res, m); err == nil {
		t.Fatal("Set persisted without matching signed request digest")
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3SetRejectsNonIncreasingVestingBeforePersistence(t *testing.T) {
	now := time.Now()
	for _, schedule := range [][]time.Time{{now}, {now.Add(time.Hour), now.Add(time.Hour)}, {now.Add(2 * time.Hour), now.Add(time.Hour)}} {
		res := &resolvedOnchainParams{End: now}
		for _, at := range schedule {
			res.TGERows = append(res.TGERows, entity.ProjectTge{VestingTime: at})
		}
		if err := validateV3SetVesting(res); err == nil {
			t.Fatal("invalid V3 schedule accepted")
		}
	}
	if err := validateV3SetVesting(&resolvedOnchainParams{End: now, TGERows: []entity.ProjectTge{{VestingTime: now.Add(time.Hour)}, {VestingTime: now.Add(2 * time.Hour)}}}); err != nil {
		t.Fatal(err)
	}
}
