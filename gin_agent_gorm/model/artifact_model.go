package model

import "gorm.io/gorm"

// Artifact 表示图片、HTML 等生成产物的元数据。
type Artifact struct {
	BaseModel
	ConversationID uint   `gorm:"column:conversation_id;index;not null" json:"conversation_id"` // 所属会话 ID。
	UserID         uint   `gorm:"column:user_id;index;not null" json:"user_id"`                 // 所属用户 ID。
	AgentRunID     uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`       // 生成该产物的 Agent Run ID。
	Name           string `gorm:"column:name;size:255;not null" json:"name"`                    // 文件名。
	Kind           string `gorm:"column:kind;size:64;not null" json:"kind"`                     // 产物类型：image、html 等。
	MimeType       string `gorm:"column:mime_type;size:128;not null" json:"mime_type"`          // MIME 类型。
	ObjectKey      string `gorm:"column:object_key;size:512;not null" json:"object_key"`        // 对象存储 key。
	PreviewURL     string `gorm:"column:preview_url;size:512" json:"preview_url"`               // 前端预览地址。
	SizeBytes      int64  `gorm:"column:size_bytes;not null;default:0" json:"size_bytes"`       // 文件大小。
	Hash           string `gorm:"column:hash;size:128" json:"hash"`                             // 文件 hash。
	CommonTimestampsField
}

// TableName 返回产物表名。
func (Artifact) TableName() string {
	return "artifacts"
}

// BeforeCreate 在创建产物元数据前写入时间戳。
func (m *Artifact) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新产物元数据前刷新更新时间。
func (m *Artifact) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
