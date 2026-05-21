package model

import "time"

// setCreateTimestamps 写入模型创建时间和更新时间。
func setCreateTimestamps(timestamps *CommonTimestampsField) {
	now := int(time.Now().Unix())
	timestamps.CreatedAt = now
	timestamps.UpdatedAt = now
}

// setUpdateTimestamp 刷新模型更新时间。
func setUpdateTimestamp(timestamps *CommonTimestampsField) {
	timestamps.UpdatedAt = int(time.Now().Unix())
}
