// Package handlers 用户面板回调处理器
package handlers

import (
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// handleMembersPanel 用户面板主入口
func handleMembersPanel(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "✅ 用户界面"})

	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Edit("⚠️ 数据库没有你，请重新 /start 录入")
	}

	cfg := config.Get()
	
	// 格式化用户信息
	name := "未注册"
	if user.Name != nil {
		name = *user.Name
	}

	lvStr := user.GetLevelName()
	
	exStr := "无"
	if user.Ex != nil {
		exStr = user.Ex.Format("2006-01-02 15:04:05")
	}

	text := fmt.Sprintf(
		"▎__欢迎进入用户面板！%s__\n\n"+
			"**· 🆔 用户のID** | `%d`\n"+
			"**· 📊 当前状态** | %s\n"+
			"**· 🍒 积分%s** | %d\n"+
			"**· 💠 账号名称** | [%s](tg://user?id=%d)\n"+
			"**· 🚨 到期时间** | %s",
		c.Sender().FirstName,
		c.Sender().ID,
		lvStr,
		cfg.Money,
		user.Iv,
		name,
		c.Sender().ID,
		exStr,
	)

	hasAccount := user.EmbyID != nil && *user.EmbyID != ""
	kb := keyboards.MembersPanelKeyboard(hasAccount, cfg.IsAdmin(c.Sender().ID))
	return c.Edit(text, kb, tele.ModeMarkdown)
}

// handleDelMe 删除账户
func handleDelMe(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "未查询到账户，不许乱点！💢", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "🔴 请先进行安全码验证"})

	text := "**🔰账户安全验证**：\n\n" +
		"👮🏻 验证是否本人进行敏感操作，请对我发送您设置的安全码。\n" +
		"倒计时 60s\n" +
		"🛑 **停止请点 /cancel**"

	return c.Edit(text, keyboards.BackKeyboard("members"))
}

// handleConfirmDelMe 确认删除账户
func handleConfirmDelMe(c tele.Context, embyID string) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.EmbyID == nil || *user.EmbyID != embyID {
		return c.Respond(&tele.CallbackResponse{Text: "账户验证失败", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "⚠️ 正在删除账户..."})

	// 删除 Emby 账户
	client := emby.GetClient()
	if err := client.DeleteUser(*user.EmbyID); err != nil {
		logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("删除 Emby 账户失败")
		return c.Edit("❌ 删除 Emby 账户失败，请联系管理员")
	}

	// 清空数据库记录
	if err := repo.UpdateFields(c.Sender().ID, map[string]interface{}{
		"embyid": nil,
		"name":   nil,
		"pwd":    nil,
		"pwd2":   nil,
		"lv":     models.LevelD,
		"cr":     nil,
		"ex":     nil,
	}); err != nil {
		logger.Error().Err(err).Int64("tg", c.Sender().ID).Msg("清空用户记录失败")
	}

	logger.Info().Int64("tg", c.Sender().ID).Str("embyID", embyID).Msg("用户自助删除账户")

	return c.Edit("✅ 您的账户已成功删除\n\n如需再次使用，请重新注册", keyboards.BackKeyboard("back_start"))
}

// handleStore 积分商城
func handleStore(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "🏪 积分商城"})

	cfg := config.Get()
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Edit("⚠️ 数据库没有你，请重新 /start 录入")
	}

	text := fmt.Sprintf(
		"**🏪 积分商城**\n\n"+
			"您当前的%s: **%d**\n\n"+
			"可兑换的物品：\n"+
			"• 续期天数 - %d %s/天\n"+
			"• 白名单 - %d %s\n"+
			"• 邀请码 - %d %s\n\n"+
			"选择要兑换的物品：",
		cfg.Money, user.Iv,
		cfg.Open.ExchangeCost, cfg.Money,
		cfg.Open.WhitelistCost, cfg.Money,
		cfg.Open.InviteCost, cfg.Money,
	)

	return c.Edit(text, keyboards.StoreKeyboard(), tele.ModeMarkdown)
}

// handleStoreRenew 兑换续期
func handleStoreRenew(c tele.Context) error {
	cfg := config.Get()
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "您还没有账户", ShowAlert: true})
	}

	// 检查积分是否足够
	cost := cfg.Open.ExchangeCost
	if user.Iv < cost {
		return c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("积分不足，需要 %d %s", cost, cfg.Money),
			ShowAlert: true,
		})
	}

	// 扣除积分
	newIV := user.Iv - cost
	
	// 续期 1 天
	var newEx time.Time
	if user.Ex != nil && user.Ex.After(time.Now()) {
		newEx = user.Ex.AddDate(0, 0, 1)
	} else {
		newEx = time.Now().AddDate(0, 0, 1)
	}

	if err := repo.UpdateFields(c.Sender().ID, map[string]interface{}{
		"iv": newIV,
		"ex": newEx,
	}); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "兑换失败，请重试", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "✅ 兑换成功！续期 1 天"})

	text := fmt.Sprintf(
		"**✅ 兑换成功**\n\n"+
			"已消耗 %d %s\n"+
			"续期 1 天\n"+
			"新到期时间: %s\n"+
			"剩余积分: %d",
		cost, cfg.Money,
		newEx.Format("2006-01-02 15:04:05"),
		newIV,
	)

	return c.Edit(text, keyboards.BackKeyboard("store"))
}

// handleStoreWhitelist 兑换白名单
func handleStoreWhitelist(c tele.Context) error {
	cfg := config.Get()
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.Lv == models.LevelA {
		return c.Respond(&tele.CallbackResponse{Text: "您已是白名单用户", ShowAlert: true})
	}

	// 检查积分
	cost := cfg.Open.WhitelistCost
	if user.Iv < cost {
		return c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("积分不足，需要 %d %s", cost, cfg.Money),
			ShowAlert: true,
		})
	}

	// 扣除积分并升级
	newIV := user.Iv - cost
	if err := repo.UpdateFields(c.Sender().ID, map[string]interface{}{
		"iv": newIV,
		"lv": models.LevelA,
	}); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "兑换失败，请重试", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "✅ 成功升级为白名单！"})

	text := fmt.Sprintf(
		"**✅ 兑换成功**\n\n"+
			"已消耗 %d %s\n"+
			"您已升级为白名单用户\n"+
			"剩余积分: %d",
		cost, cfg.Money,
		newIV,
	)

	return c.Edit(text, keyboards.BackKeyboard("store"))
}

// handleStoreReborn 解封账户（积分兑换）
func handleStoreReborn(c tele.Context) error {
	cfg := config.Get()
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	// 检查是否被封禁
	if user.Lv != models.LevelC && user.Lv != models.LevelE {
		return c.Respond(&tele.CallbackResponse{Text: "您的账户未被封禁", ShowAlert: true})
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "您没有 Emby 账户", ShowAlert: true})
	}

	// 解封需要的积分（可配置）
	cost := 500 // 默认 500 积分解封
	if user.Iv < cost {
		return c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("积分不足，解封需要 %d %s", cost, cfg.Money),
			ShowAlert: true,
		})
	}

	// 解封 Emby 账户
	client := emby.GetClient()
	if err := client.EnableUser(*user.EmbyID); err != nil {
		logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("解封 Emby 账户失败")
		return c.Respond(&tele.CallbackResponse{Text: "解封失败，请联系管理员", ShowAlert: true})
	}

	// 更新数据库
	newIV := user.Iv - cost
	newEx := time.Now().AddDate(0, 0, 7) // 解封后给 7 天有效期
	if err := repo.UpdateFields(c.Sender().ID, map[string]interface{}{
		"iv": newIV,
		"lv": models.LevelB,
		"ex": newEx,
	}); err != nil {
		logger.Error().Err(err).Int64("tg", c.Sender().ID).Msg("更新用户状态失败")
	}

	c.Respond(&tele.CallbackResponse{Text: "✅ 账户已解封！"})

	text := fmt.Sprintf(
		"**✅ 账户已解封**\n\n"+
			"已消耗 %d %s\n"+
			"账户有效期: 7 天\n"+
			"到期时间: %s\n"+
			"剩余积分: %d",
		cost, cfg.Money,
		newEx.Format("2006-01-02 15:04:05"),
		newIV,
	)

	return c.Edit(text, keyboards.BackKeyboard("members"))
}

// handleEmbyBlock 媒体库管理
func handleEmbyBlock(c tele.Context) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "您还没有账户", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "📚 媒体库管理"})

	// 获取可用媒体库
	client := emby.GetClient()
	libs, err := client.GetLibraries()
	if err != nil {
		return c.Edit("获取媒体库列表失败", keyboards.BackKeyboard("members"))
	}

	// 获取用户当前策略
	embyUser, err := client.GetUser(*user.EmbyID)
	if err != nil {
		return c.Edit("获取用户信息失败", keyboards.BackKeyboard("members"))
	}

	enabledFolders := make(map[string]bool)
	if embyUser.Policy != nil {
		for _, f := range embyUser.Policy.EnabledFolders {
			enabledFolders[f] = true
		}
	}

	text := "**📚 媒体库管理**\n\n选择要显示/隐藏的媒体库："

	kb := keyboards.EmbyLibraryKeyboard(libs, enabledFolders, embyUser.Policy != nil && embyUser.Policy.EnableAllFolders)
	return c.Edit(text, kb, tele.ModeMarkdown)
}

// handleToggleLibrary 切换媒体库显示/隐藏
func handleToggleLibrary(c tele.Context, libID string, show bool) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ 数据库没有你", ShowAlert: true})
	}

	if user.EmbyID == nil || *user.EmbyID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "您还没有账户", ShowAlert: true})
	}

	client := emby.GetClient()
	
	// 获取媒体库信息以获取名称
	libs, _ := client.GetLibraries()
	libName := libs[libID]
	if libName == "" {
		libName = libID
	}

	var actionErr error
	if show {
		actionErr = client.ShowFolders(*user.EmbyID, []string{libName})
	} else {
		actionErr = client.HideFolders(*user.EmbyID, []string{libName})
	}

	if actionErr != nil {
		logger.Error().Err(actionErr).Str("libID", libID).Bool("show", show).Msg("切换媒体库失败")
		return c.Respond(&tele.CallbackResponse{Text: "操作失败，请重试", ShowAlert: true})
	}

	action := "显示"
	if !show {
		action = "隐藏"
	}
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("已%s: %s", action, libName)})

	// 刷新页面
	return handleEmbyBlock(c)
}

// handleServerInfo 服务器信息
func handleServerInfo(c tele.Context) error {
	c.Respond(&tele.CallbackResponse{Text: "📊 服务器信息"})

	cfg := config.Get()
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	
	var pwd string
	if err == nil && user.Pwd != nil {
		pwd = *user.Pwd
	} else {
		pwd = "未设置"
	}

	// 确定线路
	line := cfg.Emby.Line
	if user != nil && user.Lv == models.LevelA && cfg.Emby.WhitelistLine != nil {
		line = *cfg.Emby.WhitelistLine
	}

	text := fmt.Sprintf(
		"**📊 服务器信息**\n\n"+
			"**当前线路：**\n%s\n\n"+
			"**您的密码：** `%s`\n\n"+
			"**使用方式：**\n"+
			"1. 下载 Emby 客户端\n"+
			"2. 输入上方线路地址\n"+
			"3. 使用您的用户名和密码登录",
		line, pwd,
	)

	return c.Edit(text, keyboards.BackKeyboard("members"), tele.ModeMarkdown)
}
