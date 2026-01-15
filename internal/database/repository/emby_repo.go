// Package repository Emby 用户数据仓库
package repository

import (
	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"gorm.io/gorm"
)

// EmbyRepository Emby 用户仓库
type EmbyRepository struct {
	db *gorm.DB
}

// NewEmbyRepository 创建 Emby 用户仓库
func NewEmbyRepository() *EmbyRepository {
	return &EmbyRepository{db: database.GetDB()}
}

// Create 创建用户记录
func (r *EmbyRepository) Create(emby *models.Emby) error {
	return r.db.Create(emby).Error
}

// GetByTG 根据 TG ID 获取用户
func (r *EmbyRepository) GetByTG(tg int64) (*models.Emby, error) {
	var emby models.Emby
	err := r.db.Where("tg = ?", tg).First(&emby).Error
	if err != nil {
		return nil, err
	}
	return &emby, nil
}

// GetByEmbyID 根据 Emby ID 获取用户
func (r *EmbyRepository) GetByEmbyID(embyID string) (*models.Emby, error) {
	var emby models.Emby
	err := r.db.Where("embyid = ?", embyID).First(&emby).Error
	if err != nil {
		return nil, err
	}
	return &emby, nil
}

// GetByName 根据用户名获取用户
func (r *EmbyRepository) GetByName(name string) (*models.Emby, error) {
	var emby models.Emby
	err := r.db.Where("name = ?", name).First(&emby).Error
	if err != nil {
		return nil, err
	}
	return &emby, nil
}

// GetByAny 根据 TG、EmbyID 或 Name 获取用户
func (r *EmbyRepository) GetByAny(key interface{}) (*models.Emby, error) {
	var emby models.Emby
	err := r.db.Where("tg = ? OR embyid = ? OR name = ?", key, key, key).First(&emby).Error
	if err != nil {
		return nil, err
	}
	return &emby, nil
}

// Update 更新用户
func (r *EmbyRepository) Update(emby *models.Emby) error {
	return r.db.Save(emby).Error
}

// UpdateFields 更新指定字段
func (r *EmbyRepository) UpdateFields(tg int64, updates map[string]interface{}) error {
	return r.db.Model(&models.Emby{}).Where("tg = ?", tg).Updates(updates).Error
}

// Delete 删除用户
func (r *EmbyRepository) Delete(tg int64) error {
	return r.db.Delete(&models.Emby{}, "tg = ?", tg).Error
}

// DeleteByEmbyID 根据 Emby ID 删除用户
func (r *EmbyRepository) DeleteByEmbyID(embyID string) error {
	return r.db.Delete(&models.Emby{}, "embyid = ?", embyID).Error
}

// GetAll 获取所有用户
func (r *EmbyRepository) GetAll() ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Find(&embies).Error
	return embies, err
}

// GetByLevel 根据等级获取用户
func (r *EmbyRepository) GetByLevel(level models.UserLevel) ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where("lv = ?", level).Find(&embies).Error
	return embies, err
}

// GetActiveUsers 获取有 Emby 账户的用户
func (r *EmbyRepository) GetActiveUsers() ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where("embyid IS NOT NULL AND embyid != ''").Find(&embies).Error
	return embies, err
}

// GetExpiredUsers 获取已过期的用户
func (r *EmbyRepository) GetExpiredUsers() ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where("ex IS NOT NULL AND ex < NOW() AND lv != ?", models.LevelE).Find(&embies).Error
	return embies, err
}

// GetNonBannedUsers 获取未封禁的用户
func (r *EmbyRepository) GetNonBannedUsers() ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where("lv != ? AND embyid IS NOT NULL", models.LevelE).Find(&embies).Error
	return embies, err
}

// CountStats 统计用户数据
func (r *EmbyRepository) CountStats() (total int64, withEmby int64, whitelist int64, err error) {
	// 总用户数
	err = r.db.Model(&models.Emby{}).Count(&total).Error
	if err != nil {
		return
	}

	// 有 Emby 账户的用户数
	err = r.db.Model(&models.Emby{}).Where("embyid IS NOT NULL AND embyid != ''").Count(&withEmby).Error
	if err != nil {
		return
	}

	// 白名单用户数
	err = r.db.Model(&models.Emby{}).Where("lv = ?", models.LevelA).Count(&whitelist).Error
	return
}

// BatchUpdateIV 批量更新邀请次数
func (r *EmbyRepository) BatchUpdateIV(updates []struct {
	TG int64
	Iv int
}) error {
	tx := r.db.Begin()
	for _, u := range updates {
		if err := tx.Model(&models.Emby{}).Where("tg = ?", u.TG).Update("iv", u.Iv).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ClearAllIV 清空所有用户的邀请次数
func (r *EmbyRepository) ClearAllIV() error {
	return r.db.Model(&models.Emby{}).Where("1 = 1").Update("iv", 0).Error
}

// Exists 检查用户是否存在
func (r *EmbyRepository) Exists(tg int64) bool {
	var count int64
	r.db.Model(&models.Emby{}).Where("tg = ?", tg).Count(&count)
	return count > 0
}

// EnsureExists 确保用户存在，不存在则创建
func (r *EmbyRepository) EnsureExists(tg int64) (*models.Emby, error) {
	emby, err := r.GetByTG(tg)
	if err == nil {
		return emby, nil
	}

	// 不存在则创建
	newEmby := &models.Emby{
		TG: tg,
		Lv: models.LevelD,
	}
	if err := r.Create(newEmby); err != nil {
		return nil, err
	}
	return newEmby, nil
}

// GetTopByScore 获取积分排行榜
func (r *EmbyRepository) GetTopByScore(limit int) ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where("us > 0").Order("us DESC").Limit(limit).Find(&embies).Error
	return embies, err
}

// GetUsersExpiringInDays 获取指定天数内过期的用户
func (r *EmbyRepository) GetUsersExpiringInDays(days int) ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where(
		"ex IS NOT NULL AND ex > NOW() AND ex < DATE_ADD(NOW(), INTERVAL ? DAY) AND lv != ?",
		days, models.LevelE,
	).Find(&embies).Error
	return embies, err
}

// GetInactiveUsers 获取超过指定天数未活跃的用户
func (r *EmbyRepository) GetInactiveUsers(inactiveDays int) ([]models.Emby, error) {
	var embies []models.Emby
	err := r.db.Where(
		"embyid IS NOT NULL AND embyid != '' AND lv != ? AND (ch IS NULL OR ch < DATE_SUB(NOW(), INTERVAL ? DAY))",
		models.LevelE, inactiveDays,
	).Find(&embies).Error
	return embies, err
}
