// Package service 签到服务
package service

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
	"github.com/smysle/sakura-embyboss-go/pkg/utils"
)

var (
	ErrCheckinDisabled  = errors.New("签到功能已关闭")
	ErrAlreadyCheckedIn = errors.New("今日已签到")
	ErrLevelNotAllowed  = errors.New("您的等级不允许签到")
	ErrUserNotFound     = errors.New("用户不存在")
)

// CheckinResult 签到结果
type CheckinResult struct {
	Success     bool
	Reward      int       // 获得的积分
	TotalScore  int       // 当前总积分
	Consecutive int       // 连续签到天数
	CheckinTime time.Time // 签到时间
	Message     string    // 提示消息
}

// CheckinService 签到服务
type CheckinService struct {
	repo *repository.EmbyRepository
	cfg  *config.Config
}

// NewCheckinService 创建签到服务
func NewCheckinService() *CheckinService {
	return &CheckinService{
		repo: repository.NewEmbyRepository(),
		cfg:  config.Get(),
	}
}

// Checkin 执行签到
func (s *CheckinService) Checkin(tgID int64) (*CheckinResult, error) {
	// 检查签到功能是否开启
	if !s.cfg.Open.Checkin {
		return nil, ErrCheckinDisabled
	}

	// 获取用户信息
	user, err := s.repo.GetByTG(tgID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// 检查用户等级是否允许签到
	if !s.isLevelAllowed(user.Lv) {
		return nil, ErrLevelNotAllowed
	}

	// 检查今日是否已签到
	now := utils.TimeNowCST()
	if s.hasCheckedInToday(user, now) {
		return nil, ErrAlreadyCheckedIn
	}

	// 计算连续签到天数
	consecutive := s.calculateConsecutiveDays(user, now)

	// 计算奖励
	reward := s.calculateReward(consecutive)

	// 更新用户信息
	newScore := user.Us + reward
	updates := map[string]interface{}{
		"ch": now,          // 签到时间
		"us": newScore,     // 更新积分
		"ck": consecutive,  // 连续签到天数
	}

	if err := s.repo.UpdateFields(tgID, updates); err != nil {
		logger.Error().Err(err).Int64("tg", tgID).Msg("签到更新失败")
		return nil, fmt.Errorf("签到失败: %w", err)
	}

	logger.Info().
		Int64("tg", tgID).
		Int("reward", reward).
		Int("consecutive", consecutive).
		Msg("用户签到成功")

	return &CheckinResult{
		Success:     true,
		Reward:      reward,
		TotalScore:  newScore,
		Consecutive: consecutive,
		CheckinTime: now,
		Message:     s.generateMessage(reward, consecutive),
	}, nil
}

// hasCheckedInToday 检查今日是否已签到
func (s *CheckinService) hasCheckedInToday(user *models.Emby, now time.Time) bool {
	if user.Ch == nil {
		return false
	}

	// 获取今日零点
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	return user.Ch.After(todayStart) || user.Ch.Equal(todayStart)
}

// calculateConsecutiveDays 计算连续签到天数
func (s *CheckinService) calculateConsecutiveDays(user *models.Emby, now time.Time) int {
	if user.Ch == nil {
		return 1 // 首次签到
	}

	// 获取昨日零点
	yesterdayStart := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	yesterdayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 检查上次签到是否是昨天
	if user.Ch.After(yesterdayStart) && user.Ch.Before(yesterdayEnd) {
		// 连续签到，天数+1
		return user.Ck + 1
	}

	return 1 // 断签，重新计算
}

// calculateReward 计算签到奖励
func (s *CheckinService) calculateReward(consecutive int) int {
	rewardRange := s.cfg.Open.CheckinReward
	if len(rewardRange) < 2 {
		rewardRange = []int{1, 10} // 默认值
	}

	minReward := rewardRange[0]
	maxReward := rewardRange[1]

	// 基础随机奖励
	baseReward := minReward + rand.Intn(maxReward-minReward+1)

	// 连续签到加成
	bonus := 0
	if consecutive >= 30 {
		bonus = 15 // 连续30天加15分
	} else if consecutive >= 14 {
		bonus = 10 // 连续14天加10分
	} else if consecutive >= 7 {
		bonus = 5 // 连续7天加5分
	} else if consecutive >= 3 {
		bonus = 2 // 连续3天加2分
	}

	return baseReward + bonus
}

// isLevelAllowed 检查用户等级是否允许签到
func (s *CheckinService) isLevelAllowed(level models.UserLevel) bool {
	// 根据配置的签到等级判断
	checkinLevel := s.cfg.Open.CheckinLevel
	if checkinLevel == "" {
		checkinLevel = "d" // 默认所有用户可签到
	}

	levelOrder := map[models.UserLevel]int{
		models.LevelA: 1,
		models.LevelB: 2,
		models.LevelC: 3,
		models.LevelD: 4,
		models.LevelE: 5, // 封禁用户
	}

	requiredLevel := models.UserLevel(checkinLevel)

	// 封禁用户不能签到
	if level == models.LevelE {
		return false
	}

	return levelOrder[level] <= levelOrder[requiredLevel]
}

// generateMessage 生成签到消息
func (s *CheckinService) generateMessage(reward, consecutive int) string {
	messages := []string{
		"🎉 签到成功！",
		"✨ 又是元气满满的一天！",
		"🌟 签到打卡成功！",
		"💫 今日份签到完成！",
		"🎊 签到成功，继续加油！",
	}

	msg := messages[rand.Intn(len(messages))]

	if consecutive >= 30 {
		msg += " 🏆 连续签到30天，超级奖励！"
	} else if consecutive >= 14 {
		msg += " 🔥 连续签到14天，获得高额奖励！"
	} else if consecutive >= 7 {
		msg += " 🔥 连续签到7天，获得额外奖励！"
	} else if consecutive >= 3 {
		msg += " ⭐ 连续签到3天，获得小额加成！"
	}

	return msg
}

// GetCheckinStatus 获取签到状态
func (s *CheckinService) GetCheckinStatus(tgID int64) (hasCheckedIn bool, consecutive int, lastCheckin *time.Time, err error) {
	user, err := s.repo.GetByTG(tgID)
	if err != nil {
		return false, 0, nil, err
	}

	now := utils.TimeNowCST()
	hasCheckedIn = s.hasCheckedInToday(user, now)

	return hasCheckedIn, user.Ck, user.Ch, nil
}
