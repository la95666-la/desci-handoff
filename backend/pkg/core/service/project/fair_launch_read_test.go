package project

import (
	"encoding/json/v2"
	"testing"

	"joincare_server/pkg/common/constant"
)

func TestFairLaunchAllocationRawUsesArbitraryPrecisionAndRoundsDown(t *testing.T) {
	got, err := FairLaunchAllocationRaw("9007199254740993", "1000000000000000000000000", "18014398509481987")
	if err != nil {
		t.Fatal(err)
	}
	if want := "499999999999999972244424"; got != want {
		t.Fatalf("allocation=%s want=%s", got, want)
	}
}

func TestFairLaunchSoftCapProgressDoesNotImplySettlement(t *testing.T) {
	progress, reached, err := FairLaunchSoftCapProgressBps("9007199254740993", "9007199254740992")
	if err != nil {
		t.Fatal(err)
	}
	if progress != "10000" || !reached {
		t.Fatalf("progress=%s reached=%v", progress, reached)
	}

	status, err := FairLaunchDisplayStatus(FairLaunchChainStatusActive, 99, 100, reached)
	if err != nil {
		t.Fatal(err)
	}
	if status != FairLaunchChainStatusActive {
		t.Fatalf("soft-cap reached before end changed status to %q", status)
	}
	status, err = FairLaunchDisplayStatus(FairLaunchChainStatusActive, 100, 100, reached)
	if err != nil {
		t.Fatal(err)
	}
	if status != FairLaunchDisplayEndedSuccessPendingSettlement {
		t.Fatalf("ended active status=%q", status)
	}
}

func TestBuildFairLaunchReadSummaryHasNoHardCapAndNoFloatFields(t *testing.T) {
	summary, err := BuildFairLaunchReadSummary(
		FairLaunchChainStatusActive,
		99,
		100,
		"15000000000000000000001",
		"10000000000000000000000",
		"1000000000000000000000000",
	)
	if err != nil {
		t.Fatal(err)
	}
	if summary.FundingMode != constant.FundingModeFairLaunch || summary.ContractVersion != constant.ContractVersionFairLaunchV1 {
		t.Fatalf("wrong mode/version: %+v", summary)
	}
	if !summary.SoftCapReached || summary.SoftCapProgressBps != "15000" || summary.DisplayStatus != FairLaunchChainStatusActive {
		t.Fatalf("wrong active summary: %+v", summary)
	}
	if summary.HardCap.State != FinancialStateNotApplicable || summary.HardCap.Raw != nil {
		t.Fatalf("hard cap must be not_applicable: %+v", summary.HardCap)
	}
	if summary.FinalPrice.State != FinancialStatePendingSettlement || summary.FinalPrice.NumeratorRaw != nil {
		t.Fatalf("active final price must remain pending settlement: %+v", summary.FinalPrice)
	}
	if summary.EstimatedPrice.NumeratorRaw == nil || *summary.EstimatedPrice.NumeratorRaw != "15000000000000000000001" {
		t.Fatalf("raw decimal string was not preserved: %+v", summary.EstimatedPrice)
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["hardCap"].(map[string]any)["raw"]; !ok {
		t.Fatal("hardCap raw null must be explicit")
	}
}

func TestBuildFairLaunchReadSummarySupportsNotCreatedAndTerminalPriceSemantics(t *testing.T) {
	for _, status := range []string{FairLaunchChainStatusNotCreated, FairLaunchChainStatusPending} {
		summary, err := BuildFairLaunchReadSummary(status, 0, 100, "0", "1", "1")
		if err != nil {
			t.Fatal(err)
		}
		if summary.DisplayStatus != status || summary.FinalPrice.State != FinancialStatePendingSettlement {
			t.Fatalf("wrong pre-settlement summary for %s: %+v", status, summary)
		}
	}
	for _, status := range []string{FairLaunchChainStatusFailed, FairLaunchChainStatusCancelled} {
		summary, err := BuildFairLaunchReadSummary(status, 101, 100, "0", "1", "1")
		if err != nil {
			t.Fatal(err)
		}
		if summary.FinalPrice.State != FinancialStateNotApplicable || summary.FinalPrice.NumeratorRaw != nil || summary.FinalPrice.DenominatorRaw != nil {
			t.Fatalf("terminal final price must be not_applicable for %s: %+v", status, summary.FinalPrice)
		}
	}
}

func TestBuildFairLaunchReadSummaryFreezesFinalPriceOnlyAfterSettlement(t *testing.T) {
	summary, err := BuildFairLaunchReadSummary(
		FairLaunchChainStatusSuccessfulPendingRelease,
		101,
		100,
		"15000000000000000000001",
		"10000000000000000000000",
		"1000000000000000000000000",
	)
	if err != nil {
		t.Fatal(err)
	}
	if summary.DisplayStatus != FairLaunchChainStatusSuccessfulPendingRelease || summary.FinalPrice.State != FinancialStateAvailable {
		t.Fatalf("wrong settled summary: %+v", summary)
	}
	if summary.FinalPrice.NumeratorRaw == nil || *summary.FinalPrice.NumeratorRaw != "15000000000000000000001" {
		t.Fatalf("wrong final price numerator: %+v", summary.FinalPrice)
	}
}

func TestFairLaunchReadHelpersFailClosed(t *testing.T) {
	invalidRaw := []string{"", "-1", "+1", "01", "1.5", "not-a-number"}
	for _, raw := range invalidRaw {
		if _, _, err := FairLaunchSoftCapProgressBps(raw, "1"); err == nil {
			t.Fatalf("invalid raw %q accepted", raw)
		}
	}
	if _, err := FairLaunchDisplayStatus("future_status", 0, 0, false); err == nil {
		t.Fatal("unknown status accepted")
	}
	if _, err := BuildFairLaunchReadSummary(FairLaunchChainStatusActive, 0, 1, "0", "1", "0"); err == nil {
		t.Fatal("zero sale token amount accepted")
	}
}
