package idov3whitelist

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// ApprovedStoredSaleSeconds accepts only the exact DATETIME(0) rounding of
// the immutable reviewed request. It returns the original contract seconds,
// not the rounded database seconds. Call only after ReadApprovedSetRecord has
// authenticated the executed decision, binding and calldata commitment.
func ApprovedStoredSaleSeconds(record ApprovedSetRecord, storedStart, storedEnd time.Time) (int64, int64, error) {
	invalid := errors.New("V3 approved sale time unavailable")
	if record.Identity.CreateDeadline <= 1 || len(record.Request) == 0 {
		return 0, 0, invalid
	}
	digest := sha256.Sum256(record.Request)
	if "0x"+hex.EncodeToString(digest[:]) != record.Identity.RequestDigest {
		return 0, 0, invalid
	}
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(record.Request))
	if decoder.Decode(&raw) != nil || decoder.Decode(new(any)) != io.EOF || len(raw) == 0 {
		return 0, 0, invalid
	}
	var startText, endText string
	if json.Unmarshal(raw["start_time"], &startText) != nil || json.Unmarshal(raw["end_time"], &endText) != nil {
		return 0, 0, invalid
	}
	start, e1 := time.Parse(time.RFC3339, strings.TrimSpace(startText))
	end, e2 := time.Parse(time.RFC3339, strings.TrimSpace(endText))
	if e1 != nil || e2 != nil || start.Unix() != record.Identity.CreateDeadline ||
		!end.After(start) || !storedStart.Equal(start.Round(time.Second)) ||
		!storedEnd.Equal(end.Round(time.Second)) {
		return 0, 0, invalid
	}
	return start.Unix(), end.Unix(), nil
}
