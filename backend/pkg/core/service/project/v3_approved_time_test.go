package project

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestNewApprovedV3SetRefusesFractionalTimeWithoutChangingReviewedRequest(t *testing.T) {
	start := time.Date(2030, 1, 1, 2, 0, 53, 586000000, time.UTC)
	end := start.Add(time.Hour)
	req := OnchainParamsRequest{StartTime: start.Format(time.RFC3339Nano), EndTime: end.Format(time.RFC3339Nano)}
	rows := []entity.ProjectTge{{Index: 1, VestingTime: end.Add(10 * time.Minute)}}
	res := &resolvedOnchainParams{Req: req, Start: start, End: end, TGERows: rows}
	if validateNewV3SetTimePrecision(res) == nil {
		t.Fatal("fractional approval silently became a Set")
	}
	res.Start = start.Truncate(time.Second)
	res.End = end.Truncate(time.Second)
	res.TGERows[0].VestingTime = rows[0].VestingTime.Truncate(time.Second)
	if validateNewV3SetTimePrecision(res) == nil {
		t.Fatal("rounding a reviewed request after approval was accepted")
	}
	res.Req.StartTime = res.Start.Format(time.RFC3339)
	res.Req.EndTime = res.End.Format(time.RFC3339)
	if err := validateNewV3SetTimePrecision(res); err != nil {
		t.Fatalf("exact-second Set refused: %v", err)
	}
	res.TGERows[0].VestingTime = end.Add(10 * time.Minute)
	if validateNewV3SetTimePrecision(res) == nil {
		t.Fatal("fractional vesting accepted")
	}
}

func TestApprovedV3CreateReconstructsOnlyReviewedDATETIME0Rounding(t *testing.T) {
	start := time.Date(2030, 1, 1, 7, 0, 53, 586000000, time.UTC)
	end := start.Add(2 * time.Hour)
	req := OnchainParamsRequest{StartTime: start.Format(time.RFC3339Nano),
		EndTime: end.Format(time.RFC3339Nano), TGEAfterEndMin: 10, TGEIntervalMin: 60,
		TGE: []OnchainTGEEntry{{Percent: decimal.NewFromInt(40)}, {Percent: decimal.NewFromInt(60)}}}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	record := idov3whitelist.ApprovedSetRecord{Request: raw,
		Identity: idov3whitelist.ApprovedSetIdentity{CreateDeadline: start.Unix(), RequestDigest: fmt.Sprintf("0x%x", digest)}}
	project := entity.Project{StartTime: start.Round(time.Second), EndTime: end.Round(time.Second),
		TgeTime: end.Add(10 * time.Minute).Round(time.Second)}
	rows := []entity.ProjectTge{
		{Index: 1, VestingTime: end.Add(10 * time.Minute).Round(time.Second)},
		{Index: 2, VestingTime: end.Add(70 * time.Minute).Round(time.Second)},
	}
	check := func(want bool) {
		t.Helper()
		s, e, vesting, err := approvedV3CreateTimes(project, rows, record)
		if (err == nil) != want {
			t.Fatalf("admission=%t err=%v", want, err)
		}
		if want && (s != start.Unix() || e != end.Unix() || len(vesting) != 2 ||
			vesting[0].Int64() != end.Add(10*time.Minute).Unix() ||
			vesting[1].Int64() != end.Add(70*time.Minute).Unix()) {
			t.Fatalf("approved calldata times changed: start=%d end=%d vesting=%v", s, e, vesting)
		}
	}
	check(true)
	// Production reads DATETIME with loc=Local (Asia/Shanghai), while the
	// reviewed RFC3339 timestamps are UTC. Compare instants, not wall clocks.
	local := time.FixedZone("Asia/Shanghai", 8*60*60)
	project.StartTime = project.StartTime.In(local)
	project.EndTime = project.EndTime.In(local)
	project.TgeTime = project.TgeTime.In(local)
	for i := range rows {
		rows[i].VestingTime = rows[i].VestingTime.In(local)
	}
	check(true)
	project.StartTime = project.StartTime.Add(time.Second)
	check(false)
	project.StartTime = start.Round(time.Second)
	rows[1].VestingTime = rows[1].VestingTime.Add(time.Second)
	check(false)
	rows[1].VestingTime = end.Add(70 * time.Minute).Round(time.Second)
	project.EndTime = project.EndTime.Add(time.Second)
	check(false)
	project.EndTime = end.Round(time.Second)
	project.TgeTime = project.TgeTime.Add(time.Second)
	check(false)
	project.TgeTime = end.Add(10 * time.Minute).Round(time.Second)
	record.Request = append([]byte(nil), raw...)
	record.Request = append(record.Request, ' ')
	check(false)
	record.Request = raw
	record.Identity.CreateDeadline++
	check(false)
	record.Identity.CreateDeadline--
	record.Identity.RequestDigest = "0x" + fmt.Sprintf("%064x", 1)
	check(false)
}

func TestApprovedV3CreateRejectsUnreviewedRounding(t *testing.T) {
	start := time.Date(2030, 1, 1, 7, 0, 53, 420000000, time.UTC)
	end := start.Add(2 * time.Hour)
	req := OnchainParamsRequest{StartTime: start.Format(time.RFC3339Nano), EndTime: end.Format(time.RFC3339Nano),
		TGEAfterEndMin: 10, TGE: []OnchainTGEEntry{{Percent: decimal.NewFromInt(100)}}}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	record := idov3whitelist.ApprovedSetRecord{Request: raw,
		Identity: idov3whitelist.ApprovedSetIdentity{CreateDeadline: start.Unix(), RequestDigest: fmt.Sprintf("0x%x", digest)}}
	project := entity.Project{StartTime: start.Round(time.Second), EndTime: end.Round(time.Second),
		TgeTime: end.Add(10 * time.Minute).Round(time.Second)}
	rows := []entity.ProjectTge{{Index: 1, VestingTime: project.TgeTime}}
	s, e, vesting, err := approvedV3CreateTimes(project, rows, record)
	if err != nil || s != start.Unix() || e != end.Unix() || len(vesting) != 1 || vesting[0].Int64() != end.Add(10*time.Minute).Unix() {
		t.Fatalf("exact reviewed seconds unavailable: %d %d %v %v", s, e, vesting, err)
	}
	project.StartTime = project.StartTime.Add(time.Second)
	if _, _, _, err := approvedV3CreateTimes(project, rows, record); err == nil {
		t.Fatal("a stored value outside MySQL rounding was accepted")
	}
}
