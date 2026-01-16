// Package utils Bot 工具函数
package utils

import (
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// DeleteAfter 定时删除消息
func DeleteAfter(b *tele.Bot, msg *tele.Message, seconds int) {
	if msg == nil || b == nil {
		return
	}
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		if err := b.Delete(msg); err != nil {
			logger.Debug().Err(err).Msg("删除消息失败")
		}
	}()
}

// SendAndDelete 发送消息并定时删除
func SendAndDelete(c tele.Context, text string, seconds int, opts ...interface{}) error {
	msg, err := c.Bot().Send(c.Chat(), text, opts...)
	if err != nil {
		return err
	}
	DeleteAfter(c.Bot(), msg, seconds)
	return nil
}

// ReplyAndDelete 回复消息并定时删除
func ReplyAndDelete(c tele.Context, text string, seconds int, opts ...interface{}) error {
	msg, err := c.Bot().Reply(c.Message(), text, opts...)
	if err != nil {
		return err
	}
	DeleteAfter(c.Bot(), msg, seconds)
	// 同时删除原消息
	DeleteAfter(c.Bot(), c.Message(), 0)
	return nil
}

// IsCallbackOwner 检查回调是否来自原始消息发送者
// 用于防止其他用户点击别人的按钮
func IsCallbackOwner(c tele.Context, originalUserID int64) bool {
	return c.Sender().ID == originalUserID
}

// RespondNotOwner 回复非所有者的提示
func RespondNotOwner(c tele.Context) error {
	return c.Respond(&tele.CallbackResponse{
		Text:      "❌ 这不是给你的按钮哦~",
		ShowAlert: true,
	})
}

// DeleteOriginalMessage 删除原消息（用于命令处理后）
func DeleteOriginalMessage(c tele.Context) {
	if c.Message() != nil {
		go func() {
			if err := c.Bot().Delete(c.Message()); err != nil {
				logger.Debug().Err(err).Msg("删除原消息失败")
			}
		}()
	}
}
