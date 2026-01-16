// Package handlers 批量管理扩展命令
package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// KickNotEmby /kick_not_emby 踢出无Emby账户的群成员
func KickNotEmby(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return c.Send("❌ 此命令只能在群组中使用")
	}

	args := c.Args()
	if len(args) == 0 || args[0] != "true" {
		return c.Send("⚠️ 此命令将踢出所有没有 Emby 账户的群成员\n\n确认执行请发送: `/kick_not_emby true`", tele.ModeMarkdown)
	}

	// 删除命令消息
	c.Delete()

	waitMsg, _ := c.Bot().Send(c.Chat(), "⏳ 正在获取群组成员列表...")

	// 获取所有有Emby账户的用户TG ID
	repo := repository.NewEmbyRepository()
	embyUsers, err := repo.GetAllWithEmby()
	if err != nil {
		return c.Send("❌ 获取用户数据失败")
	}

	embyTGs := make(map[int64]bool)
	for _, u := range embyUsers {
		embyTGs[u.TG] = true
	}

	cfg := config.Get()
	if len(cfg.Groups) == 0 {
		return c.Send("❌ 未配置群组")
	}

	// 获取群组成员（这里简化处理，实际需要分页获取）
	// telebot v3 不直接支持获取所有成员，需要通过其他方式
	// 这里返回提示信息
	if waitMsg != nil {
		c.Bot().Delete(waitMsg)
	}

	var kicked int
	var sb strings.Builder
	sb.WriteString("🔍 **踢出无账户用户结果**\n\n")
	sb.WriteString("⚠️ 由于 Telegram API 限制，无法直接获取所有群成员\n")
	sb.WriteString("建议使用 `/syncgroupm true` 命令从数据库端进行同步\n\n")
	sb.WriteString(fmt.Sprintf("📊 当前数据库中有 %d 个有效 Emby 用户", len(embyUsers)))

	_ = kicked // 避免未使用变量警告

	return c.Send(sb.String(), tele.ModeMarkdown)
}

// ScanEmbyName /scan_embyname 扫描重复的Emby用户名
func ScanEmbyName(c tele.Context) error {
	c.Delete()

	waitMsg, _ := c.Bot().Send(c.Chat(), "⏳ 正在扫描重复用户名...")

	repo := repository.NewEmbyRepository()
	users, err := repo.GetAllWithName()
	if err != nil {
		if waitMsg != nil {
			c.Bot().Delete(waitMsg)
		}
		return c.Send("❌ 获取用户数据失败")
	}

	// 统计重复用户名
	nameCount := make(map[string][]models.Emby)
	for _, u := range users {
		if u.Name != nil && *u.Name != "" {
			nameCount[*u.Name] = append(nameCount[*u.Name], u)
		}
	}

	// 筛选重复的
	var duplicates []string
	for name, userList := range nameCount {
		if len(userList) > 1 {
			var userInfo strings.Builder
			userInfo.WriteString(fmt.Sprintf("\n**用户名**: `%s`\n", name))
			for _, u := range userList {
				embyID := "无"
				if u.EmbyID != nil {
					embyID = *u.EmbyID
				}
				userInfo.WriteString(fmt.Sprintf("  - TG ID: `%d` | Emby ID: `%s`\n", u.TG, embyID))
			}
			duplicates = append(duplicates, userInfo.String())
		}
	}

	if waitMsg != nil {
		c.Bot().Delete(waitMsg)
	}

	if len(duplicates) == 0 {
		return c.Send("✅ 未发现重复用户名")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 **发现 %d 个重复用户名**\n", len(duplicates)))
	for _, dup := range duplicates {
		sb.WriteString(dup)
	}
	sb.WriteString("\n💡 使用 `/only_rm_record <tg_id>` 删除多余记录")

	return c.Send(sb.String(), tele.ModeMarkdown)
}

// OnlyRmEmby /only_rm_emby 仅删除Emby账户（保留数据库记录）
func OnlyRmEmby(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: `/only_rm_emby <emby_id 或 emby用户名>`\n\n仅删除 Emby 服务器上的账户，保留 Bot 数据库记录", tele.ModeMarkdown)
	}

	target := args[0]
	client := emby.GetClient()

	// 先尝试直接用ID删除
	err := client.DeleteUser(target)
	if err == nil {
		logger.Info().Str("emby_id", target).Int64("admin", c.Sender().ID).Msg("仅删除Emby账户")
		return c.Send(fmt.Sprintf("✅ 已删除 Emby 账户: `%s`\n\n⚠️ 数据库记录已保留", target), tele.ModeMarkdown)
	}

	// 尝试用用户名查询
	user, err := client.GetUserByName(target)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ 未找到 Emby 用户: %s", target))
	}

	err = client.DeleteUser(user.ID)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ 删除失败: %s", err.Error()))
	}

	logger.Info().Str("emby_id", user.ID).Str("name", target).Int64("admin", c.Sender().ID).Msg("仅删除Emby账户")
	return c.Send(fmt.Sprintf("✅ 已删除 Emby 账户: `%s` (ID: `%s`)\n\n⚠️ 数据库记录已保留", target, user.ID), tele.ModeMarkdown)
}

// OnlyRmRecord /only_rm_record 仅删除数据库记录（保留Emby账户）
func OnlyRmRecord(c tele.Context) error {
	args := c.Args()
	var tgID int64
	var err error

	if len(args) > 0 {
		tgID, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return c.Send("❌ 无效的 TG ID")
		}
	} else if c.Message().ReplyTo != nil {
		tgID = c.Message().ReplyTo.Sender.ID
	} else {
		return c.Send("用法: `/only_rm_record <tg_id>` 或回复用户消息\n\n仅删除 Bot 数据库记录，保留 Emby 服务器账户", tele.ModeMarkdown)
	}

	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(tgID)
	if err != nil || user == nil {
		return c.Send("❌ 未找到该用户的数据库记录")
	}

	userName := "无"
	if user.Name != nil {
		userName = *user.Name
	}

	// 删除记录
	err = repo.DeleteByTG(tgID)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ 删除失败: %s", err.Error()))
	}

	logger.Info().Int64("tg", tgID).Str("name", userName).Int64("admin", c.Sender().ID).Msg("仅删除数据库记录")
	return c.Send(fmt.Sprintf("✅ 已删除数据库记录\n\nTG ID: `%d`\n用户名: `%s`\n\n⚠️ Emby 服务器账户已保留", tgID, userName), tele.ModeMarkdown)
}

// RestoreFromDB /restore_from_db 从数据库恢复账户
func RestoreFromDB(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 || args[0] != "true" {
		return c.Send("⚠️ 此命令将根据数据库记录在 Emby 服务器上重建账户\n\n确认执行请发送: `/restore_from_db true`", tele.ModeMarkdown)
	}

	c.Delete()

	waitMsg, _ := c.Bot().Send(c.Chat(), "⏳ 正在恢复账户...")

	repo := repository.NewEmbyRepository()
	users, err := repo.GetAllWithName()
	if err != nil {
		if waitMsg != nil {
			c.Bot().Delete(waitMsg)
		}
		return c.Send("❌ 获取用户数据失败")
	}

	client := emby.GetClient()
	cfg := config.Get()

	var restored, failed int
	var sb strings.Builder
	sb.WriteString("🔄 **账户恢复结果**\n\n")

	for _, u := range users {
		if u.Name == nil || *u.Name == "" {
			continue
		}

		// 计算剩余天数
		var days int
		if u.Ex != nil {
			remaining := time.Until(*u.Ex)
			if remaining > 0 {
				days = int(remaining.Hours() / 24)
			} else {
				days = 30 // 默认30天
			}
		} else {
			days = 30
		}

		// 在Emby创建账户
		result, err := client.CreateUser(*u.Name, days)
		if err != nil {
			failed++
			logger.Error().Err(err).Str("name", *u.Name).Msg("恢复账户失败")
			continue
		}

		// 更新数据库
		repo.UpdateFields(u.TG, map[string]interface{}{
			"embyid": result.UserID,
			"pwd":    result.Password,
		})

		restored++

		// 通知用户
		userChat := &tele.Chat{ID: u.TG}
		notifyMsg := fmt.Sprintf(
			"🤖 **账户恢复成功**\n\n"+
				"🧬 用户名: `%s`\n"+
				"🪅 新密码: `%s`\n"+
				"🔮 安全码: `%s`\n\n"+
				"🔗 登录地址: %s",
			*u.Name,
			result.Password,
			getSecurityCode(u.Pwd2),
			cfg.Emby.Line,
		)
		c.Bot().Send(userChat, notifyMsg, tele.ModeMarkdown)
	}

	if waitMsg != nil {
		c.Bot().Delete(waitMsg)
	}

	sb.WriteString(fmt.Sprintf("✅ 成功恢复: %d 个\n", restored))
	sb.WriteString(fmt.Sprintf("❌ 失败: %d 个\n", failed))

	logger.Info().Int("restored", restored).Int("failed", failed).Int64("admin", c.Sender().ID).Msg("从数据库恢复账户")
	return c.Send(sb.String(), tele.ModeMarkdown)
}

// EmbyAdmin /embyadmin 设置自己的Emby管理员权限
func EmbyAdmin(c tele.Context) error {
	c.Delete()

	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(c.Sender().ID)
	if err != nil || user == nil || !user.HasEmbyAccount() {
		return c.Send("❌ 您没有绑定 Emby 账户")
	}

	client := emby.GetClient()
	err = client.SetUserAdminPolicy(*user.EmbyID, true)
	if err != nil {
		logger.Error().Err(err).Int64("tg", c.Sender().ID).Msg("设置Emby管理员权限失败")
		return c.Send(fmt.Sprintf("❌ 设置失败: %s", err.Error()))
	}

	logger.Info().Int64("tg", c.Sender().ID).Str("emby_id", *user.EmbyID).Msg("设置Emby管理员权限")
	
	msg, _ := c.Bot().Send(c.Chat(), "✅ 已开启 Emby 控制台权限\n\n⚠️ 注意：此权限可能在续期时被重置")
	
	// 60秒后删除消息
	go func() {
		time.Sleep(60 * time.Second)
		c.Bot().Delete(msg)
	}()

	return nil
}
