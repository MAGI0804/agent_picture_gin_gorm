package agent_svc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/model"
)

func TestHTTPProviderChatGoogleOpenAICompatibleOmitsReturnReasoning(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/openai/chat/completions" {
			t.Fatalf("path = %q, want Google OpenAI-compatible chat completions path", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": `{"ok":true}`}},
			},
		})
	}))
	defer server.Close()

	provider := &HTTPProvider{
		client: server.Client(),
		config: model.UserModelConfig{
			Provider:  "google",
			ChatModel: "gemini-2.5-flash",
			BaseURL:   server.URL + "/v1beta/openai",
			APIKey:    "test-key",
		},
	}

	_, err := provider.Chat(ChatRequest{
		Messages:        []ChatMessage{{Role: "user", Content: "返回 JSON"}},
		ReturnReasoning: true,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if _, ok := payload["return_reasoning"]; ok {
		t.Fatalf("payload contains unsupported return_reasoning: %#v", payload)
	}
}

func TestHTTPProviderChatOpenAICompatibleAllowsConfiguredReturnReasoning(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok", "reasoning_content": "trace"}},
			},
		})
	}))
	defer server.Close()

	provider := &HTTPProvider{
		client: server.Client(),
		config: model.UserModelConfig{
			Provider:  "openai-compatible",
			ChatModel: "custom-reasoning-model",
			BaseURL:   server.URL,
			APIKey:    "test-key",
			RuntimeConfig: model.JSONMap{
				"return_reasoning": true,
			},
		},
	}

	_, err := provider.Chat(ChatRequest{
		Messages:        []ChatMessage{{Role: "user", Content: "hello"}},
		ReturnReasoning: true,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if payload["return_reasoning"] != true {
		t.Fatalf("return_reasoning = %#v, want true", payload["return_reasoning"])
	}
}
