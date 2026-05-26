package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gin-biz-web-api/model"
)

func TestGoogleVisionProviderAnalyzeImage(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "generated-image.png")
	imageBytes := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, []byte("fake png bytes")...)
	if err := os.WriteFile(imagePath, imageBytes, 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	var gotAuthorization string
	var gotModel string
	var gotImageDataURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.URL.Path != "/v1beta/openai/chat/completions" {
			t.Fatalf("path = %q, want openai chat completions path", r.URL.Path)
		}
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content []struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		gotModel = payload.Model
		if len(payload.Messages) != 1 || len(payload.Messages[0].Content) != 2 {
			t.Fatalf("messages = %#v, want one multimodal message", payload.Messages)
		}
		gotImageDataURL = payload.Messages[0].Content[1].ImageURL.URL

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": `{"summary":"clean image","overall_score":0.88,"issues":["small glare"],"should_refine":false}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := NewGoogleVisionProviderWithClient(model.UserModelConfig{
		Provider:  "google",
		ChatModel: "gemini-3.5-flash",
		BaseURL:   server.URL + "/v1beta/openai",
		APIKey:    "test-key",
	}, server.Client())

	result, err := provider.AnalyzeImage(context.Background(), VisionRequest{
		ImageRef: imagePath,
		Prompt:   "review this image",
	})
	if err != nil {
		t.Fatalf("AnalyzeImage() error = %v", err)
	}
	if gotAuthorization != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuthorization)
	}
	if gotModel != "gemini-3.5-flash" {
		t.Fatalf("model = %q, want gemini-3.5-flash", gotModel)
	}
	wantDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)
	if gotImageDataURL != wantDataURL {
		t.Fatalf("image URL = %q, want data URL", gotImageDataURL)
	}
	if result.Summary != "clean image" {
		t.Fatalf("summary = %q, want clean image", result.Summary)
	}
	if result.Scores["overall"] != 0.88 {
		t.Fatalf("overall score = %f, want 0.88", result.Scores["overall"])
	}
	if len(result.Issues) != 1 || result.Issues[0] != "small glare" {
		t.Fatalf("issues = %#v, want small glare", result.Issues)
	}
	if result.ShouldRefine {
		t.Fatal("should refine = true, want false")
	}
}
