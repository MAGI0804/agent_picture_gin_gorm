package memory

import (
	"testing"

	"gin-biz-web-api/internal/dao/agent_v2_dao"
	"gin-biz-web-api/model"
)

func TestServiceSearchUsesNamespaceAndMarksUsage(t *testing.T) {
	repo := &fakeRepository{
		memories: []model.ContextMemory{
			{BaseModel: model.BaseModel{ID: 10}, UserID: 7, Namespace: NamespaceConversation},
			{BaseModel: model.BaseModel{ID: 11}, UserID: 7, Namespace: NamespaceConversation},
		},
	}
	svc := NewService(repo)

	memories, err := svc.Search(SearchRequest{
		UserID:         7,
		ConversationID: 9,
		Namespace:      NamespaceConversation,
		Limit:          5,
		MarkUsed:       true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(memories) != 2 {
		t.Fatalf("Search() returned %d memories, want 2", len(memories))
	}
	if repo.filter.UserID != 7 || repo.filter.ConversationID != 9 || repo.filter.Namespace != NamespaceConversation {
		t.Fatalf("unexpected filter: %#v", repo.filter)
	}
	if len(repo.usedIDs) != 2 || repo.usedIDs[0] != 10 || repo.usedIDs[1] != 11 {
		t.Fatalf("used IDs = %#v, want [10 11]", repo.usedIDs)
	}
}

func TestServiceWriteCreatesMemoryAndAuditEvent(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	memory, err := svc.Write(WriteRequest{
		UserID:         7,
		ConversationID: 9,
		Namespace:      NamespaceVisualStyle,
		Content:        "prefer low saturation",
		Confidence:     0.8,
		SourceType:     "feedback",
		SourceID:       12,
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if memory.ID == 0 {
		t.Fatal("memory ID was not assigned")
	}
	if repo.event.EventType != EventTypeCreated || repo.event.MemoryID != memory.ID {
		t.Fatalf("event = %#v, want created event for memory %d", repo.event, memory.ID)
	}
}

func TestServiceProposesVisualStyleMemoryFromSelectedFeedback(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	memory, proposed, err := svc.ProposeFromArtifactFeedback(ArtifactFeedbackProposalInput{
		UserID:            7,
		ConversationID:    9,
		AgentRunID:        12,
		ArtifactID:        30,
		ArtifactVersionID: 40,
		FeedbackType:      "selected",
	})
	if err != nil {
		t.Fatalf("ProposeFromArtifactFeedback() error = %v", err)
	}
	if !proposed {
		t.Fatal("proposed = false, want true")
	}
	if memory.Namespace != NamespaceVisualStyle || memory.Kind != KindMemoryProposal {
		t.Fatalf("memory namespace/kind = %q/%q, want visual_style/memory_proposal", memory.Namespace, memory.Kind)
	}
	if memory.ArtifactID != 30 || memory.SourceType != SourceTypeArtifactFeedback {
		t.Fatalf("memory source = artifact %d source %q, want artifact 30 artifact_feedback", memory.ArtifactID, memory.SourceType)
	}
	if repo.event.AgentRunID != 12 {
		t.Fatalf("event agent_run_id = %d, want 12", repo.event.AgentRunID)
	}
}

func TestServiceMergesDuplicateArtifactFeedbackProposal(t *testing.T) {
	repo := &fakeRepository{
		memories: []model.ContextMemory{
			{
				BaseModel:  model.BaseModel{ID: 88},
				UserID:     7,
				Namespace:  NamespaceVisualStyle,
				Scope:      "artifact:30",
				Kind:       KindMemoryProposal,
				Content:    "previous proposal",
				Confidence: 0.55,
			},
		},
	}
	svc := NewService(repo)

	memory, proposed, err := svc.ProposeFromArtifactFeedback(ArtifactFeedbackProposalInput{
		UserID:       7,
		ArtifactID:   30,
		FeedbackType: "positive",
		Comment:      "prefer warmer color",
	})
	if err != nil {
		t.Fatalf("ProposeFromArtifactFeedback() error = %v", err)
	}
	if !proposed {
		t.Fatal("proposed = false, want true")
	}
	if memory.ID != 88 {
		t.Fatalf("memory ID = %d, want existing 88", memory.ID)
	}
	if repo.memory.ID != 0 {
		t.Fatalf("created memory %#v, want merge into existing", repo.memory)
	}
	if repo.updatedMemoryID != 88 {
		t.Fatalf("updated memory ID = %d, want 88", repo.updatedMemoryID)
	}
	if repo.event.EventType != EventTypeMerged {
		t.Fatalf("event type = %q, want %q", repo.event.EventType, EventTypeMerged)
	}
}

func TestServiceSkipsEmptyNeutralFeedbackProposal(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	_, proposed, err := svc.ProposeFromArtifactFeedback(ArtifactFeedbackProposalInput{
		UserID:       7,
		ArtifactID:   30,
		FeedbackType: "neutral",
	})
	if err != nil {
		t.Fatalf("ProposeFromArtifactFeedback() error = %v", err)
	}
	if proposed {
		t.Fatal("proposed = true, want false")
	}
	if repo.memory.ID != 0 {
		t.Fatalf("created memory %#v, want none", repo.memory)
	}
}

func TestServiceProposesReflectionMemoryFromLowReview(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	memory, proposed, err := svc.ProposeFromReview(ReviewProposalInput{
		UserID:            7,
		ConversationID:    9,
		AgentRunID:        12,
		ArtifactID:        30,
		ArtifactVersionID: 40,
		OverallScore:      0.42,
		Issues:            []string{"subject missing", "text unreadable"},
		ShouldRefine:      true,
		Reviewer:          "google_vision_review",
		MinScore:          0.70,
	})
	if err != nil {
		t.Fatalf("ProposeFromReview() error = %v", err)
	}
	if !proposed {
		t.Fatal("proposed = false, want true")
	}
	if memory.Namespace != NamespaceReflection || memory.SourceType != SourceTypeReview {
		t.Fatalf("memory namespace/source = %q/%q, want reflection/review", memory.Namespace, memory.SourceType)
	}
	if memory.Confidence >= 0.7 {
		t.Fatalf("memory confidence = %.2f, want draft confidence below confirmed memory", memory.Confidence)
	}
}

func TestServiceSkipsHighScoreReviewProposal(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	_, proposed, err := svc.ProposeFromReview(ReviewProposalInput{
		UserID:       7,
		ArtifactID:   30,
		OverallScore: 0.92,
		MinScore:     0.70,
	})
	if err != nil {
		t.Fatalf("ProposeFromReview() error = %v", err)
	}
	if proposed {
		t.Fatal("proposed = true, want false")
	}
	if repo.memory.ID != 0 {
		t.Fatalf("created memory %#v, want none", repo.memory)
	}
}

func TestServicePromotesMemoryProposal(t *testing.T) {
	repo := &fakeRepository{
		memories: []model.ContextMemory{
			{
				BaseModel:  model.BaseModel{ID: 55},
				UserID:     7,
				Namespace:  NamespaceVisualStyle,
				Scope:      "artifact:30",
				Kind:       KindMemoryProposal,
				Content:    "prefer warmer color",
				Confidence: 0.65,
			},
		},
	}
	svc := NewService(repo)

	memory, promoted, err := svc.PromoteProposal(PromoteProposalInput{
		UserID:     7,
		MemoryID:   55,
		Confidence: 0.86,
	})
	if err != nil {
		t.Fatalf("PromoteProposal() error = %v", err)
	}
	if !promoted {
		t.Fatal("promoted = false, want true")
	}
	if memory.Kind != NamespaceVisualStyle || memory.Confidence != 0.86 {
		t.Fatalf("memory kind/confidence = %q/%.2f, want visual_style/0.86", memory.Kind, memory.Confidence)
	}
	if repo.updatedMemoryID != 55 {
		t.Fatalf("updated memory ID = %d, want 55", repo.updatedMemoryID)
	}
	if repo.event.EventType != EventTypePromoted {
		t.Fatalf("event type = %q, want %q", repo.event.EventType, EventTypePromoted)
	}
}

func TestServiceSkipsPromptContextDraftsAndLowConfidence(t *testing.T) {
	repo := &fakeRepository{
		memories: []model.ContextMemory{
			{BaseModel: model.BaseModel{ID: 1}, Namespace: NamespaceVisualStyle, Kind: NamespaceVisualStyle, Content: "stable warm colors", Confidence: 0.90},
			{BaseModel: model.BaseModel{ID: 2}, Namespace: NamespaceVisualStyle, Kind: KindMemoryProposal, Content: "draft preference", Confidence: 0.95},
			{BaseModel: model.BaseModel{ID: 3}, Namespace: NamespaceVisualStyle, Kind: NamespaceVisualStyle, Content: "weak preference", Confidence: 0.30},
		},
	}
	svc := NewService(repo)

	items, err := svc.PromptContext(PromptContextRequest{UserID: 7, Limit: 5})
	if err != nil {
		t.Fatalf("PromptContext() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Content != "stable warm colors" {
		t.Fatalf("item content = %q, want stable warm colors", items[0].Content)
	}
}

func TestServiceDeleteSoftDeletesMemoryAndWritesAuditEvent(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	err := svc.Delete(7, 10)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repo.deletedUserID != 7 || repo.deletedMemoryID != 10 {
		t.Fatalf("deleted userID=%d memoryID=%d, want 7 and 10", repo.deletedUserID, repo.deletedMemoryID)
	}
	if repo.event.EventType != EventTypeDeleted || repo.event.MemoryID != 10 {
		t.Fatalf("event = %#v, want deleted event for memory 10", repo.event)
	}
}

type fakeRepository struct {
	filter          agent_v2_dao.MemoryFilter
	memories        []model.ContextMemory
	memory          model.ContextMemory
	updatedMemoryID uint
	updatedAttrs    map[string]interface{}
	usedIDs         []uint
	event           model.MemoryEvent
	deletedUserID   uint
	deletedMemoryID uint
}

func (repo *fakeRepository) CreateMemory(memory *model.ContextMemory) error {
	memory.ID = 100
	repo.memory = *memory
	return nil
}

func (repo *fakeRepository) ListMemories(filter agent_v2_dao.MemoryFilter) ([]model.ContextMemory, error) {
	repo.filter = filter
	result := make([]model.ContextMemory, 0, len(repo.memories))
	for _, memory := range repo.memories {
		if filter.UserID != 0 && memory.UserID != 0 && memory.UserID != filter.UserID {
			continue
		}
		if filter.Namespace != "" && memory.Namespace != "" && memory.Namespace != filter.Namespace {
			continue
		}
		if filter.Scope != "" && memory.Scope != filter.Scope {
			continue
		}
		if filter.Kind != "" && memory.Kind != filter.Kind {
			continue
		}
		if filter.MinConfidence > 0 && memory.Confidence < filter.MinConfidence {
			continue
		}
		result = append(result, memory)
	}
	return result, nil
}

func (repo *fakeRepository) FindMemory(userID uint, memoryID uint) (model.ContextMemory, error) {
	for _, memory := range repo.memories {
		if memory.ID == memoryID && memory.UserID == userID {
			return memory, nil
		}
	}
	return model.ContextMemory{}, nil
}

func (repo *fakeRepository) UpdateMemoryUsage(memoryID uint) error {
	repo.usedIDs = append(repo.usedIDs, memoryID)
	return nil
}

func (repo *fakeRepository) UpdateMemory(memoryID uint, attrs map[string]interface{}) error {
	repo.updatedMemoryID = memoryID
	repo.updatedAttrs = attrs
	for index := range repo.memories {
		if repo.memories[index].ID != memoryID {
			continue
		}
		if kind, ok := attrs["kind"].(string); ok {
			repo.memories[index].Kind = kind
		}
		if content, ok := attrs["content"].(string); ok {
			repo.memories[index].Content = content
		}
		if confidence, ok := attrs["confidence"].(float64); ok {
			repo.memories[index].Confidence = confidence
		}
		return nil
	}
	return nil
}

func (repo *fakeRepository) SoftDeleteMemory(userID uint, memoryID uint) error {
	repo.deletedUserID = userID
	repo.deletedMemoryID = memoryID
	return nil
}

func (repo *fakeRepository) CreateMemoryEvent(event *model.MemoryEvent) error {
	repo.event = *event
	return nil
}
