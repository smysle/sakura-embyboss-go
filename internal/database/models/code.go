// Package models 数据模型 - 注册码
package models

import (
	"time"
)

// Code 注册码表
type Code struct {
	Code     string     `gorm:"column:code;primaryKey;size:50" json:"code"`
	TG       int64      `gorm:"column:tg;index" json:"tg"`
	Us       int        `gorm:"column:us" json:"us"`                  // 有效天数
	Used     *int64     `gorm:"column:used" json:"used,omitempty"`    // 使用者 TG ID
	UsedTime *time.Time `gorm:"column:usedtime" json:"used_time,omitempty"` // 使用时间
}

// TableName 表名
func (Code) TableName() string {
	return "Rcode"
}

// IsUsed 是否已使用
func (c *Code) IsUsed() bool {
	return c.Used != nil
}
