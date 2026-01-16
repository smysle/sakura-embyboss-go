// Package repository 注册码数据仓库
package repository

import (
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"gorm.io/gorm"
)

// CodeRepository 注册码仓库
type CodeRepository struct {
	db *gorm.DB
}

// NewCodeRepository 创建注册码仓库
func NewCodeRepository() *CodeRepository {
	return &CodeRepository{db: database.GetDB()}
}

// Create 创建注册码
func (r *CodeRepository) Create(code *models.Code) error {
	return r.db.Create(code).Error
}

// BatchCreate 批量创建注册码
func (r *CodeRepository) BatchCreate(codes []string, tg int64, days int) error {
	var codeModels []models.Code
	for _, c := range codes {
		codeModels = append(codeModels, models.Code{
			Code: c,
			TG:   tg,
			Us:   days,
		})
	}
	return r.db.Create(&codeModels).Error
}

// GetByCode 根据注册码获取
func (r *CodeRepository) GetByCode(code string) (*models.Code, error) {
	var c models.Code
	err := r.db.Where("code = ?", code).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// MarkUsed 标记注册码已使用
func (r *CodeRepository) MarkUsed(code string, usedBy int64) error {
	now := time.Now()
	return r.db.Model(&models.Code{}).Where("code = ?", code).Updates(map[string]interface{}{
		"used":     usedBy,
		"usedtime": now,
	}).Error
}

// CountStats 统计注册码
func (r *CodeRepository) CountStats(tgID *int64) (*CodeStats, error) {
	stats := &CodeStats{}

	query := r.db.Model(&models.Code{})
	if tgID != nil {
		query = query.Where("tg = ?", *tgID)
	}

	// 已使用数量
	query.Where("used IS NOT NULL").Count(&stats.Used)

	// 未使用数量
	query = r.db.Model(&models.Code{})
	if tgID != nil {
		query = query.Where("tg = ?", *tgID)
	}
	query.Where("used IS NULL").Count(&stats.Unused)

	// 各期限未使用数量
	for _, days := range []int{30, 90, 180, 365} {
		var count int64
		q := r.db.Model(&models.Code{}).Where("used IS NULL AND us = ?", days)
		if tgID != nil {
			q = q.Where("tg = ?", *tgID)
		}
		q.Count(&count)

		switch days {
		case 30:
			stats.Mon = count
		case 90:
			stats.Sea = count
		case 180:
			stats.Half = count
		case 365:
			stats.Year = count
		}
	}

	return stats, nil
}

// CodeStats 注册码统计
type CodeStats struct {
	Used   int64 // 已使用
	Unused int64 // 未使用
	Mon    int64 // 30天
	Sea    int64 // 90天
	Half   int64 // 180天
	Year   int64 // 365天
}

// GetUnusedByTG 获取用户未使用的注册码
func (r *CodeRepository) GetUnusedByTG(tg int64, limit, offset int) ([]models.Code, error) {
	var codes []models.Code
	err := r.db.Where("tg = ? AND used IS NULL", tg).
		Order("us ASC").
		Limit(limit).
		Offset(offset).
		Find(&codes).Error
	return codes, err
}

// GetUsedByTG 获取用户已使用的注册码
func (r *CodeRepository) GetUsedByTG(tg int64, limit, offset int) ([]models.Code, error) {
	var codes []models.Code
	err := r.db.Where("tg = ? AND used IS NOT NULL", tg).
		Order("usedtime DESC").
		Limit(limit).
		Offset(offset).
		Find(&codes).Error
	return codes, err
}

// DeleteUnusedByDays 删除指定天数的未使用注册码
func (r *CodeRepository) DeleteUnusedByDays(days []int, tgID *int64) (int64, error) {
	query := r.db.Where("used IS NULL AND us IN ?", days)
	if tgID != nil {
		query = query.Where("tg = ?", *tgID)
	}
	result := query.Delete(&models.Code{})
	return result.RowsAffected, result.Error
}

// DeleteAllUnused 删除所有未使用的注册码
func (r *CodeRepository) DeleteAllUnused(tgID *int64) (int64, error) {
	query := r.db.Where("used IS NULL")
	if tgID != nil {
		query = query.Where("tg = ?", *tgID)
	}
	result := query.Delete(&models.Code{})
	return result.RowsAffected, result.Error
}

// ListWithPagination 分页获取注册码列表
func (r *CodeRepository) ListWithPagination(page, pageSize int, filter string) ([]CodeInfo, int64, error) {
	var codes []models.Code
	var total int64

	query := r.db.Model(&models.Code{})

	// 根据 filter 添加条件
	switch filter {
	case "used":
		query = query.Where("used IS NOT NULL")
	case "unused":
		query = query.Where("used IS NULL")
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&codes).Error
	if err != nil {
		return nil, 0, err
	}

	// 转换为 CodeInfo
	var infos []CodeInfo
	for _, c := range codes {
		infos = append(infos, CodeInfo{
			Code: c.Code,
			Days: c.Us,
			Used: c.Used != nil,
		})
	}

	return infos, total, nil
}

// CodeInfo 注册码信息（用于显示）
type CodeInfo struct {
	Code   string
	Days   int
	Used   bool
	UsedBy *int64
}

// GetByCreator 获取某用户创建的所有注册码
func (r *CodeRepository) GetByCreator(tgID int64) ([]models.Code, error) {
	var codes []models.Code
	err := r.db.Where("tg = ?", tgID).Order("id DESC").Find(&codes).Error
	return codes, err
}

// CreatorStats 创建者统计
type CreatorStats struct {
	Creator int64
	Total   int
	Used    int
}

// GetStatsByCreator 获取各管理员创建的注册码统计
func (r *CodeRepository) GetStatsByCreator() ([]CreatorStats, error) {
	var results []struct {
		Cr    int64
		Total int64
		Used  int64
	}

	err := r.db.Model(&models.Code{}).
		Select("cr, COUNT(*) as total, SUM(CASE WHEN used = true THEN 1 ELSE 0 END) as used").
		Group("cr").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	var stats []CreatorStats
	for _, r := range results {
		stats = append(stats, CreatorStats{
			Creator: r.Cr,
			Total:   int(r.Total),
			Used:    int(r.Used),
		})
	}

	return stats, nil
}
