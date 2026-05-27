package artifact

import (
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestServiceCreateArtifactWithVersionCreatesBothRecords(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	artifact, version, err := svc.CreateArtifactWithVersion(CreateArtifactWithVersionInput{
		Artifact: model.Artifact{
			UserID:         7,
			ConversationID: 9,
			AgentRunID:     11,
			Name:           "poster.png",
			Kind:           "image",
			MimeType:       "image/png",
			ObjectKey:      "objects/poster.png",
		},
		Version: model.ArtifactVersion{
			AgentRunID: 11,
			VersionNo:  1,
			Operation:  "generate",
			ObjectKey:  "objects/poster.png",
		},
	})
	if err != nil {
		t.Fatalf("CreateArtifactWithVersion() error = %v", err)
	}
	if artifact.ID == 0 {
		t.Fatal("artifact ID was not assigned")
	}
	if version.ArtifactID != artifact.ID {
		t.Fatalf("version artifact ID = %d, want %d", version.ArtifactID, artifact.ID)
	}
	if !repo.createdArtifact || !repo.createdVersion {
		t.Fatalf("createdArtifact=%v createdVersion=%v, want both true", repo.createdArtifact, repo.createdVersion)
	}
}

func TestServiceCreateArtifactWithVersionRequiresVersion(t *testing.T) {
	svc := NewService(&fakeRepository{})

	_, _, err := svc.CreateArtifactWithVersion(CreateArtifactWithVersionInput{
		Artifact: model.Artifact{
			UserID:         7,
			ConversationID: 9,
			Name:           "poster.png",
			Kind:           "image",
			MimeType:       "image/png",
			ObjectKey:      "objects/poster.png",
		},
	})
	if err == nil {
		t.Fatal("CreateArtifactWithVersion() error = nil, want validation error")
	}
}

func TestServiceAuthorizeDownloadUsesRepositoryOwnershipCheck(t *testing.T) {
	expected := model.Artifact{BaseModel: model.BaseModel{ID: 3}, UserID: 7}
	repo := &fakeRepository{artifact: expected}
	svc := NewService(repo)

	artifact, err := svc.AuthorizeDownload(7, 3)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if artifact.ID != expected.ID {
		t.Fatalf("AuthorizeDownload() ID = %d, want %d", artifact.ID, expected.ID)
	}
	if repo.findUserID != 7 || repo.findArtifactID != 3 {
		t.Fatalf("FindArtifact called with userID=%d artifactID=%d", repo.findUserID, repo.findArtifactID)
	}
}

func TestServiceCreateCandidateGroupAssignsSharedGroupID(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	artifacts, versions, err := svc.CreateCandidateGroup(CreateCandidateGroupInput{
		AgentRunID:     11,
		UserID:         7,
		ConversationID: 9,
		Artifacts: []CreateArtifactWithVersionInput{
			{
				Artifact: model.Artifact{Name: "candidate-1.png", Kind: "image", MimeType: "image/png", ObjectKey: "objects/1.png"},
				Version:  model.ArtifactVersion{Operation: "generate", ObjectKey: "objects/1.png"},
			},
			{
				Artifact: model.Artifact{Name: "candidate-2.png", Kind: "image", MimeType: "image/png", ObjectKey: "objects/2.png"},
				Version:  model.ArtifactVersion{Operation: "generate", ObjectKey: "objects/2.png"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateCandidateGroup() error = %v", err)
	}
	if len(artifacts) != 2 || len(versions) != 2 {
		t.Fatalf("got %d artifacts and %d versions, want 2 and 2", len(artifacts), len(versions))
	}
	if artifacts[0].ArtifactGroupID == "" {
		t.Fatal("candidate group ID was empty")
	}
	if artifacts[0].ArtifactGroupID != artifacts[1].ArtifactGroupID {
		t.Fatalf("group IDs = %q and %q, want same", artifacts[0].ArtifactGroupID, artifacts[1].ArtifactGroupID)
	}
	if artifacts[0].AgentRunID != 11 || artifacts[0].UserID != 7 || artifacts[0].ConversationID != 9 {
		t.Fatalf("artifact defaults were not applied: %#v", artifacts[0])
	}
}

func TestServiceSelectArtifactMarksSelectedAndWritesFeedback(t *testing.T) {
	repo := &fakeRepository{artifact: model.Artifact{BaseModel: model.BaseModel{ID: 3}, UserID: 7}}
	svc := NewService(repo)

	err := svc.SelectArtifact(SelectArtifactInput{
		UserID:            7,
		ArtifactID:        3,
		ArtifactVersionID: 5,
	})
	if err != nil {
		t.Fatalf("SelectArtifact() error = %v", err)
	}
	if repo.updatedArtifactID != 3 {
		t.Fatalf("updated artifact ID = %d, want 3", repo.updatedArtifactID)
	}
	if selectedAt, ok := repo.updatedArtifactAttrs["selected_at"].(int); !ok || selectedAt == 0 {
		t.Fatalf("selected_at update = %#v, want non-zero int", repo.updatedArtifactAttrs["selected_at"])
	}
	if repo.feedback.FeedbackType != FeedbackTypeSelected {
		t.Fatalf("feedback type = %q, want %q", repo.feedback.FeedbackType, FeedbackTypeSelected)
	}
}

func TestServiceRecordReviewScoresChecksOwnershipAndUpdatesVersions(t *testing.T) {
	repo := &fakeRepository{artifact: model.Artifact{BaseModel: model.BaseModel{ID: 3}, UserID: 7}}
	svc := NewService(repo)

	err := svc.RecordReviewScores(ReviewScoresInput{
		UserID:       7,
		ArtifactID:   3,
		VersionID:    5,
		OverallScore: 0.82,
		Issues:       []string{"minor composition risk"},
		ShouldRefine: false,
		Reviewer:     "mock_vision_review",
	})
	if err != nil {
		t.Fatalf("RecordReviewScores() error = %v", err)
	}
	if repo.findUserID != 7 || repo.findArtifactID != 3 {
		t.Fatalf("FindArtifact called with userID=%d artifactID=%d, want 7/3", repo.findUserID, repo.findArtifactID)
	}
	if repo.updatedVersionID != 5 {
		t.Fatalf("updated version ID = %d, want 5", repo.updatedVersionID)
	}
	if repo.updatedVersionArtifactID != 3 {
		t.Fatalf("updated version artifact ID = %d, want 3", repo.updatedVersionArtifactID)
	}
	qualityScores, ok := repo.updatedVersionAttrs["quality_scores"].(string)
	if !ok || qualityScores == "" {
		t.Fatalf("quality_scores update = %#v, want JSON string", repo.updatedVersionAttrs["quality_scores"])
	}
	if want := `"overall_score":0.82`; !strings.Contains(qualityScores, want) {
		t.Fatalf("quality_scores = %s, want %s", qualityScores, want)
	}
	if rankScore, ok := repo.updatedArtifactAttrs["rank_score"].(float64); !ok || rankScore != 0.82 {
		t.Fatalf("rank_score update = %#v, want 0.82", repo.updatedArtifactAttrs["rank_score"])
	}
}

func TestServiceRecordReviewScoresUsesExplicitRankScore(t *testing.T) {
	repo := &fakeRepository{artifact: model.Artifact{BaseModel: model.BaseModel{ID: 3}, UserID: 7}}
	svc := NewService(repo)

	err := svc.RecordReviewScores(ReviewScoresInput{
		UserID:       7,
		ArtifactID:   3,
		VersionID:    5,
		OverallScore: 0.82,
		RankScore:    0.91,
		Reviewer:     "ranker_agent",
	})
	if err != nil {
		t.Fatalf("RecordReviewScores() error = %v", err)
	}
	if rankScore, ok := repo.updatedArtifactAttrs["rank_score"].(float64); !ok || rankScore != 0.91 {
		t.Fatalf("rank_score update = %#v, want 0.91", repo.updatedArtifactAttrs["rank_score"])
	}
}

func TestServiceCreateRefinedVersionKeepsParentAndUpdatesArtifactPreview(t *testing.T) {
	repo := &fakeRepository{
		artifact: model.Artifact{BaseModel: model.BaseModel{ID: 3}, UserID: 7},
		versions: []model.ArtifactVersion{
			{BaseModel: model.BaseModel{ID: 5}, ArtifactID: 3, VersionNo: 1, Operation: "generate", ObjectKey: "objects/original.png"},
		},
	}
	svc := NewService(repo)

	version, err := svc.CreateRefinedVersion(CreateRefinedVersionInput{
		UserID:          7,
		ArtifactID:      3,
		ParentVersionID: 5,
		AgentRunID:      11,
		Image: model.ArtifactVersion{
			Operation:      "refine",
			Prompt:         "improve readability",
			ModelProvider:  "google",
			ModelName:      "imagen",
			ObjectKey:      "objects/refined.png",
			PreviewURL:     "/preview/refined",
			Hash:           "hash-refined",
			SourceRefs:     `[{"version_id":5}]`,
			QualityScores:  "",
			NegativePrompt: "blur",
		},
	})
	if err != nil {
		t.Fatalf("CreateRefinedVersion() error = %v", err)
	}
	if version.ParentVersionID != 5 || version.VersionNo != 2 || version.Operation != "refine" {
		t.Fatalf("version = %#v, want parent 5, v2 refine", version)
	}
	if repo.updatedArtifactID != 3 {
		t.Fatalf("updated artifact ID = %d, want 3", repo.updatedArtifactID)
	}
	if repo.updatedArtifactAttrs["object_key"] != "objects/refined.png" || repo.updatedArtifactAttrs["preview_url"] != "/preview/refined" {
		t.Fatalf("artifact update = %#v, want refined object and preview", repo.updatedArtifactAttrs)
	}
}

func TestServiceCreateRefinedVersionAllowsManualEditOperation(t *testing.T) {
	repo := &fakeRepository{
		artifact: model.Artifact{BaseModel: model.BaseModel{ID: 3}, UserID: 7},
		versions: []model.ArtifactVersion{
			{BaseModel: model.BaseModel{ID: 5}, ArtifactID: 3, VersionNo: 1, Operation: "upload", ObjectKey: "objects/original.png"},
		},
	}
	svc := NewService(repo)

	version, err := svc.CreateRefinedVersion(CreateRefinedVersionInput{
		UserID:          7,
		ArtifactID:      3,
		ParentVersionID: 5,
		Image: model.ArtifactVersion{
			Operation:  "edit",
			Prompt:     "change background",
			ObjectKey:  "objects/edited.png",
			PreviewURL: "/preview/edited",
		},
	})
	if err != nil {
		t.Fatalf("CreateRefinedVersion() error = %v", err)
	}
	if version.Operation != "edit" || version.ParentVersionID != 5 || version.VersionNo != 2 {
		t.Fatalf("version = %#v, want edit child version", version)
	}
}

type fakeRepository struct {
	artifact                 model.Artifact
	createdArtifact          bool
	createdVersion           bool
	feedback                 model.ArtifactFeedback
	findUserID               uint
	findArtifactID           uint
	updatedArtifactID        uint
	updatedArtifactAttrs     map[string]interface{}
	updatedVersionArtifactID uint
	updatedVersionID         uint
	updatedVersionAttrs      map[string]interface{}
	versions                 []model.ArtifactVersion
}

func (repo *fakeRepository) CreateArtifact(artifact *model.Artifact) error {
	repo.createdArtifact = true
	artifact.ID = 100
	return nil
}

func (repo *fakeRepository) FindArtifact(userID uint, artifactID uint) (model.Artifact, error) {
	repo.findUserID = userID
	repo.findArtifactID = artifactID
	if repo.artifact.ID == 0 {
		return model.Artifact{}, errors.New("not found")
	}
	return repo.artifact, nil
}

func (repo *fakeRepository) UpdateArtifact(artifactID uint, attrs map[string]interface{}) error {
	repo.updatedArtifactID = artifactID
	repo.updatedArtifactAttrs = attrs
	return nil
}

func (repo *fakeRepository) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	return []model.Artifact{}, nil
}

func (repo *fakeRepository) CreateArtifactVersion(version *model.ArtifactVersion) error {
	repo.createdVersion = true
	version.ID = 200
	repo.versions = append(repo.versions, *version)
	return nil
}

func (repo *fakeRepository) UpdateArtifactVersion(artifactID uint, versionID uint, attrs map[string]interface{}) error {
	repo.updatedVersionArtifactID = artifactID
	repo.updatedVersionID = versionID
	repo.updatedVersionAttrs = attrs
	return nil
}

func (repo *fakeRepository) ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	return append([]model.ArtifactVersion{}, repo.versions...), nil
}

func (repo *fakeRepository) CreateArtifactFeedback(feedback *model.ArtifactFeedback) error {
	repo.feedback = *feedback
	return nil
}
