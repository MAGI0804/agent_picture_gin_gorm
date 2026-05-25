package eval

import (
	"errors"
	"fmt"
	"strings"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
)

const (
	FailureTypeLowReviewScore = "low_review_score"
)

// ReflectionRepository defines persistence needed by ReflectionService.
type ReflectionRepository interface {
	CreateReflection(reflection *model.AgentReflection) error
}

// ReflectionService creates draft reflections from review outcomes.
type ReflectionService struct {
	repo           ReflectionRepository
	minReviewScore float64
}

// DraftReflectionInput describes a potential reflection source.
type DraftReflectionInput struct {
	RunID     uint
	AgentName string
	Review    domain.ReviewResult
}

// NewReflectionService creates a reflection service.
func NewReflectionService(repo ReflectionRepository, minReviewScore float64) *ReflectionService {
	return &ReflectionService{
		repo:           repo,
		minReviewScore: minReviewScore,
	}
}

// DraftLowScoreReflection creates a draft reflection only for low review scores.
func (svc *ReflectionService) DraftLowScoreReflection(
	input DraftReflectionInput,
) (model.AgentReflection, bool, error) {
	if input.RunID == 0 {
		return model.AgentReflection{}, false, errors.New("reflection run_id is required")
	}
	if input.AgentName == "" {
		return model.AgentReflection{}, false, errors.New("reflection agent_name is required")
	}
	if input.Review.OverallScore >= svc.minReviewScore {
		return model.AgentReflection{}, false, nil
	}

	issueSummary := strings.Join(input.Review.Issues, "; ")
	if issueSummary == "" {
		issueSummary = "review score below threshold"
	}
	reflection := model.AgentReflection{
		AgentRunID:       input.RunID,
		AgentName:        input.AgentName,
		FailureType:      FailureTypeLowReviewScore,
		Reflection:       fmt.Sprintf("Review score %.2f below threshold %.2f: %s", input.Review.OverallScore, svc.minReviewScore, issueSummary),
		ActionItem:       "Keep this as draft reflection; review before promoting to memory or prompt changes.",
		PromotedToMemory: false,
	}
	if err := svc.repo.CreateReflection(&reflection); err != nil {
		return model.AgentReflection{}, false, err
	}
	return reflection, true, nil
}
