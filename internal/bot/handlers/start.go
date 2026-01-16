// Package handlers Bot 命令处理器
package handlers

import (
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Start /start 命令处理器
func Start(c tele.Context) error {
	cfg := config.Get()
	user := c.Sender()

	// 检查是否在群组
	if c.Chat().Type != tele.ChatPrivate {
		return c.Send(
			fmt.Sprintf("🤖 亲爱的 [%s](tg://user?id=%d) 这是一条私聊命令", user.FirstName, user.ID),
			keyboards.JoinGroupKeyboard(),
			tele.ModeMarkdown,
		)
	}

	// 处理 /start 参数（如注册码）
	args := c.Args()
	if len(args) > 0 {
		return handleStartArgs(c, args[0])
	}

	// 确保用户存在于数据库
	repo := repository.NewEmbyRepository()
	embyUser, err := repo.EnsureExists(user.ID)
	if err != nil {
		logger.Error().Err(err).Int64("tg", user.ID).Msg("创建用户记录失败")
		return c.Send("❌ 系统错误，请稍后重试")
	}

	// 构建欢迎消息
	isAdmin := cfg.IsAdmin(user.ID)
	hasAccount := embyUser.HasEmbyAccount()

	var text string
	var keyboard *tele.ReplyMarkup

	if hasAccount {
		text = fmt.Sprintf(
			"**✨ 只有你想见我的时候我们的相遇才有意义**\n\n"+
				"🍉__你好鸭 [%s](tg://user?id=%d) 请选择功能__👇",
			user.FirstName, user.ID,
		)
		keyboard = keyboards.StartPanelKeyboardWithAccount(isAdmin)
	} else {
		// 获取开放注册信息
		statText := "❌ 关闭"
		if cfg.Open.Status {
			statText = "✅ 开放"
		}
		remaining := cfg.Open.MaxUsers - cfg.Open.Temp

		text = fmt.Sprintf(
			"▎__欢迎进入用户面板！%s__\n\n"+
				"**· 🆔 用户のID** | `%d`\n"+
				"**· 📊 当前状态** | %s\n"+
				"**· 🍒 积分%s** | %d\n"+
				"**· ®️ 注册状态** | %s\n"+
				"**· 🎫 总注册限制** | %d\n"+
				"**· 🎟️ 可注册席位** | %d\n",
			user.FirstName, user.ID,
			embyUser.GetLevelName(),
			cfg.Money, embyUser.Us,
			statText,
			cfg.Open.MaxUsers,
			remaining,
		)
		keyboard = keyboards.StartPanelKeyboard(isAdmin)
	}

	// 发送带图片的消息
	if cfg.BotPhoto != "" {
		photo := &tele.Photo{File: tele.FromURL(cfg.BotPhoto)}
		photo.Caption = text
		return c.Send(photo, keyboard, tele.ModeMarkdown)
	}

	return c.Send(text, keyboard, tele.ModeMarkdown)
}

// handleStartArgs 处理 /start 参数
func handleStartArgs(c tele.Context, arg string) error {
	cfg := config.Get()

	// 检查是否是用户IP查询（管理员功能）
	if strings.HasPrefix(arg, "userip-") {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ 您没有权限执行此操作")
		}
		name := strings.TrimPrefix(arg, "userip-")
		return handleUserIP(c, name)
	}

	// 检查是否是注册码
	if strings.HasPrefix(arg, "SAKURA-") || strings.HasPrefix(arg, cfg.BotName) {
		return handleRegisterCode(c, arg)
	}

	return c.Send("🤺 你也想和bot击剑吗 ?")
}

// handleUserIP 处理用户IP查询
func handleUserIP(c tele.Context, name string) error {
	// 获取用户的 Emby 会话信息
	client := emby.GetClient()
	user, err := client.GetUserByName(name)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ 未找到用户 %s", name))
	}

	// 获取会话信息（需要 Emby API 支持）
	text := fmt.Sprintf(
		"👤 **用户 IP 信息**\n\n"+
			"**用户名**: %s\n"+
			"**用户ID**: `%s`\n\n"+
			"_注：详细 IP 信息需要查看 Emby 后台_",
		name, user.ID,
	)

	return c.Send(text, keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// handleRegisterCode 处理注册码
func handleRegisterCode(c tele.Context, code string) error {
	// 设置会话状态
	sessionMgr := session.GetManager()
	sessionMgr.SetState(c.Sender().ID, session.StateWaitingCode)
	sessionMgr.SetData(c.Sender().ID, "pending_code", code)

	// 验证注册码
	codeSvc := service.NewCodeService()
	days, err := codeSvc.ValidateCode(code)
	if err != nil {
		sessionMgr.ClearSession(c.Sender().ID)
		return c.Send(fmt.Sprintf("❌ %s", err.Error()))
	}

	// 检查用户是否已有账户
	repo := repository.NewEmbyRepository()
	user, _ := repo.GetByTG(c.Sender().ID)

	if user != nil && user.HasEmbyAccount() {
		// 已有账户，直接续期
		addedDays, err := codeSvc.ExtendByCode(c.Sender().ID, code)
		sessionMgr.ClearSession(c.Sender().ID)

		if err != nil {
			return c.Send(fmt.Sprintf("❌ 续期失败: %s", err.Error()))
		}

		return c.Send(
			fmt.Sprintf(
				"✅ **续期成功！**\n\n"+
					"🎁 已增加 **%d** 天有效期",
				addedDays,
			),
			keyboards.BackKeyboard("back_start"),
			tele.ModeMarkdown,
		)
	}

	// 没有账户，需要输入用户名
	sessionMgr.SetState(c.Sender().ID, session.StateWaitingName)
	sessionMgr.SetData(c.Sender().ID, "code", code)
	sessionMgr.SetData(c.Sender().ID, "days", days)

	return c.Send(
		"✅ 注册码验证成功！\n\n"+
			"📝 请输入您想要的 **Emby 用户名**\n"+
			"（仅支持英文字母和数字）\n\n"+
			"_发送 /cancel 取消操作_",
		tele.ModeMarkdown,
	)
}

// MyInfo /myinfo 命令处理器
func MyInfo(c tele.Context) error {
	user := c.Sender()
	cfg := config.Get()

	// 先删除用户的命令消息
	if c.Message() != nil {
		go c.Bot().Delete(c.Message())
	}

	repo := repository.NewEmbyRepository()
	embyUser, err := repo.GetByTG(user.ID)
	if err != nil {
		msg, _ := c.Bot().Send(c.Chat(), "❌ 未找到您的账户信息，请先 /start", tele.ModeMarkdown)
		// 60秒后删除
		go func() {
			time.Sleep(60 * time.Second)
			c.Bot().Delete(msg)
		}()
		return nil
	}

	var expiryText string
	if embyUser.Ex != nil {
		days := embyUser.DaysUntilExpiry()
		if days < 0 {
			expiryText = "**已过期**"
		} else {
			expiryText = fmt.Sprintf("%d 天后", days)
		}
	} else {
		expiryText = "永久"
	}

	// 构建格式化文本（与 Python 版本一致）
	text := fmt.Sprintf(
		"**· 🍉 TG&名称** | [%s](tg://user?id=%d)\n"+
			"**· 🍒 识别のID** | `%d`\n"+
			"**· 🍓 当前状态** | %s\n"+
			"**· 🍥 持有%s** | %d\n"+
			"**· 💠 账号名称** | %s\n"+
			"**· 🚨 到期时间** | **%s**\n",
		user.FirstName, user.ID,
		user.ID,
		embyUser.GetLevelName(),
		cfg.Money, embyUser.Us,
		getEmbyName(embyUser.Name),
		expiryText,
	)

	markup := &tele.ReplyMarkup{}
	closeBtn := markup.Data("❌ 删除消息", "closeit")
	markup.Inline(
		markup.Row(closeBtn),
	)

	// 发送消息并60秒后自动删除
	msg, err := c.Bot().Send(c.Chat(), text, markup, tele.ModeMarkdown)
	if err != nil {
		return err
	}

	// 60秒后自动删除
	go func() {
		time.Sleep(60 * time.Second)
		c.Bot().Delete(msg)
	}()

	return nil
}

func getEmbyName(name *string) string {
	if name == nil || *name == "" {
		return "未绑定"
	}
	return *name
}

// Count /count 命令处理器
func Count(c tele.Context) error {
	client := emby.GetClient()
	counts, err := client.GetMediaCounts()
	if err != nil {
		logger.Error().Err(err).Msg("获取媒体统计失败")
		return c.Send("🤕 Emby 服务器连接失败!")
	}

	return c.Send(counts.FormatText(), keyboards.CloseKeyboard())
}

// RedEnvelope /red 命令处理器 - 转发到红包处理器
func RedEnvelope(c tele.Context) error {
	return HandleRedEnvelope(c)
}

// ScoreRank /srank 命令处理器
func ScoreRank(c tele.Context) error {
	cfg := config.Get()
	repo := repository.NewEmbyRepository()

	// 获取积分排行榜前 20 名
	users, err := repo.GetTopByScore(20)
	if err != nil {
		logger.Error().Err(err).Msg("获取积分排行失败")
		return c.Send("❌ 获取积分排行失败")
	}

	if len(users) == 0 {
		return c.Send("📊 暂无积分数据")
	}

	text := fmt.Sprintf("🏆 **%s 排行榜**\n\n", cfg.Money)

	medals := []string{"🥇", "🥈", "🥉"}
	for i, u := range users {
		var prefix string
		if i < 3 {
			prefix = medals[i]
		} else {
			prefix = fmt.Sprintf("%d.", i+1)
		}

		userName := "匿名用户"
		if u.Name != nil && *u.Name != "" {
			userName = *u.Name
		}

		text += fmt.Sprintf("%s **%s** - %d %s\n", prefix, userName, u.Us, cfg.Money)
	}

	return c.Send(text, keyboards.CloseKeyboard(), tele.ModeMarkdown)
}
