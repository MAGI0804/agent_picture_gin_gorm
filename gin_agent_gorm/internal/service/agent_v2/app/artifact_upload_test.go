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

	metadata, err := validateImageUpload(content, "poster.png", "image/png")
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

	if _, err := validateImageUpload(content, "poster.png", "image/jpeg"); err == nil {
		t.Fatal("validateImageUpload() error = nil, want mime mismatch")
	}
}

func TestValidateImageUploadRejectsDisallowedExtension(t *testing.T) {
	content, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lz5C2wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if _, err := validateImageUpload(content, "poster.txt", "image/png"); err == nil {
		t.Fatal("validateImageUpload() error = nil, want extension rejection")
	}
}

func TestUploadObjectKeyUsesRandomSegment(t *testing.T) {
	first := uploadObjectKey(7, 8, "poster.png")
	second := uploadObjectKey(7, 8, "poster.png")
	if first == second {
		t.Fatalf("uploadObjectKey() returned predictable duplicate %q", first)
	}
	wantPrefix := "user-7/conversation-8/uploads/"
	if len(first) <= len(wantPrefix) || first[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("uploadObjectKey() = %q, want prefix %q", first, wantPrefix)
	}
	if first == wantPrefix+"poster.png" {
		t.Fatalf("uploadObjectKey() = %q, want random path segment", first)
	}
}
