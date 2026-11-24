package project

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3ApprovedSetIdentityBindsApprovalAndExactCreateCall(t *testing.T) {
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		PaymentToken: "USDT", TokenAddress: v3TokenTESTCAT, OwnerAddress: v3DiagnosticOperator,
		TokenPrice: "1", HardCap: "10", SoftCap: "5", MaxPerUser: "2"}
	res := &resolvedOnchainParams{Req: req, Trusted: trustedPaymentToken{
		Symbol: "USDT", Address: v3TokenUSDT, ChainID: 56, Contract: v3TokenProxy,
		Decimals: 18, FeeReceiver: v3DiagnosticGovernor, SuccessFeeRate: 100},
		TokenDecimals: 18, PriceRaw: "1", HardRaw: "10", SoftRaw: "5", MaxRaw: "2", Start: start,
		End: start.Add(time.Hour), TGERows: []entity.ProjectTge{{Index: 1,
			VestingTime: start.Add(2 * time.Hour), VestingPercent: decimal.NewFromInt(100)}}}
	call, err := packV3CreateFromResolved(19, res, req.TokenAddress, v3TokenUSDT,
		req.OwnerAddress, res.Trusted.FeeReceiver, res.Trusted.SuccessFeeRate)
	if err != nil {
		t.Fatal(err)
	}
	base, digest, err := makeV3ApprovedSetIdentity(19, 33, 4, 5, req, res, call)
	if err != nil || digest == ([32]byte{}) || base.CreateCallHash == "" {
		t.Fatalf("valid approved terms rejected: %v", err)
	}
	other, changed, err := makeV3ApprovedSetIdentity(19, 34, 4, 5, req, res, call)
	if err != nil || changed == digest || other.ApprovalID == base.ApprovalID {
		t.Fatal("approval identity not bound")
	}
	if _, _, err := makeV3ApprovedSetIdentity(19, 33, 4, 4, req, res, call); err == nil {
		t.Fatal("self-approval accepted")
	}
	mutated := req
	mutated.TokenPrice = "2"
	if _, _, err := makeV3ApprovedSetIdentity(19, 33, 4, 5, mutated, res, call); err == nil {
		t.Fatal("changed request accepted")
	}
	mutatedCall := append([]byte(nil), call...)
	mutatedCall[len(mutatedCall)-1] ^= 1
	if _, _, err := makeV3ApprovedSetIdentity(19, 33, 4, 5, req, res, mutatedCall); err == nil {
		t.Fatal("changed create call accepted")
	}
	if err := verifyV3ApprovedSetCommitment(base, 33, 4, 5, true, base.RequestDigest, digest.Hex(), base.CreateCallHash); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	record := idov3whitelist.ApprovedSetRecord{Identity: base, Request: encoded}
	if got, err := ValidateApprovedV3SetRequest(record, 19, call); err != nil || got.TokenPrice != req.TokenPrice {
		t.Fatalf("exact approval payload rejected: %v", err)
	}
	changedPrice := req
	changedPrice.TokenPrice = "2"
	changedPayload, _ := json.Marshal(changedPrice)
	for _, tc := range []struct {
		name   string
		modify func(*idov3whitelist.ApprovedSetRecord, *[]byte)
	}{
		{"foreign project", func(r *idov3whitelist.ApprovedSetRecord, _ *[]byte) { r.Identity.ProjectID++ }},
		{"changed amount", func(r *idov3whitelist.ApprovedSetRecord, _ *[]byte) { r.Request = changedPayload }},
		{"changed call", func(_ *idov3whitelist.ApprovedSetRecord, b *[]byte) { (*b)[len(*b)-1] ^= 1 }},
		{"trailing payload", func(r *idov3whitelist.ApprovedSetRecord, _ *[]byte) { r.Request = append(r.Request, []byte(" {}")...) }},
		{"unknown parameter", func(r *idov3whitelist.ApprovedSetRecord, _ *[]byte) {
			r.Request = append(r.Request[:len(r.Request)-1], []byte(`,"unreviewed":1}`)...)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := record
			candidate.Request = append([]byte(nil), record.Request...)
			bytes := append([]byte(nil), call...)
			tc.modify(&candidate, &bytes)
			if _, err := ValidateApprovedV3SetRequest(candidate, 19, bytes); err == nil {
				t.Fatal("altered approval payload or call accepted")
			}
		})
	}
	if err := verifyV3ApprovedSetCommitment(base, 33, 5, 4, true, base.RequestDigest, digest.Hex(), base.CreateCallHash); err == nil {
		t.Fatal("swapped approval actors accepted")
	}
	if _, _, err := makeV3ApprovedSetIdentity(20, 33, 4, 5, req, res, call); err == nil {
		t.Fatal("different project ID borrowed approved calldata")
	}
	for _, tc := range []struct {
		name                    string
		identity                v3ApprovedSetIdentity
		approved                bool
		request, digest, create string
	}{
		{"pending", base, false, base.RequestDigest, digest.Hex(), base.CreateCallHash},
		{"different request", base, true, changed.Hex(), digest.Hex(), base.CreateCallHash},
		{"different create calldata", base, true, base.RequestDigest, digest.Hex(), changed.Hex()},
		{"different commitment", base, true, base.RequestDigest, changed.Hex(), base.CreateCallHash},
		{"changed approval", other, true, other.RequestDigest, changed.Hex(), other.CreateCallHash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyV3ApprovedSetCommitment(tc.identity, 33, 4, 5, tc.approved, tc.request, tc.digest, tc.create); err == nil {
				t.Fatal("unapproved or altered Set commitment accepted")
			}
		})
	}
	for _, mutate := range []func(*v3ApprovedSetIdentity){
		func(v *v3ApprovedSetIdentity) { v.ChainID = 97 },
		func(v *v3ApprovedSetIdentity) { v.Proxy = v.PaymentToken },
		func(v *v3ApprovedSetIdentity) { v.ProjectToken = v.PaymentToken },
		func(v *v3ApprovedSetIdentity) { v.TokenDecimals = 0 },
		func(v *v3ApprovedSetIdentity) { v.ApproverID = v.InitiatorID },
		func(v *v3ApprovedSetIdentity) { v.CreateCallHash = "0xnot-a-hash" },
		func(v *v3ApprovedSetIdentity) { v.CreateDeadline = 0 },
		func(v *v3ApprovedSetIdentity) { v.RequestDigest = "0x" + strings.Repeat("g", 64) },
	} {
		candidate := base
		mutate(&candidate)
		if _, err := digestV3ApprovedSetIdentity(candidate); err == nil {
			t.Fatalf("foreign approved Set identity accepted: %+v", candidate)
		}
	}
}
