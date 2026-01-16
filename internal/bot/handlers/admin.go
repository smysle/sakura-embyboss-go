// Package handlers 管理员命令处理器
package handlers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// KK /kk 用户管理命令
// 支持: /kk <用户ID/用户名/@mention> 或回复消息 /kk
func KK(c tele.Context) error {
	args := c.Args()
	
	var target string

	// 检查是否是回复消息
	if c.Message().ReplyTo != nil && c.Message().ReplyTo.Sender != nil {
		// 回复消息模式：/kk
		target = strconv.FormatInt(c.Message().ReplyTo.Sender.ID, 10)
	} else if len(args) > 0 {
		// 普通模式：/kk <用户ID/用户名/@mention>
		target = args[0]
	} else {
		return c.Send("用法: /kk <用户ID/用户名/@mention>\n\n或回复某人消息后发送:\n/kk")
	}

	// 处理 @ 提及
	if strings.HasPrefix(target, "@") {
		target = strings.TrimPrefix(target, "@")
	}

	repo := repository.NewEmbyRepository()

	// 尝试解析为数字
	if tgID, err := strconv.ParseInt(target, 10, 64); err == nil {
		embyUser, err := repo.GetByTG(tgID)
		if err != nil {
			return c.Send("❌ 未找到该用户")
		}
		return showUserInfo(c, embyUser)
	}

	// 尝试按名称查找
	embyUser, err := repo.GetByName(target)
	if err != nil {
		return c.Send("❌ 未找到该用户")
	}
	return showUserInfo(c, embyUser)
}

func showUserInfo(c tele.Context, user *models.Emby) error {
	cfg := config.Get()

	var expiryText string
	if user.Ex != nil {
		days := user.DaysUntilExpiry()
		if days < 0 {
			expiryText = fmt.Sprintf("**已过期 %d 天**", -days)
		} else {
			expiryText = fmt.Sprintf("%s (%d天后)", user.Ex.Format("2006-01-02"), days)
		}
	} else {
		expiryText = "未设置"
	}

	text := fmt.Sprintf(
		"👤 **用户管理**\n\n"+
			"**· TG ID** | `%d`\n"+
			"**· 用户名** | %s\n"+
			"**· Emby ID** | %s\n"+
			"**· 等级** | %s\n"+
			"**· 积分** | %d %s\n"+
			"**· 到期时间** | %s\n"+
			"**· 邀请次数** | %d\n",
		user.TG,
		getEmbyName(user.Name),
		getEmbyID(user.EmbyID),
		user.GetLevelName(),
		user.Us, cfg.Money,
		expiryText,
		user.Iv,
	)

	// 检查是否配置了额外媒体库
	hasExtraLibs := len(cfg.Emby.ExtraLibs) > 0

	// 检查用户额外媒体库状态
	extraLibsEnabled := false
	hasEmby := user.EmbyID != nil && *user.EmbyID != ""
	isBanned := user.Lv == "e" // 'e' 等级表示被封禁
	
	if hasExtraLibs && hasEmby {
		client := emby.GetClient()
		if embyUser, err := client.GetUser(*user.EmbyID); err == nil && embyUser.Policy != nil {
			// 如果额外库不在阻止列表中，则认为已启用
			extraLibsEnabled = true
			for _, blocked := range embyUser.Policy.BlockedFolders {
				for _, extraLib := range cfg.Emby.ExtraLibs {
					if blocked == extraLib {
						extraLibsEnabled = false
						break
					}
				}
				if !extraLibsEnabled {
					break
				}
			}
		}
	}

	return c.Send(text, keyboards.UserManageKeyboard(user.TG, hasExtraLibs, extraLibsEnabled, isBanned, hasEmby), tele.ModeMarkdown)
}

func getEmbyID(id *string) string {
	if id == nil || *id == "" {
		return "未绑定"
	}
	return fmt.Sprintf("`%s`", *id)
}

// Score /score 积分命令
// 支持: /score <用户ID/@用户名> <积分> 或回复消息 /score <积分>
func Score(c tele.Context) error {
	args := c.Args()
	
	var tgID int64
	var scoreStr string
	var err error

	// 检查是否是回复消息
	if c.Message().ReplyTo != nil && c.Message().ReplyTo.Sender != nil {
		// 回复消息模式：/score <积分>
		if len(args) < 1 {
			return c.Send("用法: 回复消息后发送 /score <+/-积分>\n\n例如: /score 100")
		}
		tgID = c.Message().ReplyTo.Sender.ID
		scoreStr = args[0]
	} else {
		// 普通模式：/score <用户ID/@用户名> <积分>
		if len(args) < 2 {
			return c.Send("用法: /score <用户ID/@用户名> <+/-积分>\n\n例如:\n/score 123456789 100\n/score @username 100\n\n或回复某人消息后发送:\n/score 100")
		}
		
		// 支持 @username 格式
		target := args[0]
		if strings.HasPrefix(target, "@") {
			// 通过用户名查找
			username := strings.TrimPrefix(target, "@")
			repo := repository.NewEmbyRepository()
			user, err := repo.GetByName(username)
			if err != nil {
				return c.Send(fmt.Sprintf("❌ 未找到用户名为 %s 的用户", target))
			}
			tgID = user.TG
		} else {
			tgID, err = strconv.ParseInt(target, 10, 64)
			if err != nil {
				return c.Send("❌ 无效的用户ID\n\n支持格式: 用户ID 或 @用户名")
			}
		}
		scoreStr = args[1]
	}

	score, err := strconv.Atoi(scoreStr)
	if err != nil {
		return c.Send("❌ 无效的积分值")
	}

	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(tgID)
	if err != nil {
		return c.Send("❌ 未找到该用户")
	}

	newScore := user.Us + score
	if newScore < 0 {
		newScore = 0
	}

	if err := repo.UpdateFields(tgID, map[string]interface{}{"us": newScore}); err != nil {
		return c.Send("❌ 更新积分失败")
	}

	userName := "未知"
	if user.Name != nil {
		userName = *user.Name
	}

	cfg := config.Get()
	return c.Send(fmt.Sprintf("✅ 用户 %s (ID: %d) 积分已更新: %d -> %d %s", userName, tgID, user.Us, newScore, cfg.Money))
}

// Coins /coins 花币命令（同 Score）
func Coins(c tele.Context) error {
	return Score(c)
}

// Renew /renew 续期命令
// 支持: /renew <用户ID/@用户名> <天数> 或回复消息 /renew <天数>
func Renew(c tele.Context) error {
	args := c.Args()
	
	var tgID int64
	var daysStr string
	var err error

	// 检查是否是回复消息
	if c.Message().ReplyTo != nil && c.Message().ReplyTo.Sender != nil {
		// 回复消息模式：/renew <天数>
		if len(args) < 1 {
			return c.Send("用法: 回复消息后发送 /renew <+/-天数>\n\n例如: /renew 30")
		}
		tgID = c.Message().ReplyTo.Sender.ID
		daysStr = args[0]
	} else {
		// 普通模式：/renew <用户ID/@用户名> <天数>
		if len(args) < 2 {
			return c.Send("用法: /renew <用户ID/@用户名> <+/-天数>\n\n例如:\n/renew 123456789 30\n/renew @username 30\n\n或回复某人消息后发送:\n/renew 30")
		}
		
		// 支持 @username 格式
		target := args[0]
		if strings.HasPrefix(target, "@") {
			username := strings.TrimPrefix(target, "@")
			repo := repository.NewEmbyRepository()
			user, err := repo.GetByName(username)
			if err != nil {
				return c.Send(fmt.Sprintf("❌ 未找到用户名为 %s 的用户", target))
			}
			tgID = user.TG
		} else {
			tgID, err = strconv.ParseInt(target, 10, 64)
			if err != nil {
				return c.Send("❌ 无效的用户ID\n\n支持格式: 用户ID 或 @用户名")
			}
		}
		daysStr = args[1]
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil {
		return c.Send("❌ 无效的天数")
	}

	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(tgID)
	if err != nil {
		return c.Send("❌ 未找到该用户")
	}

	var newExpiry time.Time
	if user.Ex != nil {
		newExpiry = user.Ex.AddDate(0, 0, days)
	} else {
		newExpiry = time.Now().AddDate(0, 0, days)
	}

	if err := repo.UpdateFields(tgID, map[string]interface{}{"ex": newExpiry}); err != nil {
		return c.Send("❌ 更新到期时间失败")
	}

	userName := "未知"
	if user.Name != nil {
		userName = *user.Name
	}

	return c.Send(fmt.Sprintf("✅ 用户 %s (ID: %d) 到期时间已更新为: %s", userName, tgID, newExpiry.Format("2006-01-02 15:04:05")))
}

// RemoveEmby /rmemby 删除用户命令
func RemoveEmby(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /rmemby <用户ID/Emby用户名>")
	}

	target := args[0]
	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	var user *models.Emby
	var err error

	// 尝试解析为数字
	if tgID, parseErr := strconv.ParseInt(target, 10, 64); parseErr == nil {
		user, err = repo.GetByTG(tgID)
	} else {
		user, err = repo.GetByName(target)
	}

	if err != nil {
		return c.Send("❌ 未找到该用户")
	}

	// 删除 Emby 账户
	if user.EmbyID != nil && *user.EmbyID != "" {
		if err := client.DeleteUser(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Str("embyID", *user.EmbyID).Msg("删除 Emby 账户失败")
		}
	}

	// 删除数据库记录
	if err := repo.Delete(user.TG); err != nil {
		return c.Send("❌ 删除数据库记录失败")
	}

	return c.Send(fmt.Sprintf("✅ 已删除用户 %d (%s)", user.TG, getEmbyName(user.Name)))
}

// ProUser /prouser 添加白名单
func ProUser(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /prouser <用户ID>")
	}

	tgID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的用户ID")
	}

	repo := repository.NewEmbyRepository()
	if err := repo.UpdateFields(tgID, map[string]interface{}{"lv": models.LevelA}); err != nil {
		return c.Send("❌ 设置白名单失败")
	}

	return c.Send(fmt.Sprintf("✅ 用户 %d 已设为白名单", tgID))
}

// RevUser /revuser 取消白名单
func RevUser(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /revuser <用户ID>")
	}

	tgID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的用户ID")
	}

	repo := repository.NewEmbyRepository()
	if err := repo.UpdateFields(tgID, map[string]interface{}{"lv": models.LevelD}); err != nil {
		return c.Send("❌ 取消白名单失败")
	}

	return c.Send(fmt.Sprintf("✅ 用户 %d 已取消白名单", tgID))
}

// CheckExpired /check_ex 检查过期用户
func CheckExpired(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	expiredUsers, err := repo.GetExpiredUsers()
	if err != nil {
		return c.Send("❌ 查询过期用户失败")
	}

	if len(expiredUsers) == 0 {
		return c.Send("✅ 没有过期用户")
	}

	text := fmt.Sprintf("📋 **过期用户列表** (%d人)\n\n", len(expiredUsers))
	for i, u := range expiredUsers {
		if i >= 20 {
			text += fmt.Sprintf("\n... 还有 %d 人", len(expiredUsers)-20)
			break
		}
		text += fmt.Sprintf("%d. `%d` - %s\n", i+1, u.TG, getEmbyName(u.Name))
	}

	return c.Send(text, keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// UserRanks /uranks 用户观影排行
func UserRanks(c tele.Context) error {
	c.Send("⏳ 正在生成用户播放排行...")

	leaderboardSvc := service.NewLeaderboardService()
	stats, err := leaderboardSvc.GetUserPlayStats(20)
	if err != nil {
		logger.Error().Err(err).Msg("获取用户播放统计失败")
		return c.Send("❌ 获取播放统计失败: " + err.Error())
	}

	if len(stats) == 0 {
		return c.Send("📊 暂无播放数据")
	}

	text := "📊 **用户播放排行**\n\n"
	for i, stat := range stats {
		text += fmt.Sprintf("%d. **%s** - %.1f 小时\n", i+1, stat.UserName, stat.TotalHours)
	}

	return c.Send(text, keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// DayRanks /days_ranks 日榜
func DayRanks(c tele.Context) error {
	c.Send("⏳ 正在生成日榜...")

	leaderboardSvc := service.NewLeaderboardService()
	imgPath, err := leaderboardSvc.GenerateDailyRank()
	if err != nil {
		logger.Error().Err(err).Msg("生成日榜失败")
		return c.Send("❌ 生成日榜失败: " + err.Error())
	}

	// 发送图片
	photo := &tele.Photo{File: tele.FromDisk(imgPath)}
	return c.Send(photo)
}

// WeekRanks /week_ranks 周榜
func WeekRanks(c tele.Context) error {
	c.Send("⏳ 正在生成周榜...")

	leaderboardSvc := service.NewLeaderboardService()
	imgPath, err := leaderboardSvc.GenerateWeeklyRank()
	if err != nil {
		logger.Error().Err(err).Msg("生成周榜失败")
		return c.Send("❌ 生成周榜失败: " + err.Error())
	}

	// 发送图片
	photo := &tele.Photo{File: tele.FromDisk(imgPath)}
	return c.Send(photo)
}

// Restart /restart 重启 Bot
func Restart(c tele.Context) error {
	c.Send("🔄 Bot 正在重启...")

	// 使用 SIGHUP 信号重启（需要外部进程管理器支持）
	logger.Info().Int64("by", c.Sender().ID).Msg("收到重启命令")

	// 发送信号给自己
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return c.Send("❌ 重启失败: " + err.Error())
	}

	// 发送 SIGTERM 信号，让外部管理器（如 Docker、systemd）重启
	go func() {
		time.Sleep(1 * time.Second)
		p.Signal(syscall.SIGTERM)
	}()

	return nil
}

// UpdateBot /update_bot 更新 Bot
func UpdateBot(c tele.Context) error {
	// 在容器化部署中，更新通常由 CI/CD 处理
	return c.Send(
		"📥 **更新说明**\n\n" +
			"本 Bot 使用 Docker 容器化部署，更新方式：\n\n" +
			"1. 推送代码到 GitHub\n" +
			"2. GitHub Actions 自动构建镜像\n" +
			"3. 在服务器执行 `docker-compose pull && docker-compose up -d`\n\n" +
			"或使用 Watchtower 自动更新",
		tele.ModeMarkdown,
	)
}
