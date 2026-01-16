// Package handlers Owner 命令处理器
package handlers

import (
	"fmt"
	"os"
	"strconv"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// OwnerConfig 显示系统配置信息（旧版本，保留兼容）
// 新版本配置面板使用 config_panel.go 中的 Config 函数
func OwnerConfig(c tele.Context) error {
	cfg := config.Get()

	text := fmt.Sprintf(
		"⚙️ **系统配置**\n\n"+
			"**Bot 名称**: %s\n"+
			"**注册状态**: %s\n"+
			"**最大用户数**: %d\n"+
			"**签到功能**: %s\n"+
			"**兑换功能**: %s\n"+
			"**邀请功能**: %s\n",
		cfg.BotName,
		boolToStatus(cfg.Open.Status),
		cfg.Open.MaxUsers,
		boolToStatus(cfg.Open.Checkin),
		boolToStatus(cfg.Open.Exchange),
		boolToStatus(cfg.Open.Invite),
	)

	return c.Send(text, keyboards.AdminPanelKeyboard(true), tele.ModeMarkdown)
}

func boolToStatus(b bool) string {
	if b {
		return "✅ 开启"
	}
	return "❌ 关闭"
}

// ProAdmin /proadmin 添加管理员
func ProAdmin(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /proadmin <用户ID>")
	}

	tgID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的用户ID")
	}

	// 使用热重载方式添加管理员
	err = config.UpdateAndSave(func(cfg *config.Config) {
		cfg.AddAdmin(tgID)
	})

	if err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 用户 %d 已添加为管理员", tgID))
}

// RevAdmin /revadmin 移除管理员
func RevAdmin(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("用法: /revadmin <用户ID>")
	}

	tgID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("❌ 无效的用户ID")
	}

	cfg := config.Get()
	if !cfg.IsAdmin(tgID) {
		return c.Send("❌ 该用户不是管理员")
	}

	// 使用热重载方式移除管理员
	err = config.UpdateAndSave(func(cfg *config.Config) {
		cfg.RemoveAdmin(tgID)
	})

	if err != nil {
		return c.Send("❌ 保存配置失败")
	}

	return c.Send(fmt.Sprintf("✅ 用户 %d 已移除管理员权限", tgID))
}

// BackupDB /backup_db 备份数据库
func BackupDB(c tele.Context) error {
	c.Send("⏳ 正在备份数据库...")

	backupSvc := service.NewBackupService()
	result, err := backupSvc.Backup(true) // 压缩备份
	if err != nil {
		logger.Error().Err(err).Msg("数据库备份失败")
		return c.Send("❌ 备份失败: " + err.Error())
	}

	// 发送备份文件
	file, err := os.Open(result.FilePath)
	if err != nil {
		return c.Send(fmt.Sprintf(
			"✅ 备份完成\n"+
				"文件: %s\n"+
				"大小: %s\n"+
				"记录数: %d\n"+
				"耗时: %v",
			result.Filename,
			service.FormatSize(result.Size),
			result.Records,
			result.Duration,
		))
	}
	defer file.Close()

	doc := &tele.Document{
		File:     tele.FromReader(file),
		FileName: result.Filename,
		Caption: fmt.Sprintf(
			"💾 数据库备份\n大小: %s | 记录: %d",
			service.FormatSize(result.Size),
			result.Records,
		),
	}

	return c.Send(doc)
}

// BanAll /banall 禁用所有用户
func BanAll(c tele.Context) error {
	c.Send("⏳ 正在禁用所有用户...")

	repo := repository.NewEmbyRepository()
	users, err := repo.GetActiveUsers()
	if err != nil {
		return c.Send("❌ 获取用户列表失败")
	}

	client := emby.GetClient()
	successCount := 0
	failCount := 0

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		if err := client.DisableUser(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("禁用用户失败")
			failCount++
			continue
		}

		// 更新数据库状态
		repo.UpdateFields(user.TG, map[string]interface{}{"lv": models.LevelE})
		successCount++
	}

	return c.Send(fmt.Sprintf("✅ 禁用完成\n成功: %d\n失败: %d", successCount, failCount))
}

// UnbanAll /unbanall 解禁所有用户
func UnbanAll(c tele.Context) error {
	c.Send("⏳ 正在解禁所有用户...")

	repo := repository.NewEmbyRepository()
	users, err := repo.GetByLevel(models.LevelE)
	if err != nil {
		return c.Send("❌ 获取用户列表失败")
	}

	client := emby.GetClient()
	successCount := 0
	failCount := 0

	for _, user := range users {
		if user.EmbyID == nil || *user.EmbyID == "" {
			continue
		}

		if err := client.EnableUser(*user.EmbyID); err != nil {
			logger.Warn().Err(err).Int64("tg", user.TG).Msg("解禁用户失败")
			failCount++
			continue
		}

		// 更新数据库状态
		repo.UpdateFields(user.TG, map[string]interface{}{"lv": models.LevelD})
		successCount++
	}

	return c.Send(fmt.Sprintf("✅ 解禁完成\n成功: %d\n失败: %d", successCount, failCount))
}

// Paolu /paolu 跑路
func Paolu(c tele.Context) error {
	return c.Send(
		"⚠️ **危险操作**\n\n"+
			"此操作将删除所有用户账户和数据!\n"+
			"确定要继续吗?",
		keyboards.ConfirmKeyboard("confirm_paolu", "cancel_paolu"),
		tele.ModeMarkdown,
	)
}
