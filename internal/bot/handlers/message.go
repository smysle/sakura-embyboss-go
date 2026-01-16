// Package handlers 消息处理器
package handlers

import (
	"fmt"
	"strings"
	"unicode"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
	"github.com/smysle/sakura-embyboss-go/pkg/utils"
)

// OnText 处理文本消息
func OnText(c tele.Context) error {
	// 只处理私聊消息
	if c.Chat().Type != tele.ChatPrivate {
		return nil
	}

	text := strings.TrimSpace(c.Text())
	userID := c.Sender().ID

	// 检查用户会话状态
	sessionMgr := session.GetManager()
	state := sessionMgr.GetState(userID)

	switch state {
	case session.StateWaitingCode:
		return handleCodeInput(c, text)
	case session.StateWaitingName:
		return handleNameInput(c, text)
	case session.StateWaitingCreateInfo:
		return handleCreateInfoInput(c, text)
	case session.StateWaitingSecurityCode:
		return handleSecurityCodeInput(c, text)
	case session.StateWaitingNewPassword:
		return handleNewPasswordInput(c, text)
	case session.StateWaitingDeleteConfirm:
		return handleDeleteConfirmInput(c, text)
	case session.StateWaitingChangeTGInfo:
		return handleChangeTGInfoInput(c, text)
	case session.StateWaitingBindTGInfo:
		return handleBindTGInfoInput(c, text)
	case session.StateMoviePilotSearch:
		return HandleMoviePilotSearchInput(c)
	case session.StateMoviePilotSelectMedia:
		return HandleMPSelectDownload(c)
	case session.StateWaitingInput:
		// 配置面板输入处理
		action := sessionMgr.GetStringAction(userID)
		if action != "" {
			return ProcessConfigInput(c, action)
		}
		return nil
	default:
		// 没有特殊状态，忽略消息
		return nil
	}
}

// Cancel /cancel 取消当前操作
func Cancel(c tele.Context) error {
	sessionMgr := session.GetManager()
	sessionMgr.ClearSession(c.Sender().ID)

	return c.Send("✅ 已取消操作\n\n发送 /start 返回主菜单")
}

// handleCodeInput 处理注册码输入
func handleCodeInput(c tele.Context, code string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 验证注册码格式
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return c.Send("❌ 请输入有效的注册码")
	}

	// 先验证注册码是否有效
	codeSvc := service.NewCodeService()
	days, err := codeSvc.ValidateCode(code)
	if err != nil {
		sessionMgr.ClearSession(userID)
		return c.Send(fmt.Sprintf("❌ %s\n\n发送 /start 返回主菜单", err.Error()))
	}

	// 检查用户是否已有账户
	repo := repository.NewEmbyRepository()
	user, _ := repo.GetByTG(userID)

	if user != nil && user.HasEmbyAccount() {
		// 已有账户，直接续期
		addedDays, err := codeSvc.ExtendByCode(userID, code)
		sessionMgr.ClearSession(userID)

		if err != nil {
			return c.Send(fmt.Sprintf("❌ 续期失败: %s", err.Error()))
		}

		return c.Send(
			fmt.Sprintf(
				"✅ **续期成功！**\n\n"+
					"🎁 已增加 **%d** 天有效期",
				addedDays,
			),
			keyboards.BackKeyboard("back_start"),
			tele.ModeMarkdown,
		)
	}

	// 没有账户，需要输入用户名和安全码
	sessionMgr.SetState(userID, session.StateWaitingCreateInfo)
	sessionMgr.SetData(userID, "code", code)
	sessionMgr.SetData(userID, "days", days)

	return c.Send(
		"✅ **注册码验证成功！**\n\n"+
			"📝 请输入 `[用户名] [安全码]`\n"+
			"🌰 例如：`sakura 1234`\n\n"+
			"• 用户名支持中/英文/emoji，禁止特殊字符\n"+
			"• 安全码为4-6位数字，用于敏感操作验证\n\n"+
			"_发送 /cancel 取消操作_",
		tele.ModeMarkdown,
	)
}

// handleCreateInfoInput 处理用户创建信息输入（用户名+安全码）
func handleCreateInfoInput(c tele.Context, input string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 解析用户名和安全码
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return c.Send("❌ 格式错误\n\n请输入 `[用户名] [安全码]`\n例如：`sakura 1234`", tele.ModeMarkdown)
	}

	username := parts[0]
	securityCode := parts[1]

	// 验证安全码格式（4-6位数字）
	if !isValidSecurityCode(securityCode) {
		return c.Send("❌ 安全码格式错误\n\n安全码必须为4-6位数字")
	}

	// 验证用户名（允许中英文和emoji）
	if !isValidDisplayName(username) {
		return c.Send("❌ 用户名格式无效\n\n请使用2-20位字符，不含特殊符号")
	}

	// 获取之前保存的注册码
	codeVal, ok := sessionMgr.GetData(userID, "code")
	if !ok {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 会话已过期，请重新操作\n\n发送 /start 返回主菜单")
	}
	code := codeVal.(string)

	// 发送等待消息
	waitMsg, _ := c.Bot().Send(c.Chat(), "⏳ 正在创建账户，请稍候...")

	// 使用注册码创建账户
	codeSvc := service.NewCodeService()
	result, err := codeSvc.UseCodeWithSecurity(userID, username, code, securityCode)

	// 清除会话
	sessionMgr.ClearSession(userID)

	if err != nil {
		logger.Error().Err(err).Int64("tg", userID).Str("code", code).Msg("使用注册码失败")
		if waitMsg != nil {
			c.Bot().Delete(waitMsg)
		}
		return c.Send(fmt.Sprintf("❌ 创建账户失败: %s", err.Error()))
	}

	cfg := config.Get()
	text := fmt.Sprintf(
		"🎉 **账户创建成功！**\n\n"+
			"**用户名**: `%s`\n"+
			"**密码**: `%s`\n"+
			"**安全码**: `%s` (仅此一次显示)\n"+
			"**有效期**: %d 天\n"+
			"**到期时间**: %s\n\n"+
			"🔗 **登录地址**: %s\n\n"+
			"⚠️ _请妥善保管您的账户信息，安全码用于敏感操作验证_",
		result.Username,
		result.Password,
		securityCode,
		result.Days,
		result.ExpiryDate.Format("2006-01-02"),
		cfg.Emby.Line,
	)

	if waitMsg != nil {
		c.Bot().Delete(waitMsg)
	}
	return c.Send(text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
}

// handleNameInput 处理用户名输入（旧版兼容）
func handleNameInput(c tele.Context, username string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 验证用户名格式
	username = strings.TrimSpace(username)
	if !isValidUsername(username) {
		return c.Send("❌ 用户名格式无效\n\n请使用 3-20 位英文字母和数字")
	}

	// 获取之前保存的注册码
	codeVal, ok := sessionMgr.GetData(userID, "code")
	if !ok {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 会话已过期，请重新操作\n\n发送 /start 返回主菜单")
	}
	code := codeVal.(string)

	// 使用注册码创建账户（生成随机安全码）
	securityCode, _ := utils.GenerateNumericCode(4)
	codeSvc := service.NewCodeService()
	result, err := codeSvc.UseCodeWithSecurity(userID, username, code, securityCode)

	// 清除会话
	sessionMgr.ClearSession(userID)

	if err != nil {
		logger.Error().Err(err).Int64("tg", userID).Str("code", code).Msg("使用注册码失败")
		return c.Send(fmt.Sprintf("❌ 创建账户失败: %s", err.Error()))
	}

	cfg := config.Get()
	text := fmt.Sprintf(
		"🎉 **账户创建成功！**\n\n"+
			"**用户名**: `%s`\n"+
			"**密码**: `%s`\n"+
			"**安全码**: `%s` (仅此一次显示)\n"+
			"**有效期**: %d 天\n"+
			"**到期时间**: %s\n\n"+
			"🔗 **登录地址**: %s\n\n"+
			"_请妥善保管您的账户信息_",
		result.Username,
		result.Password,
		securityCode,
		result.Days,
		result.ExpiryDate.Format("2006-01-02"),
		cfg.Emby.Line,
	)

	return c.Send(text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
}

// handleSecurityCodeInput 处理安全码验证输入
func handleSecurityCodeInput(c tele.Context, inputCode string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()
	action := sessionMgr.GetAction(userID)

	// 获取用户的安全码
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(userID)
	if err != nil || user == nil {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 用户不存在")
	}

	// 验证安全码
	if user.Pwd2 == nil || *user.Pwd2 != inputCode {
		return c.Send("❌ 安全码错误，请重新输入\n\n_发送 /cancel 取消操作_", tele.ModeMarkdown)
	}

	// 根据操作类型执行不同逻辑
	switch action {
	case session.ActionResetPwd:
		// 安全码验证通过，进入密码设置阶段
		sessionMgr.SetState(userID, session.StateWaitingNewPassword)
		return c.Send(
			"✅ **安全码验证通过**\n\n"+
				"请输入新密码 (留空直接回车则重置为空密码)\n\n"+
				"_发送 /cancel 取消操作_",
			tele.ModeMarkdown,
		)

	case session.ActionDeleteAccount:
		// 安全码验证通过，显示确认删除
		sessionMgr.SetState(userID, session.StateWaitingDeleteConfirm)
		return c.Send(
			"⚠️ **确认删除账户**\n\n"+
				"如果您的账户到期，我们将封存您的账户，但仍保留数据。\n"+
				"而如果您选择删除，服务器会将您此前的活动数据**全部删除**。\n\n"+
				"确认删除请输入: `确认删除`\n\n"+
				"_发送 /cancel 取消操作_",
			tele.ModeMarkdown,
		)

	default:
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 未知操作")
	}
}

// handleNewPasswordInput 处理新密码输入
func handleNewPasswordInput(c tele.Context, newPassword string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 获取用户信息
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(userID)
	if err != nil || user == nil || user.EmbyID == nil {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 用户不存在或没有账户")
	}

	// 发送等待消息
	waitMsg, _ := c.Bot().Send(c.Chat(), "⏳ 正在重置密码...")

	// 重置密码
	client := emby.GetClient()
	var resetErr error
	
	newPassword = strings.TrimSpace(newPassword)
	if newPassword == "" || newPassword == "/cancel" {
		// 重置为空密码
		resetErr = client.ResetPassword(*user.EmbyID)
		newPassword = "(空密码)"
	} else {
		// 设置新密码
		resetErr = client.SetPassword(*user.EmbyID, newPassword)
	}

	sessionMgr.ClearSession(userID)

	if waitMsg != nil {
		c.Bot().Delete(waitMsg)
	}

	if resetErr != nil {
		logger.Error().Err(resetErr).Str("embyID", *user.EmbyID).Msg("重置密码失败")
		return c.Send("❌ 重置密码失败，请稍后重试")
	}

	logger.Info().Int64("tg", userID).Msg("用户重置密码成功")

	return c.Send(
		fmt.Sprintf("✅ **密码重置成功**\n\n新密码: `%s`", newPassword),
		keyboards.BackKeyboard("back_start"),
		tele.ModeMarkdown,
	)
}

// handleDeleteConfirmInput 处理删除账户确认输入
func handleDeleteConfirmInput(c tele.Context, input string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 验证确认文本
	input = strings.TrimSpace(input)
	if input != "确认删除" {
		return c.Send("❌ 输入错误\n\n确认删除请输入: `确认删除`\n\n_发送 /cancel 取消操作_", tele.ModeMarkdown)
	}

	// 获取用户信息
	repo := repository.NewEmbyRepository()
	user, err := repo.GetByTG(userID)
	if err != nil || user == nil || user.EmbyID == nil {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 用户不存在或没有账户")
	}

	embyID := *user.EmbyID

	// 发送等待消息
	waitMsg, _ := c.Bot().Send(c.Chat(), "⏳ 正在删除账户...")

	// 删除 Emby 账户
	client := emby.GetClient()
	if err := client.DeleteUser(embyID); err != nil {
		logger.Error().Err(err).Str("embyID", embyID).Msg("删除 Emby 账户失败")
		sessionMgr.ClearSession(userID)
		if waitMsg != nil {
			c.Bot().Delete(waitMsg)
		}
		return c.Send("❌ 删除 Emby 账户失败，请联系管理员")
	}

	// 清空数据库记录
	if err := repo.UpdateFields(userID, map[string]interface{}{
		"embyid": nil,
		"name":   nil,
		"pwd":    nil,
		"pwd2":   nil,
		"lv":     "d",
		"cr":     nil,
		"ex":     nil,
	}); err != nil {
		logger.Error().Err(err).Int64("tg", userID).Msg("清空用户记录失败")
	}

	sessionMgr.ClearSession(userID)

	if waitMsg != nil {
		c.Bot().Delete(waitMsg)
	}

	logger.Info().Int64("tg", userID).Str("embyID", embyID).Msg("用户自助删除账户")

	return c.Send(
		"✅ **您的账户已成功删除**\n\n如需再次使用，请重新注册",
		keyboards.BackKeyboard("back_start"),
		tele.ModeMarkdown,
	)
}

// handleChangeTGInfoInput 处理换绑TG信息输入
func handleChangeTGInfoInput(c tele.Context, input string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 解析输入：用户名 安全码/密码
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return c.Send("❌ 格式错误\n\n请输入 `[Emby用户名] [安全码/密码]`\n例如：`sakura 1234`", tele.ModeMarkdown)
	}

	embyName := parts[0]
	credential := parts[1]

	// 查找原账户
	repo := repository.NewEmbyRepository()
	
	// 先在主表查找
	originalUser, err := repo.GetByName(embyName)
	if err != nil || originalUser == nil {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 未找到该Emby账户\n\n如果账户不在Bot中，请使用【绑定TG】功能")
	}

	// 验证安全码或密码
	validCredential := false
	if originalUser.Pwd2 != nil && *originalUser.Pwd2 == credential {
		validCredential = true
	}
	if !validCredential && originalUser.Pwd != nil && *originalUser.Pwd == credential {
		validCredential = true
	}

	if !validCredential {
		return c.Send("❌ 安全码/密码验证失败\n\n_发送 /cancel 取消操作_", tele.ModeMarkdown)
	}

	// 验证通过，需要管理员审核
	sessionMgr.ClearSession(userID)

	cfg := config.Get()
	// 发送给管理员审核
	adminText := fmt.Sprintf(
		"⭕ **#TG改绑申请**\n\n"+
			"用户 [%d](tg://user?id=%d) 申请改绑Emby: `%s`\n"+
			"原TG: `%d`\n\n"+
			"已通过安全码/密码验证\n"+
			"请管理员审核：",
		userID, userID, embyName, originalUser.TG,
	)

	// 发送给owner
	if cfg.Owner != 0 {
		ownerChat := &tele.Chat{ID: cfg.Owner}
		c.Bot().Send(ownerChat, adminText, keyboards.ChangeTGApproveKeyboard(userID, originalUser.TG), tele.ModeMarkdown)
	}

	return c.Send(
		"✅ **验证成功**\n\n"+
			"已向管理员发送换绑申请，请等待审核。",
		keyboards.BackKeyboard("back_start"),
		tele.ModeMarkdown,
	)
}

// handleBindTGInfoInput 处理绑定TG信息输入
func handleBindTGInfoInput(c tele.Context, input string) error {
	userID := c.Sender().ID
	sessionMgr := session.GetManager()

	// 解析输入：用户名 密码
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return c.Send("❌ 格式错误\n\n请输入 `[Emby用户名] [密码]`\n密码为空请填写 `None`", tele.ModeMarkdown)
	}

	embyName := parts[0]
	password := parts[1]
	if password == "None" || password == "none" {
		password = ""
	}

	// 验证Emby账户
	client := emby.GetClient()
	embyUser, err := client.GetUserByName(embyName)
	if err != nil {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 未找到该Emby账户")
	}

	// TODO: 验证密码（需要Emby API支持）
	// 这里暂时跳过密码验证，直接绑定

	// 生成安全码
	securityCode, _ := utils.GenerateNumericCode(4)

	// 绑定到当前用户
	repo := repository.NewEmbyRepository()
	updates := map[string]interface{}{
		"embyid": embyUser.ID,
		"name":   embyName,
		"pwd":    password,
		"pwd2":   securityCode,
		"lv":     "b",
	}

	if err := repo.UpdateFields(userID, updates); err != nil {
		sessionMgr.ClearSession(userID)
		return c.Send("❌ 绑定失败，请稍后重试")
	}

	sessionMgr.ClearSession(userID)

	cfg := config.Get()
	text := fmt.Sprintf(
		"✅ **绑定成功**\n\n"+
			"**用户名**: `%s`\n"+
			"**安全码**: `%s` (仅此一次显示)\n\n"+
			"🔗 **登录地址**: %s",
		embyName, securityCode, cfg.Emby.Line,
	)

	logger.Info().Int64("tg", userID).Str("embyName", embyName).Msg("用户绑定Emby账户")

	return c.Send(text, keyboards.BackKeyboard("back_start"), tele.ModeMarkdown)
}

// isValidUsername 验证用户名格式（仅英文数字下划线）
func isValidUsername(username string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}

	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// isValidDisplayName 验证显示名称格式（允许中英文emoji）
func isValidDisplayName(name string) bool {
	if len(name) < 1 || len([]rune(name)) > 20 {
		return false
	}

	for _, r := range name {
		// 禁止特殊控制字符
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// isValidSecurityCode 验证安全码格式（4-6位数字）
func isValidSecurityCode(code string) bool {
	if len(code) < 4 || len(code) > 6 {
		return false
	}

	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
