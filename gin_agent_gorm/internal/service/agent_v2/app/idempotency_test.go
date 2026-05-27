package app

import (
	"testing"

	"gin-biz-web-api/model"
)

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

func TestIdempotencyKeyUniqueValueUsesNilForMissingKey(t *testing.T) {
	if got := idempotencyKeyUniqueValue("   "); got != nil {
		t.Fatalf("idempotencyKeyUniqueValue() = %v, want nil", *got)
	}
}

func TestIdempotencyKeyUniqueValueNormalizesUserKey(t *testing.T) {
	got := idempotencyKeyUniqueValue("  run-key  ")
	if got == nil || *got != "run-key" {
		t.Fatalf("idempotencyKeyUniqueValue() = %v, want run-key", got)
	}
}

func TestIsUniqueConstraintErrorRecognizesCommonDrivers(t *testing.T) {
	tests := []string{
		"Error 1062: Duplicate entry '1-key' for key 'idx_agent_runs_user_idempotency_unique'",
		"UNIQUE constraint failed: agent_runs.user_id, agent_runs.idempotency_key_unique",
		"duplicate key value violates unique constraint",
	}
	for _, message := range tests {
		t.Run(message, func(t *testing.T) {
			if !isUniqueConstraintError(fakeError(message)) {
				t.Fatalf("isUniqueConstraintError(%q) = false, want true", message)
			}
		})
	}
}

type fakeError string

func (err fakeError) Error() string {
	return string(err)
}

func TestPublicArtifactsHideObjectKeyAndUsePreviewProxy(t *testing.T) {
	artifacts := publicArtifacts([]model.Artifact{
		{
			BaseModel:  model.BaseModel{ID: 12},
			ObjectKey:  "user-1/private.png",
			PreviewURL: "/artifacts/user-1/private.png",
		},
	})

	if artifacts[0].ObjectKey != "" {
		t.Fatalf("ObjectKey = %q, want hidden", artifacts[0].ObjectKey)
	}
	if artifacts[0].PreviewURL != "/api/v2/artifacts/12/preview" {
		t.Fatalf("PreviewURL = %q, want preview proxy", artifacts[0].PreviewURL)
	}
}

func TestPublicArtifactVersionsHideStorageRefs(t *testing.T) {
	versions := publicArtifactVersions([]model.ArtifactVersion{
		{
			BaseModel:  model.BaseModel{ID: 22},
			ObjectKey:  "user-1/private.png",
			PreviewURL: "/artifacts/user-1/private.png",
		},
	})

	if versions[0].ObjectKey != "" || versions[0].PreviewURL != "" {
		t.Fatalf("version storage refs = %q/%q, want hidden", versions[0].ObjectKey, versions[0].PreviewURL)
	}
}

func repeat(value string, count int) string {
	output := ""
	for i := 0; i < count; i++ {
		output += value
	}
	return output
}
