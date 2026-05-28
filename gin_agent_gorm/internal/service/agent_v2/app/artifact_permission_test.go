package app

import (
	"context"
	"errors"
	"testing"

	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/model"
)

func TestArtifactOperationsRejectNonOwner(t *testing.T) {
	svc := &Service{
		artifacts: artifactsvc.NewService(&denyingArtifactRepository{}),
	}

	if _, _, err := svc.DownloadArtifact(8, 3); err == nil {
		t.Fatal("DownloadArtifact() error = nil, want owner rejection")
	}
	if _, _, err := svc.PreviewArtifact(8, 3); err == nil {
		t.Fatal("PreviewArtifact() error = nil, want owner rejection")
	}
	if _, err := svc.EditArtifact(context.Background(), 8, 3, EditArtifactRequest{Prompt: "edit image"}); err == nil {
		t.Fatal("EditArtifact() error = nil, want owner rejection")
	}
	if err := svc.RecordArtifactFeedback(8, 3, ArtifactFeedbackRequest{FeedbackType: "negative"}); err == nil {
		t.Fatal("RecordArtifactFeedback() error = nil, want owner rejection")
	}
	if err := svc.SelectArtifact(8, 3, SelectArtifactRequest{}); err == nil {
		t.Fatal("SelectArtifact() error = nil, want owner rejection")
	}
}

type denyingArtifactRepository struct{}

func (repo *denyingArtifactRepository) CreateArtifact(artifact *model.Artifact) error {
	return nil
}

func (repo *denyingArtifactRepository) FindArtifact(userID uint, artifactID uint) (model.Artifact, error) {
	return model.Artifact{}, errors.New("artifact not found")
}

func (repo *denyingArtifactRepository) UpdateArtifact(artifactID uint, attrs map[string]interface{}) error {
	return nil
}

func (repo *denyingArtifactRepository) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	return nil, nil
}

func (repo *denyingArtifactRepository) CreateArtifactVersion(version *model.ArtifactVersion) error {
	return nil
}

func (repo *denyingArtifactRepository) UpdateArtifactVersion(artifactID uint, versionID uint, attrs map[string]interface{}) error {
	return nil
}

func (repo *denyingArtifactRepository) ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	return nil, nil
}

func (repo *denyingArtifactRepository) CreateArtifactFeedback(feedback *model.ArtifactFeedback) error {
	return nil
}
