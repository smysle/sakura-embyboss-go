// Package service 注册码服务
package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

var (
	ErrCodeNotFound      = errors.New("注册码不存在")
	ErrCodeAlreadyUsed   = errors.New("注册码已被使用")
	ErrCodeInvalid       = errors.New("无效的注册码")
	ErrAlreadyHasAccount = errors.New("您已有账户")
	ErrExchangeDisabled  = errors.New("兑换功能已关闭")
)

// CodeService 注册码服务
type CodeService struct {
	codeRepo *repository.CodeRepository
	embyRepo *repository.EmbyRepository
	cfg      *config.Config
}

// NewCodeService 创建注册码服务
func NewCodeService() *CodeService {
	return &CodeService{
		codeRepo: repository.NewCodeRepository(),
		embyRepo: repository.NewEmbyRepository(),
		cfg:      config.Get(),
	}
}

// GenerateResult 生成结果
type GenerateResult struct {
	Codes    []string
	Count    int
	Days     int
	CreateBy int64
}

// GenerateCodes 生成注册码
func (s *CodeService) GenerateCodes(createBy int64, days int, count int) (*GenerateResult, error) {
	if count <= 0 || count > 100 {
		return nil, errors.New("生成数量应在 1-100 之间")
	}

	if days <= 0 {
		return nil, errors.New("有效天数必须大于 0")
	}

	// 生成注册码
	codes := make([]string, 0, count)
	prefix := s.cfg.Ranks.Logo
	if prefix == "" {
		prefix = "SAKURA"
	}

	for i := 0; i < count; i++ {
		code := s.generateCodeString(prefix)
		codes = append(codes, code)
	}

	// 批量保存到数据库
	if err := s.codeRepo.BatchCreate(codes, createBy, days); err != nil {
		logger.Error().Err(err).Int64("createBy", createBy).Msg("保存注册码失败")
		return nil, fmt.Errorf("保存注册码失败: %w", err)
	}

	logger.Info().
		Int64("createBy", createBy).
		Int("count", count).
		Int("days", days).
		Msg("成功生成注册码")

	return &GenerateResult{
		Codes:    codes,
		Count:    count,
		Days:     days,
		CreateBy: createBy,
	}, nil
}

// generateCodeString 生成注册码字符串
func (s *CodeService) generateCodeString(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	randomPart := strings.ToUpper(hex.EncodeToString(bytes))
	return fmt.Sprintf("%s-%s", prefix, randomPart[:12])
}

// GenerateCode 生成单个注册码字符串（公开方法）
func GenerateCode() string {
	cfg := config.Get()
	prefix := cfg.Ranks.Logo
	if prefix == "" {
		prefix = "SAKURA"
	}
	bytes := make([]byte, 8)
	rand.Read(bytes)
	randomPart := strings.ToUpper(hex.EncodeToString(bytes))
	return fmt.Sprintf("%s-%s", prefix, randomPart[:12])
}

// UseCodeResult 使用注册码结果
type UseCodeResult struct {
	Success    bool
	UserID     string    // Emby 用户 ID
	Username   string    // Emby 用户名
	Password   string    // Emby 密码
	ExpiryDate time.Time // 到期时间
	Days       int       // 获得的天数
}

// UseCode 使用注册码
func (s *CodeService) UseCode(tgID int64, username string, codeStr string) (*UseCodeResult, error) {
	// 检查兑换功能是否开启
	if !s.cfg.Open.Exchange {
		return nil, ErrExchangeDisabled
	}

	// 检查用户是否已有账户
	user, err := s.embyRepo.GetByTG(tgID)
	if err == nil && user.HasEmbyAccount() {
		return nil, ErrAlreadyHasAccount
	}

	// 查找注册码
	code, err := s.codeRepo.GetByCode(codeStr)
	if err != nil {
		return nil, ErrCodeNotFound
	}

	// 检查注册码是否已使用
	if code.IsUsed() {
		return nil, ErrCodeAlreadyUsed
	}

	// 创建 Emby 账户
	embyClient := emby.GetClient()
	createResult, err := embyClient.CreateUser(username, code.Us)
	if err != nil {
		logger.Error().Err(err).Int64("tg", tgID).Str("code", codeStr).Msg("使用注册码创建账户失败")
		return nil, fmt.Errorf("创建账户失败: %w", err)
	}

	// 标记注册码已使用
	if err := s.codeRepo.MarkUsed(codeStr, tgID); err != nil {
		logger.Error().Err(err).Str("code", codeStr).Msg("标记注册码失败")
		// 注意：这里账户已创建，但标记失败，可能需要清理
	}

	// 更新用户数据库记录
	updates := map[string]interface{}{
		"embyid": createResult.UserID,
		"name":   username,
		"pwd":    createResult.Password,
		"ex":     createResult.ExpiryDate,
		"cr":     time.Now(),
	}

	// 确保用户存在
	s.embyRepo.EnsureExists(tgID)
	if err := s.embyRepo.UpdateFields(tgID, updates); err != nil {
		logger.Error().Err(err).Int64("tg", tgID).Msg("更新用户记录失败")
	}

	logger.Info().
		Int64("tg", tgID).
		Str("code", codeStr).
		Str("embyID", createResult.UserID).
		Msg("用户使用注册码成功")

	return &UseCodeResult{
		Success:    true,
		UserID:     createResult.UserID,
		Username:   username,
		Password:   createResult.Password,
		ExpiryDate: createResult.ExpiryDate,
		Days:       code.Us,
	}, nil
}

// ExtendByCode 使用注册码续期（已有账户）
func (s *CodeService) ExtendByCode(tgID int64, codeStr string) (int, error) {
	// 检查用户是否有账户
	user, err := s.embyRepo.GetByTG(tgID)
	if err != nil || !user.HasEmbyAccount() {
		return 0, errors.New("您还没有账户，请先注册")
	}

	// 查找注册码
	code, err := s.codeRepo.GetByCode(codeStr)
	if err != nil {
		return 0, ErrCodeNotFound
	}

	// 检查注册码是否已使用
	if code.IsUsed() {
		return 0, ErrCodeAlreadyUsed
	}

	// 计算新的到期时间
	var newExpiry time.Time
	if user.Ex != nil {
		// 如果还没过期，在现有基础上加
		if user.Ex.After(time.Now()) {
			newExpiry = user.Ex.AddDate(0, 0, code.Us)
		} else {
			// 已过期，从今天开始算
			newExpiry = time.Now().AddDate(0, 0, code.Us)
		}
	} else {
		newExpiry = time.Now().AddDate(0, 0, code.Us)
	}

	// 标记注册码已使用
	if err := s.codeRepo.MarkUsed(codeStr, tgID); err != nil {
		logger.Error().Err(err).Str("code", codeStr).Msg("标记注册码失败")
		return 0, fmt.Errorf("处理注册码失败: %w", err)
	}

	// 更新用户到期时间
	if err := s.embyRepo.UpdateFields(tgID, map[string]interface{}{"ex": newExpiry}); err != nil {
		logger.Error().Err(err).Int64("tg", tgID).Msg("更新到期时间失败")
		return 0, fmt.Errorf("更新到期时间失败: %w", err)
	}

	logger.Info().
		Int64("tg", tgID).
		Str("code", codeStr).
		Int("days", code.Us).
		Time("newExpiry", newExpiry).
		Msg("用户使用注册码续期成功")

	return code.Us, nil
}

// GetCodeStats 获取注册码统计
func (s *CodeService) GetCodeStats(tgID *int64) (*repository.CodeStats, error) {
	return s.codeRepo.CountStats(tgID)
}

// ValidateCode 验证注册码（不使用）
func (s *CodeService) ValidateCode(codeStr string) (days int, err error) {
	code, err := s.codeRepo.GetByCode(codeStr)
	if err != nil {
		return 0, ErrCodeNotFound
	}

	if code.IsUsed() {
		return 0, ErrCodeAlreadyUsed
	}

	return code.Us, nil
}

// DeleteUnusedCodes 删除未使用的注册码
func (s *CodeService) DeleteUnusedCodes(days []int, tgID *int64) (int64, error) {
	if len(days) == 0 {
		return s.codeRepo.DeleteAllUnused(tgID)
	}
	return s.codeRepo.DeleteUnusedByDays(days, tgID)
}

// UseCodeWithSecurity 使用注册码（带安全码）
func (s *CodeService) UseCodeWithSecurity(tgID int64, username string, codeStr string, securityCode string) (*UseCodeResult, error) {
	// 检查兑换功能是否开启
	if !s.cfg.Open.Exchange {
		return nil, ErrExchangeDisabled
	}

	// 检查用户是否已有账户
	user, err := s.embyRepo.GetByTG(tgID)
	if err == nil && user.HasEmbyAccount() {
		return nil, ErrAlreadyHasAccount
	}

	// 查找注册码
	code, err := s.codeRepo.GetByCode(codeStr)
	if err != nil {
		return nil, ErrCodeNotFound
	}

	// 检查注册码是否已使用
	if code.IsUsed() {
		return nil, ErrCodeAlreadyUsed
	}

	// 创建 Emby 账户
	embyClient := emby.GetClient()
	createResult, err := embyClient.CreateUser(username, code.Us)
	if err != nil {
		logger.Error().Err(err).Int64("tg", tgID).Str("code", codeStr).Msg("使用注册码创建账户失败")
		return nil, fmt.Errorf("创建账户失败: %w", err)
	}

	// 标记注册码已使用
	if err := s.codeRepo.MarkUsed(codeStr, tgID); err != nil {
		logger.Error().Err(err).Str("code", codeStr).Msg("标记注册码失败")
	}

	// 更新用户数据库记录（包含安全码）
	updates := map[string]interface{}{
		"embyid": createResult.UserID,
		"name":   username,
		"pwd":    createResult.Password,
		"pwd2":   securityCode, // 安全码
		"ex":     createResult.ExpiryDate,
		"cr":     time.Now(),
		"lv":     "b", // 普通用户
	}

	// 确保用户存在
	s.embyRepo.EnsureExists(tgID)
	if err := s.embyRepo.UpdateFields(tgID, updates); err != nil {
		logger.Error().Err(err).Int64("tg", tgID).Msg("更新用户记录失败")
	}

	logger.Info().
		Int64("tg", tgID).
		Str("code", codeStr).
		Str("embyID", createResult.UserID).
		Str("username", username).
		Msg("用户使用注册码成功（带安全码）")

	return &UseCodeResult{
		Success:    true,
		UserID:     createResult.UserID,
		Username:   username,
		Password:   createResult.Password,
		ExpiryDate: createResult.ExpiryDate,
		Days:       code.Us,
	}, nil
}
