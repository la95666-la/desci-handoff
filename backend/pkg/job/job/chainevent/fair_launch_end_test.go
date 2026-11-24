package chainevent

import (
	"errors"
	"testing"
	"time"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

func enabledFairLaunchRuntime() projectruntime.Runtime {
	return projectruntime.Runtime{
		Enabled: true, Version: constant.ContractVersionFairLaunchV1, FundingMode: constant.FundingModeFairLaunch,
		ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001",
		OperatorAddress: "0x2000000000000000000000000000000000000002",
	}
}

func TestReserveFairLaunchEndRejectsBeforeDatabase(t *testing.T) {
	_, err := reserveFairLaunchEndTx(nil, 8, enabledFairLaunchRuntime(), time.Now().UTC())
	if !errors.Is(err, chaintx.ErrFairLaunchMaterializationUnavailable) || err.Error() != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestFairLaunchEndSchedulerIsStaticallyDisabled(t *testing.T) {
	if fairLaunchEndSchedulingEnabled() {
		t.Fatal("Fair Launch scheduler enabled before materialization closure")
	}
}

func TestFairLaunchEndCandidateUsesExplicitClockAndTrustedIdentity(t *testing.T) {
	now := time.Date(2026, 10, 8, 4, 0, 0, 0, time.UTC)
	project := entity.Project{
		ProjectID: 8, Status: constant.ProjectStatusActive, ContractVersion: constant.ContractVersionFairLaunchV1,
		ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", EndTime: now,
	}
	if err := fairLaunchEndCandidate(project, now, enabledFairLaunchRuntime()); err != nil {
		t.Fatalf("due project rejected: %v", err)
	}
	project.EndTime = now.Add(time.Second)
	if err := fairLaunchEndCandidate(project, now, enabledFairLaunchRuntime()); err == nil {
		t.Fatal("future project accepted early")
	}
}

func TestFairLaunchEndCandidateFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	project := entity.Project{Status: constant.ProjectStatusActive, ContractVersion: constant.ContractVersionFairLaunchV1, ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", EndTime: now.Add(-time.Second)}
	runtime := enabledFairLaunchRuntime()
	runtime.Enabled = false
	if err := fairLaunchEndCandidate(project, now, runtime); err == nil {
		t.Fatal("disabled runtime accepted")
	}
	runtime = enabledFairLaunchRuntime()
	project.ContractAddress = "0x3000000000000000000000000000000000000003"
	if err := fairLaunchEndCandidate(project, now, runtime); err == nil {
		t.Fatal("runtime identity drift accepted")
	}
}
