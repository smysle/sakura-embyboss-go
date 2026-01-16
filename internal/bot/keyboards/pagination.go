// Package keyboards 分页组件
package keyboards

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// Paginator 分页器配置
type Paginator struct {
	Total        int                     // 总页数
	Current      int                     // 当前页码
	PageSize     int                     // 每页大小
	CallbackFmt  string                  // 回调格式，如 "users_page:%d"
	ShowQuickNav bool                    // 是否显示快速翻页按钮 (+5/-5)
	QuickStep    int                     // 快速翻页步长，默认5
	ShowFirst    bool                    // 是否显示首页按钮
	ShowLast     bool                    // 是否显示末页按钮
	MaxButtons   int                     // 最大页码按钮数（不含导航按钮）
}

// NewPaginator 创建分页器
func NewPaginator(total, current int, callbackFmt string) *Paginator {
	return &Paginator{
		Total:        total,
		Current:      current,
		CallbackFmt:  callbackFmt,
		ShowQuickNav: true,
		QuickStep:    5,
		ShowFirst:    true,
		ShowLast:     true,
		MaxButtons:   5,
	}
}

// BuildKeyboard 构建分页键盘
func (p *Paginator) BuildKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	if p.Total <= 1 {
		return markup
	}

	var navRow []tele.Btn
	var pageRow []tele.Btn

	// 快速后退 -5
	if p.ShowQuickNav && p.Current > p.QuickStep {
		navRow = append(navRow, markup.Data("⏮️-5", fmt.Sprintf(p.CallbackFmt, p.Current-p.QuickStep)))
	}

	// 上一页
	if p.Current > 1 {
		navRow = append(navRow, markup.Data("◀️", fmt.Sprintf(p.CallbackFmt, p.Current-1)))
	}

	// 页码按钮
	start, end := p.calculatePageRange()
	for i := start; i <= end; i++ {
		if i == p.Current {
			// 当前页高亮
			pageRow = append(pageRow, markup.Data(fmt.Sprintf("·%d·", i), "noop"))
		} else {
			pageRow = append(pageRow, markup.Data(fmt.Sprintf("%d", i), fmt.Sprintf(p.CallbackFmt, i)))
		}
	}

	// 下一页
	if p.Current < p.Total {
		navRow = append(navRow, markup.Data("▶️", fmt.Sprintf(p.CallbackFmt, p.Current+1)))
	}

	// 快速前进 +5
	if p.ShowQuickNav && p.Current+p.QuickStep <= p.Total {
		navRow = append(navRow, markup.Data("⏭️+5", fmt.Sprintf(p.CallbackFmt, p.Current+p.QuickStep)))
	}

	// 组装键盘
	var rows []tele.Row
	if len(pageRow) > 0 {
		rows = append(rows, markup.Row(pageRow...))
	}
	if len(navRow) > 0 {
		rows = append(rows, markup.Row(navRow...))
	}

	markup.Inline(rows...)
	return markup
}

// BuildKeyboardWithExtra 构建带额外按钮的分页键盘
func (p *Paginator) BuildKeyboardWithExtra(extraRows ...tele.Row) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	var allRows []tele.Row

	// 添加分页按钮
	if p.Total > 1 {
		var navRow []tele.Btn
		var pageRow []tele.Btn

		// 快速后退
		if p.ShowQuickNav && p.Current > p.QuickStep {
			navRow = append(navRow, markup.Data("⏮️-5", fmt.Sprintf(p.CallbackFmt, p.Current-p.QuickStep)))
		}

		// 上一页
		if p.Current > 1 {
			navRow = append(navRow, markup.Data("◀️", fmt.Sprintf(p.CallbackFmt, p.Current-1)))
		}

		// 页码
		start, end := p.calculatePageRange()
		for i := start; i <= end; i++ {
			if i == p.Current {
				pageRow = append(pageRow, markup.Data(fmt.Sprintf("·%d·", i), "noop"))
			} else {
				pageRow = append(pageRow, markup.Data(fmt.Sprintf("%d", i), fmt.Sprintf(p.CallbackFmt, i)))
			}
		}

		// 下一页
		if p.Current < p.Total {
			navRow = append(navRow, markup.Data("▶️", fmt.Sprintf(p.CallbackFmt, p.Current+1)))
		}

		// 快速前进
		if p.ShowQuickNav && p.Current+p.QuickStep <= p.Total {
			navRow = append(navRow, markup.Data("⏭️+5", fmt.Sprintf(p.CallbackFmt, p.Current+p.QuickStep)))
		}

		if len(pageRow) > 0 {
			allRows = append(allRows, markup.Row(pageRow...))
		}
		if len(navRow) > 0 {
			allRows = append(allRows, markup.Row(navRow...))
		}
	}

	// 添加额外行
	allRows = append(allRows, extraRows...)

	markup.Inline(allRows...)
	return markup
}

// calculatePageRange 计算页码范围
func (p *Paginator) calculatePageRange() (start, end int) {
	maxButtons := p.MaxButtons
	if maxButtons <= 0 {
		maxButtons = 5
	}

	half := maxButtons / 2

	start = p.Current - half
	end = p.Current + half

	// 边界调整
	if start < 1 {
		end += (1 - start)
		start = 1
	}

	if end > p.Total {
		start -= (end - p.Total)
		end = p.Total
	}

	if start < 1 {
		start = 1
	}

	return
}

// SimplePagination 简单分页（只有上一页/下一页）
func SimplePagination(page, total int, prevFmt, nextFmt string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	if total <= 1 {
		return markup
	}

	var btns []tele.Btn

	if page > 1 {
		btns = append(btns, markup.Data("« 上一页", fmt.Sprintf(prevFmt, page-1)))
	}

	btns = append(btns, markup.Data(fmt.Sprintf("%d/%d", page, total), "noop"))

	if page < total {
		btns = append(btns, markup.Data("下一页 »", fmt.Sprintf(nextFmt, page+1)))
	}

	markup.Inline(markup.Row(btns...))
	return markup
}

// UserListPagination 用户列表分页键盘
func UserListPagination(page, total int, filter string) *tele.ReplyMarkup {
	p := NewPaginator(total, page, "users_page|%d|"+filter)
	return p.BuildKeyboardWithExtra(
		tele.Row{tele.Btn{Text: "« 返回", Data: "admin_panel"}},
	)
}

// WhitelistPagination 白名单列表分页键盘
func WhitelistPagination(page, total int) *tele.ReplyMarkup {
	p := NewPaginator(total, page, "whitelist_page|%d")
	return p.BuildKeyboardWithExtra(
		tele.Row{tele.Btn{Text: "« 返回", Data: "admin_users"}},
	)
}

// FavoritesPagination 收藏列表分页键盘
func FavoritesPagination(page, total int) *tele.ReplyMarkup {
	p := NewPaginator(total, page, "favorites_page|%d")
	return p.BuildKeyboardWithExtra(
		tele.Row{tele.Btn{Text: "« 返回", Data: "members"}},
	)
}

// DevicesPagination 设备列表分页键盘
func DevicesPagination(page, total int) *tele.ReplyMarkup {
	p := NewPaginator(total, page, "devices_page|%d")
	return p.BuildKeyboardWithExtra(
		tele.Row{tele.Btn{Text: "« 返回", Data: "members"}},
	)
}

// RanksPagination 排行榜分页键盘
func RanksPagination(page, total int, rankType string) *tele.ReplyMarkup {
	p := NewPaginator(total, page, fmt.Sprintf("ranks_page|%s|%%d", rankType))
	return p.BuildKeyboardWithExtra(
		tele.Row{tele.Btn{Text: "❌ 关闭", Data: "close"}},
	)
}

// CodesPagination 注册码列表分页键盘
func CodesPagination(page, total int, filter string) *tele.ReplyMarkup {
	p := NewPaginator(total, page, "codes_page|%d|"+filter)
	return p.BuildKeyboardWithExtra(
		tele.Row{tele.Btn{Text: "« 返回", Data: "admin_codes"}},
	)
}
