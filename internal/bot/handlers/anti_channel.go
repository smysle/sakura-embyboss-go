// Package handlers 反皮套人命令处理器
package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// UnbanChannel /unban_channel 解封频道账号
func UnbanChannel(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /unban_channel <频道ID>\n\n示例: /unban_channel -1001234567890")
	}

	channelID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的频道ID")
	}

	cfg := config.Get()
	chat := &tele.Chat{ID: cfg.GroupID}

	// 解封频道
	member := &tele.ChatMember{
		User: &tele.User{ID: channelID},
	}

	err = c.Bot().Unban(chat, member.User)
	if err != nil {
		logger.Error().Err(err).Int64("channel", channelID).Msg("解封频道失败")
		return c.Send(fmt.Sprintf("❌ 解封失败: %s", err.Error()))
	}

	logger.Info().Int64("channel", channelID).Int64("admin", c.Sender().ID).Msg("解封频道")
	return c.Send(fmt.Sprintf("✅ 已解封频道 `%d`", channelID), tele.ModeMarkdown)
}

// WhiteChannel /white_channel 添加频道白名单
func WhiteChannel(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /white_channel <频道ID>\n\n将频道添加到白名单，不受自动封禁影响")
	}

	channelID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的频道ID")
	}

	cfg := config.Get()

	// 检查是否已在白名单
	for _, id := range cfg.AntiChannel.WhiteList {
		if id == channelID {
			return c.Send("⚠️ 该频道已在白名单中")
		}
	}

	// 添加到白名单
	err = config.UpdateAndSave(func(cfg *config.Config) {
		cfg.AntiChannel.WhiteList = append(cfg.AntiChannel.WhiteList, channelID)
	})

	if err != nil {
		return c.Send("❌ 保存配置失败")
	}

	logger.Info().Int64("channel", channelID).Int64("admin", c.Sender().ID).Msg("添加频道白名单")
	return c.Send(fmt.Sprintf("✅ 已将频道 `%d` 添加到白名单", channelID), tele.ModeMarkdown)
}

// RevWhiteChannel /rev_white_channel 移除频道白名单
func RevWhiteChannel(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /rev_white_channel <频道ID>\n\n将频道从白名单移除")
	}

	channelID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的频道ID")
	}

	cfg := config.Get()

	// 查找并移除
	found := false
	var newList []int64
	for _, id := range cfg.AntiChannel.WhiteList {
		if id == channelID {
			found = true
			continue
		}
		newList = append(newList, id)
	}

	if !found {
		return c.Send("⚠️ 该频道不在白名单中")
	}

	// 更新配置
	err = config.UpdateAndSave(func(cfg *config.Config) {
		cfg.AntiChannel.WhiteList = newList
	})

	if err != nil {
		return c.Send("❌ 保存配置失败")
	}

	// 可选：封禁该频道
	chat := &tele.Chat{ID: cfg.GroupID}
	c.Bot().Ban(chat, &tele.ChatMember{User: &tele.User{ID: channelID}})

	logger.Info().Int64("channel", channelID).Int64("admin", c.Sender().ID).Msg("移除频道白名单并封禁")
	return c.Send(fmt.Sprintf("✅ 已将频道 `%d` 从白名单移除并封禁", channelID), tele.ModeMarkdown)
}

// ListWhiteChannels /list_white_channels 列出频道白名单
func ListWhiteChannels(c tele.Context) error {
	cfg := config.Get()

	if len(cfg.AntiChannel.WhiteList) == 0 {
		return c.Send("📋 频道白名单为空")
	}

	var sb strings.Builder
	sb.WriteString("📋 **频道白名单**\n\n")
	for i, id := range cfg.AntiChannel.WhiteList {
		sb.WriteString(fmt.Sprintf("%d. `%d`\n", i+1, id))
	}

	return c.Send(sb.String(), tele.ModeMarkdown)
}

// ToggleAntiChannel /anti_channel 开关频道过滤
func ToggleAntiChannel(c tele.Context) error {
	cfg := config.Get()

	newStatus := !cfg.AntiChannel.Enabled

	err := config.UpdateAndSave(func(cfg *config.Config) {
		cfg.AntiChannel.Enabled = newStatus
	})

	if err != nil {
		return c.Send("❌ 保存配置失败")
	}

	status := "已关闭"
	if newStatus {
		status = "已开启"
	}

	logger.Info().Bool("enabled", newStatus).Int64("admin", c.Sender().ID).Msg("切换反皮套人开关")
	return c.Send(fmt.Sprintf("✅ 反皮套人功能 %s", status))
}
