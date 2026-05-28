package agent_svc

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
