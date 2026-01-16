// Package handlers 管理面板处理器
package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// ==================== 注册状态面板 ====================

// handleOpenMenu 注册状态面板
func handleOpenMenu(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	text := "⭕ **注册状态设置**\n\n" +
		"在这里可以控制用户注册相关的设置"

	return editOrReply(c, text, keyboards.OpenMenuKeyboard(cfg), tele.ModeMarkdown)
}

// handleOpenStat 切换自由注册状态
func handleOpenStat(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	// 切换状态
	cfg.Open.Status = !cfg.Open.Status
	if err := config.Save(); err != nil {
		logger.Error().Err(err).Msg("保存配置失败")
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if cfg.Open.Status {
		status = "开启"
	}

	// 发送群组通知
	if len(cfg.Groups) > 0 {
		notifyText := fmt.Sprintf("📢 自由注册已%s", status)
		c.Bot().Send(&tele.Chat{ID: cfg.Groups[0]}, notifyText)
	}

	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ 自由注册已%s", status)})
	return handleOpenMenu(c)
}

// handleOpenTiming 定时注册设置
func handleOpenTiming(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	// 设置会话状态等待输入
	session.Set(c.Sender().ID, session.StateWaitingOpenTiming, nil)

	return c.Send("请输入定时注册参数：`时长(分钟) 人数`\n\n例如：`30 10` 表示开放30分钟，限制10人\n\n发送 `0` 取消定时注册", tele.ModeMarkdown)
}

// handleOpenDays 设置注册天数
func handleOpenDays(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	session.Set(c.Sender().ID, session.StateWaitingOpenDays, nil)
	return c.Send("请输入新用户注册时获得的账户天数：\n\n当前：" + strconv.Itoa(cfg.Open.Temp) + " 天")
}

// handleAllUserLimit 设置注册人数限制
func handleAllUserLimit(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	session.Set(c.Sender().ID, session.StateWaitingUserLimit, nil)
	return c.Send("请输入注册人数上限：\n\n当前：" + strconv.Itoa(cfg.Open.MaxUsers) + " 人\n\n输入 0 表示不限制")
}

// ==================== 注册码面板 ====================

// handleCrLink 创建注册/续期码
func handleCrLink(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	session.Set(c.Sender().ID, session.StateWaitingCodeCreate, nil)

	return c.Send(
		"🎟️ **创建注册/续期码**\n\n"+
			"请输入参数：`天数 数量`\n\n"+
			"例如：\n"+
			"• `30 5` - 生成5个30天的注册码\n"+
			"• `90 10` - 生成10个90天的注册码\n\n"+
			"发送 `取消` 取消操作",
		tele.ModeMarkdown,
	)
}

// handleChLink 查询注册码
func handleChLink(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	codeRepo := repository.NewCodeRepository()

	// 获取各管理员创建的注册码统计
	stats, err := codeRepo.GetStatsByCreator()
	if err != nil {
		return c.Send("❌ 获取统计信息失败")
	}

	text := "💊 **注册码统计**\n\n"
	for _, stat := range stats {
		text += fmt.Sprintf("👤 管理员 `%d`:\n", stat.Creator)
		text += fmt.Sprintf("   • 总数: %d\n", stat.Total)
		text += fmt.Sprintf("   • 已用: %d\n", stat.Used)
		text += fmt.Sprintf("   • 未用: %d\n\n", stat.Total-stat.Used)
	}

	if len(stats) == 0 {
		text += "暂无注册码记录"
	}

	return editOrReply(c, text, keyboards.BackKeyboard("admin_panel"), tele.ModeMarkdown)
}

// ==================== 兑换设置面板 ====================

// handleSetRenew 兑换设置面板
func handleSetRenew(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	text := "🏬 **兑换设置**\n\n" +
		"在这里可以控制用户兑换相关的功能开关"

	return editOrReply(c, text, keyboards.SetRenewKeyboard(cfg), tele.ModeMarkdown)
}

// handleSetRenewCheckin 切换签到功能
func handleSetRenewCheckin(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	cfg.Open.Checkin = !cfg.Open.Checkin
	if err := config.Save(); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if cfg.Open.Checkin {
		status = "开启"
	}
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ 每日签到已%s", status)})
	return handleSetRenew(c)
}

// handleSetRenewExchange 切换自动续期功能
func handleSetRenewExchange(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	cfg.Open.Exchange = !cfg.Open.Exchange
	if err := config.Save(); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if cfg.Open.Exchange {
		status = "开启"
	}
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ 自动币续期已%s", status)})
	return handleSetRenew(c)
}

// handleSetRenewWhitelist 切换白名单兑换功能
func handleSetRenewWhitelist(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	cfg.Open.Whitelist = !cfg.Open.Whitelist
	if err := config.Save(); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if cfg.Open.Whitelist {
		status = "开启"
	}
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ 兑换白名单已%s", status)})
	return handleSetRenew(c)
}

// handleSetRenewInvite 切换邀请码兑换功能
func handleSetRenewInvite(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	cfg.Open.Invite = !cfg.Open.Invite
	if err := config.Save(); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if cfg.Open.Invite {
		status = "开启"
	}
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ 兑换邀请码已%s", status)})
	return handleSetRenew(c)
}

// handleSetLevelMenu 等级设置菜单
func handleSetLevelMenu(c tele.Context, action string) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	markup := &tele.ReplyMarkup{}
	var targetAction string
	var title string

	if action == "set_checkin_lv" {
		targetAction = "do_set_checkin_lv"
		title = "签到功能"
	} else {
		targetAction = "do_set_invite_lv"
		title = "邀请码兑换"
	}

	markup.Inline(
		markup.Row(
			markup.Data("🅰️ 白名单可用", fmt.Sprintf("%s|a", targetAction)),
			markup.Data("🅱️ 普通用户及以上", fmt.Sprintf("%s|b", targetAction)),
		),
		markup.Row(
			markup.Data("©️ 已禁用及以上", fmt.Sprintf("%s|c", targetAction)),
			markup.Data("🅳️ 所有用户", fmt.Sprintf("%s|d", targetAction)),
		),
		markup.Row(
			markup.Data("« 返回", "set_renew"),
		),
	)

	return editOrReply(c, fmt.Sprintf("请选择 **%s** 的权限等级：", title), markup, tele.ModeMarkdown)
}

// ==================== 定时任务面板 ====================

// handleSchedAll 定时任务面板
func handleSchedAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	text := "🌏 **定时任务管理**\n\n" +
		"点击按钮可以开启/关闭对应的定时任务\n\n" +
		"• 播放日榜: 每日 18:30\n" +
		"• 播放周榜: 每周日 23:59\n" +
		"• 观影日榜: 每日 23:00\n" +
		"• 观影周榜: 每周日 23:00\n" +
		"• 到期检测: 每日 01:30\n" +
		"• 活跃检测: 每日 08:30\n" +
		"• 自动备份: 每日 02:30"

	return editOrReply(c, text, keyboards.SchedAllKeyboard(cfg), tele.ModeMarkdown)
}

// handleSchedToggle 切换定时任务
func handleSchedToggle(c tele.Context, action string) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}

	var taskName string
	var enabled *bool

	switch action {
	case "sched_dayrank":
		cfg.Scheduler.DayRank = !cfg.Scheduler.DayRank
		enabled = &cfg.Scheduler.DayRank
		taskName = "播放日榜"
	case "sched_weekrank":
		cfg.Scheduler.WeekRank = !cfg.Scheduler.WeekRank
		enabled = &cfg.Scheduler.WeekRank
		taskName = "播放周榜"
	case "sched_dayplayrank":
		cfg.Scheduler.DayPlayRank = !cfg.Scheduler.DayPlayRank
		enabled = &cfg.Scheduler.DayPlayRank
		taskName = "观影日榜"
	case "sched_weekplayrank":
		cfg.Scheduler.WeekPlayRank = !cfg.Scheduler.WeekPlayRank
		enabled = &cfg.Scheduler.WeekPlayRank
		taskName = "观影周榜"
	case "sched_check_ex":
		cfg.Scheduler.CheckExpired = !cfg.Scheduler.CheckExpired
		enabled = &cfg.Scheduler.CheckExpired
		taskName = "到期检测"
	case "sched_low_activity":
		cfg.Scheduler.LowActivity = !cfg.Scheduler.LowActivity
		enabled = &cfg.Scheduler.LowActivity
		taskName = "活跃检测"
	case "sched_backup_db":
		cfg.Scheduler.BackupDB = !cfg.Scheduler.BackupDB
		enabled = &cfg.Scheduler.BackupDB
		taskName = "自动备份"
	}

	if err := config.Save(); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if enabled != nil && *enabled {
		status = "开启"
	}

	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ %s已%s", taskName, status)})
	return handleSchedAll(c)
}

// ==================== 用户列表 ====================

// handleAdminWhitelist 白名单列表
func handleAdminWhitelist(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond()

	repo := repository.NewEmbyRepository()
	users, err := repo.GetByLevel(models.LevelA)
	if err != nil {
		return c.Send("❌ 获取白名单失败")
	}

	text := "👑 **白名单用户列表**\n\n"
	for i, user := range users {
		if i >= 50 {
			text += fmt.Sprintf("\n... 共 %d 人", len(users))
			break
		}
		name := "未知"
		if user.Name != nil {
			name = *user.Name
		}
		text += fmt.Sprintf("%d. `%s` (ID: %d)\n", i+1, name, user.TG)
	}

	if len(users) == 0 {
		text += "暂无白名单用户"
	}

	return editOrReply(c, text, keyboards.BackKeyboard("admin_panel"), tele.ModeMarkdown)
}

// handleAdminDevices 设备列表
func handleAdminDevices(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您没有权限", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "📱 设备列表功能开发中..."})
	return nil
}

// ==================== Owner配置面板 ====================

// handleCfgExportLog 导出日志
func handleCfgExportLog(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}
	c.Respond(&tele.CallbackResponse{Text: "📄 正在导出日志..."})

	// 发送日志文件
	logFile := &tele.Document{File: tele.FromDisk("logs/app.log")}
	logFile.FileName = "app.log"
	return c.Send(logFile)
}

// handleCfgToggle 切换配置开关
func handleCfgToggle(c tele.Context, action string) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}

	var optionName string
	var enabled *bool

	switch action {
	case "cfg_toggle_leave_ban":
		cfg.Open.LeaveBan = !cfg.Open.LeaveBan
		enabled = &cfg.Open.LeaveBan
		optionName = "退群封禁"
	case "cfg_toggle_play_reward":
		cfg.Open.UserPlays = !cfg.Open.UserPlays
		enabled = &cfg.Open.UserPlays
		optionName = "观影奖励"
	case "cfg_toggle_red":
		cfg.RedEnvelope.Enabled = !cfg.RedEnvelope.Enabled
		enabled = &cfg.RedEnvelope.Enabled
		optionName = "红包功能"
	case "cfg_toggle_red_private":
		cfg.RedEnvelope.AllowPrivate = !cfg.RedEnvelope.AllowPrivate
		enabled = &cfg.RedEnvelope.AllowPrivate
		optionName = "专属红包"
	}

	if err := config.Save(); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败", ShowAlert: true})
	}

	status := "关闭"
	if enabled != nil && *enabled {
		status = "开启"
	}

	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ %s已%s", optionName, status)})
	return handleOwnerConfig(c)
}

// handleCfgSetDays 设置天数配置
func handleCfgSetDays(c tele.Context, action string) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}
	c.Respond()

	var prompt string
	var state session.State

	switch action {
	case "cfg_set_gift_days":
		state = session.StateWaitingGiftDays
		prompt = fmt.Sprintf("请输入赠送资格的天数（当前：%d 天）：", cfg.KKGiftDays)
	case "cfg_set_activity_days":
		state = session.StateWaitingActivityDays
		prompt = fmt.Sprintf("请输入活跃检测的天数阈值（当前：%d 天）：", cfg.ActivityCheckDays)
	case "cfg_set_freeze_days":
		state = session.StateWaitingFreezeDays
		prompt = fmt.Sprintf("请输入封存账号的天数（当前：%d 天）：", cfg.FreezeDays)
	}

	session.Set(c.Sender().ID, state, nil)
	return c.Send(prompt)
}

// handleCfgSetLine 设置线路
func handleCfgSetLine(c tele.Context, action string) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}
	c.Respond()

	var state session.State
	var prompt string

	if action == "cfg_set_line" {
		state = session.StateWaitingLine
		prompt = "请输入普通用户线路信息：\n\n当前：\n" + cfg.Emby.Line
	} else {
		state = session.StateWaitingWhitelistLine
		prompt = "请输入白名单用户线路信息：\n\n当前：\n" + cfg.Emby.WhitelistLine
	}

	session.Set(c.Sender().ID, state, nil)
	return c.Send(prompt)
}

// handleCfgMP MoviePilot 设置
func handleCfgMP(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 仅 Owner 可用", ShowAlert: true})
	}
	c.Respond()

	text := "🎬 **MoviePilot 点播设置**\n\n" +
		fmt.Sprintf("• 状态: %s\n", getStatusText(cfg.MoviePilot.Enabled)) +
		fmt.Sprintf("• 价格: %d 积分/GB\n", cfg.MoviePilot.Price) +
		fmt.Sprintf("• 用户权限: %s\n", keyboards.GetLevelName(cfg.MoviePilot.Level))

	markup := &tele.ReplyMarkup{}

	statusText := "❌ 关闭点播"
	if cfg.MoviePilot.Enabled {
		statusText = "✅ 开启点播"
	}

	markup.Inline(
		markup.Row(
			markup.Data(statusText, "cfg_mp_toggle"),
		),
		markup.Row(
			markup.Data("💰 设置价格", "cfg_mp_price"),
			markup.Data("👥 设置权限", "cfg_mp_level"),
		),
		markup.Row(
			markup.Data("« 返回", "owner_config"),
		),
	)

	return editOrReply(c, text, markup, tele.ModeMarkdown)
}

func getStatusText(enabled bool) string {
	if enabled {
		return "✅ 开启"
	}
	return "❌ 关闭"
}

// ==================== 输入处理函数 ====================

// handleOpenTimingInput 处理定时注册输入
func handleOpenTimingInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	if text == "0" {
		// 取消定时注册
		cfg.Open.Timing = 0
		if err := config.Save(); err != nil {
			return c.Send("❌ 保存配置失败")
		}
		return c.Send("✅ 已取消定时注册")
	}

	// 解析参数：时长 人数
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return c.Send("❌ 格式错误\n\n请输入：`时长(分钟) 人数`", tele.ModeMarkdown)
	}

	minutes, err := strconv.Atoi(parts[0])
	if err != nil || minutes <= 0 {
		return c.Send("❌ 时长必须是正整数")
	}

	limit, err := strconv.Atoi(parts[1])
	if err != nil || limit <= 0 {
		return c.Send("❌ 人数必须是正整数")
	}

	cfg.Open.Timing = minutes
	cfg.Open.MaxUsers = limit
	cfg.Open.Status = true

	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 定时注册已设置\n\n开放时长：%d 分钟\n人数限制：%d 人", minutes, limit))
}

// handleOpenDaysInput 处理注册天数输入
func handleOpenDaysInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	days, err := strconv.Atoi(text)
	if err != nil || days <= 0 {
		return c.Send("❌ 请输入有效的正整数天数")
	}

	cfg.Open.Temp = days
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 注册天数已设置为 %d 天", days))
}

// handleUserLimitInput 处理用户限制输入
func handleUserLimitInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	limit, err := strconv.Atoi(text)
	if err != nil || limit < 0 {
		return c.Send("❌ 请输入有效的非负整数")
	}

	cfg.Open.MaxUsers = limit
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	if limit == 0 {
		return c.Send("✅ 已取消注册人数限制")
	}
	return c.Send(fmt.Sprintf("✅ 注册人数限制已设置为 %d 人", limit))
}

// handleCodeCreateInput 处理注册码创建输入
func handleCodeCreateInput(c tele.Context, text string) error {
	session.Clear(c.Sender().ID)

	if text == "取消" {
		return c.Send("✅ 已取消操作")
	}

	// 解析参数：天数 数量
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return c.Send("❌ 格式错误\n\n请输入：`天数 数量`", tele.ModeMarkdown)
	}

	days, err := strconv.Atoi(parts[0])
	if err != nil || days <= 0 {
		return c.Send("❌ 天数必须是正整数")
	}

	count, err := strconv.Atoi(parts[1])
	if err != nil || count <= 0 || count > 100 {
		return c.Send("❌ 数量必须是 1-100 的正整数")
	}

	// 生成注册码
	codeSvc := service.NewCodeService()
	codes, err := codeSvc.GenerateCodes(c.Sender().ID, days, count)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ 生成注册码失败: %s", err.Error()))
	}

	// 构建回复
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎟️ **生成 %d 个注册码成功**\n\n", count))
	sb.WriteString(fmt.Sprintf("有效期：%d 天\n\n", days))
	for i, code := range codes {
		sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, code))
	}

	return c.Send(sb.String(), tele.ModeMarkdown)
}

// handleGiftDaysInput 处理赠送天数输入
func handleGiftDaysInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	days, err := strconv.Atoi(text)
	if err != nil || days <= 0 {
		return c.Send("❌ 请输入有效的正整数天数")
	}

	cfg.KKGiftDays = days
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 赠送资格天数已设置为 %d 天", days))
}

// handleActivityDaysInput 处理活跃检测天数输入
func handleActivityDaysInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	days, err := strconv.Atoi(text)
	if err != nil || days <= 0 {
		return c.Send("❌ 请输入有效的正整数天数")
	}

	cfg.ActivityCheckDays = days
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 活跃检测天数已设置为 %d 天", days))
}

// handleFreezeDaysInput 处理封存天数输入
func handleFreezeDaysInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	days, err := strconv.Atoi(text)
	if err != nil || days <= 0 {
		return c.Send("❌ 请输入有效的正整数天数")
	}

	cfg.FreezeDays = days
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 封存账号天数已设置为 %d 天", days))
}

// handleLineInput 处理线路输入
func handleLineInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	cfg.Emby.Line = text
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send("✅ 普通用户线路已更新")
}

// handleWhitelistLineInput 处理白名单线路输入
func handleWhitelistLineInput(c tele.Context, text string) error {
	cfg := config.Get()
	session.Clear(c.Sender().ID)

	cfg.Emby.WhitelistLine = &text
	if err := config.Save(); err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send("✅ 白名单用户线路已更新")
}
