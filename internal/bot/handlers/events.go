// Package handlers 群组事件处理器
package handlers

import (
	"fmt"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// OnChatMember 处理群组成员变更事件
func OnChatMember(c tele.Context) error {
	update := c.ChatMember()
	if update == nil {
		return nil
	}

	cfg := config.Get()

	// 检查是否是配置的群组
	if !cfg.IsInGroup(c.Chat().ID) {
		return nil
	}

	// 检查是否是成员离开
	// OldChatMember 是 member/administrator，NewChatMember 是 left/kicked
	oldStatus := update.OldChatMember.Role
	newStatus := update.NewChatMember.Role

	// 用户离开群组的情况
	isLeaving := (oldStatus == tele.Member || oldStatus == tele.Administrator) &&
		(newStatus == tele.Left || newStatus == tele.Kicked)

	if !isLeaving {
		return nil
	}

	user := update.NewChatMember.User
	if user == nil {
		return nil
	}

	logger.Info().
		Int64("tg", user.ID).
		Str("username", user.Username).
		Str("old_status", string(oldStatus)).
		Str("new_status", string(newStatus)).
		Msg("用户退出群组")

	// 检查是否开启退群封禁
	if !cfg.Open.LeaveBan {
		logger.Debug().Int64("tg", user.ID).Msg("退群封禁未开启，跳过处理")
		return nil
	}

	// 查找用户的 Emby 账户
	repo := repository.NewEmbyRepository()
	embyUser, err := repo.GetByTG(user.ID)
	if err != nil || embyUser == nil {
		logger.Debug().Int64("tg", user.ID).Msg("用户不在数据库中")
		return nil
	}

	// 检查是否有 Emby 账户
	if !embyUser.HasEmbyAccount() {
		logger.Debug().Int64("tg", user.ID).Msg("用户没有 Emby 账户")
		return nil
	}

	// 白名单用户不受影响
	if embyUser.Lv == models.LevelA {
		logger.Debug().Int64("tg", user.ID).Msg("白名单用户退群，不处理")
		return nil
	}

	// 禁用 Emby 账户
	client := emby.GetClient()
	if embyUser.EmbyID != nil && *embyUser.EmbyID != "" {
		if err := client.DisableUser(*embyUser.EmbyID); err != nil {
			logger.Error().Err(err).Int64("tg", user.ID).Msg("禁用 Emby 账户失败")
		} else {
			logger.Info().Int64("tg", user.ID).Msg("用户退群，已禁用 Emby 账户")
		}
	}

	// 更新数据库状态为封禁
	repo.UpdateFields(user.ID, map[string]interface{}{
		"lv": models.LevelE,
	})

	// 通知 Owner
	if cfg.Owner != 0 {
		ownerChat := &tele.Chat{ID: cfg.Owner}
		userName := user.FirstName
		if user.Username != "" {
			userName = "@" + user.Username
		}

		notifyMsg := fmt.Sprintf(
			"⚠️ **用户退群通知**\n\n"+
				"用户: %s (ID: `%d`)\n"+
				"Emby用户名: `%s`\n"+
				"操作: 已自动禁用账户",
			userName,
			user.ID,
			getEmbyName(embyUser.Name),
		)

		if bot := c.Bot(); bot != nil {
			bot.Send(ownerChat, notifyMsg, tele.ModeMarkdown)
		}
	}

	return nil
}

// OnUserJoined 处理用户加入群组（可选：自动解封）
func OnUserJoined(c tele.Context) error {
	update := c.ChatMember()
	if update == nil {
		return nil
	}

	cfg := config.Get()

	// 检查是否是配置的群组
	if !cfg.IsInGroup(c.Chat().ID) {
		return nil
	}

	// 检查是否是成员加入
	oldStatus := update.OldChatMember.Role
	newStatus := update.NewChatMember.Role

	isJoining := (oldStatus == tele.Left || oldStatus == tele.Kicked || oldStatus == "") &&
		(newStatus == tele.Member || newStatus == tele.Administrator)

	if !isJoining {
		return nil
	}

	user := update.NewChatMember.User
	if user == nil {
		return nil
	}

	logger.Info().
		Int64("tg", user.ID).
		Str("username", user.Username).
		Msg("用户加入群组")

	// 初始化用户记录（如果不存在）
	repo := repository.NewEmbyRepository()
	_, err := repo.GetByTG(user.ID)
	if err != nil {
		// 用户不存在，创建记录
		newUser := &models.Emby{
			TG: user.ID,
			Lv: models.LevelD, // 游客
		}
		repo.Create(newUser)
		logger.Info().Int64("tg", user.ID).Msg("新用户加入群组，已创建记录")
	}

	return nil
}
