package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

	KindMemoryProposal = "memory_proposal"

	EventTypeCreated  = "created"
	EventTypeDeleted  = "deleted"
	EventTypeUsed     = "used"
	EventTypeMerged   = "merged"
	EventTypePromoted = "promoted"

	SourceTypeArtifactFeedback = "artifact_feedback"
	SourceTypeReview           = "review"

	defaultPromptMemoryConfidence = 0.70
)

// Repository 定义记忆服务所需的持久化操作接口。
type Repository interface {
	CreateMemory(memory *model.ContextMemory) error
	FindMemory(userID uint, memoryID uint) (model.ContextMemory, error)
	ListMemories(filter agent_v2_dao.MemoryFilter) ([]model.ContextMemory, error)
	UpdateMemoryUsage(memoryID uint) error
	UpdateMemory(memoryID uint, attrs map[string]interface{}) error
	SoftDeleteMemory(userID uint, memoryID uint) error
	CreateMemoryEvent(event *model.MemoryEvent) error
}

// Service 负责 V2 记忆的读取、写入和审计事件。
type Service struct {
	repo Repository
}

// SearchRequest 描述范围限定的记忆检索请求。
type SearchRequest struct {
	UserID         uint
	ConversationID uint
	Namespace      string
	Scope          string
	Limit          int
	MarkUsed       bool
}

// WriteRequest 描述一次记忆写入操作。
type WriteRequest struct {
	UserID         uint
	ConversationID uint
	AgentRunID     uint
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

// PromptContextRequest loads stable memories that can safely influence prompt generation.
type PromptContextRequest struct {
	UserID         uint
	ConversationID uint
	Limit          int
	MinConfidence  float64
}

// PromoteProposalInput confirms a draft memory proposal as stable memory.
type PromoteProposalInput struct {
	UserID     uint
	MemoryID   uint
	Confidence float64
}

// ArtifactFeedbackProposalInput describes user feedback that can become a draft memory.
type ArtifactFeedbackProposalInput struct {
	UserID            uint
	ConversationID    uint
	AgentRunID        uint
	ArtifactID        uint
	ArtifactVersionID uint
	FeedbackType      string
	Rating            int
	Comment           string
}

// ReviewProposalInput describes review output that can become a draft memory.
type ReviewProposalInput struct {
	UserID            uint
	ConversationID    uint
	AgentRunID        uint
	ArtifactID        uint
	ArtifactVersionID uint
	OverallScore      float64
	Issues            []string
	ShouldRefine      bool
	Reviewer          string
	MinScore          float64
}

// NewService 创建记忆服务实例。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Search 返回范围限定的记忆并可选记录使用情况。
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

// PromptContext returns stable, high-confidence preference memories for prompt generation.
func (svc *Service) PromptContext(request PromptContextRequest) ([]model.ContextMemory, error) {
	if request.UserID == 0 {
		return nil, errors.New("prompt memory user_id is required")
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 8
	}
	minConfidence := request.MinConfidence
	if minConfidence <= 0 {
		minConfidence = defaultPromptMemoryConfidence
	}

	namespaces := []string{NamespaceVisualStyle, NamespaceUserProfile}
	result := make([]model.ContextMemory, 0, limit)
	for _, namespace := range namespaces {
		memories, err := svc.repo.ListMemories(agent_v2_dao.MemoryFilter{
			UserID:         request.UserID,
			ConversationID: request.ConversationID,
			Namespace:      namespace,
			MinConfidence:  minConfidence,
			Limit:          limit,
		})
		if err != nil {
			return result, err
		}
		for _, memory := range memories {
			if len(result) >= limit {
				return result, nil
			}
			if memory.Kind == KindMemoryProposal || memory.Confidence < minConfidence {
				continue
			}
			result = append(result, memory)
			if err := svc.repo.UpdateMemoryUsage(memory.ID); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// Write 创建一条记忆并记录审计事件。
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
		AgentRunID:     request.AgentRunID,
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

// ProposeFromArtifactFeedback turns explicit user feedback into a draft memory.
func (svc *Service) ProposeFromArtifactFeedback(
	input ArtifactFeedbackProposalInput,
) (model.ContextMemory, bool, error) {
	if input.UserID == 0 {
		return model.ContextMemory{}, false, errors.New("feedback proposal user_id is required")
	}
	if input.ArtifactID == 0 {
		return model.ContextMemory{}, false, errors.New("feedback proposal artifact_id is required")
	}

	feedbackType := strings.TrimSpace(input.FeedbackType)
	comment := strings.TrimSpace(input.Comment)
	if !shouldProposeFeedbackMemory(feedbackType, input.Rating, comment) {
		return model.ContextMemory{}, false, nil
	}

	namespace := NamespaceVisualStyle
	confidence := 0.55
	if comment != "" {
		confidence = 0.65
	}
	if isNegativeFeedback(feedbackType, input.Rating) {
		namespace = NamespaceReflection
		confidence = 0.60
	}

	content := feedbackProposalContent(input.ArtifactID, feedbackType, input.Rating, comment)
	memory, err := svc.writeOrMergeProposal(WriteRequest{
		UserID:         input.UserID,
		ConversationID: input.ConversationID,
		AgentRunID:     input.AgentRunID,
		Namespace:      namespace,
		Scope:          artifactScope(input.ArtifactID),
		Kind:           KindMemoryProposal,
		Content:        content,
		TagsJSON:       tagsJSON("artifact_feedback", feedbackType, fmt.Sprintf("artifact:%d", input.ArtifactID)),
		Confidence:     confidence,
		SourceType:     SourceTypeArtifactFeedback,
		SourceID:       input.ArtifactID,
		ArtifactID:     input.ArtifactID,
	})
	if err != nil {
		return model.ContextMemory{}, false, err
	}
	return memory, true, nil
}

// ProposeFromReview turns low-score review output into a draft reflection memory.
func (svc *Service) ProposeFromReview(input ReviewProposalInput) (model.ContextMemory, bool, error) {
	if input.UserID == 0 {
		return model.ContextMemory{}, false, errors.New("review proposal user_id is required")
	}
	if input.ArtifactID == 0 {
		return model.ContextMemory{}, false, errors.New("review proposal artifact_id is required")
	}
	minScore := input.MinScore
	if minScore <= 0 {
		minScore = 0.70
	}
	if input.OverallScore >= minScore && !input.ShouldRefine {
		return model.ContextMemory{}, false, nil
	}

	issueSummary := strings.Join(input.Issues, "; ")
	if issueSummary == "" {
		issueSummary = "review score below threshold"
	}
	content := fmt.Sprintf(
		"Review flagged artifact %d version %d with score %.2f below %.2f: %s. Treat this as a draft failure pattern before promoting it to stable memory.",
		input.ArtifactID,
		input.ArtifactVersionID,
		input.OverallScore,
		minScore,
		issueSummary,
	)
	memory, err := svc.writeOrMergeProposal(WriteRequest{
		UserID:         input.UserID,
		ConversationID: input.ConversationID,
		AgentRunID:     input.AgentRunID,
		Namespace:      NamespaceReflection,
		Scope:          artifactScope(input.ArtifactID),
		Kind:           KindMemoryProposal,
		Content:        content,
		TagsJSON:       tagsJSON("review", input.Reviewer, fmt.Sprintf("artifact:%d", input.ArtifactID)),
		Confidence:     0.50,
		SourceType:     SourceTypeReview,
		SourceID:       input.AgentRunID,
		ArtifactID:     input.ArtifactID,
	})
	if err != nil {
		return model.ContextMemory{}, false, err
	}
	return memory, true, nil
}

// PromoteProposal confirms a memory proposal and makes it available to prompt context retrieval.
func (svc *Service) PromoteProposal(input PromoteProposalInput) (model.ContextMemory, bool, error) {
	if input.UserID == 0 {
		return model.ContextMemory{}, false, errors.New("promote memory user_id is required")
	}
	if input.MemoryID == 0 {
		return model.ContextMemory{}, false, errors.New("promote memory id is required")
	}
	memory, err := svc.repo.FindMemory(input.UserID, input.MemoryID)
	if err != nil {
		return model.ContextMemory{}, false, err
	}
	if memory.Kind != KindMemoryProposal {
		return memory, false, nil
	}
	confidence := input.Confidence
	if confidence <= 0 {
		confidence = 0.80
	}
	if confidence > 1 {
		confidence = 1
	}
	stableKind := memory.Namespace
	if stableKind == "" {
		stableKind = NamespaceUserProfile
	}
	attrs := map[string]interface{}{
		"kind":       stableKind,
		"confidence": confidence,
	}
	if err := svc.repo.UpdateMemory(memory.ID, attrs); err != nil {
		return model.ContextMemory{}, false, err
	}
	before := memory.Content
	memory.Kind = stableKind
	memory.Confidence = confidence
	if err := svc.repo.CreateMemoryEvent(&model.MemoryEvent{
		MemoryID:       memory.ID,
		UserID:         memory.UserID,
		ConversationID: memory.ConversationID,
		AgentRunID:     memory.SourceID,
		EventType:      EventTypePromoted,
		SourceType:     memory.SourceType,
		SourceID:       memory.SourceID,
		BeforeJSON:     before,
		AfterJSON:      memory.Content,
		Reason:         "memory proposal promoted to stable memory",
	}); err != nil {
		return memory, true, err
	}
	return memory, true, nil
}

// Delete 软删除一条记忆并记录审计事件。
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

func (svc *Service) writeOrMergeProposal(request WriteRequest) (model.ContextMemory, error) {
	existing, err := svc.repo.ListMemories(agent_v2_dao.MemoryFilter{
		UserID:         request.UserID,
		ConversationID: request.ConversationID,
		Namespace:      request.Namespace,
		Scope:          request.Scope,
		Kind:           KindMemoryProposal,
		Limit:          1,
	})
	if err != nil {
		return model.ContextMemory{}, err
	}
	if len(existing) == 0 {
		return svc.Write(request)
	}

	memory := existing[0]
	mergedContent := mergeContent(memory.Content, request.Content)
	confidence := maxFloat(memory.Confidence, request.Confidence)
	attrs := map[string]interface{}{
		"content":     mergedContent,
		"confidence":  confidence,
		"tags_json":   request.TagsJSON,
		"source_type": request.SourceType,
		"source_id":   request.SourceID,
		"artifact_id": request.ArtifactID,
	}
	if err := svc.repo.UpdateMemory(memory.ID, attrs); err != nil {
		return model.ContextMemory{}, err
	}
	if err := svc.repo.CreateMemoryEvent(&model.MemoryEvent{
		MemoryID:       memory.ID,
		UserID:         memory.UserID,
		ConversationID: memory.ConversationID,
		AgentRunID:     request.AgentRunID,
		EventType:      EventTypeMerged,
		SourceType:     request.SourceType,
		SourceID:       request.SourceID,
		BeforeJSON:     memory.Content,
		AfterJSON:      mergedContent,
		Reason:         "merged duplicate memory proposal",
	}); err != nil {
		return memory, err
	}
	memory.Content = mergedContent
	memory.Confidence = confidence
	memory.TagsJSON = request.TagsJSON
	memory.SourceType = request.SourceType
	memory.SourceID = request.SourceID
	memory.ArtifactID = request.ArtifactID
	return memory, nil
}

func shouldProposeFeedbackMemory(feedbackType string, rating int, comment string) bool {
	if comment != "" || rating != 0 {
		return true
	}
	switch strings.ToLower(feedbackType) {
	case "selected", "positive", "negative":
		return true
	default:
		return false
	}
}

func isNegativeFeedback(feedbackType string, rating int) bool {
	return strings.EqualFold(feedbackType, "negative") || (rating > 0 && rating <= 2)
}

func feedbackProposalContent(artifactID uint, feedbackType string, rating int, comment string) string {
	parts := []string{fmt.Sprintf("User feedback on artifact %d was %s", artifactID, coalesceText(feedbackType, "unspecified"))}
	if rating != 0 {
		parts = append(parts, fmt.Sprintf("rating=%d", rating))
	}
	if comment != "" {
		parts = append(parts, fmt.Sprintf("comment=%q", comment))
	}
	return strings.Join(parts, "; ") + ". Treat this as a draft memory proposal until confirmed by repeated feedback."
}

func artifactScope(artifactID uint) string {
	return fmt.Sprintf("artifact:%d", artifactID)
}

func tagsJSON(tags ...string) string {
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleaned = append(cleaned, tag)
		}
	}
	data, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func coalesceText(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func mergeContent(existing string, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if existing == "" {
		return next
	}
	if next == "" || strings.Contains(existing, next) {
		return existing
	}
	return existing + "\n" + next
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
