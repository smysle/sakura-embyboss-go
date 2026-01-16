// Package keyboards 键盘按钮
package keyboards

import (
	"fmt"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
)

// JoinGroupKeyboard 加入群组键盘
func JoinGroupKeyboard() *tele.ReplyMarkup {
	cfg := config.Get()
	markup := &tele.ReplyMarkup{}

	btnGroup := markup.URL("📢 加入群组", fmt.Sprintf("https://t.me/%s", cfg.MainGroup))
	btnChannel := markup.URL("📣 加入频道", fmt.Sprintf("https://t.me/%s", cfg.Channel))

	markup.Inline(
		markup.Row(btnGroup, btnChannel),
	)
	return markup
}

// StartPanelKeyboard 开始面板键盘（无账户）
func StartPanelKeyboard(isAdmin bool) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var rows []tele.Row

	// 基础功能行
	rows = append(rows, markup.Row(
		markup.Data("📝 注册账户", "register"),
		markup.Data("🎫 使用注册码", "use_code"),
	))

	rows = append(rows, markup.Row(
		markup.Data("📊 媒体库统计", "count"),
		markup.Data("📋 我的信息", "myinfo"),
	))

	if isAdmin {
		rows = append(rows, markup.Row(
			markup.Data("⚙️ 管理面板", "admin_panel"),
		))
	}

	markup.Inline(rows...)
	return markup
}

// StartPanelKeyboardWithAccount 开始面板键盘（有账户）
func StartPanelKeyboardWithAccount(isAdmin bool) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var rows []tele.Row

	// 账户功能行
	rows = append(rows, markup.Row(
		markup.Data("👤 账户信息", "account_info"),
		markup.Data("🔑 重置密码", "reset_pwd"),
	))

	rows = append(rows, markup.Row(
		markup.Data("📊 媒体库统计", "count"),
		markup.Data("🎯 签到", "checkin"),
	))

	rows = append(rows, markup.Row(
		markup.Data("📈 我的观影", "my_plays"),
		markup.Data("⭐ 我的收藏", "my_favorites"),
	))

	if isAdmin {
		rows = append(rows, markup.Row(
			markup.Data("⚙️ 管理面板", "admin_panel"),
		))
	}

	markup.Inline(rows...)
	return markup
}

// AdminPanelKeyboard 管理面板键盘
func AdminPanelKeyboard(isOwner bool) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var rows []tele.Row

	// 用户管理
	rows = append(rows, markup.Row(
		markup.Data("👥 用户管理", "admin_users"),
		markup.Data("📝 注册码管理", "admin_codes"),
	))

	// 系统功能
	rows = append(rows, markup.Row(
		markup.Data("📊 统计信息", "admin_stats"),
		markup.Data("🔍 到期检测", "admin_check_ex"),
	))

	// 排行榜
	rows = append(rows, markup.Row(
		markup.Data("📈 日榜", "admin_day_ranks"),
		markup.Data("📊 周榜", "admin_week_ranks"),
	))

	if isOwner {
		rows = append(rows, markup.Row(
			markup.Data("⚙️ 系统配置", "owner_config"),
			markup.Data("💾 备份数据库", "owner_backup"),
		))
	}

	// 返回按钮
	rows = append(rows, markup.Row(
		markup.Data("« 返回", "back_start"),
	))

	markup.Inline(rows...)
	return markup
}

// AccountInfoKeyboard 账户信息键盘
func AccountInfoKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("🔑 重置密码", "reset_pwd"),
			markup.Data("📱 设备管理", "devices"),
		),
		markup.Row(
			markup.Data("« 返回", "back_start"),
		),
	)
	return markup
}

// ConfirmKeyboard 确认操作键盘
func ConfirmKeyboard(confirmData, cancelData string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("✅ 确认", confirmData),
			markup.Data("❌ 取消", cancelData),
		),
	)
	return markup
}

// BackKeyboard 返回键盘
func BackKeyboard(backData string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("« 返回", backData),
		),
	)
	return markup
}

// PaginationKeyboard 分页键盘
func PaginationKeyboard(prevData, nextData string, hasPrev, hasNext bool, page, total int) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var btns []tele.Btn

	if hasPrev {
		btns = append(btns, markup.Data("« 上一页", prevData))
	}

	btns = append(btns, markup.Data(fmt.Sprintf("%d/%d", page, total), "noop"))

	if hasNext {
		btns = append(btns, markup.Data("下一页 »", nextData))
	}

	markup.Inline(markup.Row(btns...))
	return markup
}

// CloseKeyboard 关闭键盘
func CloseKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("❌ 关闭", "close"),
		),
	)
	return markup
}

// UserLevelKeyboard 用户等级选择键盘
func UserLevelKeyboard(userTG int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("🌟 白名单 (A)", fmt.Sprintf("set_lv:%d:a", userTG)),
			markup.Data("🔮 高级 (B)", fmt.Sprintf("set_lv:%d:b", userTG)),
		),
		markup.Row(
			markup.Data("💎 普通 (C)", fmt.Sprintf("set_lv:%d:c", userTG)),
			markup.Data("🎫 基础 (D)", fmt.Sprintf("set_lv:%d:d", userTG)),
		),
		markup.Row(
			markup.Data("🚫 封禁 (E)", fmt.Sprintf("set_lv:%d:e", userTG)),
		),
		markup.Row(
			markup.Data("« 返回", "back_kk"),
		),
	)
	return markup
}

// UserManageKeyboard 用户管理键盘（包含额外媒体库控制）
func UserManageKeyboard(userTG int64, hasExtraLibs bool, extraLibsEnabled bool, isBanned bool, hasEmby bool) *tele.ReplyMarkup {
	cfg := config.Get()
	markup := &tele.ReplyMarkup{}

	var rows []tele.Row

	// 封禁/解封按钮
	if isBanned {
		rows = append(rows, markup.Row(
			markup.Data("🌟 解除禁用", fmt.Sprintf("user_unban|%d", userTG)),
		))
	} else if hasEmby {
		rows = append(rows, markup.Row(
			markup.Data("💢 禁用账户", fmt.Sprintf("user_ban|%d", userTG)),
		))
	}

	// 删除账户按钮（仅有Emby账户时显示）
	if hasEmby {
		rows = append(rows, markup.Row(
			markup.Data("⚠️ 删除账户", fmt.Sprintf("user_delete|%d", userTG)),
		))
	}

	// 等级设置行
	rows = append(rows, markup.Row(
		markup.Data("🌟 白名单 (A)", fmt.Sprintf("set_lv:%d:a", userTG)),
		markup.Data("🔮 高级 (B)", fmt.Sprintf("set_lv:%d:b", userTG)),
	))
	rows = append(rows, markup.Row(
		markup.Data("💎 普通 (C)", fmt.Sprintf("set_lv:%d:c", userTG)),
		markup.Data("🎫 基础 (D)", fmt.Sprintf("set_lv:%d:d", userTG)),
	))
	rows = append(rows, markup.Row(
		markup.Data("🚫 封禁 (E)", fmt.Sprintf("set_lv:%d:e", userTG)),
	))

	// 额外媒体库控制（如果配置了额外库）
	if hasExtraLibs && len(cfg.Emby.ExtraLibs) > 0 && hasEmby {
		if extraLibsEnabled {
			rows = append(rows, markup.Row(
				markup.Data("🎬 关闭额外媒体库", fmt.Sprintf("embyextralib_block|%d", userTG)),
			))
		} else {
			rows = append(rows, markup.Row(
				markup.Data("🎬 开启额外媒体库", fmt.Sprintf("embyextralib_unblock|%d", userTG)),
			))
		}
	}

	// 赠送资格按钮（无Emby账户时显示）
	if !hasEmby {
		rows = append(rows, markup.Row(
			markup.Data("✨ 赠送资格", fmt.Sprintf("user_gift|%d", userTG)),
		))
	}

	// 踢出并封禁
	rows = append(rows, markup.Row(
		markup.Data("🚫 踢出并封禁", fmt.Sprintf("user_kick|%d", userTG)),
	))

	// 关闭按钮
	rows = append(rows, markup.Row(
		markup.Data("❌ 关闭", "close"),
	))

	markup.Inline(rows...)
	return markup
}

// CodeDaysKeyboard 注册码天数选择键盘
func CodeDaysKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("30天 (月)", "code_days:30"),
			markup.Data("90天 (季)", "code_days:90"),
		),
		markup.Row(
			markup.Data("180天 (半年)", "code_days:180"),
			markup.Data("365天 (年)", "code_days:365"),
		),
		markup.Row(
			markup.Data("❌ 取消", "close"),
		),
	)
	return markup
}

// MembersPanelKeyboard 用户面板键盘
func MembersPanelKeyboard(hasAccount bool, isAdmin bool) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var rows []tele.Row

	if hasAccount {
		// 有账户的功能
		rows = append(rows, markup.Row(
			markup.Data("📊 服务器", "server"),
			markup.Data("🔑 重置密码", "reset_pwd"),
		))
		rows = append(rows, markup.Row(
			markup.Data("📈 我的观影", "my_plays"),
			markup.Data("⭐ 我的收藏", "my_favorites"),
		))
		rows = append(rows, markup.Row(
			markup.Data("📱 我的设备", "devices"),
			markup.Data("📚 媒体库管理", "embyblock"),
		))
		rows = append(rows, markup.Row(
			markup.Data("🏪 积分商城", "store"),
			markup.Data("🗑️ 删除账户", "delme"),
		))
	} else {
		// 无账户的功能
		rows = append(rows, markup.Row(
			markup.Data("📝 创建账户", "register"),
			markup.Data("🎫 使用注册码", "use_code"),
		))
		if isAdmin {
			rows = append(rows, markup.Row(
				markup.Data("🔗 换绑TG", "changetg"),
				markup.Data("🔗 绑定TG", "bindtg"),
			))
		}
	}

	rows = append(rows, markup.Row(
		markup.Data("« 返回", "back_start"),
	))

	markup.Inline(rows...)
	return markup
}

// StoreKeyboard 积分商城键盘
func StoreKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("📅 续期天数", "store_renew"),
			markup.Data("⭐ 白名单", "store_whitelist"),
		),
		markup.Row(
			markup.Data("🎫 邀请码", "store_invite"),
			markup.Data("🔓 解封账户", "store_reborn"),
		),
		markup.Row(
			markup.Data("📋 查询我的码", "store_query"),
		),
		markup.Row(
			markup.Data("« 返回", "members"),
		),
	)
	return markup
}

// DeleteAccountKeyboard 删除账户确认键盘
func DeleteAccountKeyboard(embyID string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("✅ 确认删除", fmt.Sprintf("delemby|%s", embyID)),
			markup.Data("❌ 取消", "members"),
		),
	)
	return markup
}

// EmbyLibraryKeyboard 媒体库管理键盘
func EmbyLibraryKeyboard(libs map[string]string, enabledMap map[string]bool, enableAll bool) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var rows []tele.Row

	for libID, libName := range libs {
		var status, action, callback string
		if enableAll || enabledMap[libID] {
			status = "✅"
			action = "隐藏"
			callback = fmt.Sprintf("emby_block|%s", libID)
		} else {
			status = "❌"
			action = "显示"
			callback = fmt.Sprintf("emby_unblock|%s", libID)
		}
		rows = append(rows, markup.Row(
			markup.Data(fmt.Sprintf("%s %s - %s", status, libName, action), callback),
		))
	}

	rows = append(rows, markup.Row(
		markup.Data("« 返回", "members"),
	))

	markup.Inline(rows...)
	return markup
}

// ChangeTGApproveKeyboard 换绑TG审核键盘
func ChangeTGApproveKeyboard(newTG, oldTG int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	markup.Inline(
		markup.Row(
			markup.Data("✅ 同意换绑", fmt.Sprintf("changetg_%d_%d", newTG, oldTG)),
			markup.Data("❌ 拒绝", fmt.Sprintf("nochangetg_%d_%d", newTG, oldTG)),
		),
	)
	return markup
}

// BackToMemberKeyboard 返回用户面板键盘
func BackToMemberKeyboard() *tele.ReplyMarkup {
	return BackKeyboard("members")
}

