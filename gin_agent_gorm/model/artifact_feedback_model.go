package model

import "gorm.io/gorm"

// ArtifactFeedback 记录用户对产物或产物版本的选择、下载、评分和差评原因。
type ArtifactFeedback struct {
	BaseModel
	ArtifactID        uint   `gorm:"column:artifact_id;index;not null" json:"artifact_id"`
	ArtifactVersionID uint   `gorm:"column:artifact_version_id;index;not null;default:0" json:"artifact_version_id"`
	UserID            uint   `gorm:"column:user_id;index;not null" json:"user_id"`
	FeedbackType      string `gorm:"column:feedback_type;size:64;not null" json:"feedback_type"`
	Rating            int    `gorm:"column:rating;not null;default:0" json:"rating"`
	Comment           string `gorm:"column:comment;type:text" json:"comment"`
	CommonTimestampsField
}

// TableName 返回产物反馈表名。
func (ArtifactFeedback) TableName() string {
	return "artifact_feedback"
}

// BeforeCreate 创建产物反馈前写入时间戳。
func (m *ArtifactFeedback) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 更新产物反馈前刷新时间戳。
func (m *ArtifactFeedback) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
