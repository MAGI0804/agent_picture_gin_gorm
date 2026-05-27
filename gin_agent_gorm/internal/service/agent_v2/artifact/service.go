package artifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/model"
)

const (
	FeedbackTypeSelected = "selected"
)

// Repository 定义产物服务所需的持久化操作接口。
type Repository interface {
	CreateArtifact(artifact *model.Artifact) error
	FindArtifact(userID uint, artifactID uint) (model.Artifact, error)
	UpdateArtifact(artifactID uint, attrs map[string]interface{}) error
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
	CreateArtifactVersion(version *model.ArtifactVersion) error
	UpdateArtifactVersion(artifactID uint, versionID uint, attrs map[string]interface{}) error
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

// CreateCandidateGroupInput contains a set of generated candidates for one run.
type CreateCandidateGroupInput struct {
	AgentRunID      uint
	UserID          uint
	ConversationID  uint
	ArtifactGroupID string
	Artifacts       []CreateArtifactWithVersionInput
}

// SelectArtifactInput records a user's selected candidate.
type SelectArtifactInput struct {
	UserID            uint
	ArtifactID        uint
	ArtifactVersionID uint
}

// ReviewScoresInput records the review score for one artifact version.
type ReviewScoresInput struct {
	UserID           uint
	ArtifactID       uint
	VersionID        uint
	OverallScore     float64
	RequirementMatch float64
	CompositionScore float64
	TextReadability  float64
	LayoutScore      float64
	RankScore        float64
	Issues           []string
	ShouldRefine     bool
	Reviewer         string
	ExtractedText    string
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

// CreateCandidateGroup creates multiple artifact candidates under one group ID.
func (svc *Service) CreateCandidateGroup(
	input CreateCandidateGroupInput,
) ([]model.Artifact, []model.ArtifactVersion, error) {
	if input.UserID == 0 {
		return nil, nil, errors.New("candidate group user_id is required")
	}
	if input.ConversationID == 0 {
		return nil, nil, errors.New("candidate group conversation_id is required")
	}
	if len(input.Artifacts) == 0 {
		return nil, nil, errors.New("candidate group requires at least one artifact")
	}
	groupID := input.ArtifactGroupID
	if groupID == "" {
		groupID = fmt.Sprintf("run-%d-candidates-%d", input.AgentRunID, time.Now().UnixNano())
	}

	artifacts := make([]model.Artifact, 0, len(input.Artifacts))
	versions := make([]model.ArtifactVersion, 0, len(input.Artifacts))
	for _, candidate := range input.Artifacts {
		candidate.Artifact.UserID = input.UserID
		candidate.Artifact.ConversationID = input.ConversationID
		candidate.Artifact.AgentRunID = input.AgentRunID
		candidate.Artifact.ArtifactGroupID = groupID
		if candidate.Version.AgentRunID == 0 {
			candidate.Version.AgentRunID = input.AgentRunID
		}

		artifact, version, err := svc.CreateArtifactWithVersion(candidate)
		if err != nil {
			return artifacts, versions, err
		}
		artifacts = append(artifacts, artifact)
		versions = append(versions, version)
	}
	return artifacts, versions, nil
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

// SelectArtifact marks an artifact as the selected candidate and records feedback.
func (svc *Service) SelectArtifact(input SelectArtifactInput) error {
	if input.UserID == 0 {
		return errors.New("select artifact user_id is required")
	}
	if input.ArtifactID == 0 {
		return errors.New("select artifact artifact_id is required")
	}
	if _, err := svc.repo.FindArtifact(input.UserID, input.ArtifactID); err != nil {
		return err
	}
	if err := svc.repo.UpdateArtifact(input.ArtifactID, map[string]interface{}{
		"selected_at": int(time.Now().Unix()),
	}); err != nil {
		return err
	}
	return svc.RecordFeedback(model.ArtifactFeedback{
		ArtifactID:        input.ArtifactID,
		ArtifactVersionID: input.ArtifactVersionID,
		UserID:            input.UserID,
		FeedbackType:      FeedbackTypeSelected,
	})
}

// RecordReviewScores stores structured review output on the artifact version.
func (svc *Service) RecordReviewScores(input ReviewScoresInput) error {
	if input.UserID == 0 {
		return errors.New("review score user_id is required")
	}
	if input.ArtifactID == 0 {
		return errors.New("review score artifact_id is required")
	}
	if input.VersionID == 0 {
		return errors.New("review score version_id is required")
	}
	if _, err := svc.repo.FindArtifact(input.UserID, input.ArtifactID); err != nil {
		return err
	}
	rankScore := input.RankScore
	if rankScore == 0 {
		rankScore = input.OverallScore
	}
	payload := map[string]interface{}{
		"overall_score":     input.OverallScore,
		"requirement_match": input.RequirementMatch,
		"composition_score": input.CompositionScore,
		"text_readability":  input.TextReadability,
		"layout_score":      input.LayoutScore,
		"rank_score":        rankScore,
		"issues":            input.Issues,
		"should_refine":     input.ShouldRefine,
		"reviewer":          coalesceReviewer(input.Reviewer),
		"reviewed_at":       time.Now().Unix(),
	}
	if input.ExtractedText != "" {
		payload["extracted_text"] = input.ExtractedText
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := svc.repo.UpdateArtifactVersion(input.ArtifactID, input.VersionID, map[string]interface{}{
		"quality_scores": string(data),
	}); err != nil {
		return err
	}
	return svc.repo.UpdateArtifact(input.ArtifactID, map[string]interface{}{
		"rank_score": rankScore,
	})
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

func coalesceReviewer(value string) string {
	if value == "" {
		return "vision_review"
	}
	return value
}
