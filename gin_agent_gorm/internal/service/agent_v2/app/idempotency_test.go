package app

import "testing"

func TestNormalizeIdempotencyKeyTrimsAndBoundsKey(t *testing.T) {
	key := normalizeIdempotencyKey("  " + repeat("a", 160) + "  ")

	if len(key) != maxIdempotencyKeyLength {
		t.Fatalf("len(key) = %d, want %d", len(key), maxIdempotencyKeyLength)
	}
	for _, r := range key {
		if r != 'a' {
			t.Fatalf("key contains %q, want only a", r)
		}
	}
}

func repeat(value string, count int) string {
	output := ""
	for i := 0; i < count; i++ {
		output += value
	}
	return output
}
