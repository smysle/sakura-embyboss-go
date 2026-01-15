// Sakura EmbyBoss - Go Version
// Telegram Bot for Emby Server Management
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/smysle/sakura-embyboss-go/internal/bot"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/scheduler"
	"github.com/smysle/sakura-embyboss-go/internal/web"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

var (
	configPath = flag.String("config", "config.json", "配置文件路径")
	debug      = flag.Bool("debug", false, "调试模式")
)

func main() {
	flag.Parse()

	// 初始化日志
	logger.Init(*debug)
	logger.Info().Msg("🌸 Sakura EmbyBoss Go 启动中...")

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("加载配置失败")
	}
	// 保存配置文件路径，用于热重载
	config.SetConfigPath(*configPath)
	logger.Info().Msg("✅ 配置加载完成")

	// 初始化数据库
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal().Err(err).Msg("初始化数据库失败")
	}
	defer database.Close()
	logger.Info().Msg("✅ 数据库连接成功")

	// 初始化定时任务调度器
	sched := scheduler.New(cfg)
	sched.Start()
	defer sched.Stop()
	logger.Info().Msg("✅ 定时任务调度器启动")

	// 初始化 Web API 服务
	webServer := web.New(&cfg.API)
	go func() {
		if err := webServer.Start(); err != nil {
			logger.Error().Err(err).Msg("Web API 服务启动失败")
		}
	}()
	defer webServer.Stop()

	// 初始化 Telegram Bot
	tgBot, err := bot.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("初始化 Telegram Bot 失败")
	}
	logger.Info().Str("bot", cfg.BotName).Msg("✅ Telegram Bot 初始化完成")

	// 监听系统信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 在后台运行 Bot
	go tgBot.Run()

	logger.Info().Msg("🚀 Sakura EmbyBoss Go 启动成功!")
	logger.Info().Msg("按 Ctrl+C 停止...")

	// 等待退出信号
	<-quit

	logger.Info().Msg("正在关闭服务...")
	tgBot.Stop()
	logger.Info().Msg("👋 再见!")
}
