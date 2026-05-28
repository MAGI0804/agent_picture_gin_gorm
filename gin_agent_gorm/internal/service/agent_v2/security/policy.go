package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// SafetyRequest is the provider-neutral content safety check input.
type SafetyRequest struct {
	Text     string
	ImageRef string
}

// SafetyResult is the provider-neutral content safety check output.
type SafetyResult struct {
	Allowed bool
	Reason  string
}

// StaticSafetyPolicy is a local fail-closed safety policy used when no external
// moderation provider is configured yet.
type StaticSafetyPolicy struct {
	Enabled      bool
	FailClosed   bool
	BlockedTerms []string
}

// CheckContent validates text/image references without logging sensitive input.
func (policy StaticSafetyPolicy) CheckContent(ctx context.Context, request SafetyRequest) (SafetyResult, error) {
	if err := ctx.Err(); err != nil {
		return SafetyResult{}, err
	}
	if !policy.Enabled {
		if policy.FailClosed {
			return SafetyResult{Allowed: false, Reason: "safety provider disabled"}, nil
		}
		return SafetyResult{Allowed: true, Reason: "safety provider disabled in fail-open mode"}, nil
	}
	normalizedText := strings.ToLower(request.Text)
	for _, term := range policy.BlockedTerms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term != "" && strings.Contains(normalizedText, term) {
			return SafetyResult{Allowed: false, Reason: "blocked text policy term"}, nil
		}
	}
	if strings.Contains(normalizedText, "data:image/") || strings.Contains(normalizedText, "base64,") {
		return SafetyResult{Allowed: false, Reason: "inline binary image data is not allowed in prompt text"}, nil
	}
	return SafetyResult{Allowed: true, Reason: "allowed"}, nil
}

// RandomObjectKeyPart returns an unguessable object-key path segment.
func RandomObjectKeyPart() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err == nil {
		return hex.EncodeToString(data[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ValidateImageExtension requires the filename extension to match the detected MIME type.
func ValidateImageExtension(fileName string, mimeType string) error {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	allowed := allowedExtensionsForMime(mimeType)
	if len(allowed) == 0 {
		return fmt.Errorf("unsupported image mime %q", mimeType)
	}
	if extension == "" {
		return errorsForExtension(mimeType, allowed)
	}
	for _, candidate := range allowed {
		if extension == candidate {
			return nil
		}
	}
	return errorsForExtension(mimeType, allowed)
}

func allowedExtensionsForMime(mimeType string) []string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return []string{".png"}
	case "image/jpeg":
		return []string{".jpg", ".jpeg"}
	case "image/gif":
		return []string{".gif"}
	default:
		return nil
	}
}

func errorsForExtension(mimeType string, allowed []string) error {
	return fmt.Errorf("upload extension is not allowed for %s: expected %s", mimeType, strings.Join(allowed, ", "))
}
