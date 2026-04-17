package app

import (
	"strings"
	"testing"
)

func TestNewIDReturnsPrefixedULID(t *testing.T) {
	t.Parallel()

	got := newID("apply")
	if !strings.HasPrefix(got, "apply-") {
		t.Fatalf("newID() = %q, want prefix %q", got, "apply-")
	}

	ulid := strings.TrimPrefix(got, "apply-")
	if len(ulid) != 26 {
		t.Fatalf("ULID length = %d, want 26", len(ulid))
	}

	for _, r := range ulid {
		if !strings.ContainsRune(crockfordBase32, r) {
			t.Fatalf("ULID contains invalid character %q", r)
		}
	}
}
