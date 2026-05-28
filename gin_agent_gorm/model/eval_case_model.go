package model

import "gorm.io/gorm"

// EvalCase stores a stable evaluation input for prompt/version governance.
type EvalCase struct {
	BaseModel
	AgentName    string  `gorm:"column:agent_name;size:128;index;not null" json:"agent_name"`
	Name         string  `gorm:"column:name;size:255;not null" json:"name"`
	InputJSON    string  `gorm:"column:input_json;type:text;not null" json:"input_json"`
	ExpectedJSON string  `gorm:"column:expected_json;type:text" json:"expected_json"`
	TagsJSON     string  `gorm:"column:tags_json;type:text" json:"tags_json"`
	Status       string  `gorm:"column:status;size:32;index;not null;default:active" json:"status"`
	Weight       float64 `gorm:"column:weight;type:decimal(8,4);not null;default:1" json:"weight"`
	CommonTimestampsField
}

func (EvalCase) TableName() string {
	return "eval_cases"
}

func (m *EvalCase) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

func (m *EvalCase) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
