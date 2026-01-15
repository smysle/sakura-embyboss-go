// Package repository 红包数据仓库
package repository

import (
	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"gorm.io/gorm"
)

// RedEnvelopeRepository 红包仓库
type RedEnvelopeRepository struct {
	db *gorm.DB
}

// NewRedEnvelopeRepository 创建红包仓库
func NewRedEnvelopeRepository() *RedEnvelopeRepository {
	return &RedEnvelopeRepository{db: database.GetDB()}
}

// Create 创建红包
func (r *RedEnvelopeRepository) Create(envelope *models.RedEnvelope) error {
	return r.db.Create(envelope).Error
}

// GetByUUID 根据 UUID 获取红包
func (r *RedEnvelopeRepository) GetByUUID(uuid string) (*models.RedEnvelope, error) {
	var envelope models.RedEnvelope
	err := r.db.Where("uuid = ?", uuid).First(&envelope).Error
	if err != nil {
		return nil, err
	}
	return &envelope, nil
}

// Update 更新红包
func (r *RedEnvelopeRepository) Update(envelope *models.RedEnvelope) error {
	return r.db.Save(envelope).Error
}

// UpdateRemain 更新剩余金额和数量（原子操作）
func (r *RedEnvelopeRepository) UpdateRemain(uuid string, amount int) error {
	return r.db.Model(&models.RedEnvelope{}).
		Where("uuid = ? AND remain_count > 0 AND remain_amount >= ?", uuid, amount).
		Updates(map[string]interface{}{
			"remain_amount": gorm.Expr("remain_amount - ?", amount),
			"remain_count":  gorm.Expr("remain_count - 1"),
		}).Error
}

// SetFinished 设置红包为已完成
func (r *RedEnvelopeRepository) SetFinished(uuid string) error {
	return r.db.Model(&models.RedEnvelope{}).
		Where("uuid = ?", uuid).
		Update("status", "finished").Error
}

// SetExpired 设置红包为已过期
func (r *RedEnvelopeRepository) SetExpired(uuid string) error {
	return r.db.Model(&models.RedEnvelope{}).
		Where("uuid = ?", uuid).
		Update("status", "expired").Error
}

// CreateRecord 创建领取记录
func (r *RedEnvelopeRepository) CreateRecord(record *models.RedEnvelopeRecord) error {
	return r.db.Create(record).Error
}

// GetRecordsByEnvelope 获取红包的所有领取记录
func (r *RedEnvelopeRepository) GetRecordsByEnvelope(uuid string) ([]models.RedEnvelopeRecord, error) {
	var records []models.RedEnvelopeRecord
	err := r.db.Where("envelope_uuid = ?", uuid).Order("created_at ASC").Find(&records).Error
	return records, err
}

// HasReceived 检查用户是否已领取
func (r *RedEnvelopeRepository) HasReceived(uuid string, tgID int64) bool {
	var count int64
	r.db.Model(&models.RedEnvelopeRecord{}).
		Where("envelope_uuid = ? AND receiver_tg = ?", uuid, tgID).
		Count(&count)
	return count > 0
}

// GetLuckyRecord 获取手气最佳记录
func (r *RedEnvelopeRepository) GetLuckyRecord(uuid string) (*models.RedEnvelopeRecord, error) {
	var record models.RedEnvelopeRecord
	err := r.db.Where("envelope_uuid = ?", uuid).Order("amount DESC").First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetExpiredEnvelopes 获取过期的红包
func (r *RedEnvelopeRepository) GetExpiredEnvelopes() ([]models.RedEnvelope, error) {
	var envelopes []models.RedEnvelope
	err := r.db.Where("status = 'active' AND expired_at < NOW()").Find(&envelopes).Error
	return envelopes, err
}
