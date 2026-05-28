package security

import (
	"context"
	"strings"
	"testing"
)

func TestStaticSafetyPolicyBlocksConfiguredTextTerm(t *testing.T) {
	result, err := StaticSafetyPolicy{
		Enabled:      true,
		FailClosed:   true,
		BlockedTerms: []string{"blocked"},
	}.CheckContent(context.Background(), SafetyRequest{Text: "a blocked request"})
	if err != nil {
		t.Fatalf("CheckContent() error = %v", err)
	}
	if result.Allowed {
		t.Fatalf("Allowed = true, want blocked")
	}
}

func TestValidateImageExtensionRejectsMismatch(t *testing.T) {
	err := ValidateImageExtension("poster.txt", "image/png")
	if err == nil {
		t.Fatal("ValidateImageExtension() error = nil, want mismatch")
	}
	if !strings.Contains(err.Error(), ".png") {
		t.Fatalf("error = %q, want allowed extension detail", err.Error())
	}
}

func TestRandomObjectKeyPartIsRandom(t *testing.T) {
	first := RandomObjectKeyPart()
	second := RandomObjectKeyPart()
	if first == "" || second == "" || first == second {
		t.Fatalf("RandomObjectKeyPart() values = %q/%q, want non-empty unique values", first, second)
	}
}
