// Package handlers 回调处理器
package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// OnCallback 回调查询处理器
func OnCallback(c tele.Context) error {
	data := c.Callback().Data

	// 解析回调数据
	parts := strings.Split(data, ":")
	action := parts[0]

	switch action {
	case "back_start":
		return handleBackStart(c)
	case "close":
		return handleClose(c)
	case "myinfo":
		return MyInfo(c)
	case "count":
		return Count(c)
	case "register":
		return handleRegister(c)
	case "use_code":
		return handleUseCode(c)
	case "account_info":
		return handleAccountInfo(c)
	case "reset_pwd":
		return handleResetPwd(c)
	case "checkin":
		return handleCheckin(c)
	case "admin_panel":
		return handleAdminPanel(c)
	case "set_lv":
		return handleSetLevel(c, parts)
	case "grab_red":
		// 抢红包
		if len(parts) >= 2 {
			return HandleGrabRedEnvelope(c, parts[1])
		}
		return c.Respond(&tele.CallbackResponse{Text: "无效的红包"})
	case "noop":
		return c.Respond()
	default:
		logger.Debug().Str("data", data).Msg("未知回调")
		return c.Respond(&tele.CallbackResponse{Text: "未知操作"})
	}
}

func handleBackStart(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "⭐ 返回"})
	
	cfg := config.Get()
	user := c.Sender()
	isAdmin := cfg.IsAdmin(user.ID)

	repo := repository.NewEmbyRepository()
	embyUser, _ := repo.GetByTG(user.ID)
	hasAccount := embyUser != nil && embyUser.HasEmbyAccount()

	text := fmt.Sprintf(
		"**✨ 只有你想见我的时候我们的相遇才有意义**\n\n"+
			"🍉__你好鸭 [%s](tg://user?id=%d) 请选择功能__👇",
		user.FirstName, user.ID,
	)

	var keyboard *tele.ReplyMarkup
	if hasAccount {
		keyboard = keyboards.StartPanelKeyboardWithAccount(isAdmin)
	} else {
		keyboard = keyboards.StartPanelKeyboard(isAdmin)
	}

	return c.Edit(text, keyboard, tele.ModeMarkdown)
}

func handleClose(c tele.Context) error {
	return c.Delete()
}

func handleRegister(c tele.Context) error {
	cfg := config.Get()

	// 检查注册是否开放
	if !cfg.Open.Status {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 注册暂未开放",
			ShowAlert: true,
		})
	}

	// 检查是否已有账户
	repo := repository.NewEmbyRepository()
	user, _ := repo.GetByTG(c.Sender().ID)
	if user != nil && user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 您已有账户",
			ShowAlert: true,
		})
	}

	// 检查席位
	if cfg.Open.Temp >= cfg.Open.MaxUsers {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 注册席位已满",
			ShowAlert: true,
		})
	}

	c.Respond(&tele.CallbackResponse{Text: "⏳ 正在创建账户..."})

	// 创建 Emby 账户
	client := emby.GetClient()
	result, err := client.CreateUser(c.Sender().Username, cfg.Open.Temp)
	if err != nil {
		logger.Error().Err(err).Msg("创建 Emby 账户失败")
		return c.Edit("❌ 创建账户失败，请稍后重试")
	}

	// 更新数据库
	updates := map[string]interface{}{
		"embyid": result.UserID,
		"name":   c.Sender().Username,
		"pwd":    result.Password,
		"ex":     result.ExpiryDate,
		"cr":     result.ExpiryDate.AddDate(0, 0, -cfg.Open.Temp),
	}
	repo.UpdateFields(c.Sender().ID, updates)

	// 更新临时计数
	cfg.Open.Temp++
	cfg.Save("config.json")

	text := fmt.Sprintf(
		"✅ **账户创建成功!**\n\n"+
			"**用户名**: `%s`\n"+
			"**密码**: `%s`\n"+
			"**到期时间**: %s\n\n"+
			"🔗 登录地址: %s",
		c.Sender().Username,
		result.Password,
		result.ExpiryDate.Format("2006-01-02"),
		cfg.Emby.Line,
	)

	return c.Edit(text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
}

func handleUseCode(c tele.Context) error {
	cfg := config.Get()
	if !cfg.Open.Exchange {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 兑换功能已关闭",
			ShowAlert: true,
		})
	}

	// 设置用户会话状态为等待输入注册码
	sessionMgr := session.GetManager()
	sessionMgr.SetState(c.Sender().ID, session.StateWaitingCode)

	c.Respond()
	return c.Edit(
		"🎫 **请发送您的注册码**\n\n"+
			"格式示例: `SAKURA-XXXXXXXXXXXX`\n\n"+
			"_发送 /cancel 取消操作_",
		keyboards.BackKeyboard("back_start"),
		tele.ModeMarkdown,
	)
}

func handleAccountInfo(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || !user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 您还没有账户",
			ShowAlert: true,
		})
	}

	cfg := config.Get()
	var expiryText string
	if user.Ex != nil {
		days := user.DaysUntilExpiry()
		if days < 0 {
			expiryText = "**已过期**"
		} else {
			expiryText = fmt.Sprintf("%s (%d天后)", user.Ex.Format("2006-01-02"), days)
		}
	} else {
		expiryText = "永久"
	}

	text := fmt.Sprintf(
		"👤 **账户信息**\n\n"+
			"**用户名**: `%s`\n"+
			"**密码**: ||`%s`||\n"+
			"**等级**: %s\n"+
			"**到期时间**: %s\n\n"+
			"🔗 登录地址: %s",
		getEmbyName(user.Name),
		getPassword(user.Pwd),
		user.GetLevelName(),
		expiryText,
		cfg.Emby.Line,
	)

	c.Respond()
	return c.Edit(text, keyboards.AccountInfoKeyboard(), tele.ModeMarkdown)
}

func getPassword(pwd *string) string {
	if pwd == nil || *pwd == "" {
		return "(空密码)"
	}
	return *pwd
}

func handleResetPwd(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || !user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 您还没有账户",
			ShowAlert: true,
		})
	}

	c.Respond(&tele.CallbackResponse{Text: "⏳ 正在重置密码..."})

	client := emby.GetClient()
	if err := client.ResetPassword(*user.EmbyID); err != nil {
		return c.Edit("❌ 重置密码失败")
	}

	// 更新数据库
	repo.UpdateFields(c.Sender().ID, map[string]interface{}{"pwd": nil})

	return c.Edit(
		"✅ 密码已重置为空\n\n您可以登录后自行设置新密码",
		keyboards.BackKeyboard("back_start"),
	)
}

func handleCheckin(c tele.Context) error {
	cfg := config.Get()
	if !cfg.Open.Checkin {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 签到功能已关闭",
			ShowAlert: true,
		})
	}

	// 使用签到服务
	checkinSvc := service.NewCheckinService()
	result, err := checkinSvc.Checkin(c.Sender().ID)

	if err != nil {
		var errMsg string
		switch {
		case errors.Is(err, service.ErrAlreadyCheckedIn):
			errMsg = "❌ 今日已签到，明天再来吧~"
		case errors.Is(err, service.ErrLevelNotAllowed):
			errMsg = "❌ 您的等级不允许签到"
		case errors.Is(err, service.ErrUserNotFound):
			errMsg = "❌ 请先 /start 初始化账户"
		default:
			errMsg = "❌ 签到失败，请稍后重试"
		}
		return c.Respond(&tele.CallbackResponse{
			Text:      errMsg,
			ShowAlert: true,
		})
	}

	// 签到成功，更新消息
	text := fmt.Sprintf(
		"%s\n\n"+
			"🎁 **获得积分**: +%d %s\n"+
			"💰 **当前积分**: %d %s\n"+
			"📅 **签到时间**: %s",
		result.Message,
		result.Reward, cfg.Money,
		result.TotalScore, cfg.Money,
		result.CheckinTime.Format("2006-01-02 15:04:05"),
	)

	c.Respond(&tele.CallbackResponse{Text: "🎯 签到成功！"})
	return c.Edit(text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
}

func handleAdminPanel(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 您没有权限",
			ShowAlert: true,
		})
	}

	c.Respond()
	isOwner := cfg.IsOwner(c.Sender().ID)
	return c.Edit("⚙️ **管理面板**\n\n请选择操作:", keyboards.AdminPanelKeyboard(isOwner), tele.ModeMarkdown)
}

func handleSetLevel(c tele.Context, parts []string) error {
	if len(parts) < 3 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 您没有权限",
			ShowAlert: true,
		})
	}

	// 解析参数: set_lv:<tgID>:<level>
	tgID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "无效的用户ID"})
	}

	level := parts[2]
	if level != "a" && level != "b" && level != "c" && level != "d" && level != "e" {
		return c.Respond(&tele.CallbackResponse{Text: "无效的等级"})
	}

	repo := repository.NewEmbyRepository()
	if err := repo.UpdateFields(tgID, map[string]interface{}{"lv": level}); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "更新失败"})
	}

	levelNames := map[string]string{
		"a": "白名单",
		"b": "普通用户",
		"c": "观察用户",
		"d": "游客",
		"e": "封禁",
	}

	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ 等级已更新为: %s", levelNames[level])})
}

// OnInlineQuery 内联查询处理器
func OnInlineQuery(c tele.Context) error {
	query := c.Query().Text

	// 内联查询功能暂时返回空结果
	// 可以用于未来扩展：搜索电影、查询用户等
	logger.Debug().Str("query", query).Msg("收到内联查询")

	// 返回空结果
	return c.Answer(&tele.QueryResponse{
		Results:   []tele.Result{},
		CacheTime: 60,
	})
}
