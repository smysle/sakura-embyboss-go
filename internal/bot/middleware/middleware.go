// Package middleware Bot 中间件
package middleware

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Logger 日志中间件
func Logger() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := c.Sender()
			if user != nil {
				logger.Debug().
					Int64("user_id", user.ID).
					Str("username", user.Username).
					Str("text", c.Text()).
					Msg("收到消息")
			}
			return next(c)
		}
	}
}

// Recover 恢复中间件
func Recover() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			defer func() {
				if r := recover(); r != nil {
					logger.Error().
						Interface("panic", r).
						Str("stack", string(debug.Stack())).
						Msg("处理器 panic")

					c.Send("❌ 处理请求时发生错误，请稍后重试")
				}
			}()
			return next(c)
		}
	}
}

// AdminOnly 管理员权限中间件
func AdminOnly() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			cfg := config.Get()
			if cfg == nil {
				return c.Send("❌ 配置加载失败")
			}

			user := c.Sender()
			if user == nil {
				return c.Send("❌ 无法获取用户信息")
			}

			if !cfg.IsAdmin(user.ID) {
				return c.Send("❌ 您没有权限执行此操作")
			}

			return next(c)
		}
	}
}

// OwnerOnly Owner 权限中间件
func OwnerOnly() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			cfg := config.Get()
			if cfg == nil {
				return c.Send("❌ 配置加载失败")
			}

			user := c.Sender()
			if user == nil {
				return c.Send("❌ 无法获取用户信息")
			}

			if !cfg.IsOwner(user.ID) {
				return c.Send("❌ 此命令仅限 Owner 使用")
			}

			return next(c)
		}
	}
}

// GroupOnly 群组中间件
func GroupOnly() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			chat := c.Chat()
			if chat == nil || (chat.Type != tele.ChatGroup && chat.Type != tele.ChatSuperGroup) {
				return c.Send("❌ 此命令仅可在群组中使用")
			}
			return next(c)
		}
	}
}

// PrivateOnly 私聊中间件
func PrivateOnly() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			chat := c.Chat()
			if chat == nil || chat.Type != tele.ChatPrivate {
				return c.Send("❌ 此命令仅可在私聊中使用")
			}
			return next(c)
		}
	}
}

// InGroup 检查用户是否在群组中
func InGroup() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			cfg := config.Get()
			if cfg == nil {
				return c.Send("❌ 配置加载失败")
			}

			user := c.Sender()
			if user == nil {
				return c.Send("❌ 无法获取用户信息")
			}

			// 检查用户是否在配置的群组中
			for _, groupID := range cfg.Groups {
				member, err := c.Bot().ChatMemberOf(&tele.Chat{ID: groupID}, user)
				if err != nil {
					continue
				}

				if member.Role != tele.Left && member.Role != tele.Kicked {
					return next(c)
				}
			}

			// 发送加入群组提示
			return c.Send(fmt.Sprintf(
				"💢 请先加入我们的群组 @%s 和频道 @%s，然后再 /start",
				cfg.MainGroup, cfg.Channel,
			))
		}
	}
}

// rateLimitEntry 速率限制条目
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// rateLimiter 速率限制器
type rateLimiter struct {
	mu       sync.RWMutex
	entries  map[int64]*rateLimitEntry
	limit    int
	window   time.Duration
	lastClean time.Time
}

// newRateLimiter 创建速率限制器
func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		entries:   make(map[int64]*rateLimitEntry),
		limit:     requestsPerMinute,
		window:    time.Minute,
		lastClean: time.Now(),
	}
}

// allow 检查是否允许请求
func (rl *rateLimiter) allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 定期清理过期条目
	if now.Sub(rl.lastClean) > 5*time.Minute {
		for id, entry := range rl.entries {
			if now.After(entry.resetTime) {
				delete(rl.entries, id)
			}
		}
		rl.lastClean = now
	}

	entry, exists := rl.entries[userID]
	if !exists || now.After(entry.resetTime) {
		// 新条目或已过期，重置
		rl.entries[userID] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	// 检查是否超过限制
	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// RateLimit 速率限制中间件
func RateLimit(requestsPerMinute int) tele.MiddlewareFunc {
	limiter := newRateLimiter(requestsPerMinute)

	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := c.Sender()
			if user == nil {
				return next(c)
			}

			// 管理员不受限制
			cfg := config.Get()
			if cfg != nil && cfg.IsAdmin(user.ID) {
				return next(c)
			}

			if !limiter.allow(user.ID) {
				logger.Warn().
					Int64("user_id", user.ID).
					Int("limit", requestsPerMinute).
					Msg("用户触发速率限制")

				return c.Send("⏳ 操作太频繁，请稍后再试")
			}

			return next(c)
		}
	}
}

// AntiFlood 防刷屏中间件（更严格的短时间限制）
func AntiFlood(maxPerSecond int) tele.MiddlewareFunc {
	var (
		mu       sync.RWMutex
		lastCall = make(map[int64]time.Time)
	)

	interval := time.Second / time.Duration(maxPerSecond)

	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := c.Sender()
			if user == nil {
				return next(c)
			}

			now := time.Now()

			mu.RLock()
			last, exists := lastCall[user.ID]
			mu.RUnlock()

			if exists && now.Sub(last) < interval {
				// 太快了，忽略
				return nil
			}

			mu.Lock()
			lastCall[user.ID] = now
			mu.Unlock()

			return next(c)
		}
	}
}
