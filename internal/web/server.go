// Package web Web API 服务
package web

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	pkglogger "github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Server Web 服务器
type Server struct {
	app       *fiber.App
	cfg       *config.APIConfig
	startTime time.Time
}

// New 创建 Web 服务器
func New(cfg *config.APIConfig) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// 中间件
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	server := &Server{
		app:       app,
		cfg:       cfg,
		startTime: time.Now(),
	}

	// 注册路由
	server.registerRoutes()

	return server
}

// registerRoutes 注册路由
func (s *Server) registerRoutes() {
	// 健康检查
	s.app.Get("/health", s.healthCheck)
	s.app.Get("/", s.healthCheck)

	// 详细状态
	s.app.Get("/status", s.detailedStatus)

	// API v1
	v1 := s.app.Group("/api/v1")

	// 用户相关
	v1.Get("/user/:id", s.getUser)

	// 统计
	v1.Get("/stats", s.getStats)
	v1.Get("/stats/users", s.getUserStats)
	v1.Get("/stats/media", s.getMediaStats)

	// Webhook
	webhook := v1.Group("/webhook")
	webhook.Post("/emby", s.embyWebhook)
	webhook.Post("/favorites", s.favoritesWebhook)
}

// Start 启动服务器
func (s *Server) Start() error {
	if !s.cfg.Enabled {
		pkglogger.Info().Msg("【API服务】未启用，跳过...")
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	pkglogger.Info().Str("addr", addr).Msg("【API服务】启动中...")

	return s.app.Listen(addr)
}

// Stop 停止服务器
func (s *Server) Stop() error {
	return s.app.Shutdown()
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
}

// healthCheck 健康检查
func (s *Server) healthCheck(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
		Uptime:    time.Since(s.startTime).Round(time.Second).String(),
	})
}

// StatusResponse 详细状态响应
type StatusResponse struct {
	Status   string         `json:"status"`
	Version  string         `json:"version"`
	Uptime   string         `json:"uptime"`
	System   SystemInfo     `json:"system"`
	Database DatabaseStatus `json:"database"`
	Emby     EmbyStatus     `json:"emby"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	GoVersion    string `json:"go_version"`
	NumCPU       int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	MemAlloc     string `json:"mem_alloc"`
}

// DatabaseStatus 数据库状态
type DatabaseStatus struct {
	Connected bool  `json:"connected"`
	UserCount int64 `json:"user_count"`
}

// EmbyStatus Emby 状态
type EmbyStatus struct {
	Connected  bool   `json:"connected"`
	URL        string `json:"url"`
	PlayingNow int    `json:"playing_now"`
}

// detailedStatus 详细状态
func (s *Server) detailedStatus(c *fiber.Ctx) error {
	// 系统信息
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 数据库状态
	dbConnected := false
	var userCount int64
	if db := database.GetDB(); db != nil {
		sqlDB, err := db.DB()
		if err == nil && sqlDB.Ping() == nil {
			dbConnected = true
			repo := repository.NewEmbyRepository()
			userCount, _, _, _ = repo.CountStats()
		}
	}

	// Emby 状态
	embyConnected := false
	playingNow := 0
	cfg := config.Get()
	if embyClient := emby.GetClient(); embyClient != nil {
		if count, err := embyClient.GetCurrentPlayingCount(); err == nil {
			embyConnected = true
			playingNow = count
		}
	}

	return c.JSON(StatusResponse{
		Status:  "ok",
		Version: "2.0.0",
		Uptime:  time.Since(s.startTime).Round(time.Second).String(),
		System: SystemInfo{
			GoVersion:    runtime.Version(),
			NumCPU:       runtime.NumCPU(),
			NumGoroutine: runtime.NumGoroutine(),
			MemAlloc:     fmt.Sprintf("%.2f MB", float64(memStats.Alloc)/1024/1024),
		},
		Database: DatabaseStatus{
			Connected: dbConnected,
			UserCount: userCount,
		},
		Emby: EmbyStatus{
			Connected:  embyConnected,
			URL:        cfg.Emby.URL,
			PlayingNow: playingNow,
		},
	})
}

// StatsResponse 统计响应
type StatsResponse struct {
	TotalUsers     int64 `json:"total_users"`
	EmbyUsers      int64 `json:"emby_users"`
	WhitelistUsers int64 `json:"whitelist_users"`
	PlayingNow     int   `json:"playing_now"`
}

// getStats 获取统计
func (s *Server) getStats(c *fiber.Ctx) error {
	repo := repository.NewEmbyRepository()
	total, withEmby, whitelist, err := repo.CountStats()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "获取统计失败",
		})
	}

	playingNow := 0
	if embyClient := emby.GetClient(); embyClient != nil {
		if count, err := embyClient.GetCurrentPlayingCount(); err == nil {
			playingNow = count
		}
	}

	return c.JSON(StatsResponse{
		TotalUsers:     total,
		EmbyUsers:      withEmby,
		WhitelistUsers: whitelist,
		PlayingNow:     playingNow,
	})
}

// getUserStats 获取用户统计
func (s *Server) getUserStats(c *fiber.Ctx) error {
	repo := repository.NewEmbyRepository()
	total, withEmby, whitelist, err := repo.CountStats()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "获取统计失败",
		})
	}

	return c.JSON(fiber.Map{
		"total":     total,
		"with_emby": withEmby,
		"whitelist": whitelist,
	})
}

// getMediaStats 获取媒体统计
func (s *Server) getMediaStats(c *fiber.Ctx) error {
	embyClient := emby.GetClient()
	if embyClient == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Emby 服务不可用",
		})
	}

	counts, err := embyClient.GetMediaCounts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"movies":   counts.Movies,
		"series":   counts.Series,
		"episodes": counts.Episodes,
		"songs":    counts.Songs,
	})
}

// UserResponse 用户响应
type UserResponse struct {
	TG       int64   `json:"tg"`
	Name     *string `json:"name"`
	EmbyID   *string `json:"emby_id"`
	Level    string  `json:"level"`
	Score    int     `json:"score"`
	ExpiryAt *string `json:"expiry_at"`
}

// getUser 获取用户信息
func (s *Server) getUser(c *fiber.Ctx) error {
	idStr := c.Params("id")

	// 尝试解析为 TG ID
	tgID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "无效的用户ID",
		})
	}

	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(tgID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "用户不存在",
		})
	}

	var expiryAt *string
	if user.Ex != nil {
		t := user.Ex.Format("2006-01-02 15:04:05")
		expiryAt = &t
	}

	return c.JSON(UserResponse{
		TG:       user.TG,
		Name:     user.Name,
		EmbyID:   user.EmbyID,
		Level:    user.GetLevelName(),
		Score:    user.Us,
		ExpiryAt: expiryAt,
	})
}

// EmbyWebhookPayload Emby Webhook 载荷
type EmbyWebhookPayload struct {
	Event     string `json:"Event"`
	User      string `json:"User"`
	ItemName  string `json:"ItemName"`
	ItemType  string `json:"ItemType"`
	ClientIP  string `json:"ClientIP"`
	DeviceID  string `json:"DeviceId"`
	Device    string `json:"DeviceName"`
	Client    string `json:"Client"`
	SessionID string `json:"SessionId"`
}

// embyWebhook 处理 Emby Webhook
func (s *Server) embyWebhook(c *fiber.Ctx) error {
	var payload EmbyWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		pkglogger.Warn().Err(err).Msg("解析 Emby Webhook 失败")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "无效的请求体",
		})
	}

	pkglogger.Debug().
		Str("event", payload.Event).
		Str("user", payload.User).
		Str("item", payload.ItemName).
		Str("client", payload.Client).
		Str("ip", payload.ClientIP).
		Msg("收到 Emby Webhook")

	// 根据事件类型处理
	switch payload.Event {
	case "playback.start":
		// 可以在这里检查客户端限制等
		pkglogger.Info().
			Str("user", payload.User).
			Str("item", payload.ItemName).
			Msg("用户开始播放")

	case "playback.stop":
		pkglogger.Info().
			Str("user", payload.User).
			Str("item", payload.ItemName).
			Msg("用户停止播放")

	case "user.authenticated":
		pkglogger.Info().
			Str("user", payload.User).
			Str("ip", payload.ClientIP).
			Msg("用户登录")
	}

	return c.SendStatus(fiber.StatusOK)
}

// FavoritesWebhookPayload 收藏 Webhook 载荷
type FavoritesWebhookPayload struct {
	UserID   string `json:"user_id"`
	ItemID   string `json:"item_id"`
	ItemName string `json:"item_name"`
	Action   string `json:"action"` // add / remove
}

// favoritesWebhook 处理收藏 Webhook
func (s *Server) favoritesWebhook(c *fiber.Ctx) error {
	var payload FavoritesWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		pkglogger.Warn().Err(err).Msg("解析收藏 Webhook 失败")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "无效的请求体",
		})
	}

	pkglogger.Debug().
		Str("user_id", payload.UserID).
		Str("item_id", payload.ItemID).
		Str("action", payload.Action).
		Msg("收到收藏 Webhook")

	// 暂时只记录日志，不做额外处理
	// 如需同步收藏到其他用户，可以在这里实现

	return c.SendStatus(fiber.StatusOK)
}
