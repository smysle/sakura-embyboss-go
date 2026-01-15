// Package scheduler 定时任务调度
package scheduler

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/handlers"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron *gocron.Scheduler
	cfg  *config.Config
	bot  *tele.Bot
}

var instance *Scheduler

// New 创建调度器
func New(cfg *config.Config) *Scheduler {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	s := gocron.NewScheduler(loc)
	s.SetMaxConcurrentJobs(5, gocron.RescheduleMode)

	instance = &Scheduler{
		cron: s,
		cfg:  cfg,
	}

	return instance
}

// Get 获取调度器实例
func Get() *Scheduler {
	return instance
}

// SetBot 设置 Bot 实例（用于发送消息）
func (s *Scheduler) SetBot(bot *tele.Bot) {
	s.bot = bot
}

// Start 启动调度器
func (s *Scheduler) Start() {
	logger.Info().Msg("启动定时任务调度器")

	// 注册定时任务
	s.registerJobs()

	// 异步启动
	s.cron.StartAsync()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	logger.Info().Msg("停止定时任务调度器")
	s.cron.Stop()
}

// registerJobs 注册所有定时任务
func (s *Scheduler) registerJobs() {
	cfg := s.cfg.Scheduler

	// 到期检测 - 每天凌晨 1 点
	if cfg.CheckExpired {
		s.cron.Every(1).Day().At("01:00").Do(s.checkExpired)
		logger.Info().Msg("已注册: 到期检测任务 (每天 01:00)")
	}

	// 日榜 - 每天晚上 22 点
	if cfg.DayRank {
		s.cron.Every(1).Day().At("22:00").Do(s.generateDayRanks)
		logger.Info().Msg("已注册: 日榜任务 (每天 22:00)")
	}

	// 周榜 - 每周日晚上 22 点
	if cfg.WeekRank {
		s.cron.Every(1).Week().Sunday().At("22:00").Do(s.generateWeekRanks)
		logger.Info().Msg("已注册: 周榜任务 (每周日 22:00)")
	}

	// 活跃度检测 - 每天凌晨 2 点
	if cfg.LowActivity {
		s.cron.Every(1).Day().At("02:00").Do(s.checkLowActivity)
		logger.Info().Msg("已注册: 活跃度检测任务 (每天 02:00)")
	}

	// 数据库备份 - 每天凌晨 3 点
	if cfg.BackupDB {
		s.cron.Every(1).Day().At("03:00").Do(s.backupDatabase)
		logger.Info().Msg("已注册: 数据库备份任务 (每天 03:00)")
	}
}

// AddJob 添加自定义任务
func (s *Scheduler) AddJob(cronExpr string, job func()) error {
	_, err := s.cron.Cron(cronExpr).Do(job)
	return err
}

// RemoveJob 移除任务
func (s *Scheduler) RemoveJob(tag string) {
	s.cron.RemoveByTag(tag)
}

// checkExpired 检查过期用户
func (s *Scheduler) checkExpired() {
	logger.Info().Msg("执行定时任务: 到期检测")

	expirySvc := service.NewExpiryService()
	expirySvc.SetBot(s.bot)

	// 检测并处理过期用户
	result, err := expirySvc.CheckExpired()
	if err != nil {
		logger.Error().Err(err).Msg("到期检测失败")
		return
	}

	logger.Info().
		Int("checked", result.Checked).
		Int("expired", result.Expired).
		Int("disabled", result.Disabled).
		Int("failed", result.Failed).
		Msg("到期检测完成")

	// 发送预警（提前 3 天）
	warningResult, err := expirySvc.CheckWarning(3)
	if err != nil {
		logger.Warn().Err(err).Msg("发送预警失败")
	} else if warningResult.WarningSent > 0 {
		logger.Info().Int("sent", warningResult.WarningSent).Msg("已发送过期预警")
	}

	// 向 Owner 发送报告
	if s.bot != nil && s.cfg.OwnerID != 0 && result.Expired > 0 {
		report := fmt.Sprintf(
			"📊 **到期检测报告**\n\n"+
				"检测用户: %d\n"+
				"过期用户: %d\n"+
				"成功禁用: %d\n"+
				"禁用失败: %d",
			result.Checked,
			result.Expired,
			result.Disabled,
			result.Failed,
		)
		chat := &tele.Chat{ID: s.cfg.OwnerID}
		s.bot.Send(chat, report, tele.ModeMarkdown)
	}
}

// generateDayRanks 生成并发送日榜
func (s *Scheduler) generateDayRanks() {
	logger.Info().Msg("执行定时任务: 生成日榜")

	if s.bot == nil {
		logger.Error().Msg("Bot 未设置，无法发送日榜")
		return
	}

	// 获取推送群组
	chatID := s.cfg.GroupID
	if chatID == 0 {
		logger.Warn().Msg("未配置群组 ID，跳过日榜推送")
		return
	}

	// 使用排行榜处理器发送
	handler := handlers.NewLeaderboardHandler()
	if err := handler.SendRankToChat(s.bot, chatID, service.RankTypeDay); err != nil {
		logger.Error().Err(err).Msg("发送日榜失败")
	} else {
		logger.Info().Int64("chat_id", chatID).Msg("日榜发送成功")
	}
}

// generateWeekRanks 生成并发送周榜
func (s *Scheduler) generateWeekRanks() {
	logger.Info().Msg("执行定时任务: 生成周榜")

	if s.bot == nil {
		logger.Error().Msg("Bot 未设置，无法发送周榜")
		return
	}

	// 获取推送群组
	chatID := s.cfg.GroupID
	if chatID == 0 {
		logger.Warn().Msg("未配置群组 ID，跳过周榜推送")
		return
	}

	// 使用排行榜处理器发送
	handler := handlers.NewLeaderboardHandler()
	if err := handler.SendRankToChat(s.bot, chatID, service.RankTypeWeek); err != nil {
		logger.Error().Err(err).Msg("发送周榜失败")
	} else {
		logger.Info().Int64("chat_id", chatID).Msg("周榜发送成功")
	}
}

// checkLowActivity 检查低活跃用户
func (s *Scheduler) checkLowActivity() {
	logger.Info().Msg("执行定时任务: 活跃度检测")

	activitySvc := service.NewActivityService()
	activitySvc.SetBot(s.bot)

	result, err := activitySvc.CheckLowActivity()
	if err != nil {
		logger.Error().Err(err).Msg("活跃度检测失败")
		return
	}

	logger.Info().
		Int("checked", result.Checked).
		Int("inactive", result.Inactive).
		Int("disabled", result.Disabled).
		Int("deleted", result.Deleted).
		Msg("活跃度检测完成")

	// 向 Owner 发送报告
	if s.bot != nil && s.cfg.OwnerID != 0 && (result.Inactive > 0 || result.Deleted > 0) {
		chat := &tele.Chat{ID: s.cfg.OwnerID}
		s.bot.Send(chat, result.FormatResult(), tele.ModeMarkdown)
	}
}

// backupDatabase 备份数据库
func (s *Scheduler) backupDatabase() {
	logger.Info().Msg("执行定时任务: 数据库备份")

	backupSvc := service.NewBackupService()

	// 执行备份
	result, err := backupSvc.Backup(true)
	if err != nil {
		logger.Error().Err(err).Msg("定时备份失败")
		return
	}

	logger.Info().
		Str("file", result.Filename).
		Int64("size", result.Size).
		Int("records", result.Records).
		Msg("定时备份完成")

	// 清理旧备份（7天前）
	deleted, err := backupSvc.CleanOldBackups(7)
	if err != nil {
		logger.Warn().Err(err).Msg("清理旧备份失败")
	} else if deleted > 0 {
		logger.Info().Int("deleted", deleted).Msg("已清理旧备份")
	}
}

// RunNow 立即执行指定任务（用于调试）
func (s *Scheduler) RunNow(taskName string) error {
	switch taskName {
	case "dayrank":
		s.generateDayRanks()
	case "weekrank":
		s.generateWeekRanks()
	case "check_expired":
		s.checkExpired()
	case "low_activity":
		s.checkLowActivity()
	case "backup":
		s.backupDatabase()
	default:
		logger.Warn().Str("task", taskName).Msg("未知任务")
	}
	return nil
}
