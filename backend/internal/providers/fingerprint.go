package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Fingerprint identifies "the same observation" across repeated fetches so
// retries are idempotent against the quotes table's unique index (see
// PLAN.md § Data model). It's a hash of the fields that define a distinct
// quote, deliberately excluding FetchedAt and price — two fetches of the
// same flight/date/carrier are the same fingerprint even if the price
// moved, which is exactly what lets an UPSERT-by-fingerprint model work.
func Fingerprint(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])[:16]
}
