package artifact

import (
	"errors"
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

type fakeRepository struct {
	artifact             model.Artifact
	createdArtifact      bool
	createdVersion       bool
	feedback             model.ArtifactFeedback
	findUserID           uint
	findArtifactID       uint
	updatedArtifactID    uint
	updatedArtifactAttrs map[string]interface{}
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
	return nil
}

func (repo *fakeRepository) ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	return []model.ArtifactVersion{}, nil
}

func (repo *fakeRepository) CreateArtifactFeedback(feedback *model.ArtifactFeedback) error {
	repo.feedback = *feedback
	return nil
}
