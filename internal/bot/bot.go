// Package bot Telegram Bot 核心
package bot

import (
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/handlers"
	"github.com/smysle/sakura-embyboss-go/internal/bot/middleware"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Bot Telegram Bot 实例
type Bot struct {
	*tele.Bot
	cfg *config.Config
}

var instance *Bot

// New 创建新的 Bot 实例
func New(cfg *config.Config) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, c tele.Context) {
			logger.Error().Err(err).Msg("Bot 错误")
		},
	}

	// 设置代理
	if cfg.Proxy.Scheme != "" {
		// 代理支持需要使用自定义 HTTP 客户端
		// 在容器化部署中通常直接配置环境变量 HTTP_PROXY
		logger.Info().
			Str("scheme", cfg.Proxy.Scheme).
			Str("host", cfg.Proxy.Host).
			Msg("检测到代理配置，请确保已设置 HTTP_PROXY 环境变量")
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		Bot: b,
		cfg: cfg,
	}

	// 注册中间件
	bot.registerMiddleware()

	// 注册处理器
	bot.registerHandlers()

	// 设置命令列表
	bot.setCommands()

	instance = bot
	return bot, nil
}

// Get 获取 Bot 单例
func Get() *Bot {
	return instance
}

// registerMiddleware 注册中间件
func (b *Bot) registerMiddleware() {
	// 日志中间件
	b.Use(middleware.Logger())

	// 恢复中间件
	b.Use(middleware.Recover())
}

// registerHandlers 注册所有处理器
func (b *Bot) registerHandlers() {
	// 用户命令
	b.Handle("/start", handlers.Start)
	b.Handle("/myinfo", handlers.MyInfo)
	b.Handle("/count", handlers.Count)
	b.Handle("/red", handlers.RedEnvelope)
	b.Handle("/srank", handlers.ScoreRank)

	// 注册排行榜命令
	handlers.RegisterLeaderboardHandlers(b.Bot)

	// 注册 MoviePilot 回调
	handlers.RegisterMoviePilotCallbacks(b.Bot)

	// 管理员命令 (需要权限验证)
	adminGroup := b.Group()
	adminGroup.Use(middleware.AdminOnly())

	adminGroup.Handle("/kk", handlers.KK)
	adminGroup.Handle("/score", handlers.Score)
	adminGroup.Handle("/coins", handlers.Coins)
	adminGroup.Handle("/renew", handlers.Renew)
	adminGroup.Handle("/rmemby", handlers.RemoveEmby)
	adminGroup.Handle("/prouser", handlers.ProUser)
	adminGroup.Handle("/revuser", handlers.RevUser)
	adminGroup.Handle("/syncgroupm", handlers.SyncGroupMembers)
	adminGroup.Handle("/syncunbound", handlers.SyncUnbound)
	adminGroup.Handle("/bindall_id", handlers.BindAllIDs)
	adminGroup.Handle("/renewall", handlers.RenewAll)
	adminGroup.Handle("/check_ex", handlers.CheckExpiredManual)
	adminGroup.Handle("/check_activity", handlers.CheckActivityManual)
	adminGroup.Handle("/uranks", handlers.UserRanks)
	adminGroup.Handle("/days_ranks", handlers.DayRanks)
	adminGroup.Handle("/week_ranks", handlers.WeekRanks)
	adminGroup.Handle("/restart", handlers.Restart)
	adminGroup.Handle("/update_bot", handlers.UpdateBot)

	// 注册码管理命令
	adminGroup.Handle("/code", handlers.GenerateCode)
	adminGroup.Handle("/codestat", handlers.CodeStats)
	adminGroup.Handle("/mycode", handlers.MyCodeStats)
	adminGroup.Handle("/delcode", handlers.DeleteCodes)

	// 审计命令
	adminGroup.Handle("/auditip", handlers.AuditIP)
	adminGroup.Handle("/auditdevice", handlers.AuditDevice)
	adminGroup.Handle("/auditclient", handlers.AuditClient)

	// 额外管理命令
	adminGroup.Handle("/uinfo", handlers.UInfo)
	adminGroup.Handle("/coinsall", handlers.CoinsAll)
	adminGroup.Handle("/callall", handlers.CallAll)
	adminGroup.Handle("/ucr", handlers.UCr)
	adminGroup.Handle("/urm", handlers.URm)
	adminGroup.Handle("/deleted", handlers.Deleted)
	adminGroup.Handle("/low_activity", handlers.LowActivity)

	// 批量媒体库控制命令
	adminGroup.Handle("/embylibs_blockall", handlers.EmbyLibsBlockAll)
	adminGroup.Handle("/embylibs_unblockall", handlers.EmbyLibsUnblockAll)
	adminGroup.Handle("/extraembylibs_blockall", handlers.ExtraEmbyLibsBlockAll)
	adminGroup.Handle("/extraembylibs_unblockall", handlers.ExtraEmbyLibsUnblockAll)

	// Owner 命令
	ownerGroup := b.Group()
	ownerGroup.Use(middleware.OwnerOnly())

	ownerGroup.Handle("/config", handlers.Config)
	ownerGroup.Handle("/proadmin", handlers.ProAdmin)
	ownerGroup.Handle("/revadmin", handlers.RevAdmin)
	ownerGroup.Handle("/backup_db", handlers.BackupDB)
	ownerGroup.Handle("/banall", handlers.BanAll)
	ownerGroup.Handle("/unbanall", handlers.UnbanAll)
	ownerGroup.Handle("/paolu", handlers.Paolu)
	ownerGroup.Handle("/coinsclear", handlers.CoinsClear)

	// 回调查询
	b.Handle(tele.OnCallback, handlers.OnCallback)

	// 内联查询
	b.Handle(tele.OnQuery, handlers.OnInlineQuery)

	// 文本消息处理（用于会话状态）
	b.Handle(tele.OnText, handlers.OnText)

	// 取消命令
	b.Handle("/cancel", handlers.Cancel)
}

// setCommands 设置命令列表
func (b *Bot) setCommands() {
	// 用户命令
	userCmds := []tele.Command{
		{Text: "start", Description: "[私聊] 开启用户面板"},
		{Text: "myinfo", Description: "[用户] 查看状态"},
		{Text: "count", Description: "[用户] 媒体库数量"},
		{Text: "red", Description: "[用户] 发红包"},
		{Text: "srank", Description: "[用户] 查看计分"},
		{Text: "rank", Description: "[用户] 查看排行榜"},
		{Text: "dayrank", Description: "[用户] 今日播放榜"},
		{Text: "weekrank", Description: "[用户] 本周播放榜"},
	}

	// 管理员命令
	adminCmds := append(userCmds, []tele.Command{
		{Text: "kk", Description: "管理用户 [管理]"},
		{Text: "score", Description: "加/减积分 [管理]"},
		{Text: "coins", Description: "加/减花币 [管理]"},
		{Text: "renew", Description: "调整到期时间 [管理]"},
		{Text: "rmemby", Description: "删除用户 [管理]"},
		{Text: "prouser", Description: "增加白名单 [管理]"},
		{Text: "revuser", Description: "减少白名单 [管理]"},
		{Text: "check_ex", Description: "手动到期检测 [管理]"},
		{Text: "auditip", Description: "IP 审计 [管理]"},
		{Text: "auditdevice", Description: "设备审计 [管理]"},
		{Text: "auditclient", Description: "客户端审计 [管理]"},
		{Text: "uinfo", Description: "查询用户信息 [管理]"},
		{Text: "coinsall", Description: "批量发放积分 [管理]"},
		{Text: "callall", Description: "广播消息 [管理]"},
		{Text: "ucr", Description: "创建非TG用户 [管理]"},
		{Text: "urm", Description: "删除指定用户 [管理]"},
		{Text: "deleted", Description: "清理死号 [管理]"},
		{Text: "low_activity", Description: "手动活跃检测 [管理]"},
		{Text: "restart", Description: "重启bot [管理]"},
	}...)

	// Owner 命令
	ownerCmds := append(adminCmds, []tele.Command{
		{Text: "config", Description: "开启bot控制面板 [owner]"},
		{Text: "proadmin", Description: "添加bot管理 [owner]"},
		{Text: "revadmin", Description: "移除bot管理 [owner]"},
		{Text: "backup_db", Description: "手动备份数据库 [owner]"},
		{Text: "banall", Description: "禁用所有用户 [owner]"},
		{Text: "unbanall", Description: "解除所有用户禁用 [owner]"},
		{Text: "paolu", Description: "跑路! 删除所有用户 [owner]"},
		{Text: "coinsclear", Description: "清空用户积分 [owner]"},
	}...)

	// 为不同用户设置不同命令
	b.SetCommands(userCmds)

	// 为管理员设置专属命令列表
	for _, adminID := range b.cfg.Admins {
		b.SetCommands(adminCmds, tele.CommandScope{
			Type:   tele.CommandScopeChat,
			ChatID: adminID,
		})
	}

	// 为 Owner 设置专属命令列表
	b.SetCommands(ownerCmds, tele.CommandScope{
		Type:   tele.CommandScopeChat,
		ChatID: b.cfg.Owner,
	})
}

// Run 运行 Bot
func (b *Bot) Run() {
	logger.Info().Str("bot", b.cfg.BotName).Msg("Bot 启动中...")
	b.Start()
}

// Stop 停止 Bot
func (b *Bot) Stop() {
	logger.Info().Msg("Bot 停止中...")
	b.Bot.Stop()
}
