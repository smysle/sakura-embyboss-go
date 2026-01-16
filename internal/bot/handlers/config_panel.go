// Package handlers 配置面板处理器
package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Config /config 配置面板入口命令
func Config(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Send("❌ 只有 Owner 才能使用此命令")
	}

	return showConfigPanel(c)
}

// showConfigPanel 显示配置面板
func showConfigPanel(c tele.Context) error {
	cfg := config.Get()
	
	text := "🌸 **欢迎回来！**\n\n👇 点击你要修改的内容。"
	
	return editOrReply(c, text, configPanelKeyboard(cfg), tele.ModeMarkdown)
}

// configPanelKeyboard 配置面板键盘
func configPanelKeyboard(cfg *config.Config) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	
	// 状态图标
	getStatus := func(enabled bool) string {
		if enabled {
			return "✅"
		}
		return "❎"
	}
	
	var rows []tele.Row
	
	// 第一行：导出日志、探针设置
	rows = append(rows, markup.Row(
		markup.Data("📄 导出日志", "cfg_export_log"),
		markup.Data("📌 设置探针", "cfg_nezha"),
	))
	
	// 第二行：线路设置
	rows = append(rows, markup.Row(
		markup.Data("💠 普通用户线路", "cfg_line"),
		markup.Data("🌟 白名单线路", "cfg_whitelist_line"),
	))
	
	// 第三行：媒体库设置
	rows = append(rows, markup.Row(
		markup.Data("🎬 显/隐指定库", "cfg_block_libs"),
	))
	
	// 第四行：开关项
	leaveBanStatus := getStatus(cfg.Open.LeaveBan)
	userPlaysStatus := getStatus(cfg.Open.UserPlays)
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("%s 退群封禁", leaveBanStatus), "cfg_toggle|leave_ban"),
		markup.Data(fmt.Sprintf("%s 观影奖励", userPlaysStatus), "cfg_toggle|user_plays"),
	))
	
	// 第五行：更多开关
	autoUpdateStatus := getStatus(cfg.AutoUpdate.Enabled)
	mpStatus := getStatus(cfg.MoviePilot.Enabled)
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("%s 自动更新", autoUpdateStatus), "cfg_toggle|auto_update"),
		markup.Data(fmt.Sprintf("%s MoviePilot", mpStatus), "cfg_mp"),
	))
	
	// 第六行：红包设置
	redStatus := getStatus(cfg.RedEnvelope.Enabled)
	redPrivateStatus := getStatus(cfg.RedEnvelope.AllowPrivate)
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("%s 红包功能", redStatus), "cfg_toggle|red_envelope"),
		markup.Data(fmt.Sprintf("%s 专属红包", redPrivateStatus), "cfg_toggle|red_private"),
	))
	
	// 第七行：天数设置
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("🎁 赠送资格 %d天", cfg.KKGiftDays), "cfg_set|kk_gift_days"),
		markup.Data(fmt.Sprintf("📊 活跃检测 %d天", cfg.ActivityCheckDays), "cfg_set|activity_days"),
	))
	
	// 第八行：更多天数设置
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("❄️ 封存账号 %d天", cfg.FreezeDays), "cfg_set|freeze_days"),
		markup.Data(fmt.Sprintf("📝 签到权限 %s", cfg.Open.CheckinLevel), "cfg_set|checkin_level"),
	))
	
	// 第九行：签到开关、兑换开关
	checkinStatus := getStatus(cfg.Open.Checkin)
	exchangeStatus := getStatus(cfg.Open.Exchange)
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("%s 签到功能", checkinStatus), "cfg_toggle|checkin"),
		markup.Data(fmt.Sprintf("%s 兑换功能", exchangeStatus), "cfg_toggle|exchange"),
	))
	
	// 第十行：活跃检测开关
	lowActivityStatus := getStatus(cfg.Open.LowActivity)
	rows = append(rows, markup.Row(
		markup.Data(fmt.Sprintf("%s 低活跃检测", lowActivityStatus), "cfg_toggle|low_activity"),
	))
	
	// 返回
	rows = append(rows, markup.Row(
		markup.Data("« 返回管理面板", "admin_panel"),
	))
	
	markup.Inline(rows...)
	return markup
}

// handleConfigCallback 处理配置相关回调
func handleConfigCallback(c tele.Context, action string, parts []string) error {
	cfg := config.Get()
	if !cfg.IsOwner(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 只有 Owner 才能修改配置",
			ShowAlert: true,
		})
	}
	
	switch action {
	case "cfg_export_log":
		return handleExportLog(c)
	case "cfg_nezha":
		return handleNezhaConfig(c)
	case "cfg_line":
		return handleLineConfig(c)
	case "cfg_whitelist_line":
		return handleWhitelistLineConfig(c)
	case "cfg_block_libs":
		return handleBlockLibsConfig(c)
	case "cfg_mp":
		return handleMPConfig(c)
	case "cfg_toggle":
		if len(parts) >= 2 {
			return handleConfigToggle(c, parts[1])
		}
	case "cfg_set":
		if len(parts) >= 2 {
			return handleConfigSet(c, parts[1])
		}
	case "cfg_mp_set":
		if len(parts) >= 2 {
			return handleMPSet(c, parts[1])
		}
	case "cfg_mp_toggle":
		if len(parts) >= 2 {
			return handleMPToggle(c, parts[1])
		}
	}
	
	return c.Respond(&tele.CallbackResponse{Text: "未知操作"})
}

// handleExportLog 导出日志
func handleExportLog(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "📄 正在导出日志..."})
	
	// 发送日志文件
	logFile := &tele.Document{
		File:     tele.FromDisk("logs/bot.log"),
		FileName: "bot.log",
		Caption:  "📄 Bot 运行日志",
	}
	
	return c.Send(logFile)
}

// handleNezhaConfig 设置探针
func handleNezhaConfig(c tele.Context) error {
	c.Respond()
	
	// 设置会话状态
	sessionMgr := session.GetManager()
	sessionMgr.SetStateWithStringAction(c.Sender().ID, session.StateWaitingInput, "cfg_nezha")
	
	return editOrReply(c,
		"📌 **设置哪吒探针**\n\n"+
			"请发送探针配置，格式：\n"+
			"`探针地址,API Token,监控ID`\n\n"+
			"示例：\n"+
			"`https://nezha.example.com,abc123token,1`\n\n"+
			"_发送 /cancel 取消操作_",
		keyboards.BackKeyboard("owner_config"),
		tele.ModeMarkdown,
	)
}

// handleLineConfig 设置普通用户线路
func handleLineConfig(c tele.Context) error {
	c.Respond()
	
	sessionMgr := session.GetManager()
	sessionMgr.SetStateWithStringAction(c.Sender().ID, session.StateWaitingInput, "cfg_line")
	
	cfg := config.Get()
	return editOrReply(c,
		fmt.Sprintf("💠 **设置普通用户线路**\n\n"+
			"当前线路: `%s`\n\n"+
			"请发送新的线路地址\n\n"+
			"_发送 /cancel 取消操作_", cfg.Emby.Line),
		keyboards.BackKeyboard("owner_config"),
		tele.ModeMarkdown,
	)
}

// handleWhitelistLineConfig 设置白名单线路
func handleWhitelistLineConfig(c tele.Context) error {
	c.Respond()
	
	sessionMgr := session.GetManager()
	sessionMgr.SetStateWithStringAction(c.Sender().ID, session.StateWaitingInput, "cfg_whitelist_line")
	
	cfg := config.Get()
	currentLine := "未设置"
	if cfg.Emby.WhitelistLine != nil {
		currentLine = *cfg.Emby.WhitelistLine
	}
	
	return editOrReply(c,
		fmt.Sprintf("🌟 **设置白名单线路**\n\n"+
			"当前线路: `%s`\n\n"+
			"请发送新的线路地址\n\n"+
			"_发送 /cancel 取消操作_", currentLine),
		keyboards.BackKeyboard("owner_config"),
		tele.ModeMarkdown,
	)
}

// handleBlockLibsConfig 设置媒体库显隐
func handleBlockLibsConfig(c tele.Context) error {
	c.Respond()
	
	cfg := config.Get()
	
	var blockedText string
	if len(cfg.Emby.BlockedLibs) > 0 {
		blockedText = strings.Join(cfg.Emby.BlockedLibs, ", ")
	} else {
		blockedText = "无"
	}
	
	var extraText string
	if len(cfg.Emby.ExtraLibs) > 0 {
		extraText = strings.Join(cfg.Emby.ExtraLibs, ", ")
	} else {
		extraText = "无"
	}
	
	text := fmt.Sprintf("🎬 **媒体库显隐设置**\n\n"+
		"**普通库隐藏列表**:\n`%s`\n\n"+
		"**额外库列表**:\n`%s`\n\n"+
		"请选择要修改的项目:",
		blockedText, extraText,
	)
	
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("📝 修改普通库隐藏", "cfg_set|blocked_libs"),
			markup.Data("📝 修改额外库", "cfg_set|extra_libs"),
		),
		markup.Row(
			markup.Data("« 返回", "owner_config"),
		),
	)
	
	return editOrReply(c, text, markup, tele.ModeMarkdown)
}

// handleMPConfig MoviePilot 配置面板
func handleMPConfig(c tele.Context) error {
	c.Respond()
	
	cfg := config.Get()
	mp := cfg.MoviePilot
	
	getStatus := func(enabled bool) string {
		if enabled {
			return "✅"
		}
		return "❎"
	}
	
	text := fmt.Sprintf("🎬 **MoviePilot 配置**\n\n"+
		"**状态**: %s\n"+
		"**URL**: `%s`\n"+
		"**用户名**: `%s`\n"+
		"**价格**: %d 积分\n"+
		"**权限等级**: %s",
		getStatus(mp.Enabled),
		mp.URL,
		mp.Username,
		mp.Price,
		mp.Level,
	)
	
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data(fmt.Sprintf("%s 启用状态", getStatus(mp.Enabled)), "cfg_mp_toggle|enabled"),
		),
		markup.Row(
			markup.Data("🔗 设置 URL", "cfg_mp_set|url"),
			markup.Data("👤 设置用户名", "cfg_mp_set|username"),
		),
		markup.Row(
			markup.Data("🔑 设置密码", "cfg_mp_set|password"),
			markup.Data("💰 设置价格", "cfg_mp_set|price"),
		),
		markup.Row(
			markup.Data("📊 设置权限等级", "cfg_mp_set|level"),
		),
		markup.Row(
			markup.Data("« 返回", "owner_config"),
		),
	)
	
	return editOrReply(c, text, markup, tele.ModeMarkdown)
}

// handleConfigToggle 处理开关切换
func handleConfigToggle(c tele.Context, key string) error {
	cfg := config.Get()
	
	var toggleName string
	var newValue bool
	
	switch key {
	case "leave_ban":
		cfg.Open.LeaveBan = !cfg.Open.LeaveBan
		newValue = cfg.Open.LeaveBan
		toggleName = "退群封禁"
	case "user_plays":
		cfg.Open.UserPlays = !cfg.Open.UserPlays
		newValue = cfg.Open.UserPlays
		toggleName = "观影奖励"
	case "auto_update":
		cfg.AutoUpdate.Enabled = !cfg.AutoUpdate.Enabled
		newValue = cfg.AutoUpdate.Enabled
		toggleName = "自动更新"
	case "red_envelope":
		cfg.RedEnvelope.Enabled = !cfg.RedEnvelope.Enabled
		newValue = cfg.RedEnvelope.Enabled
		toggleName = "红包功能"
	case "red_private":
		cfg.RedEnvelope.AllowPrivate = !cfg.RedEnvelope.AllowPrivate
		newValue = cfg.RedEnvelope.AllowPrivate
		toggleName = "专属红包"
	case "checkin":
		cfg.Open.Checkin = !cfg.Open.Checkin
		newValue = cfg.Open.Checkin
		toggleName = "签到功能"
	case "exchange":
		cfg.Open.Exchange = !cfg.Open.Exchange
		newValue = cfg.Open.Exchange
		toggleName = "兑换功能"
	case "low_activity":
		cfg.Open.LowActivity = !cfg.Open.LowActivity
		newValue = cfg.Open.LowActivity
		toggleName = "低活跃检测"
	default:
		return c.Respond(&tele.CallbackResponse{Text: "未知配置项"})
	}
	
	// 保存配置
	if err := cfg.Save("config.json"); err != nil {
		logger.Error().Err(err).Msg("保存配置失败")
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败"})
	}
	
	status := "已关闭"
	if newValue {
		status = "已开启"
	}
	
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ %s %s", toggleName, status)})
	
	// 刷新配置面板
	return showConfigPanel(c)
}

// handleConfigSet 处理设置项
func handleConfigSet(c tele.Context, key string) error {
	c.Respond()
	
	sessionMgr := session.GetManager()
	sessionMgr.SetStateWithStringAction(c.Sender().ID, session.StateWaitingInput, "cfg_"+key)
	
	var prompt string
	cfg := config.Get()
	
	switch key {
	case "kk_gift_days":
		prompt = fmt.Sprintf("🎁 **设置赠送资格天数**\n\n当前值: %d 天\n\n请输入新的天数:", cfg.KKGiftDays)
	case "activity_days":
		prompt = fmt.Sprintf("📊 **设置活跃检测天数**\n\n当前值: %d 天\n\n请输入新的天数:", cfg.ActivityCheckDays)
	case "freeze_days":
		prompt = fmt.Sprintf("❄️ **设置封存账号天数**\n\n当前值: %d 天\n\n请输入新的天数:", cfg.FreezeDays)
	case "checkin_level":
		prompt = fmt.Sprintf("📝 **设置签到权限等级**\n\n当前值: %s\n\n可选值: a, b, c, d\n\n请输入等级:", cfg.Open.CheckinLevel)
	case "blocked_libs":
		current := "无"
		if len(cfg.Emby.BlockedLibs) > 0 {
			current = strings.Join(cfg.Emby.BlockedLibs, ", ")
		}
		prompt = fmt.Sprintf("📝 **设置普通库隐藏列表**\n\n当前: %s\n\n请输入库名列表，用逗号分隔:", current)
	case "extra_libs":
		current := "无"
		if len(cfg.Emby.ExtraLibs) > 0 {
			current = strings.Join(cfg.Emby.ExtraLibs, ", ")
		}
		prompt = fmt.Sprintf("📝 **设置额外库列表**\n\n当前: %s\n\n请输入库名列表，用逗号分隔:", current)
	default:
		return c.Respond(&tele.CallbackResponse{Text: "未知配置项"})
	}
	
	return editOrReply(c,
		prompt+"\n\n_发送 /cancel 取消操作_",
		keyboards.BackKeyboard("owner_config"),
		tele.ModeMarkdown,
	)
}

// handleMPToggle MoviePilot 开关
func handleMPToggle(c tele.Context, key string) error {
	cfg := config.Get()
	
	switch key {
	case "enabled":
		cfg.MoviePilot.Enabled = !cfg.MoviePilot.Enabled
	default:
		return c.Respond(&tele.CallbackResponse{Text: "未知配置项"})
	}
	
	if err := cfg.Save("config.json"); err != nil {
		logger.Error().Err(err).Msg("保存配置失败")
		return c.Respond(&tele.CallbackResponse{Text: "❌ 保存配置失败"})
	}
	
	status := "已关闭"
	if cfg.MoviePilot.Enabled {
		status = "已开启"
	}
	
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("✅ MoviePilot %s", status)})
	return handleMPConfig(c)
}

// handleMPSet MoviePilot 设置项
func handleMPSet(c tele.Context, key string) error {
	c.Respond()
	
	sessionMgr := session.GetManager()
	sessionMgr.SetStateWithStringAction(c.Sender().ID, session.StateWaitingInput, "cfg_mp_"+key)
	
	var prompt string
	cfg := config.Get()
	
	switch key {
	case "url":
		prompt = fmt.Sprintf("🔗 **设置 MoviePilot URL**\n\n当前: `%s`\n\n请输入新的 URL:", cfg.MoviePilot.URL)
	case "username":
		prompt = fmt.Sprintf("👤 **设置 MoviePilot 用户名**\n\n当前: `%s`\n\n请输入新的用户名:", cfg.MoviePilot.Username)
	case "password":
		prompt = "🔑 **设置 MoviePilot 密码**\n\n请输入新的密码:"
	case "price":
		prompt = fmt.Sprintf("💰 **设置 MoviePilot 价格**\n\n当前: %d 积分\n\n请输入新的价格:", cfg.MoviePilot.Price)
	case "level":
		prompt = fmt.Sprintf("📊 **设置 MoviePilot 权限等级**\n\n当前: %s\n\n可选值: a, b, c, d\n\n请输入等级:", cfg.MoviePilot.Level)
	default:
		return c.Respond(&tele.CallbackResponse{Text: "未知配置项"})
	}
	
	return editOrReply(c,
		prompt+"\n\n_发送 /cancel 取消操作_",
		keyboards.BackKeyboard("cfg_mp"),
		tele.ModeMarkdown,
	)
}

// ProcessConfigInput 处理配置输入
func ProcessConfigInput(c tele.Context, action string) error {
	cfg := config.Get()
	input := strings.TrimSpace(c.Text())
	
	var success bool
	var msg string
	
	switch action {
	case "cfg_nezha":
		// 解析探针配置：地址,Token,监控ID
		parts := strings.Split(input, ",")
		if len(parts) != 3 {
			return c.Send("❌ 格式错误\n\n请按格式输入: `探针地址,API Token,监控ID`", tele.ModeMarkdown)
		}
		cfg.Nezha.URL = strings.TrimSpace(parts[0])
		cfg.Nezha.Token = strings.TrimSpace(parts[1])
		cfg.Nezha.MonitorID = strings.TrimSpace(parts[2])
		success = true
		msg = "探针配置已更新"
		
	case "cfg_line":
		cfg.Emby.Line = input
		success = true
		msg = "普通用户线路已更新"
		
	case "cfg_whitelist_line":
		cfg.Emby.WhitelistLine = &input
		success = true
		msg = "白名单线路已更新"
		
	case "cfg_kk_gift_days":
		days, err := strconv.Atoi(input)
		if err != nil || days < 0 {
			return c.Send("❌ 请输入有效的天数")
		}
		cfg.KKGiftDays = days
		success = true
		msg = fmt.Sprintf("赠送资格天数已更新为 %d 天", days)
		
	case "cfg_activity_days":
		days, err := strconv.Atoi(input)
		if err != nil || days < 0 {
			return c.Send("❌ 请输入有效的天数")
		}
		cfg.ActivityCheckDays = days
		success = true
		msg = fmt.Sprintf("活跃检测天数已更新为 %d 天", days)
		
	case "cfg_freeze_days":
		days, err := strconv.Atoi(input)
		if err != nil || days < 0 {
			return c.Send("❌ 请输入有效的天数")
		}
		cfg.FreezeDays = days
		success = true
		msg = fmt.Sprintf("封存账号天数已更新为 %d 天", days)
		
	case "cfg_checkin_level":
		level := strings.ToLower(input)
		if level != "a" && level != "b" && level != "c" && level != "d" {
			return c.Send("❌ 请输入有效的等级 (a/b/c/d)")
		}
		cfg.Open.CheckinLevel = level
		success = true
		msg = fmt.Sprintf("签到权限等级已更新为 %s", level)
		
	case "cfg_blocked_libs":
		libs := parseLibList(input)
		cfg.Emby.BlockedLibs = libs
		success = true
		msg = fmt.Sprintf("普通库隐藏列表已更新 (%d 个)", len(libs))
		
	case "cfg_extra_libs":
		libs := parseLibList(input)
		cfg.Emby.ExtraLibs = libs
		success = true
		msg = fmt.Sprintf("额外库列表已更新 (%d 个)", len(libs))
		
	case "cfg_mp_url":
		cfg.MoviePilot.URL = input
		success = true
		msg = "MoviePilot URL 已更新"
		
	case "cfg_mp_username":
		cfg.MoviePilot.Username = input
		success = true
		msg = "MoviePilot 用户名已更新"
		
	case "cfg_mp_password":
		cfg.MoviePilot.Password = input
		success = true
		msg = "MoviePilot 密码已更新"
		
	case "cfg_mp_price":
		price, err := strconv.Atoi(input)
		if err != nil || price < 0 {
			return c.Send("❌ 请输入有效的价格")
		}
		cfg.MoviePilot.Price = price
		success = true
		msg = fmt.Sprintf("MoviePilot 价格已更新为 %d 积分", price)
		
	case "cfg_mp_level":
		level := strings.ToLower(input)
		if level != "a" && level != "b" && level != "c" && level != "d" {
			return c.Send("❌ 请输入有效的等级 (a/b/c/d)")
		}
		cfg.MoviePilot.Level = level
		success = true
		msg = fmt.Sprintf("MoviePilot 权限等级已更新为 %s", level)
		
	default:
		return c.Send("❌ 未知配置项")
	}
	
	if success {
		if err := cfg.Save("config.json"); err != nil {
			logger.Error().Err(err).Msg("保存配置失败")
			return c.Send("❌ 保存配置失败")
		}
		
		// 清除会话状态
		sessionMgr := session.GetManager()
		sessionMgr.ClearState(c.Sender().ID)
		
		return c.Send(fmt.Sprintf("✅ %s\n\n使用 /config 返回配置面板", msg))
	}
	
	return nil
}

// parseLibList 解析库名列表
func parseLibList(input string) []string {
	if input == "" || input == "无" {
		return []string{}
	}
	
	parts := strings.Split(input, ",")
	var libs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			libs = append(libs, p)
		}
	}
	return libs
}
