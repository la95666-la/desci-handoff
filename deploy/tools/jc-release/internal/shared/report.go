package shared

import "unicode/utf8"

const (
	MaxCommandLogBytes    = 256 * 1024
	MaxCommandErrorBytes  = 8 * 1024
	MaxCommandStatusBytes = len(StatusInProgress)
)

const reportTruncationMarker = "\n[report truncated]\n"

// CapUTF8Bytes returns a valid UTF-8 string no larger than limit bytes. When
// truncation is required, the marker is included within the same byte budget.
func CapUTF8Bytes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit && utf8.ValidString(value) {
		return value
	}
	marker := reportTruncationMarker
	if len(marker) > limit {
		marker = marker[:limit]
	}
	keep := limit - len(marker)
	if keep < 0 {
		keep = 0
	}
	if keep > len(value) {
		keep = len(value)
	}
	for keep > 0 && !utf8.ValidString(value[:keep]) {
		keep--
	}
	return value[:keep] + marker
}

func CapCommandReport(log, errMsg string) (string, string) {
	return CapUTF8Bytes(log, MaxCommandLogBytes), CapUTF8Bytes(errMsg, MaxCommandErrorBytes)
}

func ValidTerminalReportStatus(status string) bool {
	return status == StatusSuccess || status == StatusFailed
}
