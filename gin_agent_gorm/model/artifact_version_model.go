package model

import "gorm.io/gorm"

// ArtifactVersion 记录图片、HTML 等产物的每一次生成、编辑或放大版本。
type ArtifactVersion struct {
	BaseModel
	ArtifactID       uint   `gorm:"column:artifact_id;index;not null" json:"artifact_id"`
	ParentVersionID  uint   `gorm:"column:parent_version_id;index;not null;default:0" json:"parent_version_id"`
	AgentRunID       uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`
	VersionNo        int    `gorm:"column:version_no;not null" json:"version_no"`
	Operation        string `gorm:"column:operation;size:64;not null" json:"operation"`
	Prompt           string `gorm:"column:prompt;type:text" json:"prompt"`
	NegativePrompt   string `gorm:"column:negative_prompt;type:text" json:"negative_prompt"`
	ModelProvider    string `gorm:"column:model_provider;size:128" json:"model_provider"`
	ModelName        string `gorm:"column:model_name;size:128" json:"model_name"`
	GenerationParams string `gorm:"column:generation_params;type:text" json:"generation_params"`
	SourceRefs       string `gorm:"column:source_refs;type:text" json:"source_refs"`
	QualityScores    string `gorm:"column:quality_scores;type:text" json:"quality_scores"`
	ObjectKey        string `gorm:"column:object_key;size:512;not null" json:"object_key"`
	PreviewURL       string `gorm:"column:preview_url;size:512" json:"preview_url"`
	Hash             string `gorm:"column:hash;size:128" json:"hash"`
	CommonTimestampsField
}

// TableName 返回产物版本表名。
func (ArtifactVersion) TableName() string {
	return "artifact_versions"
}

// BeforeCreate 创建产物版本前写入时间戳。
func (m *ArtifactVersion) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 更新产物版本前刷新时间戳。
func (m *ArtifactVersion) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
