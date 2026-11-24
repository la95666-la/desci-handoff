package project

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func validProjectReviewRequest(now time.Time) SubmitProjectReviewRequest {
	return SubmitProjectReviewRequest{
		ProjectID:      1,
		Logo:           "/project/logo/20260817/image-1.png",
		TokenAddress:   "0x0000000000000000000000000000000000000001",
		TokenDecimals:  18,
		TokenPrice:     decimal.RequireFromString("1.000001"),
		PaymentToken:   "USDT",
		HardCap:        decimal.NewFromInt(1000),
		SoftCap:        decimal.NewFromInt(500),
		MaxPerUser:     decimal.NewFromInt(100),
		StartTime:      now.Add(time.Hour).Unix(),
		EndTime:        now.Add(2 * time.Hour).Unix(),
		VestingTimes:   []int64{now.Add(3 * time.Hour).Unix(), now.Add(4 * time.Hour).Unix()},
		VestingPercent: []float64{25, 75},
	}
}

func TestTrustedProjectImagePathRejectsSSRFAndTraversal(t *testing.T) {
	viper.Set("cos.host", "https://cdn.example.test")
	t.Cleanup(viper.Reset)
	valid := "/project/logo/20260817/image-1.png"
	if got, err := trustedProjectImagePath(valid, "logo"); err != nil || got != valid {
		t.Fatalf("trusted object path rejected: got=%q err=%v", got, err)
	}
	if got, err := trustedProjectImagePath("https://cdn.example.test"+valid+"?sign=x", "logo"); err != nil || got != valid {
		t.Fatalf("trusted display URL rejected: got=%q err=%v", got, err)
	}
	for _, candidate := range []string{
		"http://cdn.example.test" + valid,
		"https://127.0.0.1" + valid,
		"https://169.254.169.254/latest/meta-data",
		"https://cdn.example.test/project/logo/../banner/evil.png",
		"/project/banner/20260817/evil.png",
		"file:///etc/passwd",
	} {
		if _, err := trustedProjectImagePath(candidate, "logo"); err == nil {
			t.Fatalf("unsafe image source accepted: %s", candidate)
		}
	}
	if _, err := trustedProjectImagePath("https://cdn.example.test.evil.invalid"+valid, "logo"); err == nil || !strings.Contains(err.Error(), "trusted COS host") {
		t.Fatalf("host suffix attack was not rejected: %v", err)
	}
}

func TestValidateProjectReviewAcceptsContractCompatibleRequest(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	if err := validateProjectReview(validProjectReviewRequest(now), now); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestValidateProjectReviewRejectsInvalidContractInvariants(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tests := []struct {
		name    string
		mutate  func(*SubmitProjectReviewRequest)
		message string
	}{
		{name: "zero price", mutate: func(req *SubmitProjectReviewRequest) { req.TokenPrice = decimal.Zero }, message: "token price"},
		{name: "soft cap above hard cap", mutate: func(req *SubmitProjectReviewRequest) { req.SoftCap = decimal.NewFromInt(1001) }, message: "soft cap"},
		{name: "max per user above hard cap", mutate: func(req *SubmitProjectReviewRequest) { req.MaxPerUser = decimal.NewFromInt(1001) }, message: "max per user"},
		{name: "start not future", mutate: func(req *SubmitProjectReviewRequest) { req.StartTime = now.Unix() }, message: "future"},
		{name: "end before start", mutate: func(req *SubmitProjectReviewRequest) { req.EndTime = req.StartTime }, message: "after start"},
		{name: "too many periods", mutate: func(req *SubmitProjectReviewRequest) {
			req.VestingTimes = make([]int64, 11)
			req.VestingPercent = make([]float64, 11)
		}, message: "10 periods"},
		{name: "nonascending vesting", mutate: func(req *SubmitProjectReviewRequest) { req.VestingTimes[1] = req.VestingTimes[0] }, message: "ascending"},
		{name: "vesting before end", mutate: func(req *SubmitProjectReviewRequest) { req.VestingTimes[0] = req.EndTime }, message: "after end"},
		{name: "vesting total not 100", mutate: func(req *SubmitProjectReviewRequest) { req.VestingPercent[1] = 74 }, message: "equal 100"},
		{name: "vesting precision over 2 decimals", mutate: func(req *SubmitProjectReviewRequest) {
			req.VestingPercent = []float64{25.001, 74.999}
		}, message: "2 decimal"},
		{name: "zero token decimals", mutate: func(req *SubmitProjectReviewRequest) { req.TokenDecimals = 0 }, message: "between 1 and 18"},
		{name: "excess token decimals", mutate: func(req *SubmitProjectReviewRequest) { req.TokenDecimals = 19 }, message: "between 1 and 18"},
		{name: "zero token address", mutate: func(req *SubmitProjectReviewRequest) { req.TokenAddress = "0x0000000000000000000000000000000000000000" }, message: "address"},
		{name: "missing payment token", mutate: func(req *SubmitProjectReviewRequest) { req.PaymentToken = "" }, message: "USDT or USDC"},
		{name: "legacy JCC payment", mutate: func(req *SubmitProjectReviewRequest) { req.PaymentToken = "JCC" }, message: "USDT or USDC"},
		{name: "arbitrary payment address", mutate: func(req *SubmitProjectReviewRequest) { req.PaymentToken = "0x1111111111111111111111111111111111111111" }, message: "USDT or USDC"},
		{name: "display column overflow", mutate: func(req *SubmitProjectReviewRequest) {
			req.HardCap = decimal.RequireFromString("100000000000000000000000000000000000000000000000")
		}, message: "DECIMAL(65,18)"},
		{name: "scientific notation display overflow", mutate: func(req *SubmitProjectReviewRequest) {
			req.HardCap = decimal.RequireFromString("1e1000000")
		}, message: "DECIMAL(65,18)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := validProjectReviewRequest(now)
			test.mutate(&req)
			err := validateProjectReview(req, now)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error containing %q, got %v", test.message, err)
			}
		})
	}
}

func TestProjectRawAmountsUsesPaymentTokenDecimals(t *testing.T) {
	req := validProjectReviewRequest(time.Unix(1_800_000_000, 0))
	req.TokenPrice = decimal.RequireFromString("1.000001")
	req.HardCap = decimal.RequireFromString("1000.5")
	out, err := projectRawAmounts(req, 6)
	if err != nil {
		t.Fatalf("valid USDT amounts rejected: %v", err)
	}
	if out.TokenPrice != "1000001" || out.HardCap != "1000500000" {
		t.Fatalf("unexpected raw amounts: %+v", out)
	}
	req.TokenPrice = decimal.RequireFromString("1.0000001")
	if _, err := projectRawAmounts(req, 6); err == nil || !strings.Contains(err.Error(), "token price") {
		t.Fatalf("USDT over-precision accepted: %v", err)
	}
}

func TestLegacyRelayerRejectsV2AndUnverifiedProjects(t *testing.T) {
	if err := requireLegacyRelayerVersion("v1_legacy"); err != nil {
		t.Fatalf("verified legacy project rejected: %v", err)
	}
	for _, version := range []string{"", "unassigned", "v1_legacy_unverified", "v2_stablecoin"} {
		if err := requireLegacyRelayerVersion(version); err == nil || !strings.Contains(err.Error(), "V2 routing is not enabled") {
			t.Fatalf("unsafe relayer version %q accepted: %v", version, err)
		}
	}
}
