// Package service 到期检测服务
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

// ExpiryService 到期检测服务
type ExpiryService struct {
	embyRepo   *repository.EmbyRepository
	embyClient *emby.Client
	cfg        *config.Config
	bot        *tele.Bot
}

// ExpiryResult 检测结果
type ExpiryResult struct {
	Checked      int      // 检测的用户数
	Expired      int      // 已过期用户数
	Disabled     int      // 成功禁用数
	Failed       int      // 禁用失败数
	WarningSent  int      // 发送预警数
	ExpiredUsers []string // 过期用户列表
}

// NewExpiryService 创建到期检测服务
func NewExpiryService() *ExpiryService {
	return &ExpiryService{
		embyRepo:   repository.NewEmbyRepository(),
		embyClient: emby.GetClient(),
		cfg:        config.Get(),
	}
}

// SetBot 设置 Bot 实例（用于发送通知）
func (s *ExpiryService) SetBot(bot *tele.Bot) {
	s.bot = bot
}

// CheckExpired 检测并处理过期用户
func (s *ExpiryService) CheckExpired() (*ExpiryResult, error) {
	result := &ExpiryResult{
		ExpiredUsers: make([]string, 0),
	}

	// 获取所有有 Emby 账户的用户
	users, err := s.embyRepo.GetActiveUsers()
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Checked = len(users)
	now := time.Now()

	for _, user := range users {
		// 跳过白名单用户
		if user.Lv == models.LevelA {
			continue
		}

		// 检查是否过期
		if user.Ex == nil {
			continue
		}

		if user.Ex.After(now) {
			continue // 未过期
		}

		result.Expired++

		// 记录用户名
		username := fmt.Sprintf("TG:%d", user.TG)
		if user.Name != nil {
			username = *user.Name
		}
		result.ExpiredUsers = append(result.ExpiredUsers, username)

		// 禁用 Emby 账户
		if user.EmbyID != nil && *user.EmbyID != "" {
			if err := s.embyClient.DisableUser(*user.EmbyID); err != nil {
				logger.Warn().
					Err(err).
					Int64("tg", user.TG).
					Str("emby_id", *user.EmbyID).
					Msg("禁用过期用户失败")
				result.Failed++
			} else {
				result.Disabled++
				logger.Info().
					Int64("tg", user.TG).
					Str("username", username).
					Msg("已禁用过期用户")
			}
		}

		// 更新用户等级为封禁
		s.embyRepo.UpdateFields(user.TG, map[string]interface{}{
			"lv": models.LevelE,
		})

		// 发送通知给用户
		s.notifyUser(user.TG, "expired")
	}

	return result, nil
}

// CheckWarning 检测即将过期的用户并发送预警
func (s *ExpiryService) CheckWarning(daysBeforeExpiry int) (*ExpiryResult, error) {
	result := &ExpiryResult{}

	if daysBeforeExpiry <= 0 {
		daysBeforeExpiry = 3 // 默认提前 3 天预警
	}

	users, err := s.embyRepo.GetActiveUsers()
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}

	result.Checked = len(users)
	now := time.Now()
	warningDate := now.AddDate(0, 0, daysBeforeExpiry)

	for _, user := range users {
		// 跳过白名单用户
		if user.Lv == models.LevelA {
			continue
		}

		if user.Ex == nil {
			continue
		}

		// 检查是否在预警期内（未过期但即将过期）
		if user.Ex.After(now) && user.Ex.Before(warningDate) {
			daysLeft := int(user.Ex.Sub(now).Hours() / 24)
			s.notifyUserWarning(user.TG, daysLeft)
			result.WarningSent++
		}
	}

	return result, nil
}

// notifyUser 通知用户
func (s *ExpiryService) notifyUser(tgID int64, notifyType string) {
	if s.bot == nil {
		return
	}

	var text string
	switch notifyType {
	case "expired":
		text = "⚠️ **账户已过期**\n\n" +
			"您的 Emby 账户已到期，访问权限已被暂停。\n\n" +
			"如需续期，请联系管理员或使用注册码续期。"
	case "disabled":
		text = "🚫 **账户已被禁用**\n\n" +
			"您的 Emby 账户已被禁用，如有疑问请联系管理员。"
	}

	chat := &tele.Chat{ID: tgID}
	if _, err := s.bot.Send(chat, text, tele.ModeMarkdown); err != nil {
		logger.Debug().Err(err).Int64("tg", tgID).Msg("发送过期通知失败")
	}
}

// notifyUserWarning 发送预警通知
func (s *ExpiryService) notifyUserWarning(tgID int64, daysLeft int) {
	if s.bot == nil {
		return
	}

	text := fmt.Sprintf(
		"⏰ **账户即将过期**\n\n"+
			"您的 Emby 账户将在 **%d 天**后到期。\n\n"+
			"请及时续期以免影响使用。",
		daysLeft,
	)

	chat := &tele.Chat{ID: tgID}
	if _, err := s.bot.Send(chat, text, tele.ModeMarkdown); err != nil {
		logger.Debug().Err(err).Int64("tg", tgID).Msg("发送预警通知失败")
	}
}

// RenewUser 续期用户
func (s *ExpiryService) RenewUser(tgID int64, days int) error {
	user, err := s.embyRepo.GetByTG(tgID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	// 计算新的到期时间
	var newExpiry time.Time
	if user.Ex != nil && user.Ex.After(time.Now()) {
		// 如果未过期，在原有基础上增加
		newExpiry = user.Ex.AddDate(0, 0, days)
	} else {
		// 如果已过期，从现在开始计算
		newExpiry = time.Now().AddDate(0, 0, days)
	}

	// 更新数据库
	updates := map[string]interface{}{
		"ex": newExpiry,
	}

	// 如果用户被封禁，恢复为普通用户
	if user.Lv == models.LevelE {
		updates["lv"] = models.LevelD

		// 重新启用 Emby 账户
		if user.EmbyID != nil && *user.EmbyID != "" {
			if err := s.embyClient.EnableUser(*user.EmbyID); err != nil {
				logger.Warn().Err(err).Int64("tg", tgID).Msg("启用 Emby 账户失败")
			}
		}
	}

	return s.embyRepo.UpdateFields(tgID, updates)
}

// GetUserExpiry 获取用户到期信息
func (s *ExpiryService) GetUserExpiry(tgID int64) (*UserExpiryInfo, error) {
	user, err := s.embyRepo.GetByTG(tgID)
	if err != nil {
		return nil, err
	}

	info := &UserExpiryInfo{
		TG:          tgID,
		IsWhitelist: user.Lv == models.LevelA,
		IsBanned:    user.Lv == models.LevelE,
	}

	if user.Name != nil {
		info.Username = *user.Name
	}

	if user.Ex != nil {
		info.ExpiryTime = user.Ex
		info.IsExpired = user.Ex.Before(time.Now())
		if !info.IsExpired {
			info.DaysLeft = int(user.Ex.Sub(time.Now()).Hours() / 24)
		}
	}

	return info, nil
}

// UserExpiryInfo 用户到期信息
type UserExpiryInfo struct {
	TG          int64
	Username    string
	ExpiryTime  *time.Time
	DaysLeft    int
	IsExpired   bool
	IsWhitelist bool
	IsBanned    bool
}

// FormatExpiryInfo 格式化到期信息
func (info *UserExpiryInfo) FormatExpiryInfo() string {
	if info.IsWhitelist {
		return "✨ 白名单用户（永不过期）"
	}
	if info.IsBanned {
		return "🚫 账户已被封禁"
	}
	if info.ExpiryTime == nil {
		return "❓ 未设置到期时间"
	}
	if info.IsExpired {
		return fmt.Sprintf("❌ 已于 %s 过期", info.ExpiryTime.Format("2006-01-02"))
	}
	return fmt.Sprintf("✅ %s 到期（剩余 %d 天）",
		info.ExpiryTime.Format("2006-01-02"), info.DaysLeft)
}
