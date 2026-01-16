// Package keyboards MoviePilot 点播相关键盘
package keyboards

import tele "gopkg.in/telebot.v3"

// DownloadCenterKeyboard 点播中心菜单键盘
func DownloadCenterKeyboard() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}

	btnSearch := menu.Data("🔍 搜索资源", "get_resource")
	btnDownloads := menu.Data("📈 下载进度", "view_downloads")
	btnBack := menu.Data("↩️ 返回", "member_home")

	menu.Inline(
		menu.Row(btnSearch),
		menu.Row(btnDownloads),
		menu.Row(btnBack),
	)

	return menu
}

// MPSearchPageKeyboard 搜索结果分页键盘
func MPSearchPageKeyboard(hasPrev, hasNext bool) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}

	var row []tele.Btn

	if hasPrev {
		row = append(row, menu.Data("⬅️ 上一页", "mp_prev_page"))
	}
	if hasNext {
		row = append(row, menu.Data("下一页 ➡️", "mp_next_page"))
	}

	btnCancel := menu.Data("❌ 取消", "mp_cancel")

	if len(row) > 0 {
		menu.Inline(
			menu.Row(row...),
			menu.Row(btnCancel),
		)
	} else {
		menu.Inline(
			menu.Row(btnCancel),
		)
	}

	return menu
}

// MPConfirmDownloadKeyboard 确认下载键盘
func MPConfirmDownloadKeyboard(index int) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}

	btnConfirm := menu.Data("✅ 确认下载", "mp_confirm_dl")
	btnBack := menu.Data("⬅️ 返回列表", "mp_back_list")
	btnCancel := menu.Data("❌ 取消", "mp_cancel")

	menu.Inline(
		menu.Row(btnConfirm),
		menu.Row(btnBack, btnCancel),
	)

	return menu
}
