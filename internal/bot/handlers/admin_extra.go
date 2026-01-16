// Package handlers 额外的管理员命令
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

// UInfo 查询用户信息 /uinfo <用户名或ID>
func UInfo(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	args := c.Args()
	if len(args) < 1 {
		return c.Reply("📝 **用法：** `/uinfo <用户名或TG ID或Emby ID>`\n\n" +
			"示例：\n" +
			"• `/uinfo 小明` - 按用户名查询\n" +
			"• `/uinfo 123456789` - 按 TG ID 查询\n" +
			"• `/uinfo abc123def` - 按 Emby ID 查询", tele.ModeMarkdown)
	}

	query := strings.Join(args, " ")
	repo := repository.NewEmbyRepository()

	// 尝试多种方式查询
	user, err := repo.GetByAny(query)
	if err != nil {
		// 尝试按数字 ID 查询
		if tgID, parseErr := strconv.ParseInt(query, 10, 64); parseErr == nil {
			user, err = repo.GetByTG(tgID)
		}
	}

	if err != nil || user == nil {
		return c.Reply(fmt.Sprintf("❓ 未找到用户：`%s`", query), tele.ModeMarkdown)
	}

	// 格式化用户信息
	name := "未设置"
	if user.Name != nil {
		name = *user.Name
	}

	embyID := "未绑定"
	if user.EmbyID != nil && *user.EmbyID != "" {
		embyID = *user.EmbyID
	}

	lvStr := user.GetLevelName()

	exStr := "无"
	if user.Ex != nil {
		exStr = user.Ex.Format("2006-01-02 15:04:05")
	}

	crStr := "未知"
	if user.Cr != nil {
		crStr = user.Cr.Format("2006-01-02 15:04:05")
	}

	chStr := "从未"
	if user.Ch != nil {
		chStr = user.Ch.Format("2006-01-02 15:04:05")
	}

	// 尝试获取 Emby 用户信息
	var embyInfo string
	if user.EmbyID != nil && *user.EmbyID != "" {
		client := emby.GetClient()
		embyUser, err := client.GetUser(*user.EmbyID)
		if err == nil && embyUser != nil {
			embyInfo = fmt.Sprintf(
				"\n\n**📺 Emby 信息：**\n"+
					"• 用户名: %s\n"+
					"• 管理员: %v\n"+
					"• 已禁用: %v",
				embyUser.Name,
				embyUser.Policy != nil && embyUser.Policy.IsAdmin,
				embyUser.Policy != nil && embyUser.Policy.IsDisabled,
			)
		}
	}

	text := fmt.Sprintf(
		"**📋 用户信息**\n\n"+
			"**👤 基本信息：**\n"+
			"• TG ID: `%d`\n"+
			"• 用户名: %s\n"+
			"• Emby ID: `%s`\n"+
			"• 等级: %s\n"+
			"• 积分: %d\n"+
			"• 邀请: %d\n\n"+
			"**📅 时间信息：**\n"+
			"• 创建时间: %s\n"+
			"• 到期时间: %s\n"+
			"• 最后活跃: %s%s",
		user.TG,
		name,
		embyID,
		lvStr,
		user.Iv,
		user.Us,
		crStr,
		exStr,
		chStr,
		embyInfo,
	)

	return c.Reply(text, tele.ModeMarkdown)
}

// CoinsAll 批量发放积分 /coinsall <积分数> [等级]
func CoinsAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	args := c.Args()
	if len(args) < 1 {
		return c.Reply("📝 **用法：** `/coinsall <积分数> [等级]`\n\n" +
			"等级说明：\n" +
			"• `a` - 白名单用户\n" +
			"• `b` - 普通用户\n" +
			"• `all` - 所有有账户的用户（默认）\n\n" +
			"示例：\n" +
			"• `/coinsall 100` - 给所有用户发 100 积分\n" +
			"• `/coinsall 50 a` - 给白名单用户发 50 积分", tele.ModeMarkdown)
	}

	coins, err := strconv.Atoi(args[0])
	if err != nil {
		return c.Reply("❌ 积分数必须是整数")
	}

	level := "all"
	if len(args) >= 2 {
		level = strings.ToLower(args[1])
	}

	repo := repository.NewEmbyRepository()
	var users []models.Emby

	switch level {
	case "a":
		users, err = repo.GetByLevel(models.LevelA)
	case "b":
		users, err = repo.GetByLevel(models.LevelB)
	case "all":
		users, err = repo.GetActiveUsers()
	default:
		return c.Reply("❌ 无效的等级，请使用 a、b 或 all")
	}

	if err != nil {
		return c.Reply("❌ 获取用户列表失败")
	}

	if len(users) == 0 {
		return c.Reply("❓ 未找到符合条件的用户")
	}

	// 批量更新积分
	successCount := 0
	for _, user := range users {
		newIV := user.Iv + coins
		if err := repo.UpdateFields(user.TG, map[string]interface{}{"iv": newIV}); err != nil {
			logger.Error().Err(err).Int64("tg", user.TG).Msg("更新用户积分失败")
		} else {
			successCount++
		}
	}

	logger.Info().
		Int("coins", coins).
		Str("level", level).
		Int("success", successCount).
		Int64("admin", c.Sender().ID).
		Msg("批量发放积分")

	return c.Reply(fmt.Sprintf(
		"✅ **批量发放积分完成**\n\n"+
			"发放积分: %d %s\n"+
			"目标等级: %s\n"+
			"成功用户: %d/%d",
		coins, cfg.Money,
		level,
		successCount, len(users),
	), tele.ModeMarkdown)
}

// CallAll 广播消息 /callall <消息内容>
func CallAll(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	// 获取消息内容（支持回复消息或直接输入）
	var message string
	if c.Message().ReplyTo != nil {
		message = c.Message().ReplyTo.Text
	} else {
		args := c.Args()
		if len(args) < 1 {
			return c.Reply("📝 **用法：** `/callall <消息内容>`\n\n" +
				"或者回复一条消息并使用 `/callall`\n\n" +
				"注意：消息会发送给所有有 Emby 账户的用户", tele.ModeMarkdown)
		}
		message = strings.Join(args, " ")
	}

	if message == "" {
		return c.Reply("❌ 消息内容不能为空")
	}

	repo := repository.NewEmbyRepository()
	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Reply("❌ 获取用户列表失败")
	}

	if len(users) == 0 {
		return c.Reply("❓ 没有可发送的用户")
	}

	// 发送提示
	status, _ := c.Bot().Reply(c.Message(), fmt.Sprintf("📤 正在发送消息给 %d 个用户...", len(users)))

	// 广播消息
	successCount := 0
	failCount := 0
	
	broadcastText := fmt.Sprintf(
		"📢 **系统通知**\n\n%s\n\n—— %s",
		message,
		time.Now().Format("2006-01-02 15:04"),
	)

	for _, user := range users {
		chat := &tele.Chat{ID: user.TG}
		_, err := c.Bot().Send(chat, broadcastText, tele.ModeMarkdown)
		if err != nil {
			failCount++
			logger.Debug().Err(err).Int64("tg", user.TG).Msg("发送广播失败")
		} else {
			successCount++
		}
		
		// 避免触发 Telegram API 限制
		time.Sleep(50 * time.Millisecond)
	}

	// 更新状态消息
	resultText := fmt.Sprintf(
		"✅ **广播完成**\n\n"+
			"成功: %d\n"+
			"失败: %d\n"+
			"总计: %d",
		successCount, failCount, len(users),
	)

	if status != nil {
		if err := c.Bot().Edit(status, resultText, tele.ModeMarkdown); err != nil {
			logger.Debug().Err(err).Msg("Edit status failed")
		}
	} else {
		c.Reply(resultText, tele.ModeMarkdown)
	}

	logger.Info().
		Int("success", successCount).
		Int("fail", failCount).
		Int64("admin", c.Sender().ID).
		Msg("广播消息完成")

	return nil
}

// UCr 创建非TG用户 /ucr <用户名> <天数>
func UCr(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	args := c.Args()
	if len(args) < 2 {
		return c.Reply("📝 **用法：** `/ucr <用户名> <天数>`\n\n" +
			"创建一个不与 TG 绑定的 Emby 账户\n\n" +
			"示例：`/ucr guest01 30`", tele.ModeMarkdown)
	}

	username := args[0]
	days, err := strconv.Atoi(args[1])
	if err != nil || days <= 0 {
		return c.Reply("❌ 天数必须是正整数")
	}

	// 创建 Emby 用户
	client := emby.GetClient()
	result, err := client.CreateUser(username, days)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ 创建用户失败：%v", err))
	}

	text := fmt.Sprintf(
		"✅ **创建用户成功**\n\n"+
			"• 用户名: `%s`\n"+
			"• 密码: `%s`\n"+
			"• Emby ID: `%s`\n"+
			"• 有效期: %d 天\n"+
			"• 到期时间: %s\n\n"+
			"⚠️ 此账户未绑定 TG，密码请妥善保存",
		username,
		result.Password,
		result.UserID,
		days,
		result.ExpiryDate.Format("2006-01-02"),
	)

	logger.Info().
		Str("username", username).
		Int("days", days).
		Str("embyID", result.UserID).
		Int64("admin", c.Sender().ID).
		Msg("创建非TG用户")

	return c.Reply(text, tele.ModeMarkdown)
}

// URm 删除指定用户 /urm <用户名或Emby ID>
func URm(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	args := c.Args()
	if len(args) < 1 {
		return c.Reply("📝 **用法：** `/urm <用户名或Emby ID>`\n\n" +
			"删除指定的 Emby 账户（同时删除 Emby 账户和数据库记录）", tele.ModeMarkdown)
	}

	query := strings.Join(args, " ")
	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	// 先尝试在数据库中查找
	user, _ := repo.GetByAny(query)
	
	if user != nil && user.EmbyID != nil && *user.EmbyID != "" {
		// 删除 Emby 账户
		if err := client.DeleteUser(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Str("embyID", *user.EmbyID).Msg("删除 Emby 账户失败")
		}

		// 清空数据库记录
		if err := repo.UpdateFields(user.TG, map[string]interface{}{
			"embyid": nil,
			"name":   nil,
			"pwd":    nil,
			"pwd2":   nil,
			"lv":     models.LevelD,
			"cr":     nil,
			"ex":     nil,
		}); err != nil {
			logger.Error().Err(err).Int64("tg", user.TG).Msg("清空用户记录失败")
		}

		logger.Info().
			Str("query", query).
			Int64("tg", user.TG).
			Int64("admin", c.Sender().ID).
			Msg("删除用户账户")

		return c.Reply(fmt.Sprintf("✅ 已删除用户：`%s`", query), tele.ModeMarkdown)
	}

	// 如果数据库中没有，尝试直接按 Emby 用户名或 ID 删除
	embyUser, err := client.GetUserByName(query)
	if err != nil {
		return c.Reply(fmt.Sprintf("❓ 未找到用户：`%s`", query), tele.ModeMarkdown)
	}

	if err := client.DeleteUser(embyUser.ID); err != nil {
		return c.Reply(fmt.Sprintf("❌ 删除用户失败：%v", err))
	}

	logger.Info().
		Str("query", query).
		Str("embyID", embyUser.ID).
		Int64("admin", c.Sender().ID).
		Msg("删除 Emby 用户（不在数据库中）")

	return c.Reply(fmt.Sprintf("✅ 已删除 Emby 用户：`%s`（此用户不在数据库中）", query), tele.ModeMarkdown)
}

// CoinsClear 清空用户积分 /coinsclear [等级]
func CoinsClear(c tele.Context) error {
	cfg := config.Get()
	if c.Sender().ID != cfg.Owner {
		return c.Reply("❌ 只有 Owner 可以使用此命令")
	}

	args := c.Args()
	level := "all"
	if len(args) >= 1 {
		level = strings.ToLower(args[0])
	}

	repo := repository.NewEmbyRepository()
	var users []models.Emby
	var err error

	switch level {
	case "a":
		users, err = repo.GetByLevel(models.LevelA)
	case "b":
		users, err = repo.GetByLevel(models.LevelB)
	case "c":
		users, err = repo.GetByLevel(models.LevelC)
	case "d":
		users, err = repo.GetByLevel(models.LevelD)
	case "all":
		users, err = repo.GetAll()
	default:
		return c.Reply("❌ 无效的等级，请使用 a、b、c、d 或 all")
	}

	if err != nil {
		return c.Reply("❌ 获取用户列表失败")
	}

	// 批量清空积分
	successCount := 0
	for _, user := range users {
		if user.Iv > 0 {
			if err := repo.UpdateFields(user.TG, map[string]interface{}{"iv": 0}); err != nil {
				logger.Error().Err(err).Int64("tg", user.TG).Msg("清空用户积分失败")
			} else {
				successCount++
			}
		}
	}

	logger.Info().
		Str("level", level).
		Int("success", successCount).
		Int64("owner", c.Sender().ID).
		Msg("批量清空积分")

	return c.Reply(fmt.Sprintf(
		"✅ **清空积分完成**\n\n"+
			"目标等级: %s\n"+
			"清空用户: %d",
		level,
		successCount,
	), tele.ModeMarkdown)
}

// Deleted 清理死号（已注销的TG账号） /deleted
func Deleted(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	c.Reply("⏳ 正在清理死号，请稍候...")

	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	// 获取所有有Emby账户的用户
	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Reply("❌ 获取用户列表失败")
	}

	var deletedCount, cleanedCount int
	var deletedNames []string

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 检查用户在Emby中是否存在
		embyUser, err := client.GetUser(*user.EmbyID)
		if err != nil || embyUser == nil {
			// Emby中不存在，清理数据库记录
			if err := repo.UpdateFields(user.TG, map[string]interface{}{
				"embyid": nil,
				"name":   nil,
				"pwd":    nil,
				"pwd2":   nil,
				"lv":     models.LevelD,
			}); err != nil {
				logger.Error().Err(err).Int64("tg", user.TG).Msg("清理死号失败")
			} else {
				cleanedCount++
				if user.Name != nil {
					deletedNames = append(deletedNames, *user.Name)
				}
			}
			continue
		}

		// 检查用户是否被禁用（可能是已删除/注销）
		if embyUser.Policy != nil && embyUser.Policy.IsDisabled {
			// 删除Emby用户
			if err := client.DeleteUser(*user.EmbyID); err != nil {
				logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("删除死号Emby账户失败")
			} else {
				deletedCount++
				if user.Name != nil {
					deletedNames = append(deletedNames, *user.Name)
				}
			}

			// 清理数据库记录
			if err := repo.UpdateFields(user.TG, map[string]interface{}{
				"embyid": nil,
				"name":   nil,
				"pwd":    nil,
				"pwd2":   nil,
				"lv":     models.LevelD,
			}); err != nil {
				logger.Error().Err(err).Int64("tg", user.TG).Msg("清理死号数据库记录失败")
			}
		}
	}

	logger.Info().
		Int("deleted", deletedCount).
		Int("cleaned", cleanedCount).
		Int64("admin", c.Sender().ID).
		Msg("管理员执行清理死号")

	text := fmt.Sprintf(
		"✅ **死号清理完成**\n\n"+
			"🗑 删除Emby账户: %d\n"+
			"🧹 清理数据库记录: %d",
		deletedCount,
		cleanedCount,
	)

	if len(deletedNames) > 0 && len(deletedNames) <= 10 {
		text += "\n\n清理的用户:\n"
		for _, name := range deletedNames {
			text += fmt.Sprintf("• `%s`\n", name)
		}
	} else if len(deletedNames) > 10 {
		text += fmt.Sprintf("\n\n共清理 %d 个用户", len(deletedNames))
	}

	return c.Reply(text, tele.ModeMarkdown)
}

// LowActivity 手动运行活跃检测 /low_activity
func LowActivity(c tele.Context) error {
	cfg := config.Get()
	if !cfg.IsAdmin(c.Sender().ID) {
		return c.Reply("❌ 您没有权限使用此命令")
	}

	if !cfg.Open.LowActivity {
		return c.Reply("⚠️ 活跃检测功能已关闭\n\n请在配置文件中启用 `open.low_activity`", tele.ModeMarkdown)
	}

	c.Reply("⏳ 正在执行活跃检测，请稍候...")

	// 获取不活跃用户列表
	repo := repository.NewEmbyRepository()
	client := emby.GetClient()

	// 检测不活跃天数
	inactiveDays := cfg.Open.InactiveDays
	if inactiveDays <= 0 {
		inactiveDays = 30
	}

	users, err := repo.GetInactiveUsers(inactiveDays)
	if err != nil {
		return c.Reply("❌ 获取用户列表失败")
	}

	var inactiveCount, bannedCount int
	var inactiveNames []string

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		// 检查最后活跃时间
		embyUser, err := client.GetUser(*user.EmbyID)
		if err != nil || embyUser == nil {
			continue
		}

		// 如果超过不活跃天数，禁用账户
		var lastActivity time.Time
		if embyUser.LastActivityDate != nil {
			lastActivity = *embyUser.LastActivityDate
		} else if user.Ch != nil {
			lastActivity = *user.Ch
		}

		daysSinceActivity := int(time.Since(lastActivity).Hours() / 24)
		if daysSinceActivity >= inactiveDays {
			// 禁用账户
			if err := client.DisableUser(*user.EmbyID); err != nil {
				logger.Error().Err(err).Str("embyID", *user.EmbyID).Msg("禁用不活跃用户失败")
			} else {
				bannedCount++
				if user.Name != nil {
					inactiveNames = append(inactiveNames, fmt.Sprintf("%s (%d天)", *user.Name, daysSinceActivity))
				}

				// 更新用户等级为封禁
				repo.UpdateFields(user.TG, map[string]interface{}{"lv": models.LevelE})
			}
			inactiveCount++
		}
	}

	logger.Info().
		Int("inactive", inactiveCount).
		Int("banned", bannedCount).
		Int64("admin", c.Sender().ID).
		Msg("管理员执行活跃检测")

	text := fmt.Sprintf(
		"✅ **活跃检测完成**\n\n"+
			"📊 检测天数: %d 天\n"+
			"👤 不活跃用户: %d\n"+
			"🚫 已禁用账户: %d",
		inactiveDays,
		inactiveCount,
		bannedCount,
	)

	if len(inactiveNames) > 0 && len(inactiveNames) <= 10 {
		text += "\n\n不活跃用户:\n"
		for _, name := range inactiveNames {
			text += fmt.Sprintf("• `%s`\n", name)
		}
	} else if len(inactiveNames) > 10 {
		text += fmt.Sprintf("\n\n共 %d 个不活跃用户", len(inactiveNames))
	}

	return c.Reply(text, tele.ModeMarkdown)
}
