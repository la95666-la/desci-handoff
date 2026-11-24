package project

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func validStoredV2Project(now time.Time) entity.Project {
	return entity.Project{
		ProjectID: 7, OwnerAddress: "0x3333333333333333333333333333333333333333",
		TokenAddress: "0x4444444444444444444444444444444444444444", TokenDecimals: 18,
		PaymentToken: constant.PaymentTokenUSDT, PaymentTokenAddress: "0x2222222222222222222222222222222222222222", PaymentTokenDecimals: 6,
		TokenPrice: decimal.RequireFromString("1.000001"), TokenPriceRaw: "1000001",
		HardCap: decimal.NewFromInt(1000), HardCapRaw: "1000000000",
		SoftCap: decimal.NewFromInt(500), SoftCapRaw: "500000000",
		MaxPerUser: decimal.NewFromInt(100), MaxPerUserRaw: "100000000",
		StartTime: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour),
	}
}

func validStoredV2TGEs(project entity.Project) []entity.ProjectTge {
	return []entity.ProjectTge{
		{Index: 1, VestingTime: project.EndTime.Add(time.Hour), VestingPercent: decimal.NewFromInt(25)},
		{Index: 2, VestingTime: project.EndTime.Add(2 * time.Hour), VestingPercent: decimal.NewFromInt(75)},
	}
}

func TestValidateStoredV2Create(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	project := validStoredV2Project(now)
	times, percents, err := validateStoredV2Create(project, validStoredV2TGEs(project), now)
	if err != nil {
		t.Fatalf("valid stored V2 create rejected: %v", err)
	}
	if len(times) != 2 || percents[0].Int64() != 2500 || percents[1].Int64() != 7500 {
		t.Fatalf("unexpected vesting conversion: %v/%v", times, percents)
	}

	tests := []struct {
		name    string
		mutate  func(*entity.Project, *[]entity.ProjectTge)
		message string
	}{
		{name: "raw display mismatch", mutate: func(p *entity.Project, _ *[]entity.ProjectTge) { p.HardCapRaw = "999" }, message: "raw/display mismatch"},
		{name: "expired start", mutate: func(p *entity.Project, _ *[]entity.ProjectTge) { p.StartTime = now }, message: "sale times"},
		{name: "index gap", mutate: func(_ *entity.Project, tges *[]entity.ProjectTge) { (*tges)[1].Index = 3 }, message: "indexes"},
		{name: "wrong total", mutate: func(_ *entity.Project, tges *[]entity.ProjectTge) { (*tges)[1].VestingPercent = decimal.NewFromInt(74) }, message: "10000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := project
			tges := validStoredV2TGEs(candidate)
			test.mutate(&candidate, &tges)
			_, _, err := validateStoredV2Create(candidate, tges, now)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected %q error, got %v", test.message, err)
			}
		})
	}
}

func TestValidateDepositedProjectTokens(t *testing.T) {
	project := entity.Project{TokenDecimals: 18, TotalDepositedRaw: "1250000000000000000", TotalDeposited: decimal.RequireFromString("1.25")}
	if err := validateDepositedProjectTokens(project); err != nil {
		t.Fatalf("valid deposit rejected: %v", err)
	}
	project.TotalDepositedRaw = "0"
	if err := validateDepositedProjectTokens(project); err == nil {
		t.Fatal("zero deposit accepted")
	}
	project.TotalDepositedRaw = "1250000000000000000"
	project.TotalDeposited = decimal.NewFromInt(2)
	if err := validateDepositedProjectTokens(project); err == nil {
		t.Fatal("raw/display deposit mismatch accepted")
	}
}

func TestCreateV2ParamsUsesReservedFeeSnapshot(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	project := validStoredV2Project(now)
	project.FeeReceiver = "0x8888888888888888888888888888888888888888"
	project.SuccessFeeRate = 200
	params, err := createV2Params(&v2CreateReservation{project: project, times: []*big.Int{big.NewInt(project.EndTime.Add(time.Hour).Unix())}, percents: []*big.Int{big.NewInt(10_000)}})
	if err != nil {
		t.Fatalf("build V2 params: %v", err)
	}
	if params.ExpectedFeeReceiver.Hex() != common.HexToAddress(project.FeeReceiver).Hex() || params.ExpectedSuccessFeeRate != 200 {
		t.Fatalf("reserved fee snapshot lost: %+v", params)
	}
}
