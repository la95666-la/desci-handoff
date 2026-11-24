package shared

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCapCommandReportIncludesMarkerWithinLimits(t *testing.T) {
	log, errMsg := CapCommandReport(strings.Repeat("界", MaxCommandLogBytes), strings.Repeat("错", MaxCommandErrorBytes))
	if len(log) > MaxCommandLogBytes || len(errMsg) > MaxCommandErrorBytes {
		t.Fatalf("limits exceeded: log=%d error=%d", len(log), len(errMsg))
	}
	if !utf8.ValidString(log) || !utf8.ValidString(errMsg) {
		t.Fatal("capped report is not valid UTF-8")
	}
	if !strings.Contains(log, "[report truncated]") || !strings.Contains(errMsg, "[report truncated]") {
		t.Fatal("truncation marker missing")
	}
}
