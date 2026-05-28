package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

const (
	PromptStatusDraft    = "draft"
	PromptStatusReview   = "review"
	PromptStatusActive   = "active"
	PromptStatusArchived = "archived"
)

// EvolutionRepository defines persistence needed by prompt/eval governance.
type EvolutionRepository interface {
	ListReflections(agentName string, limit int) ([]model.AgentReflection, error)
	CreatePromptVersion(version *model.AgentPromptVersion) error
	ListPromptVersions(agentName string, limit int) ([]model.AgentPromptVersion, error)
	FindPromptVersion(versionID uint) (model.AgentPromptVersion, error)
	UpdatePromptVersion(versionID uint, attrs map[string]interface{}) error
	ArchiveActivePromptVersions(agentName string, exceptID uint) error
	CreateEvalCase(evalCase *model.EvalCase) error
	ListEvalCases(agentName string, limit int) ([]model.EvalCase, error)
	CreateEvalRun(run *model.EvalRun) error
	ListEvalRuns(agentName string, limit int) ([]model.EvalRun, error)
}

type FailureSummary struct {
	FailureType string `json:"failure_type"`
	Count       int    `json:"count"`
	ActionItem  string `json:"action_item"`
}

type DraftPromptInput struct {
	AgentName string
	Limit     int
}

type EvalCaseInput struct {
	AgentName    string
	Name         string
	InputJSON    string
	ExpectedJSON string
	TagsJSON     string
	Weight       float64
}

type EvalRunInput struct {
	EvalCaseID      uint
	PromptVersionID uint
	AgentName       string
	Status          string
	Score           float64
	MetricsJSON     string
	ErrorMessage    string
}

type EvolutionService struct {
	repo EvolutionRepository
}

func NewEvolutionService(repo EvolutionRepository) *EvolutionService {
	return &EvolutionService{repo: repo}
}

func (svc *EvolutionService) FailureSummary(agentName string, limit int) ([]FailureSummary, error) {
	reflections, err := svc.repo.ListReflections(strings.TrimSpace(agentName), normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	summaries := map[string]*FailureSummary{}
	for _, reflection := range reflections {
		key := strings.TrimSpace(reflection.FailureType)
		if key == "" {
			key = "unknown"
		}
		item := summaries[key]
		if item == nil {
			item = &FailureSummary{FailureType: key, ActionItem: reflection.ActionItem}
			summaries[key] = item
		}
		item.Count++
		if item.ActionItem == "" {
			item.ActionItem = reflection.ActionItem
		}
	}
	result := make([]FailureSummary, 0, len(summaries))
	for _, item := range summaries {
		result = append(result, *item)
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].FailureType < result[j].FailureType
	})
	if len(result) > 5 {
		result = result[:5]
	}
	return result, nil
}

func (svc *EvolutionService) DraftPromptVersion(input DraftPromptInput) (model.AgentPromptVersion, error) {
	agentName := strings.TrimSpace(input.AgentName)
	if agentName == "" {
		return model.AgentPromptVersion{}, errors.New("agent_name is required")
	}
	reflections, err := svc.repo.ListReflections(agentName, normalizeLimit(input.Limit))
	if err != nil {
		return model.AgentPromptVersion{}, err
	}
	if len(reflections) == 0 {
		return model.AgentPromptVersion{}, errors.New("no reflections available for prompt draft")
	}
	actionItems := uniqueActionItems(reflections)
	template := buildPromptTemplate(agentName, actionItems)
	metrics, _ := json.Marshal(map[string]interface{}{
		"reflection_count": len(reflections),
		"action_items":     actionItems,
	})
	version := model.AgentPromptVersion{
		AgentName:      agentName,
		Version:        fmt.Sprintf("draft-%d", time.Now().Unix()),
		PromptTemplate: template,
		Changelog:      "Drafted from recent low-score reflections.",
		Status:         PromptStatusDraft,
		Metrics:        string(metrics),
	}
	if err := svc.repo.CreatePromptVersion(&version); err != nil {
		return model.AgentPromptVersion{}, err
	}
	return version, nil
}

func (svc *EvolutionService) ListPromptVersions(agentName string, limit int) ([]model.AgentPromptVersion, error) {
	return svc.repo.ListPromptVersions(strings.TrimSpace(agentName), normalizeLimit(limit))
}

func (svc *EvolutionService) MovePromptVersionToReview(versionID uint) (model.AgentPromptVersion, error) {
	version, err := svc.repo.FindPromptVersion(versionID)
	if err != nil {
		return model.AgentPromptVersion{}, err
	}
	if version.Status != PromptStatusDraft {
		return model.AgentPromptVersion{}, errors.New("only draft prompt versions can move to review")
	}
	if err := svc.repo.UpdatePromptVersion(versionID, map[string]interface{}{"status": PromptStatusReview}); err != nil {
		return model.AgentPromptVersion{}, err
	}
	version.Status = PromptStatusReview
	return version, nil
}

func (svc *EvolutionService) ActivatePromptVersion(versionID uint) (model.AgentPromptVersion, error) {
	version, err := svc.repo.FindPromptVersion(versionID)
	if err != nil {
		return model.AgentPromptVersion{}, err
	}
	if version.Status != PromptStatusReview && version.Status != PromptStatusArchived {
		return model.AgentPromptVersion{}, errors.New("only review or archived prompt versions can activate")
	}
	if err := svc.repo.ArchiveActivePromptVersions(version.AgentName, version.ID); err != nil {
		return model.AgentPromptVersion{}, err
	}
	if err := svc.repo.UpdatePromptVersion(versionID, map[string]interface{}{"status": PromptStatusActive}); err != nil {
		return model.AgentPromptVersion{}, err
	}
	version.Status = PromptStatusActive
	return version, nil
}

func (svc *EvolutionService) ArchivePromptVersion(versionID uint) (model.AgentPromptVersion, error) {
	version, err := svc.repo.FindPromptVersion(versionID)
	if err != nil {
		return model.AgentPromptVersion{}, err
	}
	if err := svc.repo.UpdatePromptVersion(versionID, map[string]interface{}{"status": PromptStatusArchived}); err != nil {
		return model.AgentPromptVersion{}, err
	}
	version.Status = PromptStatusArchived
	return version, nil
}

func (svc *EvolutionService) CreateEvalCase(input EvalCaseInput) (model.EvalCase, error) {
	agentName := strings.TrimSpace(input.AgentName)
	if agentName == "" {
		return model.EvalCase{}, errors.New("eval case agent_name is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.EvalCase{}, errors.New("eval case name is required")
	}
	weight := input.Weight
	if weight <= 0 {
		weight = 1
	}
	evalCase := model.EvalCase{
		AgentName:    agentName,
		Name:         name,
		InputJSON:    strings.TrimSpace(input.InputJSON),
		ExpectedJSON: strings.TrimSpace(input.ExpectedJSON),
		TagsJSON:     strings.TrimSpace(input.TagsJSON),
		Status:       "active",
		Weight:       weight,
	}
	if evalCase.InputJSON == "" {
		return model.EvalCase{}, errors.New("eval case input_json is required")
	}
	if err := svc.repo.CreateEvalCase(&evalCase); err != nil {
		return model.EvalCase{}, err
	}
	return evalCase, nil
}

func (svc *EvolutionService) ListEvalCases(agentName string, limit int) ([]model.EvalCase, error) {
	return svc.repo.ListEvalCases(strings.TrimSpace(agentName), normalizeLimit(limit))
}

func (svc *EvolutionService) CreateEvalRun(input EvalRunInput) (model.EvalRun, error) {
	if input.EvalCaseID == 0 {
		return model.EvalRun{}, errors.New("eval run eval_case_id is required")
	}
	agentName := strings.TrimSpace(input.AgentName)
	if agentName == "" {
		return model.EvalRun{}, errors.New("eval run agent_name is required")
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "completed"
	}
	now := int(time.Now().Unix())
	run := model.EvalRun{
		EvalCaseID:      input.EvalCaseID,
		PromptVersionID: input.PromptVersionID,
		AgentName:       agentName,
		Status:          status,
		Score:           input.Score,
		MetricsJSON:     strings.TrimSpace(input.MetricsJSON),
		ErrorMessage:    strings.TrimSpace(input.ErrorMessage),
		StartedAt:       now,
		CompletedAt:     now,
	}
	if err := svc.repo.CreateEvalRun(&run); err != nil {
		return model.EvalRun{}, err
	}
	return run, nil
}

func (svc *EvolutionService) ListEvalRuns(agentName string, limit int) ([]model.EvalRun, error) {
	return svc.repo.ListEvalRuns(strings.TrimSpace(agentName), normalizeLimit(limit))
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func uniqueActionItems(reflections []model.AgentReflection) []string {
	seen := map[string]struct{}{}
	items := []string{}
	for _, reflection := range reflections {
		item := strings.TrimSpace(reflection.ActionItem)
		if item == "" {
			item = strings.TrimSpace(reflection.Reflection)
		}
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
		if len(items) >= 8 {
			break
		}
	}
	return items
}

func buildPromptTemplate(agentName string, actionItems []string) string {
	lines := []string{
		fmt.Sprintf("Agent: %s", agentName),
		"Apply these reviewed improvements before producing output:",
	}
	for _, item := range actionItems {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}
