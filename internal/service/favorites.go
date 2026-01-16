// Package service 收藏同步服务
package service

import (
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// FavoritesService 收藏服务
type FavoritesService struct {
	embyRepo *repository.EmbyRepository
	favRepo  *repository.FavoritesRepository
	client   *emby.Client
}

// NewFavoritesService 创建收藏服务
func NewFavoritesService() *FavoritesService {
	return &FavoritesService{
		embyRepo: repository.NewEmbyRepository(),
		favRepo:  repository.NewFavoritesRepository(),
		client:   emby.GetClient(),
	}
}

// SyncResult 同步结果
type SyncResult struct {
	Users  int // 处理的用户数
	Items  int // 同步的收藏数
	Errors int // 错误数
}

// SyncAllUserFavorites 同步所有用户的收藏
func (s *FavoritesService) SyncAllUserFavorites() (*SyncResult, error) {
	result := &SyncResult{}

	// 获取所有有 Emby 账户的用户
	users, err := s.embyRepo.GetActiveUsers()
	if err != nil {
		return nil, err
	}

	result.Users = len(users)

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 从 Emby 获取收藏（最多获取 500 个）
		favorites, _, err := s.client.GetUserFavorites(*user.EmbyID, 0, 500)
		if err != nil {
			logger.Warn().
				Err(err).
				Int64("tg", user.TG).
				Str("embyID", *user.EmbyID).
				Msg("获取用户收藏失败")
			result.Errors++
			continue
		}

		// 转换为同步格式
		var items []repository.FavoriteItemInfo
		for _, fav := range favorites {
			items = append(items, repository.FavoriteItemInfo{
				ItemID:   fav.ID,
				ItemName: fav.Name,
				ItemType: fav.Type,
			})
		}

		// 同步到数据库
		embyName := ""
		if user.Name != nil {
			embyName = *user.Name
		}

		if err := s.favRepo.SyncUserFavorites(*user.EmbyID, embyName, items); err != nil {
			logger.Warn().
				Err(err).
				Int64("tg", user.TG).
				Msg("同步用户收藏到数据库失败")
			result.Errors++
			continue
		}

		result.Items += len(items)
	}

	return result, nil
}

// SyncUserFavorites 同步单个用户的收藏
func (s *FavoritesService) SyncUserFavorites(tgID int64) error {
	user, err := s.embyRepo.GetByTG(tgID)
	if err != nil {
		return err
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return nil
	}

	// 从 Emby 获取收藏
	favorites, _, err := s.client.GetUserFavorites(*user.EmbyID, 0, 500)
	if err != nil {
		return err
	}

	// 转换为同步格式
	var items []repository.FavoriteItemInfo
	for _, fav := range favorites {
		items = append(items, repository.FavoriteItemInfo{
			ItemID:   fav.ID,
			ItemName: fav.Name,
			ItemType: fav.Type,
		})
	}

	// 同步到数据库
	embyName := ""
	if user.Name != nil {
		embyName = *user.Name
	}

	return s.favRepo.SyncUserFavorites(*user.EmbyID, embyName, items)
}

// GetUserFavoritesFromDB 从数据库获取用户收藏（分页）
func (s *FavoritesService) GetUserFavoritesFromDB(embyID string, page, pageSize int) ([]repository.FavoriteItemInfo, int64, error) {
	favs, total, err := s.favRepo.GetByEmbyID(embyID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var items []repository.FavoriteItemInfo
	for _, fav := range favs {
		items = append(items, repository.FavoriteItemInfo{
			ItemID:   fav.ItemID,
			ItemName: fav.ItemName,
			ItemType: fav.ItemType,
		})
	}

	return items, total, nil
}
