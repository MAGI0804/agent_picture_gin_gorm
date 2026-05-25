package artifact

import (
	"errors"

	"gin-biz-web-api/model"
)

// Repository defines the persistence operations required by Artifact Service.
type Repository interface {
	CreateArtifact(artifact *model.Artifact) error
	FindArtifact(userID uint, artifactID uint) (model.Artifact, error)
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
	CreateArtifactVersion(version *model.ArtifactVersion) error
	ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error)
	CreateArtifactFeedback(feedback *model.ArtifactFeedback) error
}

// Service owns all V2 artifact access and version creation.
type Service struct {
	repo Repository
}

// CreateArtifactWithVersionInput contains one logical artifact and its first version.
type CreateArtifactWithVersionInput struct {
	Artifact model.Artifact
	Version  model.ArtifactVersion
}

// NewService creates an Artifact Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateArtifactWithVersion creates an artifact and at least one version in order.
func (svc *Service) CreateArtifactWithVersion(
	input CreateArtifactWithVersionInput,
) (model.Artifact, model.ArtifactVersion, error) {
	if input.Artifact.UserID == 0 {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("artifact user_id is required")
	}
	if input.Artifact.ConversationID == 0 {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("artifact conversation_id is required")
	}
	if input.Artifact.ObjectKey == "" {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("artifact object_key is required")
	}
	if input.Version.ObjectKey == "" {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("artifact version object_key is required")
	}
	if input.Version.VersionNo == 0 {
		input.Version.VersionNo = 1
	}

	artifact := input.Artifact
	if err := svc.repo.CreateArtifact(&artifact); err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}

	version := input.Version
	version.ArtifactID = artifact.ID
	if version.AgentRunID == 0 {
		version.AgentRunID = artifact.AgentRunID
	}
	if err := svc.repo.CreateArtifactVersion(&version); err != nil {
		return artifact, model.ArtifactVersion{}, err
	}
	return artifact, version, nil
}

// ListArtifacts lists artifacts after user and conversation scoping.
func (svc *Service) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	return svc.repo.ListArtifacts(userID, conversationID)
}

// ListVersions lists versions after artifact ownership validation in the repository.
func (svc *Service) ListVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	return svc.repo.ListArtifactVersions(userID, artifactID)
}

// AuthorizeDownload resolves a downloadable artifact through ownership validation.
func (svc *Service) AuthorizeDownload(userID uint, artifactID uint) (model.Artifact, error) {
	return svc.repo.FindArtifact(userID, artifactID)
}

// RecordFeedback writes user feedback for an artifact or artifact version.
func (svc *Service) RecordFeedback(feedback model.ArtifactFeedback) error {
	if feedback.UserID == 0 {
		return errors.New("feedback user_id is required")
	}
	if feedback.ArtifactID == 0 {
		return errors.New("feedback artifact_id is required")
	}
	if feedback.FeedbackType == "" {
		return errors.New("feedback_type is required")
	}
	return svc.repo.CreateArtifactFeedback(&feedback)
}
