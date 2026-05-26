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
