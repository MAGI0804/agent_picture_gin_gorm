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
	OCRModelConfigID    uint
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
	ocrTool, hasOCR := agent.findOCRTool(state.UserID)
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
		ocrResult := tools.OCRResult{}
		if hasOCR {
			ocrResult, err = ocrTool.OCRProvider.ExtractText(ctx, tools.OCRRequest{
				UserID:   state.UserID,
				RunID:    state.RunID,
				StepID:   state.CurrentStepID,
				ImageRef: target.ImageRef,
				Prompt:   ocrReviewPrompt(state, index+1),
			})
			if err != nil {
				return domain.StepResult{}, err
			}
		}
		review := composeCandidateReview(state, target, result, ocrResult, hasOCR, agent.options.MinPassingScore)
		review.Reviewer = "real_vision_ocr_review"
		if !hasOCR {
			review.Reviewer = "real_vision_review"
		}
		reviews = append(reviews, domain.CandidateReview{
			ArtifactID:       review.ArtifactID,
			VersionID:        review.VersionID,
			ImageRef:         review.ImageRef,
			OverallScore:     review.OverallScore,
			RequirementMatch: review.RequirementMatch,
			CompositionScore: review.CompositionScore,
			TextReadability:  review.TextReadability,
			LayoutScore:      review.LayoutScore,
			Issues:           review.Issues,
			ShouldRefine:     review.ShouldRefine,
			Reviewer:         review.Reviewer,
			ExtractedText:    review.ExtractedText,
		})
		if index == 0 || review.OverallScore > bestScore {
			bestScore = review.OverallScore
			bestIndex = index
		}
	}
	bestReview := reviews[bestIndex]
	shouldRefine := bestReview.ShouldRefine || bestReview.OverallScore < agent.options.MinPassingScore
	output := map[string]interface{}{
		"overall_score":     bestReview.OverallScore,
		"requirement_match": bestReview.RequirementMatch,
		"composition_score": bestReview.CompositionScore,
		"text_readability":  bestReview.TextReadability,
		"layout_score":      bestReview.LayoutScore,
		"issues":            bestReview.Issues,
		"should_refine":     shouldRefine,
		"summary":           "best candidate " + strconv.Itoa(bestIndex+1),
		"reviewer":          bestReview.Reviewer,
		"extracted_text":    bestReview.ExtractedText,
		"candidate_reviews": reviews,
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "real vision review completed for image candidates",
		Output:  output,
	}, nil
}

func (agent *VisionReviewAgent) findOCRTool(userID uint) (tools.Tool, bool) {
	modelConfigID := agent.options.OCRModelConfigID
	if modelConfigID == 0 {
		modelConfigID = agent.options.VisionModelConfigID
	}
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindOCR,
		UserID:        userID,
		ModelConfigID: modelConfigID,
	})
	return tool, err == nil && tool.OCRProvider != nil
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

func composeCandidateReview(
	state domain.RunState,
	target candidateReviewTarget,
	visionResult tools.VisionResult,
	ocrResult tools.OCRResult,
	hasOCR bool,
	minPassingScore float64,
) domain.CandidateReview {
	requirementMatch := scoreOrDefault(visionResult.Scores, "requirement_match", visionOverallScore(visionResult))
	compositionScore := scoreOrDefault(visionResult.Scores, "composition_score", visionOverallScore(visionResult))
	textReadability := scoreOrDefault(visionResult.Scores, "text_readability", 0)
	layoutScore := scoreOrDefault(visionResult.Scores, "layout_score", compositionScore)
	if hasOCR {
		if ocrResult.TextReadability > 0 {
			textReadability = clampScore(ocrResult.TextReadability)
		}
		if ocrResult.LayoutScore > 0 {
			layoutScore = clampScore(ocrResult.LayoutScore)
		}
	}
	overallScore := weightedReviewScore(visionOverallScore(visionResult), requirementMatch, compositionScore, textReadability, layoutScore, hasOCR)
	issues := mergeIssues(visionResult.Issues, ocrResult.Issues)
	unreadableText := hasOCR && requiresReadableText(state) && textReadability > 0 && textReadability < 0.6
	if unreadableText {
		issues = append(issues, "text is not readable enough for the requested poster or brand image")
	}
	shouldRefine := visionResult.ShouldRefine || ocrResult.ShouldRefine || unreadableText || overallScore < minPassingScore
	return domain.CandidateReview{
		ArtifactID:       target.ArtifactID,
		VersionID:        target.VersionID,
		ImageRef:         target.ImageRef,
		OverallScore:     overallScore,
		RequirementMatch: requirementMatch,
		CompositionScore: compositionScore,
		TextReadability:  textReadability,
		LayoutScore:      layoutScore,
		Issues:           issues,
		ShouldRefine:     shouldRefine,
		ExtractedText:    strings.TrimSpace(ocrResult.Text),
	}
}

func weightedReviewScore(visionOverall float64, requirementMatch float64, compositionScore float64, textReadability float64, layoutScore float64, hasOCR bool) float64 {
	if !hasOCR {
		return clampScore(visionOverall)
	}
	if textReadability == 0 {
		textReadability = visionOverall
	}
	if layoutScore == 0 {
		layoutScore = compositionScore
	}
	return clampScore(visionOverall*0.35 + requirementMatch*0.25 + compositionScore*0.20 + textReadability*0.12 + layoutScore*0.08)
}

func scoreOrDefault(scores map[string]float64, key string, fallback float64) float64 {
	if scores == nil {
		return clampScore(fallback)
	}
	if score := scores[key]; score > 0 {
		return clampScore(score)
	}
	return clampScore(fallback)
}

func mergeIssues(groups ...[]string) []string {
	seen := map[string]bool{}
	issues := []string{}
	for _, group := range groups {
		for _, issue := range group {
			issue = strings.TrimSpace(issue)
			if issue == "" || seen[issue] {
				continue
			}
			seen[issue] = true
			issues = append(issues, issue)
		}
	}
	return issues
}

func requiresReadableText(state domain.RunState) bool {
	value := strings.ToLower(strings.Join([]string{
		state.UserRequest,
		state.Requirements.TextPolicy,
		state.Requirements.TargetUse,
		strings.Join(state.Requirements.LayoutHints, " "),
	}, " "))
	for _, fragment := range []string{"poster", "brand", "logo", "text", "typography", "海报", "品牌", "文字", "中文", "标题"} {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

func visionReviewPrompt(state domain.RunState, candidateIndex int) string {
	return strings.TrimSpace("Review this generated image against the user request. " +
		"Return JSON with keys summary, overall_score, requirement_match, composition_score, text_readability, layout_score, issues, should_refine. " +
		"Candidate index: " + strconv.Itoa(candidateIndex) + ". " +
		"User request: " + state.UserRequest)
}

func ocrReviewPrompt(state domain.RunState, candidateIndex int) string {
	return strings.TrimSpace("Extract visible text from this generated image and evaluate text readability and layout quality. " +
		"Return JSON with keys text, text_readability, layout_score, issues, should_refine. " +
		"Flag unreadable, garbled, misplaced, clipped, or low-contrast text as issues. " +
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
				"requirement_match": state.Review.RequirementMatch,
				"composition_score": state.Review.CompositionScore,
				"text_readability":  state.Review.TextReadability,
				"layout_score":      state.Review.LayoutScore,
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
			"requirement_match": top.RequirementMatch,
			"composition_score": top.CompositionScore,
			"text_readability":  top.TextReadability,
			"layout_score":      top.LayoutScore,
			"rank_score":        top.RankScore,
			"issues":            top.Issues,
			"should_refine":     top.ShouldRefine,
			"reviewer":          "ranker_agent",
			"extracted_text":    top.ExtractedText,
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
