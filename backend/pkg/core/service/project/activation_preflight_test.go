package project

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/entity"
)

// TestV2ActivationRequiredProjectTokens mirrors the IDO V2 proxy's
// softCapTokenAmount = Math.mulDiv(softCap, 10 ** tokenDecimals, tokenPrice):
//
//	softCap raw 500000000 USDT (6dp) = 500 USDT
//	tokenPrice raw 1000001 = 1.000001 USDT per whole JCC (18dp)
//	required = floor(500000000 * 10^18 / 1000001) = 499999500000499999500
func TestV2ActivationRequiredProjectTokens(t *testing.T) {
	project := entity.Project{
		SoftCapRaw:        "500000000",
		TokenPriceRaw:     "1000001",
		TokenDecimals:     18,
		TotalDepositedRaw: "499999500000499999500", // exactly the requirement
	}
	required, err := v2ActivationRequiredProjectTokens(project)
	if err != nil {
		t.Fatal(err)
	}
	if required.String() != "499999500000499999500" {
		t.Fatalf("required = %s, want 499999500000499999500", required.String())
	}
}

func TestV2ActivationTokenShortfall(t *testing.T) {
	project := entity.Project{TokenSymbol: "JCC", TokenDecimals: 18, SoftCapRaw: "500000000", TokenPriceRaw: "1000001", TotalDepositedRaw: "400000000000000000000"}
	shortfall, err := V2ActivationTokenShortfall(project)
	if err != nil {
		t.Fatal(err)
	}
	if shortfall == nil {
		t.Fatal("expected a shortfall for under-deposited project")
	}
	if shortfall.CurrentRaw != "400000000000000000000" || shortfall.RequiredRaw != "499999500000499999500" || shortfall.ShortfallRaw != "99999500000499999500" {
		t.Fatalf("unexpected raw amounts: current=%s required=%s shortfall=%s", shortfall.CurrentRaw, shortfall.RequiredRaw, shortfall.ShortfallRaw)
	}
	if shortfall.Symbol != "JCC" || shortfall.Decimals != 18 {
		t.Fatalf("unexpected symbol/decimals: %+v", shortfall)
	}
	if !strings.Contains(shortfall.Error(), "499999500000499999500") {
		t.Fatalf("business error should carry the required amount: %v", shortfall.Error())
	}
}

func TestV2ActivationTokenShortfallSufficient(t *testing.T) {
	project := entity.Project{TokenSymbol: "JCC", TokenDecimals: 18, SoftCapRaw: "500000000", TokenPriceRaw: "1000001", TotalDepositedRaw: "499999500000499999500"}
	shortfall, err := V2ActivationTokenShortfall(project)
	if err != nil || shortfall != nil {
		t.Fatalf("exact deposit must be sufficient: shortfall=%v err=%v", shortfall, err)
	}
	project.TotalDepositedRaw = "999999999999999999999999999999"
	if shortfall, err = V2ActivationTokenShortfall(project); err != nil || shortfall != nil {
		t.Fatalf("large deposit must be sufficient: shortfall=%v err=%v", shortfall, err)
	}
}

func TestV2ActivationTokenShortfallSixDecimals(t *testing.T) {
	project := entity.Project{TokenDecimals: 6, SoftCapRaw: "1000000000", TokenPriceRaw: "2000000", TotalDepositedRaw: "499999999"}
	shortfall, err := V2ActivationTokenShortfall(project)
	if err != nil {
		t.Fatal(err)
	}
	if shortfall == nil || shortfall.RequiredRaw != "500000000" || shortfall.ShortfallRaw != "1" {
		t.Fatalf("unexpected six-decimal shortfall: %+v", shortfall)
	}
}

func TestV2ActivationTokenShortfallMalformed(t *testing.T) {
	tests := []struct {
		name    string
		project entity.Project
		message string
	}{
		{name: "soft cap raw garbage", project: entity.Project{SoftCapRaw: "12x", TokenPriceRaw: "100", TotalDepositedRaw: "10"}, message: "soft cap raw"},
		{name: "price raw garbage", project: entity.Project{SoftCapRaw: "100", TokenPriceRaw: ".1", TotalDepositedRaw: "10"}, message: "token price raw"},
		{name: "zero price", project: entity.Project{SoftCapRaw: "100", TokenPriceRaw: "0", TotalDepositedRaw: "10"}, message: "must be positive"},
		{name: "deposit raw garbage", project: entity.Project{SoftCapRaw: "100", TokenPriceRaw: "100", TotalDepositedRaw: "-"}, message: "total deposited raw"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shortfall, err := V2ActivationTokenShortfall(test.project)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected %q error, got shortfall=%v err=%v", test.message, shortfall, err)
			}
		})
	}
}

func TestIsInsufficientProjectTokensRPCError(t *testing.T) {
	if IsInsufficientProjectTokensRPCError(nil) {
		t.Fatal("nil must not be an insufficient-token error")
	}
	for _, message := range []string{
		"execution reverted: InsufficientProjectTokens",
		"VM execution error: insufficient project tokens",
		"estimate gas failed: 0xe9548d03",
		"execution reverted (0xe9548d03)",
		"custom error 0xe9548d03",
		"REVERT with reason string 'InsufficientProjectTokens()'",
	} {
		err := fmt.Errorf("relayer failed: %s", message)
		if !IsInsufficientProjectTokensRPCError(err) {
			t.Fatalf("expected detection for %q", message)
		}
	}
	for _, message := range []string{"estimate gas failed: out of gas", "execution reverted: InvalidProjectStatus", "rpc timeout"} {
		err := fmt.Errorf("relayer failed: %s", message)
		if IsInsufficientProjectTokensRPCError(err) {
			t.Fatalf("unexpected detection for %q", message)
		}
	}
	var wrapped error = fmt.Errorf("send: %w", errors.New("estimate gas failed: execution reverted: InsufficientProjectTokens"))
	if !IsInsufficientProjectTokensRPCError(wrapped) {
		t.Fatal("wrapped revert error was not detected")
	}
}

func TestIsRelayerInternalError(t *testing.T) {
	if IsRelayerInternalError(nil) {
		t.Fatal("nil must not be an internal error")
	}
	for _, message := range []string{
		"estimate gas failed: insufficient funds for gas * price + value",
		"dial tcp 1.2.3.4:8545: connect: connection refused",
		"rpc error: code = Unavailable desc = connection error",
		"context deadline exceeded",
	} {
		if !IsRelayerInternalError(fmt.Errorf("dispatch: %s", message)) {
			t.Fatalf("expected internal-error detection for %q", message)
		}
	}
	for _, message := range []string{
		"project status must be Pending, got Set",
		"V2 activation requires a locked Pending project",
		"project not found",
	} {
		if IsRelayerInternalError(fmt.Errorf("%s", message)) {
			t.Fatalf("business error %q must not be classified as RPC internal", message)
		}
	}
}

func TestInsufficientProjectTokensErrorFields(t *testing.T) {
	shortfall := &InsufficientProjectTokensError{CurrentRaw: "1", RequiredRaw: "5", ShortfallRaw: "4", Symbol: " JCC ", Decimals: 18}
	var err error = shortfall
	var target *InsufficientProjectTokensError
	if !errors.As(err, &target) || target != shortfall {
		t.Fatal("typed error must be discoverable via errors.As")
	}
	if target.Symbol != " JCC " {
		t.Fatalf("raw symbol must round-trip: %q", target.Symbol)
	}
}

func TestRequireV2TokenSufficiency(t *testing.T) {
	project := entity.Project{TokenSymbol: "JCC", TokenDecimals: 18, SoftCapRaw: "100", TokenPriceRaw: "100", TotalDepositedRaw: "1"}
	if err := requireV2TokenSufficiency(project); err == nil {
		t.Fatal("under-deposited project must fail the preflight")
	}
	project.TotalDepositedRaw = "1000000000000000001"
	if err := requireV2TokenSufficiency(project); err != nil {
		t.Fatalf("sufficient deposit must pass the preflight: %v", err)
	}
}

func TestV2ActivationTokenShortfallDisplayConsistency(t *testing.T) {
	// 5 USDT soft cap at 2 USDT price => 2.5 JCC required (18 decimals).
	// Depositing only 1.25 JCC must produce a shortfall of exactly 1.25 JCC.
	project := entity.Project{TokenSymbol: "ABC", TokenDecimals: 18, SoftCapRaw: "5000000", TokenPriceRaw: "2000000", TotalDepositedRaw: decimal.RequireFromString("1.25").Shift(18).BigInt().String()}
	shortfall, err := V2ActivationTokenShortfall(project)
	if err != nil {
		t.Fatal(err)
	}
	if shortfall == nil {
		t.Fatal("1.25 deposited vs requirement 2.5 must shortfall")
	}
	if shortfall.RequiredRaw != "2500000000000000000" || shortfall.ShortfallRaw != "1250000000000000000" {
		t.Fatalf("unexpected raw amounts: %+v", shortfall)
	}
}
