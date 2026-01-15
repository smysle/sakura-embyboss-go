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
