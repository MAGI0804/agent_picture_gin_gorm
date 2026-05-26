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
	return repo.memories, nil
}

func (repo *fakeRepository) UpdateMemoryUsage(memoryID uint) error {
	repo.usedIDs = append(repo.usedIDs, memoryID)
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
