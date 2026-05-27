package agents

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
)

// MockVisionReviewAgent provides a deterministic first-pass review node.
type MockVisionReviewAgent struct {
	minPassingScore float64
}

// NewMockVisionReviewAgent creates a mock review agent with a pass threshold.
func NewMockVisionReviewAgent(minPassingScore float64) *MockVisionReviewAgent {
	return &MockVisionReviewAgent{minPassingScore: minPassingScore}
}

// Key returns the workflow node key.
func (agent *MockVisionReviewAgent) Key() string {
	return "vision_review_agent"
}

// Run scores existing artifacts without calling an external VLM.
func (agent *MockVisionReviewAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	score := 0.82
	issues := []string{}
	reflectionDraft := ""
	candidateReviews := []domain.CandidateReview{}

	if len(state.Artifacts) == 0 {
		score = 0.30
		issues = append(issues, "no artifact generated")
		reflectionDraft = "Image generation produced no artifact; check provider output and artifact persistence."
	}
	if state.Requirements.NeedClarification {
		score -= 0.15
		issues = append(issues, "requirements still need clarification")
	}
	for index, target := range candidateReviewTargets(state) {
		candidateScore := score - float64(index)*0.04
		if candidateScore < 0 {
			candidateScore = 0
		}
		candidateReviews = append(candidateReviews, domain.CandidateReview{
			ArtifactID:   target.ArtifactID,
			VersionID:    target.VersionID,
			ImageRef:     target.ImageRef,
			OverallScore: candidateScore,
			Issues:       append([]string{}, issues...),
			ShouldRefine: candidateScore < agent.minPassingScore,
			Reviewer:     "mock_vision_review",
		})
	}

	shouldRefine := score < agent.minPassingScore
	output := map[string]interface{}{
		"overall_score":     score,
		"issues":            issues,
		"should_refine":     shouldRefine,
		"reflection_draft":  reflectionDraft,
		"reviewer":          "mock_vision_review",
		"candidate_reviews": candidateReviews,
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "mock vision review completed",
		Output:  output,
	}, nil
}

// VisionReviewAgent calls a registered VLM/OCR-capable provider for real image review.
type VisionReviewAgent struct {
	registry *tools.Registry
	options  VisionReviewAgentOptions
}

// VisionReviewAgentOptions configures real vision review.
type VisionReviewAgentOptions struct {
	VisionModelConfigID uint
	MinPassingScore     float64
}

// NewVisionReviewAgent creates a real provider-backed review agent.
func NewVisionReviewAgent(registry *tools.Registry, options VisionReviewAgentOptions) *VisionReviewAgent {
	if options.MinPassingScore <= 0 {
		options.MinPassingScore = 0.7
	}
	return &VisionReviewAgent{registry: registry, options: options}
}

// Key returns the workflow node key.
func (agent *VisionReviewAgent) Key() string {
	return "vision_review_agent"
}

// Run reviews each generated artifact candidate with a registered VisionProvider.
func (agent *VisionReviewAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.registry == nil {
		return domain.StepResult{}, errors.New("tool registry is required")
	}
	targets := candidateReviewTargets(state)
	if len(targets) == 0 {
		return domain.StepResult{
			Status:  domain.StepStatusCompleted,
			Summary: "real vision review found no generated image",
			Output: map[string]interface{}{
				"overall_score":    0.30,
				"issues":           []string{"no generated image available for vision review"},
				"should_refine":    true,
				"reflection_draft": "Image generation produced no artifact; check provider output and artifact persistence.",
				"reviewer":         "real_vision_review",
			},
		}, nil
	}
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindVision,
		UserID:        state.UserID,
		ModelConfigID: agent.options.VisionModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	reviews := make([]domain.CandidateReview, 0, len(targets))
	bestScore := 0.0
	bestIndex := 0
	for index, target := range targets {
		result, err := tool.VisionProvider.AnalyzeImage(ctx, tools.VisionRequest{
			UserID:   state.UserID,
			RunID:    state.RunID,
			StepID:   state.CurrentStepID,
			ImageRef: target.ImageRef,
			Prompt:   visionReviewPrompt(state, index+1),
		})
		if err != nil {
			return domain.StepResult{}, err
		}
		score := visionOverallScore(result)
		shouldRefine := result.ShouldRefine || score < agent.options.MinPassingScore
		reviews = append(reviews, domain.CandidateReview{
			ArtifactID:   target.ArtifactID,
			VersionID:    target.VersionID,
			ImageRef:     target.ImageRef,
			OverallScore: score,
			Issues:       append([]string{}, result.Issues...),
			ShouldRefine: shouldRefine,
			Reviewer:     "real_vision_review",
		})
		if index == 0 || score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	bestReview := reviews[bestIndex]
	shouldRefine := bestReview.ShouldRefine || bestReview.OverallScore < agent.options.MinPassingScore
	output := map[string]interface{}{
		"overall_score":     bestReview.OverallScore,
		"issues":            bestReview.Issues,
		"should_refine":     shouldRefine,
		"summary":           "best candidate " + strconv.Itoa(bestIndex+1),
		"reviewer":          "real_vision_review",
		"candidate_reviews": reviews,
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "real vision review completed for image candidates",
		Output:  output,
	}, nil
}

type candidateReviewTarget struct {
	ArtifactID uint
	VersionID  uint
	ImageRef   string
}

func candidateReviewTargets(state domain.RunState) []candidateReviewTarget {
	targets := make([]candidateReviewTarget, 0, maxInt(len(state.Artifacts), len(state.GeneratedImages)))
	for index, artifact := range state.Artifacts {
		imageRef := ""
		if index < len(state.GeneratedImages) {
			imageRef = generatedImageRef(state.GeneratedImages[index])
		}
		if imageRef == "" {
			imageRef = strings.TrimSpace(artifact.PreviewURL)
		}
		if imageRef == "" {
			continue
		}
		targets = append(targets, candidateReviewTarget{
			ArtifactID: artifact.ID,
			VersionID:  artifact.VersionID,
			ImageRef:   imageRef,
		})
	}
	if len(targets) > 0 {
		return targets
	}
	for _, image := range state.GeneratedImages {
		imageRef := generatedImageRef(image)
		if imageRef == "" {
			continue
		}
		targets = append(targets, candidateReviewTarget{ImageRef: imageRef})
	}
	return targets
}

func generatedImageRef(image domain.GeneratedImageRef) string {
	if strings.TrimSpace(image.ObjectKey) != "" {
		return strings.TrimSpace(image.ObjectKey)
	}
	if strings.TrimSpace(image.PreviewURL) != "" {
		return strings.TrimSpace(image.PreviewURL)
	}
	return ""
}

func visionOverallScore(result tools.VisionResult) float64 {
	score := result.Scores["overall"]
	if score == 0 {
		score = result.Scores["overall_score"]
	}
	if score == 0 {
		return 0.75
	}
	return clampScore(score)
}

func visionReviewPrompt(state domain.RunState, candidateIndex int) string {
	return strings.TrimSpace("Review this generated image against the user request. " +
		"Return JSON with keys summary, overall_score, issues, should_refine. " +
		"Candidate index: " + strconv.Itoa(candidateIndex) + ". " +
		"User request: " + state.UserRequest)
}

// RankerAgent assigns rank_score to reviewed candidates.
type RankerAgent struct{}

func NewRankerAgent() *RankerAgent {
	return &RankerAgent{}
}

func (agent *RankerAgent) Key() string {
	return "ranker_agent"
}

func (agent *RankerAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	reviews := append([]domain.CandidateReview{}, state.Review.CandidateReviews...)
	if len(reviews) == 0 {
		return domain.StepResult{
			Status:  domain.StepStatusCompleted,
			Summary: "ranker found no candidate review",
			Output: map[string]interface{}{
				"overall_score":     state.Review.OverallScore,
				"issues":            state.Review.Issues,
				"should_refine":     state.Review.ShouldRefine,
				"reviewer":          "ranker_agent",
				"candidate_reviews": reviews,
			},
		}, nil
	}
	for index := range reviews {
		if reviews[index].OverallScore == 0 {
			reviews[index].OverallScore = state.Review.OverallScore
		}
		if reviews[index].Reviewer == "" {
			reviews[index].Reviewer = state.Review.Reviewer
		}
		reviews[index].RankScore = candidateRankScore(state, reviews[index], index)
		reviews[index].RankReason = "review score plus requirement, memory, and failure signals"
	}
	sort.SliceStable(reviews, func(i, j int) bool {
		if reviews[i].RankScore == reviews[j].RankScore {
			return reviews[i].ArtifactID < reviews[j].ArtifactID
		}
		return reviews[i].RankScore > reviews[j].RankScore
	})
	top := reviews[0]
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "ranked image candidates",
		Output: map[string]interface{}{
			"overall_score":     top.OverallScore,
			"rank_score":        top.RankScore,
			"issues":            top.Issues,
			"should_refine":     top.ShouldRefine,
			"reviewer":          "ranker_agent",
			"candidate_reviews": reviews,
		},
	}, nil
}

func candidateRankScore(state domain.RunState, review domain.CandidateReview, index int) float64 {
	score := review.OverallScore
	if score == 0 {
		score = state.Review.OverallScore
	}
	score += requirementMatchBonus(review.Issues)
	score += memoryPreferenceBonus(state.MemoryContext)
	if review.ShouldRefine {
		score -= 0.15
	}
	score -= float64(len(review.Issues)) * 0.02
	if hasGenerationFailure(review.Issues) {
		score -= 0.25
	}
	score -= float64(index) * 0.001
	return clampScore(score)
}

func requirementMatchBonus(issues []string) float64 {
	if len(issues) == 0 {
		return 0.06
	}
	for _, issue := range issues {
		normalized := strings.ToLower(issue)
		if strings.Contains(normalized, "missing") ||
			strings.Contains(normalized, "not match") ||
			strings.Contains(normalized, "does not match") ||
			strings.Contains(normalized, "unreadable") {
			return 0
		}
	}
	return 0.03
}

func memoryPreferenceBonus(memories []domain.MemoryItem) float64 {
	for _, memory := range memories {
		if memory.Confidence >= 0.8 && strings.TrimSpace(memory.Content) != "" {
			return 0.02
		}
	}
	return 0
}

func hasGenerationFailure(issues []string) bool {
	for _, issue := range issues {
		normalized := strings.ToLower(issue)
		if strings.Contains(normalized, "no artifact") ||
			strings.Contains(normalized, "provider") ||
			strings.Contains(normalized, "failed") {
			return true
		}
	}
	return false
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
