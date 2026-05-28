package eval

import (
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestEvolutionServiceSummarizesTopFailureTypes(t *testing.T) {
	repo := &fakeEvolutionRepository{
		reflections: []model.AgentReflection{
			{AgentName: "vision_review_agent", FailureType: "ocr_low", ActionItem: "Improve text contrast."},
			{AgentName: "vision_review_agent", FailureType: "ocr_low", ActionItem: "Improve text contrast."},
			{AgentName: "vision_review_agent", FailureType: "composition", ActionItem: "Leave title space."},
		},
	}
	svc := NewEvolutionService(repo)

	summary, err := svc.FailureSummary("vision_review_agent", 20)
	if err != nil {
		t.Fatalf("FailureSummary() error = %v", err)
	}
	if len(summary) != 2 {
		t.Fatalf("len(summary) = %d, want 2", len(summary))
	}
	if summary[0].FailureType != "ocr_low" || summary[0].Count != 2 {
		t.Fatalf("top summary = %#v, want ocr_low count 2", summary[0])
	}
}

func TestEvolutionServiceDraftsPromptVersionFromReflections(t *testing.T) {
	repo := &fakeEvolutionRepository{
		reflections: []model.AgentReflection{
			{AgentName: "prompt_agent", FailureType: "low_review_score", ActionItem: "Render Chinese title in layout layer."},
		},
	}
	svc := NewEvolutionService(repo)

	version, err := svc.DraftPromptVersion(DraftPromptInput{AgentName: "prompt_agent"})
	if err != nil {
		t.Fatalf("DraftPromptVersion() error = %v", err)
	}
	if version.Status != PromptStatusDraft || version.ID == 0 {
		t.Fatalf("version = %#v, want persisted draft", version)
	}
	if !strings.Contains(version.PromptTemplate, "Render Chinese title") {
		t.Fatalf("PromptTemplate = %q, want reflection action item", version.PromptTemplate)
	}
}

func TestEvolutionServicePromptVersionLifecycle(t *testing.T) {
	repo := &fakeEvolutionRepository{
		promptVersions: []model.AgentPromptVersion{
			{BaseModel: model.BaseModel{ID: 1}, AgentName: "prompt_agent", Status: PromptStatusActive},
			{BaseModel: model.BaseModel{ID: 2}, AgentName: "prompt_agent", Status: PromptStatusDraft},
		},
	}
	svc := NewEvolutionService(repo)

	review, err := svc.MovePromptVersionToReview(2)
	if err != nil {
		t.Fatalf("MovePromptVersionToReview() error = %v", err)
	}
	if review.Status != PromptStatusReview {
		t.Fatalf("review status = %q, want review", review.Status)
	}
	active, err := svc.ActivatePromptVersion(2)
	if err != nil {
		t.Fatalf("ActivatePromptVersion() error = %v", err)
	}
	if active.Status != PromptStatusActive {
		t.Fatalf("active status = %q, want active", active.Status)
	}
	if repo.promptVersions[0].Status != PromptStatusArchived {
		t.Fatalf("previous active status = %q, want archived", repo.promptVersions[0].Status)
	}
}

type fakeEvolutionRepository struct {
	reflections    []model.AgentReflection
	promptVersions []model.AgentPromptVersion
	evalCases      []model.EvalCase
	evalRuns       []model.EvalRun
}

func (repo *fakeEvolutionRepository) ListReflections(agentName string, limit int) ([]model.AgentReflection, error) {
	result := []model.AgentReflection{}
	for _, reflection := range repo.reflections {
		if agentName == "" || reflection.AgentName == agentName {
			result = append(result, reflection)
		}
	}
	return result, nil
}

func (repo *fakeEvolutionRepository) CreatePromptVersion(version *model.AgentPromptVersion) error {
	version.ID = uint(len(repo.promptVersions) + 1)
	repo.promptVersions = append(repo.promptVersions, *version)
	return nil
}

func (repo *fakeEvolutionRepository) ListPromptVersions(agentName string, limit int) ([]model.AgentPromptVersion, error) {
	return append([]model.AgentPromptVersion{}, repo.promptVersions...), nil
}

func (repo *fakeEvolutionRepository) FindPromptVersion(versionID uint) (model.AgentPromptVersion, error) {
	for _, version := range repo.promptVersions {
		if version.ID == versionID {
			return version, nil
		}
	}
	return model.AgentPromptVersion{}, errNotFoundForTest{}
}

func (repo *fakeEvolutionRepository) UpdatePromptVersion(versionID uint, attrs map[string]interface{}) error {
	for index := range repo.promptVersions {
		if repo.promptVersions[index].ID != versionID {
			continue
		}
		if status, ok := attrs["status"].(string); ok {
			repo.promptVersions[index].Status = status
		}
		return nil
	}
	return errNotFoundForTest{}
}

func (repo *fakeEvolutionRepository) ArchiveActivePromptVersions(agentName string, exceptID uint) error {
	for index := range repo.promptVersions {
		if repo.promptVersions[index].AgentName == agentName && repo.promptVersions[index].Status == PromptStatusActive && repo.promptVersions[index].ID != exceptID {
			repo.promptVersions[index].Status = PromptStatusArchived
		}
	}
	return nil
}

func (repo *fakeEvolutionRepository) CreateEvalCase(evalCase *model.EvalCase) error {
	evalCase.ID = uint(len(repo.evalCases) + 1)
	repo.evalCases = append(repo.evalCases, *evalCase)
	return nil
}

func (repo *fakeEvolutionRepository) ListEvalCases(agentName string, limit int) ([]model.EvalCase, error) {
	return append([]model.EvalCase{}, repo.evalCases...), nil
}

func (repo *fakeEvolutionRepository) CreateEvalRun(run *model.EvalRun) error {
	run.ID = uint(len(repo.evalRuns) + 1)
	repo.evalRuns = append(repo.evalRuns, *run)
	return nil
}

func (repo *fakeEvolutionRepository) ListEvalRuns(agentName string, limit int) ([]model.EvalRun, error) {
	return append([]model.EvalRun{}, repo.evalRuns...), nil
}

type errNotFoundForTest struct{}

func (errNotFoundForTest) Error() string { return "not found" }
