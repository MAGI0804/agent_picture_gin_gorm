package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/model"
)

// GoogleVisionProvider calls Gemini OpenAI-compatible multimodal chat for image review.
type GoogleVisionProvider struct {
	config model.UserModelConfig
	client *http.Client
}

// NewGoogleVisionProvider creates a provider backed by the configured Gemini model.
func NewGoogleVisionProvider(config model.UserModelConfig) *GoogleVisionProvider {
	return NewGoogleVisionProviderWithClient(config, &http.Client{Timeout: 90 * time.Second})
}

// NewGoogleVisionProviderWithClient creates a provider with an injected HTTP client for tests.
func NewGoogleVisionProviderWithClient(config model.UserModelConfig, client *http.Client) *GoogleVisionProvider {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &GoogleVisionProvider{config: config, client: client}
}

// AnalyzeImage implements VisionProvider.
func (provider *GoogleVisionProvider) AnalyzeImage(ctx context.Context, request VisionRequest) (VisionResult, error) {
	apiKey := strings.TrimSpace(provider.config.APIKey)
	if apiKey == "" {
		return VisionResult{}, errors.New("google vision api key is empty")
	}
	modelName := strings.TrimSpace(provider.config.ChatModel)
	if modelName == "" {
		modelName = strings.TrimSpace(provider.config.AnthropicModel)
	}
	if modelName == "" {
		return VisionResult{}, errors.New("google vision model is empty")
	}
	baseURL := strings.TrimSpace(provider.config.BaseURL)
	if baseURL == "" {
		return VisionResult{}, errors.New("google vision base url is empty")
	}
	imageDataURL, err := provider.imageDataURL(request.ImageRef)
	if err != nil {
		return VisionResult{}, err
	}

	payload := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": strings.TrimSpace(request.Prompt)},
					{"type": "image_url", "image_url": map[string]string{"url": imageDataURL}},
				},
			},
		},
		"temperature": 0.2,
		"stream":      false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return VisionResult{}, errors.Wrap(err, "encode google vision request")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", joinGoogleVisionBaseURL(baseURL, "chat/completions"), bytes.NewReader(body))
	if err != nil {
		return VisionResult{}, errors.Wrap(err, "create google vision request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := provider.client.Do(req)
	if err != nil {
		return VisionResult{}, errors.Wrap(err, "call google vision api")
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return VisionResult{}, errors.Wrap(err, "read google vision response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return VisionResult{}, errors.Errorf("google vision api http %d: %s", resp.StatusCode, truncateVisionError(string(responseBody), 500))
	}
	return parseGoogleVisionResult(responseBody)
}

func (provider *GoogleVisionProvider) imageDataURL(imageRef string) (string, error) {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return "", errors.New("google vision image ref is empty")
	}
	if parsed, err := url.Parse(imageRef); err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "data") {
		return imageRef, nil
	}
	path := imageRef
	if !filepath.IsAbs(path) {
		path = agent_svc.NewObjectStore().Path(imageRef)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Wrap(err, "read google vision image")
	}
	mimeType := http.DetectContentType(content)
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}

func parseGoogleVisionResult(body []byte) (VisionResult, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return VisionResult{}, errors.Wrap(err, "decode google vision response")
	}
	if response.Error != nil {
		return VisionResult{}, errors.Errorf("google vision api error: %s", response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return VisionResult{}, errors.New("google vision returned empty choices")
	}
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return VisionResult{}, errors.New("google vision returned empty content")
	}
	result := VisionResult{
		Summary: content,
		Scores:  map[string]float64{},
		Issues:  []string{},
	}
	var structured struct {
		Summary      string   `json:"summary"`
		OverallScore float64  `json:"overall_score"`
		Issues       []string `json:"issues"`
		ShouldRefine bool     `json:"should_refine"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(content)), &structured); err == nil {
		if strings.TrimSpace(structured.Summary) != "" {
			result.Summary = strings.TrimSpace(structured.Summary)
		}
		if structured.OverallScore > 0 {
			result.Scores["overall"] = structured.OverallScore
		}
		result.Issues = structured.Issues
		result.ShouldRefine = structured.ShouldRefine
	}
	return result, nil
}

func stripJSONFence(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	return strings.TrimSpace(value)
}

func joinGoogleVisionBaseURL(baseURL string, endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpoint = strings.TrimLeft(endpoint, "/")
	return base + "/" + endpoint
}

func truncateVisionError(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
