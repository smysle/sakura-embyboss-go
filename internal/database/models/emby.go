// Package models 数据模型 - Emby 用户
package models

import (
	"time"
)

// UserLevel 用户等级
type UserLevel string

const (
	LevelD UserLevel = "d" // 普通用户
	LevelC UserLevel = "c" // 普通用户
	LevelB UserLevel = "b" // 普通用户
	LevelA UserLevel = "a" // 白名单用户
	LevelE UserLevel = "e" // 封禁用户
)

// Emby 用户表
type Emby struct {
	TG      int64      `gorm:"column:tg;primaryKey;autoIncrement:false" json:"tg"`
	EmbyID  *string    `gorm:"column:embyid;size:255" json:"emby_id,omitempty"`
	Name    *string    `gorm:"column:name;size:255" json:"name,omitempty"`
	Pwd     *string    `gorm:"column:pwd;size:255" json:"pwd,omitempty"`
	Pwd2    *string    `gorm:"column:pwd2;size:255" json:"pwd2,omitempty"`
	Lv      UserLevel  `gorm:"column:lv;size:1;default:'d'" json:"lv"`
	Cr      *time.Time `gorm:"column:cr" json:"cr,omitempty"`         // 创建时间
	Ex      *time.Time `gorm:"column:ex" json:"ex,omitempty"`         // 过期时间
	Us      int        `gorm:"column:us;default:0" json:"us"`         // 积分
	Iv      int        `gorm:"column:iv;default:0" json:"iv"`         // 邀请次数
	Ch      *time.Time `gorm:"column:ch" json:"ch,omitempty"`         // 签到时间
	Ck      int        `gorm:"column:ck;default:0" json:"ck"`         // 连续签到天数
}

// TableName 表名
func (Emby) TableName() string {
	return "emby"
}

// HasEmbyAccount 是否有 Emby 账户
func (e *Emby) HasEmbyAccount() bool {
	return e.EmbyID != nil && *e.EmbyID != ""
}

// IsExpired 是否已过期
func (e *Emby) IsExpired() bool {
	if e.Ex == nil {
		return false
	}
	return time.Now().After(*e.Ex)
}

// IsBanned 是否被封禁
func (e *Emby) IsBanned() bool {
	return e.Lv == LevelE
}

// IsWhitelist 是否是白名单用户
func (e *Emby) IsWhitelist() bool {
	return e.Lv == LevelA
}

// GetLevelName 获取等级名称
func (e *Emby) GetLevelName() string {
	switch e.Lv {
	case LevelA:
		return "🌟 白名单用户"
	case LevelB:
		return "🔮 高级用户"
	case LevelC:
		return "💎 普通用户"
	case LevelD:
		return "🎫 基础用户"
	case LevelE:
		return "🚫 已封禁"
	default:
		return "❓ 未知"
	}
}

// DaysUntilExpiry 距离过期还有多少天
func (e *Emby) DaysUntilExpiry() int {
	if e.Ex == nil {
		return -1
	}
	duration := time.Until(*e.Ex)
	return int(duration.Hours() / 24)
}
