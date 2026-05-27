package app

import (
	"encoding/base64"
	"testing"
)

func TestValidateImageUploadAcceptsSmallPNG(t *testing.T) {
	content, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lz5C2wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	metadata, err := validateImageUpload(content, "image/png")
	if err != nil {
		t.Fatalf("validateImageUpload() error = %v", err)
	}
	if metadata.MimeType != "image/png" || metadata.Width != 1 || metadata.Height != 1 {
		t.Fatalf("metadata = %#v, want 1x1 png", metadata)
	}
}

func TestValidateImageUploadRejectsMismatchedMime(t *testing.T) {
	content, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lz5C2wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if _, err := validateImageUpload(content, "image/jpeg"); err == nil {
		t.Fatal("validateImageUpload() error = nil, want mime mismatch")
	}
}
