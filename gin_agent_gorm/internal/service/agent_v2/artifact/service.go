package artifact

import (
	"errors"

	"gin-biz-web-api/model"
)

// Repository 定义产物服务所需的持久化操作接口。
type Repository interface {
	CreateArtifact(artifact *model.Artifact) error
	FindArtifact(userID uint, artifactID uint) (model.Artifact, error)
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
	CreateArtifactVersion(version *model.ArtifactVersion) error
	ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error)
	CreateArtifactFeedback(feedback *model.ArtifactFeedback) error
}

// Service 负责所有 V2 产物的访问和版本创建。
type Service struct {
	repo Repository
}

// CreateArtifactWithVersionInput 包含一个逻辑产物及其第一个版本。
type CreateArtifactWithVersionInput struct {
	Artifact model.Artifact
	Version  model.ArtifactVersion
}

// NewService 创建产物服务实例。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateArtifactWithVersion 创建一个产物和至少一个版本。
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

// ListArtifacts 列出用户和会话范围内的产物。
func (svc *Service) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	return svc.repo.ListArtifacts(userID, conversationID)
}

// ListVersions 在仓库层验证所有权后列出版本。
func (svc *Service) ListVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	return svc.repo.ListArtifactVersions(userID, artifactID)
}

// AuthorizeDownload 通过所有权验证解析可下载的产物。
func (svc *Service) AuthorizeDownload(userID uint, artifactID uint) (model.Artifact, error) {
	return svc.repo.FindArtifact(userID, artifactID)
}

// RecordFeedback 写入用户对产物或产物版本的反馈。
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
