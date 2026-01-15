// Package service 批量用户管理服务
package service

import (
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// BatchService 批量用户管理服务
type BatchService struct {
	embyRepo   *repository.EmbyRepository
	embyClient *emby.Client
	cfg        *config.Config
	bot        *tele.Bot
}

// BatchResult 批量操作结果
type BatchResult struct {
	Total   int      // 总数
	Success int      // 成功数
	Failed  int      // 失败数
	Skipped int      // 跳过数
	Details []string // 详细信息
}

// NewBatchService 创建批量用户管理服务
func NewBatchService() *BatchService {
	return &BatchService{
		embyRepo:   repository.NewEmbyRepository(),
		embyClient: emby.GetClient(),
		cfg:        config.Get(),
	}
}

// SetBot 设置 Bot 实例
func (s *BatchService) SetBot(bot *tele.Bot) {
	s.bot = bot
}

// SyncGroupMembers 同步群组成员（删除不在群组的用户）
func (s *BatchService) SyncGroupMembers(groupID int64, memberIDs []int64) (*BatchResult, error) {
	result := &BatchResult{
		Details: make([]string, 0),
	}

	// 获取所有等级 b 的用户
	users, err := s.embyRepo.GetByLevel(models.LevelB)
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Total = len(users)

	// 构建成员 ID 集合
	memberSet := make(map[int64]bool)
	for _, id := range memberIDs {
		memberSet[id] = true
	}

	for _, user := range users {
		// 检查是否在群组中
		if memberSet[user.TG] {
			result.Skipped++
			continue
		}

		// 不在群组中，删除 Emby 账户
		if user.EmbyID != nil && *user.EmbyID != "" {
			if err := s.embyClient.DeleteUser(*user.EmbyID); err != nil {
				logger.Warn().Err(err).Int64("tg", user.TG).Msg("删除用户失败")
				result.Failed++
				continue
			}
		}

		// 更新数据库
		s.embyRepo.UpdateFields(user.TG, map[string]interface{}{
			"embyid": nil,
			"name":   nil,
			"pwd":    nil,
			"lv":     models.LevelD,
			"cr":     nil,
			"ex":     nil,
		})

		username := "未知"
		if user.Name != nil {
			username = *user.Name
		}

		result.Success++
		result.Details = append(result.Details, fmt.Sprintf("已删除: %s (TG: %d)", username, user.TG))

		// 通知用户
		if s.bot != nil {
			chat := &tele.Chat{ID: user.TG}
			s.bot.Send(chat, "⚠️ 您的 Emby 账户已被删除，因为您已不在群组中。")
		}
	}

	return result, nil
}

// SyncUnbound 同步未绑定用户（删除 Emby 中未绑定 Bot 的用户）
func (s *BatchService) SyncUnbound(dryRun bool) (*BatchResult, error) {
	result := &BatchResult{
		Details: make([]string, 0),
	}

	// 获取 Emby 中的所有用户
	embyUsers, err := s.embyClient.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("获取 Emby 用户列表失败: %w", err)
	}

	result.Total = len(embyUsers)

	for _, user := range embyUsers {
		// 跳过管理员
		if user.Policy != nil && user.Policy.IsAdmin {
			result.Skipped++
			continue
		}

		// 检查是否在数据库中
		dbUser, err := s.embyRepo.GetByName(user.Name)
		if err == nil && dbUser != nil {
			result.Skipped++
			continue
		}

		// 未绑定 Bot
		result.Success++
		detail := fmt.Sprintf("未绑定: %s (ID: %s)", user.Name, user.ID)

		if !dryRun {
			// 删除用户
			if err := s.embyClient.DeleteUser(user.ID); err != nil {
				logger.Warn().Err(err).Str("user", user.Name).Msg("删除未绑定用户失败")
				result.Failed++
				result.Success--
				continue
			}
			detail = fmt.Sprintf("已删除: %s (ID: %s)", user.Name, user.ID)
		}

		result.Details = append(result.Details, detail)
	}

	return result, nil
}

// BanAll 禁用所有用户
func (s *BatchService) BanAll() (*BatchResult, error) {
	result := &BatchResult{
		Details: make([]string, 0),
	}

	// 获取所有活跃用户
	users, err := s.embyRepo.GetActiveUsers()
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Total = len(users)

	for _, user := range users {
		// 跳过白名单
		if user.Lv == models.LevelA {
			result.Skipped++
			continue
		}

		if user.EmbyID == nil || *user.EmbyID == "" {
			result.Skipped++
			continue
		}

		// 禁用 Emby 账户
		if err := s.embyClient.DisableUser(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("禁用用户失败")
			result.Failed++
			continue
		}

		// 更新数据库
		s.embyRepo.UpdateFields(user.TG, map[string]interface{}{
			"lv": models.LevelE,
		})

		result.Success++
	}

	return result, nil
}

// UnbanAll 解禁所有用户
func (s *BatchService) UnbanAll() (*BatchResult, error) {
	result := &BatchResult{
		Details: make([]string, 0),
	}

	// 获取所有封禁用户
	users, err := s.embyRepo.GetByLevel(models.LevelE)
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Total = len(users)

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			result.Skipped++
			continue
		}

		// 启用 Emby 账户
		if err := s.embyClient.EnableUser(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("启用用户失败")
			result.Failed++
			continue
		}

		// 更新数据库
		s.embyRepo.UpdateFields(user.TG, map[string]interface{}{
			"lv": models.LevelD,
		})

		result.Success++
	}

	return result, nil
}

// BindAllIDs 批量绑定 Emby ID
func (s *BatchService) BindAllIDs() (*BatchResult, error) {
	result := &BatchResult{
		Details: make([]string, 0),
	}

	// 获取 Emby 中的所有用户
	embyUsers, err := s.embyClient.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("获取 Emby 用户列表失败: %w", err)
	}

	result.Total = len(embyUsers)

	for _, user := range embyUsers {
		// 根据用户名查找数据库记录
		dbUser, err := s.embyRepo.GetByName(user.Name)
		if err != nil || dbUser == nil {
			result.Skipped++
			result.Details = append(result.Details, fmt.Sprintf("未找到: %s", user.Name))
			continue
		}

		// 更新 EmbyID
		if err := s.embyRepo.UpdateFields(dbUser.TG, map[string]interface{}{
			"embyid": user.ID,
		}); err != nil {
			result.Failed++
			continue
		}

		result.Success++
	}

	return result, nil
}

// RenewAll 批量续期
func (s *BatchService) RenewAll(days int, level models.UserLevel) (*BatchResult, error) {
	result := &BatchResult{
		Details: make([]string, 0),
	}

	var users []models.Emby
	var err error

	if level == "" {
		users, err = s.embyRepo.GetActiveUsers()
	} else {
		users, err = s.embyRepo.GetByLevel(level)
	}

	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Total = len(users)
	newExpiry := time.Now().AddDate(0, 0, days)

	for _, user := range users {
		if user.Lv == models.LevelA {
			result.Skipped++
			continue
		}

		if err := s.embyRepo.UpdateFields(user.TG, map[string]interface{}{
			"ex": newExpiry,
		}); err != nil {
			result.Failed++
			continue
		}

		result.Success++
	}

	return result, nil
}

// DeleteAll 删除所有用户（跑路功能）
func (s *BatchService) DeleteAll(confirm bool) (*BatchResult, error) {
	if !confirm {
		return nil, fmt.Errorf("请确认此危险操作")
	}

	result := &BatchResult{
		Details: make([]string, 0),
	}

	// 获取所有用户
	users, err := s.embyRepo.GetActiveUsers()
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Total = len(users)

	for _, user := range users {
		if user.EmbyID != nil && *user.EmbyID != "" {
			// 删除 Emby 账户
			if err := s.embyClient.DeleteUser(*user.EmbyID); err != nil {
				logger.Warn().Err(err).Int64("tg", user.TG).Msg("删除用户失败")
				result.Failed++
				continue
			}
		}

		// 清空数据库记录
		s.embyRepo.UpdateFields(user.TG, map[string]interface{}{
			"embyid": nil,
			"name":   nil,
			"pwd":    nil,
			"lv":     models.LevelD,
			"cr":     nil,
			"ex":     nil,
		})

		result.Success++
	}

	return result, nil
}

// FormatResult 格式化结果
func (r *BatchResult) FormatResult(operation string) string {
	text := fmt.Sprintf("📊 **%s 结果**\n\n", operation)
	text += fmt.Sprintf("总数: %d\n", r.Total)
	text += fmt.Sprintf("成功: %d\n", r.Success)
	text += fmt.Sprintf("失败: %d\n", r.Failed)
	text += fmt.Sprintf("跳过: %d\n", r.Skipped)
	return text
}
