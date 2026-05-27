package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

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

	EventTypeCreated         = "created"
	EventTypeDeleted         = "deleted"
	EventTypeUsed            = "used"
	EventTypeMerged          = "merged"
	EventTypePromoted        = "promoted"
	EventTypeConflictDemoted = "conflict_demoted"

	SourceTypeArtifactFeedback = "artifact_feedback"
	SourceTypeReview           = "review"

	defaultPromptMemoryConfidence   = 0.70
	repeatedProposalConfidenceBoost = 0.10
	autoPromoteProposalConfidence   = 0.85
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
	Kind           string
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

// UpdateMemoryInput edits safe user-controlled memory fields.
type UpdateMemoryInput struct {
	UserID     uint
	MemoryID   uint
	Content    string
	Confidence *float64
	Disabled   bool
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
		Kind:           request.Kind,
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
	candidates := make([]model.ContextMemory, 0, limit*len(namespaces))
	for _, namespace := range namespaces {
		memories, err := svc.repo.ListMemories(agent_v2_dao.MemoryFilter{
			UserID:         request.UserID,
			ConversationID: request.ConversationID,
			Namespace:      namespace,
			MinConfidence:  minConfidence,
			Limit:          limit,
		})
		if err != nil {
			return candidates, err
		}
		for _, memory := range memories {
			if memory.Kind == KindMemoryProposal || memory.Confidence < minConfidence {
				continue
			}
			candidates = append(candidates, memory)
		}
	}
	result := rankAndResolveMemories(candidates, limit)
	for _, memory := range result {
		if err := svc.repo.UpdateMemoryUsage(memory.ID); err != nil {
			return result, err
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
	if kind != KindMemoryProposal {
		duplicate, ok, err := svc.findSemanticDuplicate(request, kind)
		if err != nil {
			return model.ContextMemory{}, err
		}
		if ok {
			return svc.mergeStableMemory(duplicate, request)
		}
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
	promoted, changed, err := svc.promoteMemory(memory, confidence, "memory proposal promoted to stable memory", 0)
	if err != nil || !changed {
		return promoted, changed, err
	}
	if err := svc.demoteConflictingStableMemories(promoted, 0); err != nil {
		return promoted, changed, err
	}
	return promoted, changed, nil
}

func (svc *Service) promoteMemory(
	memory model.ContextMemory,
	confidence float64,
	reason string,
	agentRunID uint,
) (model.ContextMemory, bool, error) {
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
		AgentRunID:     agentRunID,
		EventType:      EventTypePromoted,
		SourceType:     memory.SourceType,
		SourceID:       memory.SourceID,
		BeforeJSON:     before,
		AfterJSON:      memory.Content,
		Reason:         reason,
	}); err != nil {
		return memory, true, err
	}
	return memory, true, nil
}

// Update edits one memory or disables it while preserving auditability.
func (svc *Service) Update(input UpdateMemoryInput) (model.ContextMemory, error) {
	if input.UserID == 0 {
		return model.ContextMemory{}, errors.New("update memory user_id is required")
	}
	if input.MemoryID == 0 {
		return model.ContextMemory{}, errors.New("update memory id is required")
	}
	memory, err := svc.repo.FindMemory(input.UserID, input.MemoryID)
	if err != nil {
		return model.ContextMemory{}, err
	}
	if input.Disabled {
		if err := svc.Delete(input.UserID, input.MemoryID); err != nil {
			return model.ContextMemory{}, err
		}
		return memory, nil
	}
	attrs := map[string]interface{}{}
	if strings.TrimSpace(input.Content) != "" {
		attrs["content"] = strings.TrimSpace(input.Content)
	}
	if input.Confidence != nil {
		attrs["confidence"] = clampConfidence(*input.Confidence)
	}
	if len(attrs) == 0 {
		return memory, nil
	}
	if err := svc.repo.UpdateMemory(input.MemoryID, attrs); err != nil {
		return model.ContextMemory{}, err
	}
	before := memory.Content
	if content, ok := attrs["content"].(string); ok {
		memory.Content = content
	}
	if confidence, ok := attrs["confidence"].(float64); ok {
		memory.Confidence = confidence
	}
	if err := svc.repo.CreateMemoryEvent(&model.MemoryEvent{
		MemoryID:       memory.ID,
		UserID:         memory.UserID,
		ConversationID: memory.ConversationID,
		EventType:      EventTypeMerged,
		BeforeJSON:     before,
		AfterJSON:      memory.Content,
		Reason:         "memory manually updated",
	}); err != nil {
		return memory, err
	}
	return memory, nil
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
	if confidence < autoPromoteProposalConfidence {
		confidence = minFloat(1, confidence+repeatedProposalConfidenceBoost)
	}
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
	if confidence >= autoPromoteProposalConfidence {
		promotedMemory, _, err := svc.promoteMemory(memory, confidence, "memory proposal auto-promoted after repeated matching feedback", request.AgentRunID)
		if err == nil {
			err = svc.demoteConflictingStableMemories(promotedMemory, request.AgentRunID)
		}
		return promotedMemory, err
	}
	return memory, nil
}

func (svc *Service) findSemanticDuplicate(request WriteRequest, kind string) (model.ContextMemory, bool, error) {
	existing, err := svc.repo.ListMemories(agent_v2_dao.MemoryFilter{
		UserID:         request.UserID,
		ConversationID: request.ConversationID,
		Namespace:      request.Namespace,
		Kind:           kind,
		Limit:          50,
	})
	if err != nil {
		return model.ContextMemory{}, false, err
	}
	for _, memory := range existing {
		if memory.Kind == KindMemoryProposal {
			continue
		}
		if memory.Scope != "" && request.Scope != "" && memory.Scope != request.Scope {
			continue
		}
		if semanticSimilarity(memory.Content, request.Content) >= 0.55 {
			return memory, true, nil
		}
	}
	return model.ContextMemory{}, false, nil
}

func (svc *Service) mergeStableMemory(memory model.ContextMemory, request WriteRequest) (model.ContextMemory, error) {
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
		Reason:         "merged semantically duplicate stable memory",
	}); err != nil {
		return memory, err
	}
	memory.Content = mergedContent
	memory.Confidence = confidence
	return memory, nil
}

func (svc *Service) demoteConflictingStableMemories(memory model.ContextMemory, agentRunID uint) error {
	if strings.TrimSpace(memory.Scope) == "" {
		return nil
	}
	existing, err := svc.repo.ListMemories(agent_v2_dao.MemoryFilter{
		UserID:         memory.UserID,
		ConversationID: memory.ConversationID,
		Namespace:      memory.Namespace,
		Scope:          memory.Scope,
		Limit:          50,
	})
	if err != nil {
		return err
	}
	for _, candidate := range existing {
		if candidate.ID == memory.ID || candidate.Kind == KindMemoryProposal {
			continue
		}
		if !memoryConflicts(memory.Content, candidate.Content) {
			continue
		}
		nextConfidence := minFloat(candidate.Confidence, maxFloat(0.10, memory.Confidence-0.25))
		if err := svc.repo.UpdateMemory(candidate.ID, map[string]interface{}{"confidence": nextConfidence}); err != nil {
			return err
		}
		if err := svc.repo.CreateMemoryEvent(&model.MemoryEvent{
			MemoryID:       candidate.ID,
			UserID:         candidate.UserID,
			ConversationID: candidate.ConversationID,
			AgentRunID:     agentRunID,
			EventType:      EventTypeConflictDemoted,
			BeforeJSON:     candidate.Content,
			AfterJSON:      candidate.Content,
			Reason:         fmt.Sprintf("demoted because memory %d conflicts in scope %s", memory.ID, memory.Scope),
		}); err != nil {
			return err
		}
	}
	return nil
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

func rankAndResolveMemories(memories []model.ContextMemory, limit int) []model.ContextMemory {
	ranked := append([]model.ContextMemory{}, memories...)
	sort.SliceStable(ranked, func(i, j int) bool {
		left := memoryRankScore(ranked[i])
		right := memoryRankScore(ranked[j])
		if left == right {
			return ranked[i].ID > ranked[j].ID
		}
		return left > right
	})
	result := make([]model.ContextMemory, 0, limit)
	for _, memory := range ranked {
		conflicts := false
		for _, selected := range result {
			if selected.Namespace == memory.Namespace &&
				selected.Scope != "" &&
				selected.Scope == memory.Scope &&
				memoryConflicts(selected.Content, memory.Content) {
				conflicts = true
				break
			}
		}
		if conflicts {
			continue
		}
		result = append(result, memory)
		if limit > 0 && len(result) >= limit {
			return result
		}
	}
	return result
}

func memoryRankScore(memory model.ContextMemory) float64 {
	score := memory.Confidence * 100
	score += minFloat(float64(memory.UseCount), 10) * 2
	if memory.LastUsedAt > 0 {
		score += minFloat(float64(memory.LastUsedAt)/1000000000, 5)
	}
	switch memory.Namespace {
	case NamespaceVisualStyle:
		score += 8
	case NamespaceUserProfile:
		score += 4
	case NamespaceReflection:
		score -= 8
	}
	return score
}

func semanticSimilarity(left string, right string) float64 {
	leftTokens := tokenSet(left)
	rightTokens := tokenSet(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range leftTokens {
		if rightTokens[token] {
			intersection++
		}
	}
	union := len(leftTokens) + len(rightTokens) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(value string) map[string]bool {
	aliases := map[string]string{
		"colour":         "color",
		"colors":         "color",
		"colours":        "color",
		"palette":        "color",
		"palettes":       "color",
		"saturation":     "saturated",
		"low-saturation": "saturated",
	}
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	result := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		if alias, ok := aliases[field]; ok {
			field = alias
		}
		result[field] = true
	}
	return result
}

func memoryConflicts(left string, right string) bool {
	left = strings.ToLower(left)
	right = strings.ToLower(right)
	opposites := [][2]string{
		{"warm", "cool"},
		{"bright", "dark"},
		{"minimal", "complex"},
		{"saturated", "desaturated"},
		{"text", "no text"},
	}
	for _, pair := range opposites {
		if strings.Contains(left, pair[0]) && strings.Contains(right, pair[1]) {
			return true
		}
		if strings.Contains(left, pair[1]) && strings.Contains(right, pair[0]) {
			return true
		}
	}
	return false
}

func clampConfidence(confidence float64) float64 {
	if confidence < 0 {
		return 0
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
