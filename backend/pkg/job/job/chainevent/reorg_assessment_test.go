package chainevent

import (
	jsonv2 "encoding/json/v2"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestAssessmentRequiredBlockersKeepSwitchFailClosed(t *testing.T) {
	if len(assessmentRequiredBlockers) != reorgAssessmentGlobalBlockerCount {
		t.Fatalf("global blocker count=%d want=%d", len(assessmentRequiredBlockers), reorgAssessmentGlobalBlockerCount)
	}
	want := []string{"BOUNDARY_STATE_SNAPSHOT_REQUIRED", "V1_SOURCE_ORIGIN_UNATTESTED", "GLOBAL_CONFIG_BASELINE_REQUIRED", "OUTBOX_GENERATION_BINDING_REQUIRED", "GENERATION_CHECKPOINT_SCHEMA_REQUIRED", "DUAL_CONTROL_AUTHORIZATION_REQUIRED", "INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED"}
	for index, code := range want {
		if assessmentRequiredBlockers[index].Code != code || len(assessmentRequiredBlockers[index].Detail) < 10 {
			t.Fatalf("global blocker %d drifted: %+v", index, assessmentRequiredBlockers[index])
		}
	}
}

func TestClassifyAssessmentEffectMatrix(t *testing.T) {
	v2 := assessmentV2Effect(t, commonChain.V2Purchased, 7)
	tests := []struct {
		name   string
		effect entity.ChainEventReorgMaterializationEffect
		want   string
		code   string
		action string
	}{
		{name: "v2 deposit exact", effect: assessmentV2Effect(t, commonChain.V2ProjectTokensDeposited, 7), want: reorgAssessmentReplayExact, action: "v2_token_deposit_ledger_and_total_deposited_delta"},
		{name: "v2 purchase conditional", effect: v2, want: reorgAssessmentReplayConditional, code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", action: "v2_purchase_payment_entitlement_and_participant_delta"},
		{name: "v2 proceeds conditional", effect: assessmentV2Effect(t, commonChain.V2ProjectProceedsWithdrawn, 7), want: reorgAssessmentReplayConditional, code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", action: "v2_project_proceeds_and_fund_withdrawn_flag"},
		{name: "v2 platform fee conditional", effect: assessmentV2Effect(t, commonChain.V2PlatformFeeWithdrawn, 7), want: reorgAssessmentReplayConditional, code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", action: "v2_platform_fee_fact_and_platform_fee_withdrawn_flag"},
		{name: "v2 project token withdrawal conditional", effect: assessmentV2Effect(t, commonChain.V2ProjectTokensWithdrawn, 7), want: reorgAssessmentReplayConditional, code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", action: "v2_token_withdrawal_fact_and_token_withdrawn_flag"},
		{name: "v2 lifecycle conditional", effect: assessmentV2Effect(t, commonChain.V2ProjectCancelled, 7), want: reorgAssessmentReplayConditional, code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED"},
		{name: "v1 purchase blocked", effect: assessmentV1Effect(t, commonChain.Purchase, 7), want: reorgAssessmentReplayBlocked, code: "V1_UID_TIME_NOTICE_UNRECOVERABLE"},
		{name: "v1 claim blocked", effect: assessmentV1Effect(t, commonChain.Claimed, 7), want: reorgAssessmentReplayBlocked, code: "V1_CLAIM_DERIVED_STATE_UNRECOVERABLE"},
		{name: "approved noop", effect: entity.ChainEventReorgMaterializationEffect{Branch: reorgMaterializationBranchReplacement, ContractVersion: constant.ContractVersionV2Stablecoin, EventType: "ignored", Classification: reorgMaterializationEffectIgnored, SemanticPayload: []byte("{}")}, want: reorgAssessmentReplayNoop},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classifyAssessmentEffect(test.effect)
			if err != nil {
				t.Fatal(err)
			}
			if got.Replayability != test.want || got.BlockerCode != test.code || (test.action != "" && got.ActionKind != test.action) {
				t.Fatalf("classification=%+v want=%s/%s/%s", got, test.want, test.code, test.action)
			}
		})
	}
}

func TestAssessmentV2SemanticIdentityAndRawAreStrict(t *testing.T) {
	effect := assessmentV2Effect(t, commonChain.V2Purchased, 7)
	effect.ProjectID++
	if _, err := classifyAssessmentEffect(effect); err == nil {
		t.Fatal("V2 payload/project identity drift was accepted")
	}
	effect = assessmentV2Effect(t, commonChain.V2Purchased, 7)
	var event commonChain.V2Event
	if err := jsonv2.Unmarshal(effect.SemanticPayload, &event); err != nil {
		t.Fatal(err)
	}
	event.PaymentRaw = "01"
	payload, err := jsonv2.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	effect.SemanticPayload, effect.SemanticDigest = payload, sha256Hex(payload)
	if _, err := classifyAssessmentEffect(effect); err == nil {
		t.Fatal("noncanonical V2 raw amount was accepted")
	}
	effect = assessmentV2Effect(t, commonChain.V2Purchased, 7)
	if err := jsonv2.Unmarshal(effect.SemanticPayload, &event); err != nil {
		t.Fatal(err)
	}
	event.User = common.Address{}
	payload, err = jsonv2.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	effect.SemanticPayload, effect.SemanticDigest = payload, sha256Hex(payload)
	if _, err := classifyAssessmentEffect(effect); err == nil {
		t.Fatal("V2 purchase without a user was accepted")
	}
}

func TestAssessmentProjectIdentityMustFitSignedDatabaseRange(t *testing.T) {
	effect := assessmentV2Effect(t, commonChain.V2ProjectActivated, uint64(9223372036854775807))
	if _, err := classifyAssessmentEffect(effect); err != nil {
		t.Fatalf("MaxInt64 project identity was rejected: %v", err)
	}
	effect = assessmentV2Effect(t, commonChain.V2ProjectActivated, uint64(9223372036854775807)+1)
	if _, err := classifyAssessmentEffect(effect); err == nil {
		t.Fatal("project identity above MaxInt64 was accepted")
	}
}

func TestAssessmentV2GlobalConfigurationIsConditional(t *testing.T) {
	allowed, decimals := false, uint8(6)
	token := common.HexToAddress("0x2000000000000000000000000000000000000002")
	event := materializationV2GlobalEvent{Event: reorgV2PaymentTokenConfigured, ContractAddress: common.HexToAddress("0x1000000000000000000000000000000000000001"), BlockNumber: 10, BlockHash: common.HexToHash("0x01").Hex(), TxHash: common.HexToHash("0x02").Hex(), LogIndex: 3, Token: &token, Allowed: &allowed, Decimals: &decimals}
	payload, err := jsonv2.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	effect := entity.ChainEventReorgMaterializationEffect{Branch: reorgMaterializationBranchReplacement, ContractVersion: constant.ContractVersionV2Stablecoin, EventType: event.Event, ProjectID: 0, Classification: reorgMaterializationEffectSemantic, ContractAddress: event.ContractAddress.Hex(), BlockNumber: event.BlockNumber, BlockHash: event.BlockHash, TxHash: event.TxHash, LogIndex: event.LogIndex, SemanticPayload: payload, SemanticDigest: sha256Hex(payload)}
	got, err := classifyAssessmentEffect(effect)
	if err != nil {
		t.Fatal(err)
	}
	if got.Replayability != reorgAssessmentReplayConditional || got.BlockerCode != "GLOBAL_CONFIG_BASELINE_REQUIRED" || !strings.Contains(string(payload), `"allowed":false`) {
		t.Fatalf("global configuration assessment drifted: %+v payload=%s", got, payload)
	}
}

func TestAssessmentDigestsBindEvidenceFields(t *testing.T) {
	d := entity.ChainEventReorgAssessmentDisposition{RunID: 1, Ordinal: 2, MaterializationEffectID: 3, SourceEffectDigest: strings.Repeat("a", 64), Branch: reorgMaterializationBranchReplacement, ContractVersion: constant.ContractVersionV2Stablecoin, EventType: commonChain.V2Purchased, ProjectID: 7, Replayability: reorgAssessmentReplayConditional, ActionKind: "apply_v2_purchase_payment_entitlement_and_participant_delta", BlockerCode: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", PlanPayloadDigest: strings.Repeat("b", 64), PreviousRowDigest: reorgZeroDigest}
	root := assessmentDispositionDigest(d)
	d.ActionKind = "apply_v2_payment_refund_fact"
	if root == assessmentDispositionDigest(d) {
		t.Fatal("assessment disposition action was not digest-bound")
	}
	b := entity.ChainEventReorgAssessmentBlocker{RunID: 1, Ordinal: 1, Scope: "run", BlockerCode: "X", SafeDetail: "safe detail", PreviousRowDigest: reorgZeroDigest}
	bRoot := assessmentBlockerDigest(b)
	b.BlockerCode = "Y"
	if bRoot == assessmentBlockerDigest(b) {
		t.Fatal("assessment blocker code was not digest-bound")
	}
	m := entity.ChainEventReorgAssessmentManifest{RunID: 1, IncidentID: 2, MaterializationManifestDigest: strings.Repeat("c", 64), Outcome: reorgAssessmentOutcomeBlocked, DispositionCount: 3, ExactCount: 1, ConditionalCount: 1, BlockedCount: 1, BlockerCount: 9, DispositionDigest: root, BlockerDigest: bRoot}
	mRoot := assessmentManifestDigest(m)
	m.BlockerCount++
	if mRoot == assessmentManifestDigest(m) {
		t.Fatal("assessment manifest blocker count was not digest-bound")
	}
}

func TestAssessmentEffectCapacityReservesMandatoryBlockers(t *testing.T) {
	if reorgAssessmentMaxEffects+reorgAssessmentGlobalBlockerCount != reorgAssessmentMaxBlockers {
		t.Fatalf("effect capacity does not reserve mandatory blockers: effects=%d mandatory=%d blockers=%d", reorgAssessmentMaxEffects, reorgAssessmentGlobalBlockerCount, reorgAssessmentMaxBlockers)
	}
}

func TestPrepareAssessmentPageIsAllOrNothing(t *testing.T) {
	first := assessmentV2Effect(t, commonChain.V2Purchased, 7)
	first.ID, first.RunID, first.Ordinal, first.EffectDigest = 1, 9, 1, strings.Repeat("a", 64)
	second := assessmentV2Effect(t, commonChain.V2Purchased, 8)
	second.ID, second.RunID, second.Ordinal, second.EffectDigest = 2, 9, 2, strings.Repeat("b", 64)
	second.ProjectID++
	prepared, err := prepareAssessmentDispositions([]entity.ChainEventReorgMaterializationEffect{first, second})
	if err == nil || prepared != nil {
		t.Fatalf("invalid second effect returned partial prepared page: len=%d err=%v", len(prepared), err)
	}
}

func assessmentV2Effect(t *testing.T, eventType string, projectID uint64) entity.ChainEventReorgMaterializationEffect {
	t.Helper()
	event := commonChain.V2Event{Event: eventType, ContractAddress: common.HexToAddress("0x1000000000000000000000000000000000000001"), BlockNumber: 10, BlockHash: common.HexToHash("0x01").Hex(), TxHash: common.HexToHash("0x02").Hex(), LogIndex: 3, ProjectID: projectID}
	address1 := common.HexToAddress("0x2000000000000000000000000000000000000002")
	address2 := common.HexToAddress("0x3000000000000000000000000000000000000003")
	address3 := common.HexToAddress("0x4000000000000000000000000000000000000004")
	switch eventType {
	case commonChain.V2ProjectCreated:
		event.ProjectToken, event.PaymentToken, event.ProjectOwner, event.FeeReceiver = address1, address2, address3, address1
		event.ProjectDecimals, event.PaymentDecimals, event.FeeRate = 18, 6, 200
		event.TokenPriceRaw, event.HardCapRaw, event.SoftCapRaw, event.MaxPerUserRaw, event.StartTimeRaw, event.EndTimeRaw = "1", "2", "1", "1", "100", "200"
	case commonChain.V2ProjectTokensDeposited:
		event.ProjectToken, event.ProjectTokenRaw = address1, "2"
	case commonChain.V2Purchased:
		event.User, event.PaymentToken, event.ProjectToken, event.PaymentRaw, event.ProjectTokenRaw = address1, address2, address3, "1", "2"
	case commonChain.V2ProjectEnded:
		event.Status, event.PaymentToken, event.TotalRaisedRaw = 2, address1, "1"
	case commonChain.V2ProjectCancelled:
		event.PaymentToken, event.TotalRaisedRaw = address1, "1"
	case commonChain.V2Refunded:
		event.User, event.PaymentToken, event.PaymentRaw = address1, address2, "1"
	case commonChain.V2Claimed:
		event.User, event.ProjectToken, event.ProjectTokenRaw = address1, address2, "1"
	case commonChain.V2ProjectProceedsWithdrawn:
		event.PaymentToken, event.ProjectOwner, event.PaymentRaw, event.PlatformFeeRaw = address1, address2, "1", "0"
	case commonChain.V2PlatformFeeWithdrawn:
		event.PaymentToken, event.FeeReceiver, event.PaymentRaw = address1, address2, "1"
	case commonChain.V2ProjectTokensWithdrawn:
		event.ProjectToken, event.ProjectOwner, event.ProjectTokenRaw = address1, address2, "1"
	}
	payload, err := jsonv2.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return entity.ChainEventReorgMaterializationEffect{Branch: reorgMaterializationBranchReplacement, ContractVersion: constant.ContractVersionV2Stablecoin, EventType: eventType, ProjectID: projectID, Classification: reorgMaterializationEffectSemantic, ContractAddress: event.ContractAddress.Hex(), BlockNumber: event.BlockNumber, BlockHash: event.BlockHash, TxHash: event.TxHash, LogIndex: event.LogIndex, SemanticPayload: payload, SemanticDigest: sha256Hex(payload)}
}

func assessmentV1Effect(t *testing.T, eventType string, projectID int64) entity.ChainEventReorgMaterializationEffect {
	t.Helper()
	payload, err := jsonv2.Marshal(commonChain.Data{ProjectID: projectID})
	if err != nil {
		t.Fatal(err)
	}
	return entity.ChainEventReorgMaterializationEffect{Branch: reorgMaterializationBranchOld, ContractVersion: constant.ContractVersionV1Legacy, EventType: eventType, ProjectID: uint64(projectID), Classification: reorgMaterializationEffectSemantic, SemanticPayload: payload, SemanticDigest: sha256Hex(payload)}
}
