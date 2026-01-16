// Package repository 收藏数据仓库
package repository

import (
	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"gorm.io/gorm"
)

// FavoritesRepository 收藏仓库
type FavoritesRepository struct {
	db *gorm.DB
}

// NewFavoritesRepository 创建收藏仓库
func NewFavoritesRepository() *FavoritesRepository {
	return &FavoritesRepository{db: database.GetDB()}
}

// Create 创建收藏记录
func (r *FavoritesRepository) Create(fav *models.Favorites) error {
	return r.db.Create(fav).Error
}

// BatchCreate 批量创建收藏记录
func (r *FavoritesRepository) BatchCreate(favs []models.Favorites) error {
	if len(favs) == 0 {
		return nil
	}
	return r.db.CreateInBatches(favs, 100).Error
}

// GetByEmbyID 根据 EmbyID 获取收藏列表
func (r *FavoritesRepository) GetByEmbyID(embyID string, page, pageSize int) ([]models.Favorites, int64, error) {
	var favs []models.Favorites
	var total int64

	query := r.db.Model(&models.Favorites{}).Where("embyid = ?", embyID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&favs).Error
	return favs, total, err
}

// GetByEmbyName 根据 EmbyName 获取收藏列表
func (r *FavoritesRepository) GetByEmbyName(embyName string, page, pageSize int) ([]models.Favorites, int64, error) {
	var favs []models.Favorites
	var total int64

	query := r.db.Model(&models.Favorites{}).Where("embyname = ?", embyName)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&favs).Error
	return favs, total, err
}

// ClearByEmbyID 清除用户的所有收藏记录
func (r *FavoritesRepository) ClearByEmbyID(embyID string) error {
	return r.db.Where("embyid = ?", embyID).Delete(&models.Favorites{}).Error
}

// ClearByEmbyName 清除用户的所有收藏记录（按名称）
func (r *FavoritesRepository) ClearByEmbyName(embyName string) error {
	return r.db.Where("embyname = ?", embyName).Delete(&models.Favorites{}).Error
}

// SyncUserFavorites 同步用户收藏（清空旧记录后插入新记录）
func (r *FavoritesRepository) SyncUserFavorites(embyID, embyName string, items []FavoriteItemInfo) error {
	tx := r.db.Begin()

	// 清空旧记录
	if err := tx.Where("embyid = ?", embyID).Delete(&models.Favorites{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 插入新记录
	if len(items) > 0 {
		var favs []models.Favorites
		for _, item := range items {
			favs = append(favs, models.Favorites{
				EmbyID:   embyID,
				EmbyName: embyName,
				ItemID:   item.ItemID,
				ItemName: item.ItemName,
				ItemType: item.ItemType,
			})
		}

		if err := tx.CreateInBatches(favs, 100).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// FavoriteItemInfo 收藏项信息（用于同步）
type FavoriteItemInfo struct {
	ItemID   string
	ItemName string
	ItemType string
}

// CountByEmbyID 统计用户收藏数量
func (r *FavoritesRepository) CountByEmbyID(embyID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Favorites{}).Where("embyid = ?", embyID).Count(&count).Error
	return count, err
}

// GetAll 获取所有收藏记录
func (r *FavoritesRepository) GetAll() ([]models.Favorites, error) {
	var favs []models.Favorites
	err := r.db.Find(&favs).Error
	return favs, err
}
