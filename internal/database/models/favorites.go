// Package models 数据模型 - 收藏记录
package models

import (
	"time"
)

// Favorites 收藏记录表
type Favorites struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TG        int64     `gorm:"column:tg;index" json:"tg"`
	EmbyID    string    `gorm:"column:emby_id;size:255" json:"emby_id"`
	ItemID    string    `gorm:"column:item_id;size:255" json:"item_id"`
	ItemName  string    `gorm:"column:item_name;size:500" json:"item_name"`
	ItemType  string    `gorm:"column:item_type;size:50" json:"item_type"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 表名
func (Favorites) TableName() string {
	return "favorites"
}

// RequestRecord 请求记录表
type RequestRecord struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TG        int64     `gorm:"column:tg;index" json:"tg"`
	Type      string    `gorm:"column:type;size:50" json:"type"`       // movie, series
	TMDBID    string    `gorm:"column:tmdb_id;size:50" json:"tmdb_id"`
	Title     string    `gorm:"column:title;size:500" json:"title"`
	Status    string    `gorm:"column:status;size:50;default:'pending'" json:"status"` // pending, approved, rejected, completed
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 表名
func (RequestRecord) TableName() string {
	return "request_records"
}
