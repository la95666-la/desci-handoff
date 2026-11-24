package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"strings"
	"time"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// New Set decisions must use the precision supported by the existing MySQL
// DATETIME columns and the contract's uint256 seconds. Never silently change
// the reviewed request or its digest after approval.
func validateNewV3SetTimePrecision(res *resolvedOnchainParams) error {
	invalid := errors.New("V3 Set times must be whole seconds")
	if res == nil || res.Start.Nanosecond() != 0 || res.End.Nanosecond() != 0 {
		return invalid
	}
	start, e1 := time.Parse(time.RFC3339, strings.TrimSpace(res.Req.StartTime))
	end, e2 := time.Parse(time.RFC3339, strings.TrimSpace(res.Req.EndTime))
	if e1 != nil || e2 != nil || start.Nanosecond() != 0 || end.Nanosecond() != 0 ||
		!start.Equal(res.Start) || !end.Equal(res.End) {
		return invalid
	}
	for _, row := range res.TGERows {
		if row.VestingTime.Nanosecond() != 0 {
			return invalid
		}
	}
	return nil
}

// Previously executed approvals may contain fractional seconds. MySQL's
// DATETIME(0) rounded the stored wall-clock value, but the immutable approval
// and create calldata committed to the original Unix seconds. Reconstruct
// only those reviewed seconds; accept no arbitrary DB drift and never rewrite
// immutable Set evidence. A later calldata-hash check binds all the terms.
func approvedV3CreateTimes(p entity.Project, tges []entity.ProjectTge,
	record idov3whitelist.ApprovedSetRecord) (int64, int64, []*big.Int, error) {
	invalid := errors.New("V3 approved create time identity unavailable")
	var req OnchainParamsRequest
	decoder := json.NewDecoder(bytes.NewReader(record.Request))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&req) != nil || decoder.Decode(&extra) != io.EOF {
		return 0, 0, nil, invalid
	}
	encoded, err := json.Marshal(req)
	requestHash := sha256.Sum256(record.Request)
	if err != nil || !bytes.Equal(encoded, record.Request) || len(tges) == 0 || len(tges) != len(req.TGE) || len(tges) > 10 ||
		"0x"+hex.EncodeToString(requestHash[:]) != record.Identity.RequestDigest ||
		req.TGEAfterEndMin <= 0 || req.TGEIntervalMin < 0 {
		return 0, 0, nil, invalid
	}
	startSecond, endSecond, err := idov3whitelist.ApprovedStoredSaleSeconds(record, p.StartTime, p.EndTime)
	if err != nil {
		return 0, 0, nil, invalid
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndTime))
	if err != nil {
		return 0, 0, nil, invalid
	}
	times := make([]*big.Int, 0, len(tges))
	const maxMinutes = math.MaxInt64 / int64(time.Minute)
	if int64(req.TGEAfterEndMin) > maxMinutes || int64(req.TGEIntervalMin) > maxMinutes/int64(len(tges)) ||
		!p.TgeTime.Equal(end.Add(time.Duration(req.TGEAfterEndMin)*time.Minute).Round(time.Second)) {
		return 0, 0, nil, invalid
	}
	for i, row := range tges {
		if i > 0 && int64(req.TGEIntervalMin) > (maxMinutes-int64(req.TGEAfterEndMin))/int64(i) {
			return 0, 0, nil, invalid
		}
		offset := int64(req.TGEAfterEndMin) + int64(i)*int64(req.TGEIntervalMin)
		if offset > maxMinutes || row.Index != i+1 || !row.VestingTime.Equal(end.Add(time.Duration(offset)*time.Minute).Round(time.Second)) {
			return 0, 0, nil, invalid
		}
		times = append(times, big.NewInt(end.Add(time.Duration(offset)*time.Minute).Unix()))
	}
	return startSecond, endSecond, times, nil
}
