package tools

import (
	"context"
	"testing"
)

func TestRegistryFindToolMatchesKindAndModelConfig(t *testing.T) {
	registry := NewRegistry()
	tool := Tool{
		Name:          "jimeng_text_to_image",
		Kind:          KindImageGeneration,
		Provider:      "volcengine",
		Model:         "jimeng",
		ModelConfigID: 42,
		Capability: Capability{
			MaxPromptChars: 750,
			SupportedRatios: []string{
				"1:1",
				"16:9",
			},
			MaxCandidates: 4,
		},
		ImageGenerationProvider: fakeImageProvider{},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	found, err := registry.FindTool(FindToolRequest{
		Kind:          KindImageGeneration,
		UserID:        7,
		ModelConfigID: 42,
	})
	if err != nil {
		t.Fatalf("FindTool() error = %v", err)
	}
	if found.Name != tool.Name {
		t.Fatalf("FindTool() = %q, want %q", found.Name, tool.Name)
	}
	if found.Capability.MaxPromptChars != 750 {
		t.Fatalf("MaxPromptChars = %d, want 750", found.Capability.MaxPromptChars)
	}
}

func TestRegistryRejectsToolWithoutMatchingProvider(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Tool{
		Name: "bad",
		Kind: KindImageGeneration,
	})
	if err == nil {
		t.Fatal("Register() error = nil, want provider validation error")
	}
}

type fakeImageProvider struct{}

func (fakeImageProvider) GenerateImage(
	ctx context.Context,
	request ImageGenerationRequest,
) (ImageGenerationResult, error) {
	return ImageGenerationResult{}, nil
}
