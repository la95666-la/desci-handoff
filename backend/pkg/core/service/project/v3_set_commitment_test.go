package project

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestV3SetCommitmentRejectsResolvedRequestMismatchWithoutWrite(t *testing.T) {
	m := testV3SetManifest()
	req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		PaymentToken: m.PaymentSymbol, TokenAddress: m.ProjectToken, OwnerAddress: m.ProjectOwner,
		TokenPrice: "1", HardCap: "10", SoftCap: "5", MaxPerUser: "8"}
	digest, err := v3SetDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	m.RequestDigest = digest.Hex()
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	res := &resolvedOnchainParams{Req: req, Trusted: trustedPaymentToken{ChainID: m.ChainID,
		Contract: m.ProxyAddress, Address: m.PaymentToken}, Start: start, End: start.Add(time.Hour),
		PriceRaw: "1", HardRaw: "10", SoftRaw: "5", MaxRaw: "8",
		TGERows: []entity.ProjectTge{{Index: 1, VestingTime: start.Add(2 * time.Hour), VestingPercent: decimal.NewFromInt(100)}}}
	database, mock := v2AdmissionMock(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	tx := database.Begin()
	// Same signed request in the outer API, but a different payload in the
	// resolved object must never create an apparently reviewed commitment.
	res.Req.TokenPrice = "2"
	if err := storeV3SetCommitmentTx(tx, int64(m.ProjectID), req, res, m); err == nil {
		t.Fatal("mismatched resolved request committed")
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A project-specific approval can deterministically commit its create calldata
// without consuming the legacy single-project release SetManifest. This is an
// encoder test only: it does not authorize a Set write or a relayer send.
func TestV3CreateEncoderCanBindApprovedProjectWithoutReleaseManifest(t *testing.T) {
	m := testV3SetManifest()
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	res := &resolvedOnchainParams{Req: OnchainParamsRequest{TokenAddress: m.ProjectToken, OwnerAddress: m.ProjectOwner}, Trusted: trustedPaymentToken{ChainID: m.ChainID, Contract: m.ProxyAddress,
		Address: m.PaymentToken, FeeReceiver: m.FeeReceiver, SuccessFeeRate: m.FeeRate}, Start: start, End: start.Add(time.Hour),
		PriceRaw: "1", HardRaw: "10", SoftRaw: "5", MaxRaw: "8",
		TGERows: []entity.ProjectTge{{Index: 1, VestingTime: start.Add(2 * time.Hour), VestingPercent: decimal.NewFromInt(100)}}}
	legacy, err := v3SetCreateCalldata(m.ProjectID, res, m)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := packV3CreateFromResolved(m.ProjectID, res, m.ProjectToken, m.PaymentToken, m.ProjectOwner, m.FeeReceiver, m.FeeRate)
	if err != nil || !bytes.Equal(legacy, approved) {
		t.Fatalf("approved calldata differs from historical encoding: %v", err)
	}
	for _, tc := range []struct{ name, token, owner string }{
		{"payment token reused as project token", m.PaymentToken, m.ProjectOwner},
		{"missing project owner", m.ProjectToken, "0x0000000000000000000000000000000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := packV3CreateFromResolved(m.ProjectID, res, tc.token, m.PaymentToken, tc.owner, m.FeeReceiver, m.FeeRate); err == nil {
				t.Fatal("invalid approval terms accepted")
			}
		})
	}
}
