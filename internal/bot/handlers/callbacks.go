// Package handlers 回调处理器
package handlers

import (
	"errors"
	"fmt"
	"strconv"
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

// editOrReply 编辑消息或发送新消息
// 解决 Telegram "there is no text in the message to edit" 错误
// 当消息是图片/媒体消息时，使用 EditCaption；否则使用 Edit
func editOrReply(c tele.Context, text string, opts ...interface{}) error {
	msg := c.Message()
	if msg == nil {
		// 没有消息可编辑，发送新消息
		return c.Send(text, opts...)
	}

	// 检查消息是否是媒体消息（有 Photo、Video、Document 等）
	if msg.Photo != nil || msg.Video != nil || msg.Document != nil || msg.Audio != nil {
		// 媒体消息，使用 EditCaption
		// 先更新 caption
		if _, err := c.Bot().EditCaption(msg, text, opts...); err != nil {
			// 如果编辑失败，尝试发送新消息
			logger.Debug().Err(err).Msg("EditCaption failed, sending new message")
			return c.Send(text, opts...)
		}
		return nil
	}

	// 普通文本消息，使用 Edit
	if err := c.Edit(text, opts...); err != nil {
		// 如果编辑失败，尝试发送新消息
		logger.Debug().Err(err).Msg("Edit failed, sending new message")
		return c.Send(text, opts...)
	}
	return nil
}

// OnCallback 回调查询处理器
func OnCallback(c tele.Context) error {
	data := c.Callback().Data

	// telebot v3 的 Data() 生成的回调格式是 "\f{unique}|{data}"
	// 需要去掉 \f 前缀
	if len(data) > 0 && data[0] == '\f' {
		data = data[1:]
	}

	// 解析回调数据，格式可能是 "action|param" 或 "action:param"
	var action string
	var parts []string

	if strings.Contains(data, "|") {
		parts = strings.Split(data, "|")
		action = parts[0]
	} else if strings.Contains(data, ":") {
		parts = strings.Split(data, ":")
		action = parts[0]
	} else {
		action = data
		parts = []string{data}
	}

	logger.Debug().Str("raw_data", c.Callback().Data).Str("action", action).Msg("收到回调")

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
	case "my_plays":
		return handleMyPlays(c)
	case "my_favorites":
		return handleMyFavorites(c)
	case "admin_users":
		return handleAdminUsers(c)
	case "admin_codes":
		return handleAdminCodes(c)
	case "admin_stats":
		return handleAdminStats(c)
	case "admin_check_ex":
		return handleAdminCheckEx(c)
	case "admin_day_ranks":
		return handleAdminDayRanks(c)
	case "admin_week_ranks":
		return handleAdminWeekRanks(c)
	case "owner_config":
		return handleOwnerConfig(c)
	case "owner_backup":
		return handleOwnerBackup(c)
	case "devices":
		return handleDevices(c)
	case "members":
		return handleMembersPanel(c)
	case "delme":
		return handleDelMe(c)
	case "delemby":
		// 确认删除账户 delemby|{embyID}
		if len(parts) >= 2 {
			return handleConfirmDelMe(c, parts[1])
		}
		return c.Respond(&tele.CallbackResponse{Text: "无效操作"})
	case "store", "storeall":
		return handleStore(c)
	case "store_renew":
		return handleStoreRenew(c)
	case "store_whitelist":
		return handleStoreWhitelist(c)
	case "store_reborn":
		return handleStoreReborn(c)
	case "embyblock":
		return handleEmbyBlock(c)
	case "emby_block":
		// 隐藏媒体库 emby_block|{libID}
		if len(parts) >= 2 {
			return handleToggleLibrary(c, parts[1], false)
		}
		return c.Respond(&tele.CallbackResponse{Text: "无效操作"})
	case "emby_unblock":
		// 显示媒体库 emby_unblock|{libID}
		if len(parts) >= 2 {
			return handleToggleLibrary(c, parts[1], true)
		}
		return c.Respond(&tele.CallbackResponse{Text: "无效操作"})
	case "server":
		return handleServerInfo(c)
	case "changetg":
		return handleChangeTG(c)
	case "bindtg":
		return handleBindTG(c)
	case "noop":
		return c.Respond()
	case "cfg_export_log", "cfg_nezha", "cfg_line", "cfg_whitelist_line", "cfg_block_libs", "cfg_mp":
		return handleConfigCallback(c, action, parts)
	case "cfg_toggle", "cfg_set", "cfg_mp_toggle", "cfg_mp_set":
		return handleConfigCallback(c, action, parts)
	// 额外媒体库管理员控制
	case "embyextralib_unblock":
		if len(parts) >= 2 {
			return handleExtraLibToggle(c, parts[1], true)
		}
		return c.Respond(&tele.CallbackResponse{Text: "无效操作"})
	case "embyextralib_block":
		if len(parts) >= 2 {
			return handleExtraLibToggle(c, parts[1], false)
		}
		return c.Respond(&tele.CallbackResponse{Text: "无效操作"})
	// 分页回调
	case "users_page":
		return handleUsersPage(c, parts)
	case "whitelist_page":
		return handleWhitelistPage(c, parts)
	case "favorites_page":
		return handleFavoritesPage(c, parts)
	case "devices_page":
		return handleDevicesPage(c, parts)
	case "codes_page":
		return handleCodesPage(c, parts)
	default:
		// 检查是否是 changetg_xxx_xxx 格式（管理员审核）
		if strings.HasPrefix(data, "changetg_") || strings.HasPrefix(data, "nochangetg_") {
			underscoreParts := strings.Split(data, "_")
			return handleChangeTGApprove(c, underscoreParts[0], underscoreParts)
		}
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

	return editOrReply(c, text, keyboard, tele.ModeMarkdown)
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
		return editOrReply(c, "❌ 创建账户失败，请稍后重试")
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

	return editOrReply(c, text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
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
	return editOrReply(c, 
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
	return editOrReply(c, text, keyboards.AccountInfoKeyboard(), tele.ModeMarkdown)
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

	c.Respond(&tele.CallbackResponse{Text: "🔴 请先进行安全码验证"})

	// 设置会话状态为等待安全码验证
	sessionMgr := session.GetManager()
	sessionMgr.SetStateWithAction(c.Sender().ID, session.StateWaitingSecurityCode, session.ActionResetPwd)

	return editOrReply(c,
		"**🔰账户安全验证**：\n\n"+
			"👮🏻 验证是否本人进行敏感操作，请对我发送您设置的安全码。\n"+
			"倒计时 120s\n\n"+
			"🛑 **停止请点 /cancel**",
		keyboards.BackKeyboard("members"),
		tele.ModeMarkdown,
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
	return editOrReply(c, text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
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
	return editOrReply(c, "⚙️ **管理面板**\n\n请选择操作:", keyboards.AdminPanelKeyboard(isOwner), tele.ModeMarkdown)
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

// handleMyPlays 我的观影
func handleMyPlays(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "📈 获取观影记录..."})

	// 获取用户信息
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || user == nil {
		return editOrReply(c, "❌ 未找到用户信息", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return editOrReply(c, "❌ 您还没有 Emby 账户", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	// 获取播放统计
	client := emby.GetClient()
	stats, err := client.GetUserPlaybackStats(*user.EmbyID, 30)
	if err != nil {
		logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("获取播放统计失败")
		return editOrReply(c, "❌ 获取播放统计失败，请稍后重试", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	// 格式化时长
	formatDuration := func(seconds int64) string {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		if hours > 0 {
			return fmt.Sprintf("%d小时%d分钟", hours, minutes)
		}
		return fmt.Sprintf("%d分钟", minutes)
	}

	userName := "未知"
	if user.Name != nil {
		userName = *user.Name
	}

	text := fmt.Sprintf(
		"📈 **我的观影统计**\n\n"+
			"👤 用户: `%s`\n"+
			"📅 统计周期: 最近30天\n\n"+
			"📊 **播放数据:**\n"+
			"• 观看时长: %s\n"+
			"• 播放次数: %d 次\n",
		userName,
		formatDuration(stats.TotalPlayTime),
		stats.PlayCount,
	)

	// 添加最近观看的内容（如果有）
	if len(stats.RecentItems) > 0 {
		text += "\n🎬 **最近观看:**\n"
		for i, item := range stats.RecentItems {
			if i >= 5 {
				break
			}
			text += fmt.Sprintf("• %s\n", item)
		}
	}

	return editOrReply(c, text, keyboards.BackKeyboard("members"), tele.ModeMarkdown)
}

// handleMyFavorites 我的收藏
func handleMyFavorites(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "⭐ 获取收藏..."})

	// 获取用户信息
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || user == nil {
		return editOrReply(c, "❌ 未找到用户信息", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return editOrReply(c, "❌ 您还没有 Emby 账户", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	// 获取收藏列表
	client := emby.GetClient()
	favorites, err := client.GetUserFavoritesSimple(*user.EmbyID, 20)
	if err != nil {
		logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("获取收藏列表失败")
		return editOrReply(c, "❌ 获取收藏列表失败，请稍后重试", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	userName := "未知"
	if user.Name != nil {
		userName = *user.Name
	}

	text := fmt.Sprintf(
		"⭐ **我的收藏**\n\n"+
			"👤 用户: `%s`\n"+
			"📊 收藏数量: %d\n\n",
		userName,
		len(favorites),
	)

	if len(favorites) == 0 {
		text += "_暂无收藏内容_"
	} else {
		text += "🎬 **收藏列表:**\n"
		for i, item := range favorites {
			if i >= 15 {
				text += fmt.Sprintf("\n_...还有 %d 个收藏_", len(favorites)-15)
				break
			}
			text += fmt.Sprintf("• %s\n", item.Name)
		}
	}

	return editOrReply(c, text, keyboards.BackKeyboard("members"), tele.ModeMarkdown)
}

// handleAdminUsers 用户管理
func handleAdminUsers(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "👥 用户管理"})
	
	repo := repository.NewEmbyRepository()
	total, withEmby, whitelist, _ := repo.CountStats()
	
	text := fmt.Sprintf(
		"👥 **用户管理**\n\n"+
			"📊 统计:\n"+
			"• 总用户: %d\n"+
			"• 有账户: %d\n"+
			"• 白名单: %d\n\n"+
			"使用命令管理用户:\n"+
			"• `/kk @用户` - 查看/管理用户\n"+
			"• `/prouser @用户` - 提升白名单\n"+
			"• `/revuser @用户` - 降级用户\n"+
			"• `/rmemby @用户` - 删除用户",
		total, withEmby, whitelist,
	)
	return editOrReply(c, text, keyboards.BackKeyboard("admin_panel"), tele.ModeMarkdown)
}

// handleAdminCodes 注册码管理
func handleAdminCodes(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "📝 注册码管理"})
	
	text := "📝 **注册码管理**\n\n" +
		"使用命令管理注册码:\n" +
		"• `/code 天数 数量` - 生成注册码\n" +
		"• `/codestat` - 查看注册码统计\n" +
		"• `/mycode` - 查看我的注册码\n" +
		"• `/delcode 类型` - 删除注册码"
	return editOrReply(c, text, keyboards.BackKeyboard("admin_panel"), tele.ModeMarkdown)
}

// handleAdminStats 统计信息
func handleAdminStats(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "📊 统计信息"})
	
	repo := repository.NewEmbyRepository()
	total, withEmby, whitelist, _ := repo.CountStats()
	
	text := fmt.Sprintf(
		"📊 **系统统计**\n\n"+
			"👥 用户统计:\n"+
			"• 总记录: %d\n"+
			"• 有账户: %d\n"+
			"• 白名单: %d\n",
		total, withEmby, whitelist,
	)
	return editOrReply(c, text, keyboards.BackKeyboard("admin_panel"), tele.ModeMarkdown)
}

// handleAdminCheckEx 到期检测
func handleAdminCheckEx(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "🔍 请使用 /check_ex 命令", ShowAlert: true})
	return nil
}

// handleAdminDayRanks 日榜
func handleAdminDayRanks(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "📈 请使用 /days_ranks 命令", ShowAlert: true})
	return nil
}

// handleAdminWeekRanks 周榜
func handleAdminWeekRanks(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "📊 请使用 /week_ranks 命令", ShowAlert: true})
	return nil
}

// handleOwnerConfig 系统配置
func handleOwnerConfig(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}
	c.Respond()
	return showConfigPanel(c)
}

// handleOwnerBackup 备份数据库
func handleOwnerBackup(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "💾 请使用 /backup_db 命令", ShowAlert: true})
	return nil
}

// handleDevices 设备管理
func handleDevices(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "📱 获取设备列表..."})

	// 获取用户信息
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || user == nil {
		return editOrReply(c, "❌ 未找到用户信息", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return editOrReply(c, "❌ 您还没有 Emby 账户", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	// 获取设备列表
	client := emby.GetClient()
	devices, err := client.GetUserDevicesSimple(*user.EmbyID)
	if err != nil {
		logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("获取设备列表失败")
		return editOrReply(c, "❌ 获取设备列表失败，请稍后重试", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	userName := "未知"
	if user.Name != nil {
		userName = *user.Name
	}

	text := fmt.Sprintf(
		"📱 **我的设备**\n\n"+
			"👤 用户: `%s`\n"+
			"📊 在线设备: %d\n\n",
		userName,
		len(devices),
	)

	if len(devices) == 0 {
		text += "_当前没有在线设备_"
	} else {
		text += "🖥️ **设备列表:**\n"
		for i, device := range devices {
			if i >= 10 {
				text += fmt.Sprintf("\n_...还有 %d 个设备_", len(devices)-10)
				break
			}
			lastSeen := "未知"
			if device.LastSeen != nil {
				lastSeen = device.LastSeen.Format("01-02 15:04")
			}
			text += fmt.Sprintf("• **%s** (%s)\n  └ 客户端: %s | 最后活跃: %s\n",
				device.Name,
				device.RemoteAddr,
				device.Client,
				lastSeen,
			)
		}
	}

	return editOrReply(c, text, keyboards.BackKeyboard("members"), tele.ModeMarkdown)
}

// handleChangeTG 换绑TG入口
func handleChangeTG(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{Text: "⚖️ 您已经拥有账户", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "⚖️ 更换绑定的TG"})

	// 设置会话状态
	sessionMgr := session.GetManager()
	sessionMgr.SetState(c.Sender().ID, session.StateWaitingChangeTGInfo)

	return editOrReply(c,
		"🔰 **【更换绑定emby的tg】**\n\n"+
			"须知：\n"+
			"- **请确保您之前用其他tg账户注册过**\n"+
			"- **请确保您注册的其他tg账户呈已注销状态**\n"+
			"- **请确保输入正确的emby用户名，安全码/密码**\n\n"+
			"请输入 `[emby用户名] [安全码/密码]`\n"+
			"例如 `sakura 5210`\n\n"+
			"_发送 /cancel 取消操作_",
		keyboards.BackKeyboard("members"),
		tele.ModeMarkdown,
	)
}

// handleBindTG 绑定TG入口
func handleBindTG(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{Text: "⚖️ 您已经拥有账户", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "⚖️ 将账户绑定TG"})

	// 设置会话状态
	sessionMgr := session.GetManager()
	sessionMgr.SetState(c.Sender().ID, session.StateWaitingBindTGInfo)

	return editOrReply(c,
		"🔰 **【已有emby绑定至tg】**\n\n"+
			"须知：\n"+
			"- **请确保您需绑定的账户不在bot中**\n"+
			"- **请确保您不是恶意绑定他人的账户**\n"+
			"- **请确保输入正确的emby用户名，密码**\n\n"+
			"请输入 `[emby用户名] [密码]`\n"+
			"例如 `sakura 5210`，若密码为空请填写 `None`\n\n"+
			"_发送 /cancel 取消操作_",
		keyboards.BackKeyboard("members"),
		tele.ModeMarkdown,
	)
}

// handleChangeTGApprove 管理员审核换绑TG
func handleChangeTGApprove(c tele.Context, action string, parts []string) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	if len(parts) < 3 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	newTG, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "无效的用户ID"})
	}

	oldTG, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "无效的原用户ID"})
	}

	repo := repository.NewEmbyRepository()

	if action == "nochangetg" {
		// 拒绝换绑
		c.Edit(fmt.Sprintf(
			"❎ 好的，[您](tg://user?id=%d) 已拒绝 [%d](tg://user?id=%d) 的换绑请求，原TG：`%d`",
			c.Sender().ID, newTG, newTG, oldTG,
		), tele.ModeMarkdown)

		// 通知用户
		userChat := &tele.Chat{ID: newTG}
		c.Bot().Send(userChat, "❌ 您的换绑请求已被拒绝。请在群组中详细说明情况。")
		return nil
	}

	// 同意换绑
	oldUser, err := repo.GetByTG(oldTG)
	if err != nil || oldUser == nil || !oldUser.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{Text: "原账户不存在", ShowAlert: true})
	}

	// 清空原账户信息
	if err := repo.UpdateFields(oldTG, map[string]interface{}{
		"embyid": nil,
		"name":   nil,
		"pwd":    nil,
		"pwd2":   nil,
		"lv":     "d",
		"cr":     nil,
		"ex":     nil,
		"us":     0,
		"iv":     0,
	}); err != nil {
		logger.Error().Err(err).Int64("oldTG", oldTG).Msg("清空原账户失败")
		return c.Respond(&tele.CallbackResponse{Text: "处理失败", ShowAlert: true})
	}

	// 将账户转移到新TG
	if err := repo.UpdateFields(newTG, map[string]interface{}{
		"embyid": oldUser.EmbyID,
		"name":   oldUser.Name,
		"pwd":    oldUser.Pwd,
		"pwd2":   oldUser.Pwd2,
		"lv":     oldUser.Lv,
		"cr":     oldUser.Cr,
		"ex":     oldUser.Ex,
		"iv":     oldUser.Iv,
	}); err != nil {
		logger.Error().Err(err).Int64("newTG", newTG).Msg("转移账户失败")
		return c.Respond(&tele.CallbackResponse{Text: "转移失败", ShowAlert: true})
	}

	c.Edit(fmt.Sprintf(
		"✅ 好的，[您](tg://user?id=%d) 已通过 [%d](tg://user?id=%d) 的换绑请求，原TG：`%d`",
		c.Sender().ID, newTG, newTG, oldTG,
	), tele.ModeMarkdown)

	// 通知用户
	cfg = config.Get()
	text := fmt.Sprintf(
		"⭕ 请接收您的信息！\n\n"+
			"· 用户名称 | `%s`\n"+
			"· 用户密码 | `%s`\n"+
			"· 安全密码 | `%s`（仅发送一次）\n"+
			"· 到期时间 | `%s`\n\n"+
			"· 当前线路：\n%s\n\n"+
			"**·在【服务器】按钮 - 查看线路和密码**",
		getEmbyName(oldUser.Name),
		getPassword(oldUser.Pwd),
		getSecurityCode(oldUser.Pwd2),
		formatExpiryTime(oldUser.Ex),
		cfg.Emby.Line,
	)

	userChat := &tele.Chat{ID: newTG}
	c.Bot().Send(userChat, text, tele.ModeMarkdown)

	logger.Info().
		Int64("newTG", newTG).
		Int64("oldTG", oldTG).
		Str("name", getEmbyName(oldUser.Name)).
		Msg("管理员批准换绑TG")

	return nil
}

// getSecurityCode 获取安全码
func getSecurityCode(pwd2 *string) string {
	if pwd2 == nil || *pwd2 == "" {
		return "(未设置)"
	}
	return *pwd2
}

// formatExpiryTime 格式化过期时间
func formatExpiryTime(ex *time.Time) string {
	if ex == nil {
		return "永久"
	}
	return ex.Format("2006-01-02 15:04:05")
}
