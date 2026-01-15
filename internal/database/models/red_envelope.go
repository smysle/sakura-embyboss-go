// Package models 红包数据模型
package models

import (
	"time"
)

// RedEnvelope 红包表
type RedEnvelope struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UUID        string    `gorm:"column:uuid;size:36;uniqueIndex" json:"uuid"`      // 红包唯一标识
	SenderTG    int64     `gorm:"column:sender_tg;index" json:"sender_tg"`          // 发送者 TG ID
	SenderName  string    `gorm:"column:sender_name;size:255" json:"sender_name"`   // 发送者名称
	TotalAmount int       `gorm:"column:total_amount" json:"total_amount"`          // 总金额
	TotalCount  int       `gorm:"column:total_count" json:"total_count"`            // 总个数
	RemainAmount int      `gorm:"column:remain_amount" json:"remain_amount"`        // 剩余金额
	RemainCount int       `gorm:"column:remain_count" json:"remain_count"`          // 剩余个数
	Message     string    `gorm:"column:message;size:500" json:"message"`           // 祝福语
	Type        string    `gorm:"column:type;size:20;default:'random'" json:"type"` // 类型: random(拼手气), equal(均分)
	IsPrivate   bool      `gorm:"column:is_private;default:false" json:"is_private"`// 是否专属红包
	TargetTG    *int64    `gorm:"column:target_tg" json:"target_tg,omitempty"`      // 专属红包目标用户
	ChatID      int64     `gorm:"column:chat_id" json:"chat_id"`                    // 所在群组 ID
	MessageID   int       `gorm:"column:message_id" json:"message_id"`              // 消息 ID
	Status      string    `gorm:"column:status;size:20;default:'active'" json:"status"` // 状态: active, finished, expired
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	ExpiredAt   time.Time `gorm:"column:expired_at" json:"expired_at"`
}

// TableName 表名
func (RedEnvelope) TableName() string {
	return "red_envelopes"
}

// IsExpired 是否已过期
func (r *RedEnvelope) IsExpired() bool {
	return time.Now().After(r.ExpiredAt)
}

// IsFinished 是否已抢完
func (r *RedEnvelope) IsFinished() bool {
	return r.RemainCount <= 0 || r.RemainAmount <= 0
}

// RedEnvelopeRecord 红包领取记录
type RedEnvelopeRecord struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EnvelopeID    uint      `gorm:"column:envelope_id;index" json:"envelope_id"`       // 红包 ID
	EnvelopeUUID  string    `gorm:"column:envelope_uuid;size:36;index" json:"envelope_uuid"`
	ReceiverTG    int64     `gorm:"column:receiver_tg;index" json:"receiver_tg"`       // 领取者 TG ID
	ReceiverName  string    `gorm:"column:receiver_name;size:255" json:"receiver_name"`// 领取者名称
	Amount        int       `gorm:"column:amount" json:"amount"`                       // 领取金额
	IsLucky       bool      `gorm:"column:is_lucky;default:false" json:"is_lucky"`     // 是否手气最佳
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 表名
func (RedEnvelopeRecord) TableName() string {
	return "red_envelope_records"
}
