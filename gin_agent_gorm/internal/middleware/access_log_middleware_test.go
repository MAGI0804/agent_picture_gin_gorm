package middleware

import (
	"net/http"
	"strings"
	"testing"
)

func TestSanitizedHeadersRedactsAuthSecrets(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret")
	headers.Set("token", "jwt")
	headers.Set("User-Agent", "test")

	got := sanitizedHeaders(headers)
	if got["Authorization"][0] != "[REDACTED]" || got["Token"][0] != "[REDACTED]" {
		t.Fatalf("sanitized headers = %#v, want auth values redacted", got)
	}
	if got["User-Agent"][0] != "test" {
		t.Fatalf("User-Agent = %q, want preserved", got["User-Agent"][0])
	}
}

func TestSanitizedLogTextRedactsPromptAndTokens(t *testing.T) {
	body := `{"prompt":"secret launch prompt","content":"user sensitive request","nested":{"api_key":"abc123"},"safe":"ok"}`

	got := sanitizedLogText(body)
	if strings.Contains(got, "secret launch prompt") || strings.Contains(got, "user sensitive request") || strings.Contains(got, "abc123") {
		t.Fatalf("sanitizedLogText() leaked sensitive value: %s", got)
	}
	if !strings.Contains(got, `"safe":"ok"`) {
		t.Fatalf("sanitizedLogText() = %s, want non-sensitive field preserved", got)
	}
}

func TestSanitizedLogTextSkipsBinaryBodies(t *testing.T) {
	got := sanitizedLogText(`{"image":"data:image/png;base64,AAAA"}`)
	if got != "[binary content skipped]" {
		t.Fatalf("sanitizedLogText() = %q, want binary skip marker", got)
	}
}
