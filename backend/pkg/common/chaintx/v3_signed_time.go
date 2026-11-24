package chaintx

import (
	"time"

	"joincare_server/pkg/common/entity"
)

// V3SignedBeforeSet checks the immutable DB-generated UTC signing timestamp.
// MySQL DATETIME carries no zone; the shared DSN parses it using loc=Local.
// Reinterpret its wall-clock fields as UTC, rather than comparing a shifted
// local instant with the signed Set's Unix deadline. This is historical
// evidence only: it never grants permission to sign or send a new attempt.
func V3SignedBeforeSet(row entity.ChainTxSubmission, expiresAt int64) bool {
	if expiresAt <= 1 || row.V3SignedAt == nil || row.V3SignedAt.IsZero() {
		return false
	}
	v := row.V3SignedAt
	signedUTC := time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), time.UTC)
	return signedUTC.Before(time.Unix(expiresAt, 0))
}
