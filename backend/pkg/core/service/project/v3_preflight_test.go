package project

import (
	"math/big"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3CreatePreflightBindsSetAndDoesNotWrite(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	m := testV3SetManifest()
	m.ProjectID = 7
	m.ExpiresAt = now.Add(time.Hour).Unix()
	p := validStoredV2Project(now)
	p.Status, p.FundingMode, p.ContractVersion = constant.ProjectStatusSet, m.FundingMode, m.ContractVersion
	p.ChainID, p.ContractAddress = m.ChainID, m.ProxyAddress
	p.TokenAddress, p.PaymentTokenAddress, p.OwnerAddress = m.ProjectToken, m.PaymentToken, m.ProjectOwner
	p.FeeReceiver, p.SuccessFeeRate, p.PaymentTokenDecimals = m.FeeReceiver, m.FeeRate, int(m.PaymentDecimals)
	p.TokenPriceRaw, p.HardCapRaw, p.SoftCapRaw, p.MaxPerUserRaw =
		"1000000000000000001", "1000000000000000000000", "500000000000000000000", "100000000000000000000"
	// The price display must exactly round-trip through the reviewed payment
	// token decimals; using the V2 fixture's six decimals would fail.
	p.TokenPrice = decimal.RequireFromString("1.000000000000000001")
	manifestDigest, err := v3ManifestDigest(m)
	if err != nil {
		t.Fatal(err)
	}
	calldata, err := baseChain.PackV3Create(baseChain.V3CreateParams{
		ProjectID: big.NewInt(7), ProjectToken: common.HexToAddress(m.ProjectToken), PaymentToken: common.HexToAddress(m.PaymentToken),
		ProjectOwner: common.HexToAddress(m.ProjectOwner), FeeReceiver: common.HexToAddress(m.FeeReceiver), FeeRate: m.FeeRate,
		Price: mustRaw(t, p.TokenPriceRaw), HardCap: mustRaw(t, p.HardCapRaw), SoftCap: mustRaw(t, p.SoftCapRaw), MaxPerUser: mustRaw(t, p.MaxPerUserRaw),
		Start: big.NewInt(p.StartTime.Unix()), End: big.NewInt(p.EndTime.Unix()),
		VestingTimes:    []*big.Int{big.NewInt(p.EndTime.Add(time.Hour).Unix()), big.NewInt(p.EndTime.Add(2 * time.Hour).Unix())},
		VestingPercents: []*big.Int{big.NewInt(2500), big.NewInt(7500)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, status, version string
		locked, binding       bool
		badCommitment         string
		allow                 bool
	}{
		{"valid operator", p.Status, p.ContractVersion, false, true, "", true},
		{"changed reviewed calldata", p.Status, p.ContractVersion, false, true, "calldata", false},
		{"changed signed manifest", p.Status, p.ContractVersion, false, true, "manifest", false},
		{"changed typed request", p.Status, p.ContractVersion, false, true, "request", false},
		{"missing commitment", p.Status, p.ContractVersion, false, true, "missing", false},
		{"V2 identity", p.Status, constant.ContractVersionV2Stablecoin, false, false, "", false},
		{"premature locked", p.Status, p.ContractVersion, true, false, "", false},
		{"wrong status", constant.ProjectStatusPending, p.ContractVersion, false, false, "", false},
		{"missing binding", p.Status, p.ContractVersion, false, false, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			candidate := p
			candidate.Status, candidate.ContractVersion, candidate.OnchainLocked = tc.status, tc.version, tc.locked
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `projects` WHERE project_id = ? LIMIT ? FOR UPDATE")).
				WithArgs(int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "funding_mode", "contract_version", "onchain_locked", "chain_id", "contract_address", "token_address", "payment_token_address", "owner_address", "fee_receiver", "payment_token", "payment_token_decimals", "success_fee_rate", "token_decimals", "token_price_raw", "hard_cap_raw", "soft_cap_raw", "max_per_user_raw", "token_price", "hard_cap", "soft_cap", "max_per_user", "start_time", "end_time"}).
				AddRow(7, candidate.Status, candidate.FundingMode, candidate.ContractVersion, candidate.OnchainLocked, candidate.ChainID, candidate.ContractAddress, candidate.TokenAddress, candidate.PaymentTokenAddress, candidate.OwnerAddress, candidate.FeeReceiver, candidate.PaymentToken, candidate.PaymentTokenDecimals, candidate.SuccessFeeRate, candidate.TokenDecimals, candidate.TokenPriceRaw, candidate.HardCapRaw, candidate.SoftCapRaw, candidate.MaxPerUserRaw, candidate.TokenPrice.String(), candidate.HardCap.String(), candidate.SoftCap.String(), candidate.MaxPerUser.String(), candidate.StartTime, candidate.EndTime))
			if tc.version == p.ContractVersion && !tc.locked && tc.status == p.Status {
				rows := sqlmock.NewRows([]string{"project_id"})
				if tc.binding {
					rows.AddRow(7)
				}
				mock.ExpectQuery("SELECT project_id FROM ido_v3_project_binding").WithArgs(int64(7), m.ChainID, m.ProxyAddress).WillReturnRows(rows)
				if tc.binding {
					mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `project_tge` WHERE project_id = ? ORDER BY `index` asc")).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"project_id", "index", "vesting_time", "vesting_percent"}).AddRow(7, 1, p.EndTime.Add(time.Hour), "25").AddRow(7, 2, p.EndTime.Add(2*time.Hour), "75"))
					request, signed, hash := m.RequestDigest, manifestDigest.Hex(), crypto.Keccak256Hash(calldata).Hex()
					switch tc.badCommitment {
					case "request":
						request = common.HexToHash("0x8").Hex()
					case "manifest":
						signed = common.HexToHash("0x8").Hex()
					case "calldata":
						hash = common.HexToHash("0x8").Hex()
					}
					rows := sqlmock.NewRows([]string{"project_id", "chain_id", "proxy_address", "contract_version", "request_digest", "manifest_digest", "create_calldata_hash", "operator_address"})
					if tc.badCommitment != "missing" {
						rows.AddRow(7, m.ChainID, m.ProxyAddress, m.ContractVersion, request, signed, hash, m.Operator)
					}
					mock.ExpectQuery("SELECT project_id,chain_id,proxy_address,contract_version,request_digest,manifest_digest,create_calldata_hash").WithArgs(uint64(7)).WillReturnRows(rows)
				}
			}
			mock.ExpectRollback()
			tx := database.Begin()
			plan, err := prepareV3CreateTx(tx, 7, m, baseChain.V3Operator, common.HexToAddress(m.Operator), 18, now)
			if (err == nil) != tc.allow {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if err == nil && (plan.Transport != baseChain.V3RelayerTransport || plan.To != common.HexToAddress(m.ProxyAddress) || len(plan.CallData) == 0) {
				t.Fatalf("unexpected plan: %+v", plan)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3CreatePreflightRejectsDifferentConfirmedTokenDecimalsBeforeDB(t *testing.T) {
	m := testV3SetManifest()
	database, mock := v2AdmissionMock(t)
	if _, err := prepareV3CreateTx(database, int64(m.ProjectID), m, baseChain.V3Operator,
		common.HexToAddress(m.Operator), 0, time.Now()); err == nil {
		t.Fatal("missing independently confirmed token decimals accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func mustRaw(t *testing.T, value string) *big.Int {
	t.Helper()
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid fixture raw: %q", value)
	}
	return parsed
}

func TestV3LifecyclePreflightRejectsUntrustedAndMismatchedBeforeAnyQuery(t *testing.T) {
	m := testV3SetManifest()
	b := idov3whitelist.ExpectedProjectBinding{ChainID: m.ChainID, ProjectID: m.ProjectID,
		ProxyAddress: common.HexToAddress(m.ProxyAddress), PaymentToken: common.HexToAddress(m.PaymentToken),
		ProjectToken: common.HexToAddress(m.ProjectToken)}
	admitted := idov3whitelist.AdmittedProject{Binding: b}
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: b.ChainID, ProjectID: b.ProjectID,
		ProxyAddress: b.ProxyAddress, ProjectToken: b.ProjectToken, PaymentToken: b.PaymentToken,
		ProjectOwner: common.HexToAddress(m.ProjectOwner), FeeReceiver: common.HexToAddress(m.FeeReceiver),
		BlockNumber: 90, BlockHash: common.HexToHash("0x1234"), State: idov3whitelist.ProjectPending}
	database, mock := v2AdmissionMock(t)
	identities := baseChain.V3CallerIdentity{Operator: common.HexToAddress(m.Operator), OwnerSafe: common.HexToAddress(m.OwnerSafe)}
	for _, tc := range []struct {
		name     string
		admitted idov3whitelist.AdmittedProject
		observed idov3whitelist.ProjectIdentityObservation
	}{
		{"unadmitted", idov3whitelist.AdmittedProject{}, observed},
		{"foreign project", admitted, func() idov3whitelist.ProjectIdentityObservation { o := observed; o.ProjectID++; return o }()},
		{"foreign proxy", admitted, func() idov3whitelist.ProjectIdentityObservation {
			o := observed
			o.ProxyAddress = b.PaymentToken
			return o
		}()},
		{"unanchored", admitted, func() idov3whitelist.ProjectIdentityObservation { o := observed; o.BlockHash = common.Hash{}; return o }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := prepareV3LifecycleTx(database, tc.admitted, tc.observed, "activateProject", baseChain.V3Operator,
				identities.Operator, identities, time.Now()); err == nil {
				t.Fatal("untrusted V3 lifecycle input accepted")
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3CorePlanEntryPointsRemainDefaultOff(t *testing.T) {
	oldSet, oldLife := viper.Get("idoV3Set.enabled"), viper.Get("idoV3Lifecycle.enabled")
	t.Cleanup(func() { viper.Set("idoV3Set.enabled", oldSet); viper.Set("idoV3Lifecycle.enabled", oldLife) })
	viper.Set("idoV3Set.enabled", false)
	viper.Set("idoV3Lifecycle.enabled", false)
	if _, err := previewV3CoreCreate(nil, nil, 19, common.Address{}, baseChain.V3Operator); err == nil {
		t.Fatal("V3 create plan opened without admission")
	}
	if _, err := previewV3CoreLifecycle(nil, nil, nil, nil, idov3whitelist.AdmissionManifest{}, nil,
		"activateProject", common.Address{}, baseChain.V3Operator, idov3whitelist.GovernanceIdentity{}, common.Address{}); err == nil {
		t.Fatal("V3 lifecycle plan opened without admission")
	}
}

func TestV3LifecyclePreflightMatchesLockedProjectRoleAndMethod(t *testing.T) {
	m := testV3SetManifest()
	b := idov3whitelist.ExpectedProjectBinding{ChainID: m.ChainID, ProjectID: m.ProjectID,
		ProxyAddress: common.HexToAddress(m.ProxyAddress), ProjectToken: common.HexToAddress(m.ProjectToken),
		PaymentToken: common.HexToAddress(m.PaymentToken)}
	admitted := idov3whitelist.AdmittedProject{Binding: b}
	now := time.Unix(1_800_000_000, 0)
	p := validStoredV2Project(now)
	p.ProjectID = int(m.ProjectID)
	p.Status, p.FundingMode, p.ContractVersion, p.OnchainLocked = constant.ProjectStatusPending,
		m.FundingMode, m.ContractVersion, true
	p.ChainID, p.ContractAddress = m.ChainID, m.ProxyAddress
	p.TokenAddress, p.PaymentTokenAddress, p.OwnerAddress = m.ProjectToken, m.PaymentToken, m.ProjectOwner
	p.FeeReceiver, p.SuccessFeeRate, p.PaymentTokenDecimals = m.FeeReceiver, m.FeeRate, int(m.PaymentDecimals)
	p.TokenPriceRaw, p.HardCapRaw, p.SoftCapRaw, p.MaxPerUserRaw = "1000001", "1000000000", "500000000", "100000000"
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: b.ChainID, ProjectID: b.ProjectID,
		ProxyAddress: b.ProxyAddress, ProjectToken: b.ProjectToken, PaymentToken: b.PaymentToken,
		ProjectOwner: common.HexToAddress(m.ProjectOwner), FeeReceiver: common.HexToAddress(m.FeeReceiver),
		ProjectDecimals: uint8(p.TokenDecimals), PaymentDecimals: uint8(p.PaymentTokenDecimals), FeeRate: p.SuccessFeeRate,
		TokenPriceRaw: p.TokenPriceRaw, HardCapRaw: p.HardCapRaw, SoftCapRaw: p.SoftCapRaw, MaxPerUserRaw: p.MaxPerUserRaw,
		StartTimeRaw: "1800003600", EndTimeRaw: "1800007200", State: idov3whitelist.ProjectPending,
		BlockNumber: 90, BlockHash: common.HexToHash("0x1234")}
	identities := baseChain.V3CallerIdentity{Operator: common.HexToAddress(m.Operator), OwnerSafe: common.HexToAddress(m.OwnerSafe)}
	for _, tc := range []struct {
		name, status string
		role         baseChain.V3LifecycleRole
		caller       common.Address
		binding      bool
		want         bool
	}{
		{"operator activate", p.Status, baseChain.V3Operator, identities.Operator, true, true},
		{"safe activate", p.Status, baseChain.V3OwnerSafe, identities.OwnerSafe, true, true},
		{"project owner cannot activate", p.Status, baseChain.V3ProjectOwner, observed.ProjectOwner, true, false},
		{"mismatched DB status", constant.ProjectStatusActive, baseChain.V3Operator, identities.Operator, false, false},
		{"missing immutable binding", p.Status, baseChain.V3Operator, identities.Operator, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `projects` WHERE project_id = ? LIMIT ? FOR UPDATE")).
				WithArgs(b.ProjectID, 1).WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "funding_mode", "contract_version", "onchain_locked", "chain_id", "contract_address", "token_address", "payment_token_address", "owner_address", "fee_receiver", "success_fee_rate", "token_decimals", "payment_token_decimals", "token_price_raw", "hard_cap_raw", "soft_cap_raw", "max_per_user_raw", "start_time", "end_time"}).
				AddRow(p.ProjectID, tc.status, p.FundingMode, p.ContractVersion, p.OnchainLocked, p.ChainID, p.ContractAddress, p.TokenAddress, p.PaymentTokenAddress, p.OwnerAddress, p.FeeReceiver, p.SuccessFeeRate, p.TokenDecimals, p.PaymentTokenDecimals, p.TokenPriceRaw, p.HardCapRaw, p.SoftCapRaw, p.MaxPerUserRaw, p.StartTime, p.EndTime))
			if tc.status == p.Status {
				rows := sqlmock.NewRows([]string{"project_id"})
				if tc.binding {
					rows.AddRow(b.ProjectID)
				}
				mock.ExpectQuery("SELECT project_id FROM ido_v3_project_binding").WithArgs(b.ProjectID, b.ChainID, m.ProxyAddress).WillReturnRows(rows)
			}
			mock.ExpectRollback()
			tx := database.Begin()
			plan, err := prepareV3LifecycleTx(tx, admitted, observed, "activateProject", tc.role, tc.caller, identities, now)
			if (err == nil) != tc.want {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if err == nil && (plan.From != tc.caller || plan.To != b.ProxyAddress || len(plan.CallData) != 36) {
				t.Fatalf("incorrect isolated lifecycle plan: %+v", plan)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
