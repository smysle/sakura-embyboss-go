// Package models 数据模型 - 收藏记录
package models

import (
	"time"
)

// Favorites 收藏记录表
type Favorites struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TG        int64     `gorm:"column:tg;index" json:"tg"`
	EmbyID    string    `gorm:"column:embyid;size:255;index" json:"embyid"`
	EmbyName  string    `gorm:"column:embyname;size:255" json:"embyname"`
	ItemID    string    `gorm:"column:item_id;size:255" json:"item_id"`
	ItemName  string    `gorm:"column:item_name;size:500" json:"item_name"`
	ItemType  string    `gorm:"column:item_type;size:50" json:"item_type"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 表名
func (Favorites) TableName() string {
	return "favorites"
}

// RequestRecord 请求记录表（MoviePilot 下载记录）
type RequestRecord struct {
	DownloadID    string     `gorm:"column:download_id;primaryKey;size:255" json:"download_id"`
	TG            int64      `gorm:"column:tg;not null;index" json:"tg"`
	RequestName   string     `gorm:"column:request_name;size:255;not null" json:"request_name"`
	Cost          string     `gorm:"column:cost;size:255;not null" json:"cost"`
	Detail        string     `gorm:"column:detail;type:text;not null" json:"detail"`
	LeftTime      string     `gorm:"column:left_time;size:255" json:"left_time"`
	DownloadState string     `gorm:"column:download_state;size:50;default:'pending'" json:"download_state"` // pending, downloading, completed, failed
	TransferState string     `gorm:"column:transfer_state;size:50" json:"transfer_state"`                   // success, failed
	Progress      float64    `gorm:"column:progress;default:0" json:"progress"`
	CreateAt      time.Time  `gorm:"column:create_at;autoCreateTime" json:"create_at"`
	UpdateAt      time.Time  `gorm:"column:update_at;autoUpdateTime" json:"update_at"`
}

// TableName 表名
func (RequestRecord) TableName() string {
	return "request_records"
}
