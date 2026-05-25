package memory

import (
	"errors"

	"gin-biz-web-api/internal/dao/agent_v2_dao"
	"gin-biz-web-api/model"
)

const (
	NamespaceConversation    = "conversation"
	NamespaceUserProfile     = "user_profile"
	NamespaceVisualStyle     = "visual_style"
	NamespaceArtifactLineage = "artifact_lineage"
	NamespaceToolExperience  = "tool_experience"
	NamespaceReflection      = "reflection"

	EventTypeCreated = "created"
	EventTypeDeleted = "deleted"
	EventTypeUsed    = "used"
)

// Repository defines the persistence operations required by Memory Service.
type Repository interface {
	CreateMemory(memory *model.ContextMemory) error
	ListMemories(filter agent_v2_dao.MemoryFilter) ([]model.ContextMemory, error)
	UpdateMemoryUsage(memoryID uint) error
	SoftDeleteMemory(userID uint, memoryID uint) error
	CreateMemoryEvent(event *model.MemoryEvent) error
}

// Service owns V2 memory reads, writes, and audit events.
type Service struct {
	repo Repository
}

// SearchRequest describes a scoped memory retrieval.
type SearchRequest struct {
	UserID         uint
	ConversationID uint
	Namespace      string
	Scope          string
	Limit          int
	MarkUsed       bool
}

// WriteRequest describes one memory write.
type WriteRequest struct {
	UserID         uint
	ConversationID uint
	Namespace      string
	Scope          string
	Kind           string
	Content        string
	TagsJSON       string
	Confidence     float64
	SourceType     string
	SourceID       uint
	ArtifactID     uint
}

// NewService creates a Memory Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Search returns scoped memories and optionally records usage.
func (svc *Service) Search(request SearchRequest) ([]model.ContextMemory, error) {
	if request.UserID == 0 {
		return nil, errors.New("memory search user_id is required")
	}
	filter := agent_v2_dao.MemoryFilter{
		UserID:         request.UserID,
		ConversationID: request.ConversationID,
		Namespace:      request.Namespace,
		Scope:          request.Scope,
		Limit:          request.Limit,
	}
	memories, err := svc.repo.ListMemories(filter)
	if err != nil {
		return nil, err
	}
	if !request.MarkUsed {
		return memories, nil
	}
	for _, memory := range memories {
		if err := svc.repo.UpdateMemoryUsage(memory.ID); err != nil {
			return memories, err
		}
	}
	return memories, nil
}

// Write creates a memory and records an audit event.
func (svc *Service) Write(request WriteRequest) (model.ContextMemory, error) {
	if request.UserID == 0 {
		return model.ContextMemory{}, errors.New("memory user_id is required")
	}
	if request.Namespace == "" {
		return model.ContextMemory{}, errors.New("memory namespace is required")
	}
	if request.Content == "" {
		return model.ContextMemory{}, errors.New("memory content is required")
	}
	kind := request.Kind
	if kind == "" {
		kind = request.Namespace
	}

	memory := model.ContextMemory{
		UserID:         request.UserID,
		ConversationID: request.ConversationID,
		Namespace:      request.Namespace,
		Scope:          request.Scope,
		Kind:           kind,
		Content:        request.Content,
		TagsJSON:       request.TagsJSON,
		Confidence:     request.Confidence,
		SourceType:     request.SourceType,
		SourceID:       request.SourceID,
		ArtifactID:     request.ArtifactID,
	}
	if err := svc.repo.CreateMemory(&memory); err != nil {
		return model.ContextMemory{}, err
	}

	event := model.MemoryEvent{
		MemoryID:       memory.ID,
		UserID:         memory.UserID,
		ConversationID: memory.ConversationID,
		EventType:      EventTypeCreated,
		SourceType:     memory.SourceType,
		SourceID:       memory.SourceID,
		AfterJSON:      memory.Content,
	}
	if err := svc.repo.CreateMemoryEvent(&event); err != nil {
		return memory, err
	}
	return memory, nil
}

// Delete soft-deletes a memory and records an audit event.
func (svc *Service) Delete(userID uint, memoryID uint) error {
	if userID == 0 {
		return errors.New("memory delete user_id is required")
	}
	if memoryID == 0 {
		return errors.New("memory id is required")
	}
	if err := svc.repo.SoftDeleteMemory(userID, memoryID); err != nil {
		return err
	}
	return svc.repo.CreateMemoryEvent(&model.MemoryEvent{
		MemoryID:  memoryID,
		UserID:    userID,
		EventType: EventTypeDeleted,
	})
}
