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

type fakeRepository struct {
	artifact        model.Artifact
	createdArtifact bool
	createdVersion  bool
	findUserID      uint
	findArtifactID  uint
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
	return nil
}
