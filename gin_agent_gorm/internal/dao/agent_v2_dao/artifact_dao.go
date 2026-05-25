package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateArtifact 创建产物元数据。
func (dao *AgentV2DAO) CreateArtifact(artifact *model.Artifact) error {
	return database.DB.Create(artifact).Error
}

// FindArtifact 查询用户有权访问的产物。
func (dao *AgentV2DAO) FindArtifact(userID uint, artifactID uint) (model.Artifact, error) {
	var artifact model.Artifact
	err := database.DB.Where("user_id = ? AND id = ?", userID, artifactID).First(&artifact).Error
	return artifact, err
}

// ListArtifacts 查询指定会话下当前用户有权访问的产物。
func (dao *AgentV2DAO) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	var artifacts []model.Artifact
	err := database.DB.Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("selected_at desc, rank_score desc, id desc").
		Find(&artifacts).Error
	return artifacts, err
}

// CreateArtifactVersion 写入产物版本记录，后续图片生成/编辑 Agent 会调用这里。
func (dao *AgentV2DAO) CreateArtifactVersion(version *model.ArtifactVersion) error {
	return database.DB.Create(version).Error
}

// ListArtifactVersions 读取用户有权访问产物的版本链。
func (dao *AgentV2DAO) ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	var versions []model.ArtifactVersion
	err := database.DB.Model(&model.ArtifactVersion{}).
		Joins("JOIN artifacts ON artifacts.id = artifact_versions.artifact_id").
		Where("artifacts.user_id = ? AND artifact_versions.artifact_id = ?", userID, artifactID).
		Order("artifact_versions.version_no asc, artifact_versions.id asc").
		Find(&versions).Error
	return versions, err
}

// CreateArtifactFeedback 写入用户对产物的选择、下载、评分或差评。
func (dao *AgentV2DAO) CreateArtifactFeedback(feedback *model.ArtifactFeedback) error {
	return database.DB.Create(feedback).Error
}
