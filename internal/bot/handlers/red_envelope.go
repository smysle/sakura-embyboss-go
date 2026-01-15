// Package handlers 红包处理器
package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/service"
)

// HandleRedEnvelope /red 发红包命令
// 用法:
// - /red <金额> <个数> [祝福语] - 普通红包
// - 回复消息 /red <金额> [祝福语] - 专属红包
func HandleRedEnvelope(c tele.Context) error {
	cfg := config.Get()
	if !cfg.RedEnvelope.Enabled {
		return c.Send("❌ 红包功能已关闭")
	}

	// 检查是否在群组
	if c.Chat().Type == tele.ChatPrivate {
		return c.Send("❌ 红包只能在群组中发送")
	}

	args := c.Args()

	// 检查是否是专属红包（回复消息）
	if c.Message().ReplyTo != nil && c.Message().ReplyTo.Sender != nil {
		return handlePrivateRedEnvelope(c, args)
	}

	// 普通红包
	return handleNormalRedEnvelope(c, args)
}

// handleNormalRedEnvelope 处理普通红包
func handleNormalRedEnvelope(c tele.Context, args []string) error {
	if len(args) < 2 {
		return c.Send(
			"🧧 **发红包**\n\n"+
				"**普通红包**: `/red <金额> <个数> [祝福语]`\n"+
				"**专属红包**: 回复某人消息并发送 `/red <金额> [祝福语]`\n\n"+
				"示例:\n"+
				"- `/red 100 10` - 发 100 积分，10 个红包\n"+
				"- `/red 50 5 恭喜发财` - 带祝福语\n"+
				"- 回复 + `/red 50 给你的专属红包` - 专属红包",
			tele.ModeMarkdown,
		)
	}

	// 解析金额
	amount, err := strconv.Atoi(args[0])
	if err != nil || amount <= 0 {
		return c.Send("❌ 无效的金额")
	}

	// 解析个数
	count, err := strconv.Atoi(args[1])
	if err != nil || count <= 0 || count > 100 {
		return c.Send("❌ 个数应在 1-100 之间")
	}

	if amount < count {
		return c.Send("❌ 红包金额不能少于红包个数")
	}

	// 解析祝福语
	message := ""
	if len(args) > 2 {
		message = strings.Join(args[2:], " ")
	}

	// 创建红包
	redSvc := service.NewRedEnvelopeService()
	result, err := redSvc.CreateEnvelope(&service.CreateEnvelopeRequest{
		SenderTG:    c.Sender().ID,
		SenderName:  c.Sender().FirstName,
		TotalAmount: amount,
		TotalCount:  count,
		Message:     message,
		Type:        "random",
		IsPrivate:   false,
		TargetTG:    nil,
		ChatID:      c.Chat().ID,
	})

	if err != nil {
		return handleRedEnvelopeError(c, err, amount)
	}

	return sendRedEnvelopeMessage(c, result, false, nil)
}

// handlePrivateRedEnvelope 处理专属红包
func handlePrivateRedEnvelope(c tele.Context, args []string) error {
	cfg := config.Get()

	if !cfg.RedEnvelope.AllowPrivate {
		return c.Send("❌ 专属红包功能未开启")
	}

	if len(args) < 1 {
		return c.Send(
			"🧧 **专属红包**\n\n"+
				"用法: 回复某人消息并发送 `/red <金额> [祝福语]`\n\n"+
				"示例: `/red 50 给你的小礼物`",
			tele.ModeMarkdown,
		)
	}

	// 解析金额
	amount, err := strconv.Atoi(args[0])
	if err != nil || amount <= 0 {
		return c.Send("❌ 无效的金额")
	}

	// 检查不能给自己发专属红包
	targetUser := c.Message().ReplyTo.Sender
	if targetUser.ID == c.Sender().ID {
		return c.Send("❌ 不能给自己发专属红包")
	}

	// 解析祝福语
	message := ""
	if len(args) > 1 {
		message = strings.Join(args[1:], " ")
	}

	// 创建专属红包
	targetTG := targetUser.ID
	redSvc := service.NewRedEnvelopeService()
	result, err := redSvc.CreateEnvelope(&service.CreateEnvelopeRequest{
		SenderTG:    c.Sender().ID,
		SenderName:  c.Sender().FirstName,
		TotalAmount: amount,
		TotalCount:  1, // 专属红包只有 1 个
		Message:     message,
		Type:        "private",
		IsPrivate:   true,
		TargetTG:    &targetTG,
		TargetName:  targetUser.FirstName,
		ChatID:      c.Chat().ID,
	})

	if err != nil {
		return handleRedEnvelopeError(c, err, amount)
	}

	return sendRedEnvelopeMessage(c, result, true, targetUser)
}

// handleRedEnvelopeError 处理红包错误
func handleRedEnvelopeError(c tele.Context, err error, amount int) error {
	cfg := config.Get()
	var errMsg string
	switch {
	case errors.Is(err, service.ErrRedEnvelopeDisabled):
		errMsg = "❌ 红包功能已关闭"
	case errors.Is(err, service.ErrInsufficientBalance):
		errMsg = fmt.Sprintf("❌ 积分不足！需要 %d %s", amount, cfg.Money)
	default:
		errMsg = "❌ " + err.Error()
	}
	return c.Send(errMsg)
}

// sendRedEnvelopeMessage 发送红包消息
func sendRedEnvelopeMessage(c tele.Context, result *service.CreateEnvelopeResult, isPrivate bool, targetUser *tele.User) error {
	cfg := config.Get()

	var text string
	if isPrivate && targetUser != nil {
		// 专属红包消息
		text = fmt.Sprintf(
			"🧧 **%s 发了一个专属红包**\n\n"+
				"🎯 **收件人**: [%s](tg://user?id=%d)\n"+
				"💰 **金额**: %d %s\n"+
				"💬 **%s**",
			c.Sender().FirstName,
			targetUser.FirstName, targetUser.ID,
			result.TotalAmount, cfg.Money,
			result.Message,
		)
	} else {
		// 普通红包消息
		text = fmt.Sprintf(
			"🧧 **%s 发了一个红包**\n\n"+
				"💰 **总金额**: %d %s\n"+
				"🎁 **红包个数**: %d 个\n"+
				"💬 **%s**",
			c.Sender().FirstName,
			result.TotalAmount, cfg.Money,
			result.TotalCount,
			result.Message,
		)
	}

	// 创建抢红包按钮
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("🧧 抢红包", fmt.Sprintf("grab_red:%s", result.UUID)),
		),
	)

	// 删除原命令消息
	c.Delete()

	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleGrabRedEnvelope 处理抢红包回调
func HandleGrabRedEnvelope(c tele.Context, uuid string) error {
	cfg := config.Get()
	redSvc := service.NewRedEnvelopeService()

	result, err := redSvc.ReceiveEnvelope(uuid, c.Sender().ID, c.Sender().FirstName)
	if err != nil {
		var errMsg string
		switch {
		case errors.Is(err, service.ErrEnvelopeNotFound):
			errMsg = "❌ 红包不存在"
		case errors.Is(err, service.ErrEnvelopeExpired):
			errMsg = "❌ 红包已过期"
		case errors.Is(err, service.ErrEnvelopeFinished):
			errMsg = "❌ 红包已被抢完"
		case errors.Is(err, service.ErrAlreadyReceived):
			errMsg = "❌ 您已领取过此红包"
		case errors.Is(err, service.ErrCannotReceiveOwnRed):
			errMsg = "❌ 不能领取自己的红包"
		case errors.Is(err, service.ErrNotTargetUser):
			errMsg = "❌ 这是专属红包，您不是目标用户"
		default:
			errMsg = "❌ " + err.Error()
		}
		return c.Respond(&tele.CallbackResponse{
			Text:      errMsg,
			ShowAlert: true,
		})
	}

	// 领取成功
	alertText := fmt.Sprintf("🎉 恭喜！获得 %d %s", result.Amount, cfg.Money)
	if result.IsLucky {
		alertText += "\n👑 手气最佳！"
	}

	c.Respond(&tele.CallbackResponse{
		Text:      alertText,
		ShowAlert: true,
	})

	// 如果红包已抢完，更新消息
	if result.IsFinished {
		return updateRedEnvelopeMessage(c, uuid)
	}

	// 更新红包消息显示剩余数量
	return updateRedEnvelopeMessagePartial(c, uuid, result)
}

// updateRedEnvelopeMessage 更新红包消息（已抢完）
func updateRedEnvelopeMessage(c tele.Context, uuid string) error {
	cfg := config.Get()
	redSvc := service.NewRedEnvelopeService()

	envelope, records, err := redSvc.GetEnvelopeInfo(uuid)
	if err != nil {
		return nil
	}

	// 构建领取详情
	var sb strings.Builder

	if envelope.IsPrivate {
		// 专属红包
		if len(records) > 0 {
			r := records[0]
			sb.WriteString(fmt.Sprintf(
				"🧧 **专属红包已被领取**\n\n"+
					"💰 金额: %d %s\n"+
					"💬 %s\n\n"+
					"🕶️ **%s** 的专属红包已被 [%s](tg://user?id=%d) 领取",
				envelope.TotalAmount, cfg.Money,
				envelope.Message,
				envelope.SenderName,
				r.ReceiverName, r.ReceiverTG,
			))
		}
	} else {
		// 普通红包
		sb.WriteString(fmt.Sprintf(
			"🧧 **%s 的红包已被抢完**\n\n"+
				"💰 总金额: %d %s | 🎁 %d 个\n"+
				"💬 %s\n\n"+
				"**领取详情:**\n",
			envelope.SenderName,
			envelope.TotalAmount, cfg.Money,
			envelope.TotalCount,
			envelope.Message,
		))

		// 找出手气最佳
		var luckyTG int64
		maxAmount := 0
		for _, r := range records {
			if r.Amount > maxAmount {
				maxAmount = r.Amount
				luckyTG = r.ReceiverTG
			}
		}

		for i, r := range records {
			luckyMark := ""
			if r.ReceiverTG == luckyTG {
				luckyMark = " 👑"
			}
			sb.WriteString(fmt.Sprintf("%d. %s: %d %s%s\n", i+1, r.ReceiverName, r.Amount, cfg.Money, luckyMark))
		}
	}

	return c.Edit(sb.String(), keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// updateRedEnvelopeMessagePartial 更新红包消息（还有剩余）
func updateRedEnvelopeMessagePartial(c tele.Context, uuid string, result *service.ReceiveEnvelopeResult) error {
	cfg := config.Get()

	text := fmt.Sprintf(
		"🧧 **%s 发了一个红包**\n\n"+
			"💰 **总金额**: %d %s\n"+
			"🎁 **红包个数**: %d 个\n"+
			"📦 **剩余**: %d 个\n"+
			"💬 **%s**",
		result.SenderName,
		result.TotalAmount, cfg.Money,
		result.TotalCount,
		result.RemainCount,
		result.Message,
	)

	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("🧧 抢红包", fmt.Sprintf("grab_red:%s", uuid)),
		),
	)

	return c.Edit(text, markup, tele.ModeMarkdown)
}
