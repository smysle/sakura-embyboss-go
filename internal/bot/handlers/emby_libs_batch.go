// Package handlers 批量媒体库控制命令
package handlers

import (
	"fmt"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// EmbyLibsBlockAll /embylibs_blockall 批量关闭所有用户媒体库
func EmbyLibsBlockAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Send("❌ 您没有权限执行此操作")
	}

	c.Send("⏳ 正在关闭所有用户媒体库...")

	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Send("❌ 获取用户列表失败")
	}

	success, failed := 0, 0

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 禁用所有媒体库
		if err := client.DisableAllLibraries(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("禁用媒体库失败")
			failed++
		} else {
			success++
		}
	}

	return c.Send(fmt.Sprintf("✅ 批量关闭媒体库完成\n\n成功: %d\n失败: %d", success, failed))
}

// EmbyLibsUnblockAll /embylibs_unblockall 批量开启所有用户媒体库
func EmbyLibsUnblockAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Send("❌ 您没有权限执行此操作")
	}

	c.Send("⏳ 正在开启所有用户媒体库...")

	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Send("❌ 获取用户列表失败")
	}

	success, failed := 0, 0

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 启用所有媒体库
		if err := client.EnableAllLibraries(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("启用媒体库失败")
			failed++
		} else {
			success++
		}
	}

	return c.Send(fmt.Sprintf("✅ 批量开启媒体库完成\n\n成功: %d\n失败: %d", success, failed))
}

// ExtraEmbyLibsBlockAll /extraembylibs_blockall 批量关闭所有用户额外媒体库
func ExtraEmbyLibsBlockAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Send("❌ 您没有权限执行此操作")
	}

	if len(cfg.Emby.ExtraLibs) == 0 {
		return c.Send("❌ 未配置额外媒体库")
	}

	c.Send("⏳ 正在关闭所有用户额外媒体库...")

	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Send("❌ 获取用户列表失败")
	}

	success, failed := 0, 0

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 隐藏额外媒体库
		if err := client.HideFolders(*user.EmbyID, cfg.Emby.ExtraLibs); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("隐藏额外媒体库失败")
			failed++
		} else {
			success++
		}
	}

	return c.Send(fmt.Sprintf("✅ 批量关闭额外媒体库完成\n\n成功: %d\n失败: %d", success, failed))
}

// ExtraEmbyLibsUnblockAll /extraembylibs_unblockall 批量开启所有用户额外媒体库
func ExtraEmbyLibsUnblockAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Send("❌ 您没有权限执行此操作")
	}

	if len(cfg.Emby.ExtraLibs) == 0 {
		return c.Send("❌ 未配置额外媒体库")
	}

	c.Send("⏳ 正在开启所有用户额外媒体库...")

	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Send("❌ 获取用户列表失败")
	}

	success, failed := 0, 0

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 显示额外媒体库
		if err := client.ShowFolders(*user.EmbyID, cfg.Emby.ExtraLibs); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("显示额外媒体库失败")
			failed++
		} else {
			success++
		}
	}

	return c.Send(fmt.Sprintf("✅ 批量开启额外媒体库完成\n\n成功: %d\n失败: %d", success, failed))
}
