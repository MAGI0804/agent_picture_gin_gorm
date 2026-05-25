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

type fakeRepository struct {
	filter   agent_v2_dao.MemoryFilter
	memories []model.ContextMemory
	usedIDs  []uint
	event    model.MemoryEvent
}

func (repo *fakeRepository) CreateMemory(memory *model.ContextMemory) error {
	memory.ID = 100
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
	return nil
}

func (repo *fakeRepository) CreateMemoryEvent(event *model.MemoryEvent) error {
	repo.event = *event
	return nil
}
