package agent_svc

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gin-biz-web-api/model"
)

func TestHTTPProviderGenerateGoogleImagenImage(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotPrompt string
	var gotSampleCount float64
	var gotAspectRatio string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-goog-api-key")

		var payload struct {
			Instances []struct {
				Prompt string `json:"prompt"`
			} `json:"instances"`
			Parameters struct {
				SampleCount float64 `json:"sampleCount"`
				AspectRatio string  `json:"aspectRatio"`
			} `json:"parameters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload.Instances) > 0 {
			gotPrompt = payload.Instances[0].Prompt
		}
		gotSampleCount = payload.Parameters.SampleCount
		gotAspectRatio = payload.Parameters.AspectRatio

		response := map[string]interface{}{
			"predictions": []map[string]interface{}{
				{
					"bytesBase64Encoded": base64.StdEncoding.EncodeToString([]byte("fake png")),
					"mimeType":           "image/png",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &HTTPProvider{
		client: server.Client(),
		config: model.UserModelConfig{
			Provider:   "google",
			ImageModel: "imagen-4.0-ultra-generate-001",
			BaseURL:    server.URL + "/v1beta",
			APIKey:     "test-key",
		},
	}

	files, err := provider.Generate(GenerationRequest{Prompt: "a precise product render", AspectRatio: "3:4"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if gotPath != "/v1beta/models/imagen-4.0-ultra-generate-001:predict" {
		t.Fatalf("path = %q, want google imagen predict path", gotPath)
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("x-goog-api-key = %q, want test-key", gotAPIKey)
	}
	if gotPrompt != "a precise product render" {
		t.Fatalf("prompt = %q, want request prompt", gotPrompt)
	}
	if gotSampleCount != 1 {
		t.Fatalf("sampleCount = %v, want 1", gotSampleCount)
	}
	if gotAspectRatio != "3:4" {
		t.Fatalf("aspectRatio = %q, want request aspect ratio", gotAspectRatio)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if string(files[0].Content) != "fake png" {
		t.Fatalf("content = %q, want decoded image bytes", string(files[0].Content))
	}
	if files[0].MimeType != "image/png" {
		t.Fatalf("mime type = %q, want image/png", files[0].MimeType)
	}
}

func TestHTTPProviderGenerateGoogleGeminiImageEditUsesReferenceImages(t *testing.T) {
	refKey := filepath.Join("test-fixtures", "reference.png")
	refPath := filepath.Join("public", "artifacts", refKey)
	if err := os.MkdirAll(filepath.Dir(refPath), 0755); err != nil {
		t.Fatalf("mkdir reference dir: %v", err)
	}
	if err := os.WriteFile(refPath, []byte("fake reference png"), 0644); err != nil {
		t.Fatalf("write reference image: %v", err)
	}
	defer os.Remove(refPath)

	var gotPath string
	var gotAPIKey string
	var gotModelPrompt string
	var gotReferenceData string
	var gotReferenceMime string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-goog-api-key")
		var payload struct {
			Contents []struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"contents"`
			GenerationConfig struct {
				ResponseModalities []string `json:"responseModalities"`
			} `json:"generationConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModelPrompt = payload.Contents[0].Parts[0].Text
		gotReferenceMime = payload.Contents[0].Parts[1].InlineData.MimeType
		gotReferenceData = payload.Contents[0].Parts[1].InlineData.Data

		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"inlineData": map[string]interface{}{
									"data":     base64.StdEncoding.EncodeToString([]byte("edited png")),
									"mimeType": "image/png",
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &HTTPProvider{
		client: server.Client(),
		config: model.UserModelConfig{
			Provider:   "google",
			ImageModel: "imagen-4.0-ultra-generate-001",
			BaseURL:    server.URL + "/v1beta/generativelanguage.googleapis.com",
			APIKey:     "test-key",
		},
	}

	files, err := provider.Generate(GenerationRequest{
		Prompt:    "add exact text to the template",
		ImageRefs: []string{refKey},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if gotPath != "/v1beta/generativelanguage.googleapis.com/models/gemini-2.5-flash-image:generateContent" {
		t.Fatalf("path = %q, want Gemini generateContent image edit path", gotPath)
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("x-goog-api-key = %q, want test-key", gotAPIKey)
	}
	if gotModelPrompt != "add exact text to the template" {
		t.Fatalf("prompt = %q, want original prompt", gotModelPrompt)
	}
	if gotReferenceData == "" || gotReferenceMime == "" {
		t.Fatalf("reference inline data = %q/%q, want image bytes and mime", gotReferenceMime, gotReferenceData)
	}
	if len(files) != 1 || string(files[0].Content) != "edited png" {
		t.Fatalf("files = %#v, want decoded edited image", files)
	}
}
