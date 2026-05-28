package agent_svc

import (
	"path/filepath"
	"testing"
)

func TestNormalizeObjectKeyRejectsTraversal(t *testing.T) {
	if _, err := normalizeObjectKey("../secrets.png"); err == nil {
		t.Fatal("normalizeObjectKey() error = nil, want traversal rejection")
	}
	if _, err := normalizeObjectKey("user-1/../secrets.png"); err == nil {
		t.Fatal("normalizeObjectKey() nested traversal error = nil, want rejection")
	}
}

func TestNormalizeObjectKeyKeepsRelativeScopedKey(t *testing.T) {
	got, err := normalizeObjectKey("user-1/conversation-2/run-3/object.png")
	if err != nil {
		t.Fatalf("normalizeObjectKey() error = %v", err)
	}
	want := filepath.FromSlash("user-1/conversation-2/run-3/object.png")
	if got != want {
		t.Fatalf("normalizeObjectKey() = %q, want scoped key", got)
	}
}
