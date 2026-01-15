// Package handlers 批量管理命令处理器
package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// SyncGroupMembers /syncgroupm 同步群组成员
func SyncGroupMembers(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 || args[0] != "true" {
		return c.Send("⚠️ **注意**\n\n此操作将删除所有未在群组的 Emby 账户。\n\n确认执行请输入:\n`/syncgroupm true`", tele.ModeMarkdown)
	}

	c.Send("⏳ 正在同步群组成员...")

	// 注意：Telegram Bot API 不支持直接获取所有群组成员
	// 需要通过其他方式维护成员列表（如监听成员加入/离开事件）
	return c.Send("ℹ️ 群组同步功能需要配合成员监听事件使用。\n\n请使用 /sync_unbound 检查未绑定的用户。")
}

// SyncUnbound /syncunbound 同步未绑定用户
func SyncUnbound(c tele.Context) error {
	args := c.Args()
	dryRun := true

	if len(args) > 0 && args[0] == "true" {
		dryRun = false
	}

	c.Send("⏳ 正在检查未绑定 Bot 的 Emby 用户...")

	batchSvc := service.NewBatchService()
	batchSvc.SetBot(c.Bot())

	result, err := batchSvc.SyncUnbound(dryRun)
	if err != nil {
		logger.Error().Err(err).Msg("同步未绑定用户失败")
		return c.Send("❌ 操作失败: " + err.Error())
	}

	text := result.FormatResult("未绑定用户扫描")

	if dryRun {
		text += "\n\n📝 这是预览模式。\n使用 `/syncunbound true` 确认执行删除。"
	}

	// 显示详情（限制数量）
	if len(result.Details) > 0 && len(result.Details) <= 30 {
		text += "\n\n**详情:**\n"
		for _, detail := range result.Details {
			text += fmt.Sprintf("• %s\n", detail)
		}
	} else if len(result.Details) > 30 {
		text += fmt.Sprintf("\n\n共发现 %d 个未绑定用户", len(result.Details))
	}

	return c.Send(text, tele.ModeMarkdown)
}

// BindAllIDs /bindall_id 批量绑定 Emby ID
func BindAllIDs(c tele.Context) error {
	c.Send("⏳ 正在批量绑定 Emby ID...")

	batchSvc := service.NewBatchService()

	result, err := batchSvc.BindAllIDs()
	if err != nil {
		logger.Error().Err(err).Msg("批量绑定 ID 失败")
		return c.Send("❌ 操作失败: " + err.Error())
	}

	text := result.FormatResult("批量绑定 Emby ID")

	// 显示未找到的用户
	notFound := 0
	for _, detail := range result.Details {
		if strings.HasPrefix(detail, "未找到") {
			notFound++
		}
	}
	if notFound > 0 {
		text += fmt.Sprintf("\n\n⚠️ %d 个 Emby 用户未在数据库中找到", notFound)
	}

	return c.Send(text, tele.ModeMarkdown)
}

// RenewAll /renewall 批量续期
func RenewAll(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("📅 **批量续期**\n\n使用方式:\n`/renewall <天数> [等级]`\n\n示例:\n`/renewall 30` - 所有用户续期30天\n`/renewall 30 b` - 仅等级b用户续期30天", tele.ModeMarkdown)
	}

	days, err := strconv.Atoi(args[0])
	if err != nil || days <= 0 {
		return c.Send("❌ 请输入有效的天数")
	}

	var level models.UserLevel
	if len(args) > 1 {
		level = models.UserLevel(strings.ToLower(args[1]))
	}

	c.Send(fmt.Sprintf("⏳ 正在为用户续期 %d 天...", days))

	batchSvc := service.NewBatchService()

	result, err := batchSvc.RenewAll(days, level)
	if err != nil {
		logger.Error().Err(err).Msg("批量续期失败")
		return c.Send("❌ 操作失败: " + err.Error())
	}

	return c.Send(result.FormatResult("批量续期"), tele.ModeMarkdown)
}

// CheckExpiredManual /check_ex 手动执行到期检测
func CheckExpiredManual(c tele.Context) error {
	c.Send("⏳ 正在执行到期检测...")

	expirySvc := service.NewExpiryService()
	expirySvc.SetBot(c.Bot())

	result, err := expirySvc.CheckExpired()
	if err != nil {
		logger.Error().Err(err).Msg("到期检测失败")
		return c.Send("❌ 到期检测失败: " + err.Error())
	}

	text := fmt.Sprintf(
		"📊 **到期检测结果**\n\n"+
			"检测用户: %d\n"+
			"过期用户: %d\n"+
			"成功禁用: %d\n"+
			"禁用失败: %d",
		result.Checked,
		result.Expired,
		result.Disabled,
		result.Failed,
	)

	if len(result.ExpiredUsers) > 0 && len(result.ExpiredUsers) <= 20 {
		text += "\n\n**过期用户:**\n"
		for _, user := range result.ExpiredUsers {
			text += fmt.Sprintf("• %s\n", user)
		}
	}

	return c.Send(text, tele.ModeMarkdown)
}

// CheckActivityManual /check_activity 手动执行活跃度检测
func CheckActivityManual(c tele.Context) error {
	c.Send("⏳ 正在执行活跃度检测...")

	activitySvc := service.NewActivityService()
	activitySvc.SetBot(c.Bot())

	result, err := activitySvc.CheckLowActivity()
	if err != nil {
		logger.Error().Err(err).Msg("活跃度检测失败")
		return c.Send("❌ 活跃度检测失败: " + err.Error())
	}

	text := result.FormatResult()

	if len(result.InactiveUsers) > 0 && len(result.InactiveUsers) <= 20 {
		text += "\n\n**不活跃用户:**\n"
		for _, user := range result.InactiveUsers {
			text += fmt.Sprintf("• %s\n", user)
		}
	}

	return c.Send(text, tele.ModeMarkdown)
}

// RegisterBatchHandlers 注册批量管理命令
func RegisterBatchHandlers(bot *tele.Bot) {
	// 这些命令需要管理员权限，应在 adminGroup 中注册
	// 这里仅提供函数引用

	logger.Info().Msg("批量管理命令处理器已加载: syncgroupm, syncunbound, bindall_id, renewall, check_ex, check_activity")
}
