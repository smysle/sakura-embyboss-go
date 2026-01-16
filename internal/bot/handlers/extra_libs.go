// Package handlers 额外媒体库管理和分页处理
package handlers

import (
	"fmt"
	"strconv"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// handleExtraLibToggle 管理员为用户开关额外媒体库
func handleExtraLibToggle(c tele.Context, tgIDStr string, show bool) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 您没有权限",
			ShowAlert: true,
		})
	}

	tgID, err := strconv.ParseInt(tgIDStr, 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "无效的用户ID"})
	}

	// 获取用户信息
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(tgID)
	if err != nil || !user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 用户不存在或无账户"})
	}

	// 获取额外库列表
	extraLibs := cfg.Emby.ExtraLibs
	if len(extraLibs) == 0 {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 未配置额外媒体库",
			ShowAlert: true,
		})
	}

	client := emby.GetClient()

	if show {
		// 显示额外库
		if err := client.ShowFolders(*user.EmbyID, extraLibs); err != nil {
			logger.Error().Err(err).Int64("tg", tgID).Msg("显示额外媒体库失败")
			return c.Respond(&tele.CallbackResponse{Text: "❌ 操作失败"})
		}
		c.Respond(&tele.CallbackResponse{Text: "✅ 已为用户开启额外媒体库"})
	} else {
		// 隐藏额外库
		if err := client.HideFolders(*user.EmbyID, extraLibs); err != nil {
			logger.Error().Err(err).Int64("tg", tgID).Msg("隐藏额外媒体库失败")
			return c.Respond(&tele.CallbackResponse{Text: "❌ 操作失败"})
		}
		c.Respond(&tele.CallbackResponse{Text: "✅ 已为用户关闭额外媒体库"})
	}

	// 刷新用户信息面板
	return showUserInfo(c, user)
}

// handleUsersPage 用户列表分页
func handleUsersPage(c tele.Context, parts []string) error {
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	page, err := strconv.Atoi(parts[1])
	if err != nil || page < 1 {
		page = 1
	}

	filter := ""
	if len(parts) >= 3 {
		filter = parts[2]
	}

	return showUsersList(c, page, filter)
}

// showUsersList 显示用户列表
func showUsersList(c tele.Context, page int, filter string) error {
	repo := repository.NewEmbyRepository()
	pageSize := 10

	// 获取用户列表
	users, total, err := repo.ListWithPagination(page, pageSize, filter)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 获取用户列表失败"})
	}

	if total == 0 {
		return editOrReply(c, "📋 暂无用户数据", keyboards.BackKeyboard("admin_panel"), tele.ModeMarkdown)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	text := fmt.Sprintf("👥 **用户列表** (第 %d/%d 页)\n\n", page, totalPages)
	for i, u := range users {
		idx := (page-1)*pageSize + i + 1
		status := "🟢"
		if u.Lv == "e" {
			status = "🔴"
		}
		text += fmt.Sprintf("%d. %s `%d` - %s\n", idx, status, u.TG, getEmbyName(u.Name))
	}

	text += fmt.Sprintf("\n共 %d 位用户", total)

	kb := keyboards.UserListPagination(page, totalPages, filter)
	return editOrReply(c, text, kb, tele.ModeMarkdown)
}

// handleWhitelistPage 白名单列表分页
func handleWhitelistPage(c tele.Context, parts []string) error {
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	page, err := strconv.Atoi(parts[1])
	if err != nil || page < 1 {
		page = 1
	}

	return showWhitelistList(c, page)
}

// showWhitelistList 显示白名单列表
func showWhitelistList(c tele.Context, page int) error {
	repo := repository.NewEmbyRepository()
	pageSize := 10

	users, total, err := repo.GetWhitelistUsers(page, pageSize)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 获取白名单列表失败"})
	}

	if total == 0 {
		return editOrReply(c, "📋 暂无白名单用户", keyboards.BackKeyboard("admin_users"), tele.ModeMarkdown)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	text := fmt.Sprintf("🌟 **白名单用户** (第 %d/%d 页)\n\n", page, totalPages)
	for i, u := range users {
		idx := (page-1)*pageSize + i + 1
		text += fmt.Sprintf("%d. `%d` - %s\n", idx, u.TG, getEmbyName(u.Name))
	}

	text += fmt.Sprintf("\n共 %d 位白名单用户", total)

	kb := keyboards.WhitelistPagination(page, totalPages)
	return editOrReply(c, text, kb, tele.ModeMarkdown)
}

// handleFavoritesPage 收藏列表分页
func handleFavoritesPage(c tele.Context, parts []string) error {
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	page, err := strconv.Atoi(parts[1])
	if err != nil || page < 1 {
		page = 1
	}

	return showFavoritesList(c, page)
}

// showFavoritesList 显示收藏列表
func showFavoritesList(c tele.Context, page int) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || !user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您还没有账户"})
	}

	client := emby.GetClient()
	pageSize := 10
	offset := (page - 1) * pageSize

	// 从 Emby 获取收藏
	favorites, total, err := client.GetUserFavorites(*user.EmbyID, offset, pageSize)
	if err != nil {
		logger.Error().Err(err).Msg("获取收藏列表失败")
		return c.Respond(&tele.CallbackResponse{Text: "❌ 获取收藏失败"})
	}

	if total == 0 {
		return editOrReply(c, "⭐ 暂无收藏内容", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	totalPages := (total + pageSize - 1) / pageSize

	text := fmt.Sprintf("⭐ **我的收藏** (第 %d/%d 页)\n\n", page, totalPages)
	for i, item := range favorites {
		idx := offset + i + 1
		text += fmt.Sprintf("%d. %s\n", idx, item.Name)
	}

	text += fmt.Sprintf("\n共 %d 个收藏", total)

	kb := keyboards.FavoritesPagination(page, totalPages)
	return editOrReply(c, text, kb, tele.ModeMarkdown)
}

// handleDevicesPage 设备列表分页
func handleDevicesPage(c tele.Context, parts []string) error {
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	page, err := strconv.Atoi(parts[1])
	if err != nil || page < 1 {
		page = 1
	}

	return showDevicesList(c, page)
}

// showDevicesList 显示设备列表
func showDevicesList(c tele.Context, page int) error {
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || !user.HasEmbyAccount() {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 您还没有账户"})
	}

	client := emby.GetClient()
	pageSize := 10
	offset := (page - 1) * pageSize

	// 从 Emby 获取设备
	devices, total, err := client.GetUserDevices(*user.EmbyID, offset, pageSize)
	if err != nil {
		logger.Error().Err(err).Msg("获取设备列表失败")
		return c.Respond(&tele.CallbackResponse{Text: "❌ 获取设备失败"})
	}

	if total == 0 {
		return editOrReply(c, "📱 暂无登录设备", keyboards.BackKeyboard("members"), tele.ModeMarkdown)
	}

	totalPages := (total + pageSize - 1) / pageSize

	text := fmt.Sprintf("📱 **我的设备** (第 %d/%d 页)\n\n", page, totalPages)
	for i, device := range devices {
		idx := offset + i + 1
		text += fmt.Sprintf("%d. %s (%s)\n   最后活跃: %s\n", 
			idx, 
			device.DeviceName, 
			device.AppName,
			device.LastActivityDate,
		)
	}

	text += fmt.Sprintf("\n共 %d 个设备", total)

	kb := keyboards.DevicesPagination(page, totalPages)
	return editOrReply(c, text, kb, tele.ModeMarkdown)
}

// handleCodesPage 注册码列表分页
func handleCodesPage(c tele.Context, parts []string) error {
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "参数错误"})
	}

	page, err := strconv.Atoi(parts[1])
	if err != nil || page < 1 {
		page = 1
	}

	filter := ""
	if len(parts) >= 3 {
		filter = parts[2]
	}

	return showCodesList(c, page, filter)
}

// showCodesList 显示注册码列表
func showCodesList(c tele.Context, page int, filter string) error {
	repo := repository.NewCodeRepository()
	pageSize := 10

	codes, total, err := repo.ListWithPagination(page, pageSize, filter)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ 获取注册码列表失败"})
	}

	if total == 0 {
		return editOrReply(c, "📋 暂无注册码", keyboards.BackKeyboard("admin_codes"), tele.ModeMarkdown)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	text := fmt.Sprintf("🎫 **注册码列表** (第 %d/%d 页)\n\n", page, totalPages)
	for i, code := range codes {
		idx := (page-1)*pageSize + i + 1
		status := "🟢 可用"
		if code.Used {
			status = "🔴 已用"
		}
		text += fmt.Sprintf("%d. `%s` %s (%d天)\n", idx, code.Code, status, code.Days)
	}

	text += fmt.Sprintf("\n共 %d 个注册码", total)

	kb := keyboards.CodesPagination(page, totalPages, filter)
	return editOrReply(c, text, kb, tele.ModeMarkdown)
}
