// Package handlers 消息处理器
package handlers

import (
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// OnText 处理文本消息
func OnText(c tele.Context) error {
	// 只处理私聊消息
	if c.Chat().Type != tele.ChatPrivate {
		return nil
	}

	text := strings.TrimSpace(c.Text())
	userID := c.Sender().ID

	// 检查用户会话状态
	sessionMgr := session.GetManager()
	state := sessionMgr.GetState(userID)

	switch state {
	case session.StateWaitingCode:
		return handleCodeInput(c, text)
	case session.StateWaitingName:
		return handleNameInput(c, text)
	case session.StateMoviePilotSearch:
		return HandleMoviePilotSearchInput(c)
	case session.StateMoviePilotSelectMedia:
		return HandleMPSelectDownload(c)
	default:
		// 没有特殊状态，忽略消息
		return nil
	}
}

// Cancel /cancel 取消当前操作
func Cancel(c tele.Context) error {
	sessionMgr := session.GetManager()
	sessionMgr.ClearSession(c.Sender().ID)

	return c.Send("✅ 已取消操作\n\n发送 /start 返回主菜单")
}

// handleCodeInput 处理注册码输入
func handleCodeInput(c tele.Context, code string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 验证注册码格式
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return c.Send("❌ 请输入有效的注册码")
	}

	// 先验证注册码是否有效
	codeSvc := service.NewCodeService()
	days, err := codeSvc.ValidateCode(code)
	if err != nil {
		sessionMgr.ClearSession(userID)
		return c.Send(fmt.Sprintf("❌ %s\n\n发送 /start 返回主菜单", err.Error()))
	}

	// 检查用户是否已有账户
	repo := repository.NewEmbyRepository()
	user, _ := repo.GetByTG(userID)

	if user != nil && user.HasEmbyAccount() {
		// 已有账户，直接续期
		addedDays, err := codeSvc.ExtendByCode(userID, code)
		sessionMgr.ClearSession(userID)

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
	sessionMgr.SetState(userID, session.StateWaitingName)
	sessionMgr.SetData(userID, "code", code)
	sessionMgr.SetData(userID, "days", days)

	return c.Send(
		"✅ 注册码验证成功！\n\n"+
			"📝 请输入您想要的 **Emby 用户名**\n"+
			"（仅支持英文字母和数字）\n\n"+
			"_发送 /cancel 取消操作_",
		tele.ModeMarkdown,
	)
}

// handleNameInput 处理用户名输入
func handleNameInput(c tele.Context, username string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 验证用户名格式
	username = strings.TrimSpace(username)
	if !isValidUsername(username) {
		return c.Send("❌ 用户名格式无效\n\n请使用 3-20 位英文字母和数字")
	}

	// 获取之前保存的注册码
	codeVal, ok := sessionMgr.GetData(userID, "code")
	if !ok {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 会话已过期，请重新操作\n\n发送 /start 返回主菜单")
	}
	code := codeVal.(string)

	// 使用注册码创建账户
	codeSvc := service.NewCodeService()
	result, err := codeSvc.UseCode(userID, username, code)

	// 清除会话
	sessionMgr.ClearSession(userID)

	if err != nil {
		logger.Error().Err(err).Int64("tg", userID).Str("code", code).Msg("使用注册码失败")
		return c.Send(fmt.Sprintf("❌ 创建账户失败: %s", err.Error()))
	}

	cfg := config.Get()
	text := fmt.Sprintf(
		"🎉 **账户创建成功！**\n\n"+
			"**用户名**: `%s`\n"+
			"**密码**: `%s`\n"+
			"**有效期**: %d 天\n"+
			"**到期时间**: %s\n\n"+
			"🔗 **登录地址**: %s\n\n"+
			"_请妥善保管您的账户信息_",
		result.Username,
		result.Password,
		result.Days,
		result.ExpiryDate.Format("2006-01-02"),
		cfg.Emby.Line,
	)

	return c.Send(text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
}

// isValidUsername 验证用户名格式
func isValidUsername(username string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}

	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}
