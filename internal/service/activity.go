// Package service 活跃度检测服务
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

// ActivityService 活跃度检测服务
type ActivityService struct {
	embyRepo   *repository.EmbyRepository
	embyClient *emby.Client
	cfg        *config.Config
	bot        *tele.Bot
}

// ActivityResult 活跃度检测结果
type ActivityResult struct {
	Checked       int      // 检测的用户数
	Inactive      int      // 不活跃用户数
	Disabled      int      // 禁用的用户数
	Deleted       int      // 删除的用户数
	Failed        int      // 操作失败数
	InactiveUsers []string // 不活跃用户列表
}

// NewActivityService 创建活跃度检测服务
func NewActivityService() *ActivityService {
	return &ActivityService{
		embyRepo:   repository.NewEmbyRepository(),
		embyClient: emby.GetClient(),
		cfg:        config.Get(),
	}
}

// SetBot 设置 Bot 实例
func (s *ActivityService) SetBot(bot *tele.Bot) {
	s.bot = bot
}

// CheckLowActivity 检测低活跃用户
func (s *ActivityService) CheckLowActivity() (*ActivityResult, error) {
	result := &ActivityResult{
		InactiveUsers: make([]string, 0),
	}

	// 获取活跃度检测天数配置
	checkDays := s.cfg.ActivityCheckDays
	if checkDays <= 0 {
		checkDays = 21 // 默认 21 天
	}

	// 从 Emby 获取所有用户
	users, err := s.embyClient.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("获取 Emby 用户列表失败: %w", err)
	}

	result.Checked = len(users)
	now := time.Now()
	cutoffDate := now.AddDate(0, 0, -checkDays)

	for _, user := range users {
		// 跳过管理员
		if user.Policy != nil && user.Policy.IsAdmin {
			continue
		}

		// 从数据库获取用户信息
		embyUser, err := s.embyRepo.GetByName(user.Name)
		if err != nil {
			continue // 未绑定 Bot 的用户跳过
		}

		// 跳过白名单用户
		if embyUser.Lv == models.LevelA {
			continue
		}

		// 处理已禁用用户（等级 c）
		if embyUser.Lv == models.LevelC {
			// 检查是否需要删除
			if err := s.handleDisabledUser(embyUser, user, result); err != nil {
				logger.Warn().Err(err).Int64("tg", embyUser.TG).Msg("处理禁用用户失败")
			}
			continue
		}

		// 处理正常用户（等级 b）
		if embyUser.Lv == models.LevelB {
			// 获取最后活跃时间
			lastActivity := user.LastSeen
			isInactive := false

			if lastActivity == nil {
				// 从未活跃
				isInactive = true
			} else if lastActivity.Before(cutoffDate) {
				// 超过阈值天数未活跃
				isInactive = true
			}

			if isInactive {
				result.Inactive++
				username := user.Name
				result.InactiveUsers = append(result.InactiveUsers, username)

				// 禁用用户
				if err := s.embyClient.DisableUser(user.ID); err != nil {
					logger.Warn().Err(err).Str("user", username).Msg("禁用不活跃用户失败")
					result.Failed++
				} else {
					// 更新数据库状态
					s.embyRepo.UpdateFields(embyUser.TG, map[string]interface{}{
						"lv": models.LevelC,
					})
					result.Disabled++

					// 通知用户
					s.notifyInactiveUser(embyUser.TG, checkDays)

					logger.Info().
						Str("user", username).
						Int64("tg", embyUser.TG).
						Msg("已禁用不活跃用户")
				}
			}
		}
	}

	return result, nil
}

// handleDisabledUser 处理已禁用的用户（检查是否需要删除）
func (s *ActivityService) handleDisabledUser(embyUser *models.Emby, user emby.User, result *ActivityResult) error {
	// 检查是否超过冻结期
	freezeDays := s.cfg.FreezeDays
	if freezeDays <= 0 {
		freezeDays = 5
	}

	// 如果用户有过期时间，从过期时间开始计算
	var deleteDate time.Time
	if embyUser.Ex != nil {
		deleteDate = embyUser.Ex.AddDate(0, 0, freezeDays)
	} else {
		// 没有过期时间，从 15 天后删除
		deleteDate = time.Now().AddDate(0, 0, -15)
	}

	if time.Now().After(deleteDate) {
		// 删除用户
		if embyUser.EmbyID != nil {
			if err := s.embyClient.DeleteUser(*embyUser.EmbyID); err != nil {
				result.Failed++
				return fmt.Errorf("删除用户失败: %w", err)
			}
		}

		// 清空数据库记录
		s.embyRepo.UpdateFields(embyUser.TG, map[string]interface{}{
			"embyid": nil,
			"name":   nil,
			"pwd":    nil,
			"lv":     models.LevelD,
			"cr":     nil,
			"ex":     nil,
		})

		result.Deleted++

		// 通知用户
		s.notifyDeletedUser(embyUser.TG)

		logger.Info().Int64("tg", embyUser.TG).Msg("已删除长期禁用用户")
	}

	return nil
}

// notifyInactiveUser 通知不活跃用户
func (s *ActivityService) notifyInactiveUser(tgID int64, days int) {
	if s.bot == nil {
		return
	}

	text := fmt.Sprintf(
		"⚠️ **账户已被禁用**\n\n"+
			"由于您 **%d 天**未使用 Emby，账户已被暂停。\n\n"+
			"如需恢复，请联系管理员或通过积分解封。",
		days,
	)

	chat := &tele.Chat{ID: tgID}
	if _, err := s.bot.Send(chat, text, tele.ModeMarkdown); err != nil {
		logger.Debug().Err(err).Int64("tg", tgID).Msg("发送不活跃通知失败")
	}
}

// notifyDeletedUser 通知被删除用户
func (s *ActivityService) notifyDeletedUser(tgID int64) {
	if s.bot == nil {
		return
	}

	text := "🗑️ **账户已被删除**\n\n" +
		"由于长期未使用且未解封，您的 Emby 账户已被删除。\n\n" +
		"如需重新注册，请联系管理员。"

	chat := &tele.Chat{ID: tgID}
	if _, err := s.bot.Send(chat, text, tele.ModeMarkdown); err != nil {
		logger.Debug().Err(err).Int64("tg", tgID).Msg("发送删除通知失败")
	}
}

// FormatResult 格式化结果
func (r *ActivityResult) FormatResult() string {
	text := "📊 **活跃度检测报告**\n\n"
	text += fmt.Sprintf("检测用户: %d\n", r.Checked)
	text += fmt.Sprintf("不活跃用户: %d\n", r.Inactive)
	text += fmt.Sprintf("已禁用: %d\n", r.Disabled)
	text += fmt.Sprintf("已删除: %d\n", r.Deleted)
	if r.Failed > 0 {
		text += fmt.Sprintf("操作失败: %d\n", r.Failed)
	}
	return text
}
