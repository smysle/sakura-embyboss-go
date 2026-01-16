// Package service 红包服务
package service

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

var (
	ErrRedEnvelopeDisabled  = errors.New("红包功能已关闭")
	ErrInsufficientBalance  = errors.New("积分不足")
	ErrInvalidAmount        = errors.New("无效的金额")
	ErrInvalidCount         = errors.New("无效的个数")
	ErrEnvelopeNotFound     = errors.New("红包不存在")
	ErrEnvelopeExpired      = errors.New("红包已过期")
	ErrEnvelopeFinished     = errors.New("红包已被抢完")
	ErrAlreadyReceived      = errors.New("您已领取过此红包")
	ErrNotTargetUser        = errors.New("这是专属红包，您不是目标用户")
	ErrCannotReceiveOwnRed  = errors.New("不能领取自己的红包")
)

// RedEnvelopeService 红包服务
type RedEnvelopeService struct {
	redRepo  *repository.RedEnvelopeRepository
	embyRepo *repository.EmbyRepository
	cfg      *config.Config
	mu       sync.Mutex // 防止并发抢红包问题
}

// NewRedEnvelopeService 创建红包服务
func NewRedEnvelopeService() *RedEnvelopeService {
	return &RedEnvelopeService{
		redRepo:  repository.NewRedEnvelopeRepository(),
		embyRepo: repository.NewEmbyRepository(),
		cfg:      config.Get(),
	}
}

// CreateEnvelopeRequest 创建红包请求
type CreateEnvelopeRequest struct {
	SenderTG    int64
	SenderName  string
	TotalAmount int
	TotalCount  int
	Message     string
	Type        string // random, equal, private
	IsPrivate   bool
	TargetTG    *int64
	TargetName  string // 专属红包接收者名称
	ChatID      int64
}

// CreateEnvelopeResult 创建红包结果
type CreateEnvelopeResult struct {
	UUID        string
	TotalAmount int
	TotalCount  int
	Message     string
}

// CreateEnvelope 创建红包
func (s *RedEnvelopeService) CreateEnvelope(req *CreateEnvelopeRequest) (*CreateEnvelopeResult, error) {
	// 检查红包功能是否开启
	if !s.cfg.RedEnvelope.Enabled {
		return nil, ErrRedEnvelopeDisabled
	}

	// 检查专属红包权限
	if req.IsPrivate && !s.cfg.RedEnvelope.AllowPrivate {
		return nil, errors.New("专属红包功能已关闭")
	}

	// 验证参数
	if req.TotalAmount <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.TotalCount <= 0 || req.TotalCount > 100 {
		return nil, ErrInvalidCount
	}
	if req.TotalAmount < req.TotalCount {
		return nil, errors.New("红包金额不能少于红包个数")
	}

	// 检查发送者积分
	sender, err := s.embyRepo.GetByTG(req.SenderTG)
	if err != nil {
		return nil, errors.New("请先 /start 初始化账户")
	}
	if sender.Us < req.TotalAmount {
		return nil, ErrInsufficientBalance
	}

	// 扣除发送者积分
	newScore := sender.Us - req.TotalAmount
	if err := s.embyRepo.UpdateFields(req.SenderTG, map[string]interface{}{"us": newScore}); err != nil {
		return nil, fmt.Errorf("扣除积分失败: %w", err)
	}

	// 创建红包
	envelopeUUID := uuid.New().String()
	message := req.Message
	if message == "" {
		message = "恭喜发财，大吉大利！"
	}

	envelope := &models.RedEnvelope{
		UUID:         envelopeUUID,
		SenderTG:     req.SenderTG,
		SenderName:   req.SenderName,
		TotalAmount:  req.TotalAmount,
		TotalCount:   req.TotalCount,
		RemainAmount: req.TotalAmount,
		RemainCount:  req.TotalCount,
		Message:      message,
		Type:         req.Type,
		IsPrivate:    req.IsPrivate,
		TargetTG:     req.TargetTG,
		ChatID:       req.ChatID,
		Status:       "active",
		CreatedAt:    time.Now(),
		ExpiredAt:    time.Now().Add(24 * time.Hour), // 24小时过期
	}

	if err := s.redRepo.Create(envelope); err != nil {
		// 回滚积分
		s.embyRepo.UpdateFields(req.SenderTG, map[string]interface{}{"us": sender.Us})
		return nil, fmt.Errorf("创建红包失败: %w", err)
	}

	logger.Info().
		Str("uuid", envelopeUUID).
		Int64("sender", req.SenderTG).
		Int("amount", req.TotalAmount).
		Int("count", req.TotalCount).
		Msg("红包创建成功")

	return &CreateEnvelopeResult{
		UUID:        envelopeUUID,
		TotalAmount: req.TotalAmount,
		TotalCount:  req.TotalCount,
		Message:     message,
	}, nil
}

// ReceiveEnvelopeResult 领取红包结果
type ReceiveEnvelopeResult struct {
	Amount       int
	TotalAmount  int
	TotalCount   int
	RemainCount  int
	SenderName   string
	Message      string
	IsLucky      bool  // 是否手气最佳（红包抢完时判断）
	IsFinished   bool  // 红包是否已抢完
}

// ReceiveEnvelope 领取红包
func (s *RedEnvelopeService) ReceiveEnvelope(uuid string, receiverTG int64, receiverName string) (*ReceiveEnvelopeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取红包信息
	envelope, err := s.redRepo.GetByUUID(uuid)
	if err != nil {
		return nil, ErrEnvelopeNotFound
	}

	// 检查红包状态
	if envelope.Status != "active" {
		if envelope.Status == "expired" {
			return nil, ErrEnvelopeExpired
		}
		return nil, ErrEnvelopeFinished
	}

	if envelope.IsExpired() {
		s.redRepo.SetExpired(uuid)
		return nil, ErrEnvelopeExpired
	}

	if envelope.IsFinished() {
		s.redRepo.SetFinished(uuid)
		return nil, ErrEnvelopeFinished
	}

	// 不能领取自己的红包
	// 注意：如果 SenderTG 为 0，说明数据有问题，也需要拒绝
	if envelope.SenderTG == 0 {
		logger.Error().
			Str("uuid", uuid).
			Msg("红包 SenderTG 为 0，数据异常")
		return nil, fmt.Errorf("红包数据异常")
	}
	if envelope.SenderTG == receiverTG {
		logger.Debug().
			Int64("sender_tg", envelope.SenderTG).
			Int64("receiver_tg", receiverTG).
			Str("uuid", uuid).
			Msg("阻止用户领取自己的红包")
		return nil, ErrCannotReceiveOwnRed
	}

	// 检查专属红包
	if envelope.IsPrivate && envelope.TargetTG != nil && *envelope.TargetTG != receiverTG {
		return nil, ErrNotTargetUser
	}

	// 检查是否已领取
	if s.redRepo.HasReceived(uuid, receiverTG) {
		return nil, ErrAlreadyReceived
	}

	// 计算领取金额
	amount := s.calculateAmount(envelope)

	// 更新红包剩余
	if err := s.redRepo.UpdateRemain(uuid, amount); err != nil {
		return nil, fmt.Errorf("领取失败: %w", err)
	}

	// 更新内存中的值（用于后续判断）
	envelope.RemainAmount -= amount
	envelope.RemainCount--

	// 创建领取记录
	record := &models.RedEnvelopeRecord{
		EnvelopeID:   envelope.ID,
		EnvelopeUUID: uuid,
		ReceiverTG:   receiverTG,
		ReceiverName: receiverName,
		Amount:       amount,
		IsLucky:      false,
		CreatedAt:    time.Now(),
	}
	s.redRepo.CreateRecord(record)

	// 给领取者加积分
	receiver, _ := s.embyRepo.GetByTG(receiverTG)
	if receiver != nil {
		s.embyRepo.UpdateFields(receiverTG, map[string]interface{}{"us": receiver.Us + amount})
	}

	// 检查红包是否已抢完
	isFinished := envelope.RemainCount <= 0
	if isFinished {
		s.redRepo.SetFinished(uuid)
		// 更新手气最佳
		s.updateLuckyRecord(uuid)
	}

	// 判断是否手气最佳
	isLucky := false
	if isFinished {
		luckyRecord, _ := s.redRepo.GetLuckyRecord(uuid)
		if luckyRecord != nil && luckyRecord.ReceiverTG == receiverTG {
			isLucky = true
		}
	}

	logger.Info().
		Str("uuid", uuid).
		Int64("receiver", receiverTG).
		Int("amount", amount).
		Msg("红包领取成功")

	return &ReceiveEnvelopeResult{
		Amount:      amount,
		TotalAmount: envelope.TotalAmount,
		TotalCount:  envelope.TotalCount,
		RemainCount: envelope.RemainCount,
		SenderName:  envelope.SenderName,
		Message:     envelope.Message,
		IsLucky:     isLucky,
		IsFinished:  isFinished,
	}, nil
}

// calculateAmount 计算领取金额
func (s *RedEnvelopeService) calculateAmount(envelope *models.RedEnvelope) int {
	if envelope.Type == "equal" {
		// 均分
		return envelope.RemainAmount / envelope.RemainCount
	}

	// 拼手气红包（二倍均值法）
	if envelope.RemainCount == 1 {
		return envelope.RemainAmount
	}

	// 最大可领取金额为剩余金额的 2 倍平均值
	maxAmount := (envelope.RemainAmount / envelope.RemainCount) * 2
	if maxAmount < 1 {
		maxAmount = 1
	}

	// 最小 1 积分
	amount := rand.Intn(maxAmount) + 1

	// 确保剩余人能至少领到 1 积分
	minRemain := envelope.RemainCount - 1
	if envelope.RemainAmount-amount < minRemain {
		amount = envelope.RemainAmount - minRemain
	}

	return amount
}

// updateLuckyRecord 更新手气最佳
func (s *RedEnvelopeService) updateLuckyRecord(uuid string) {
	record, err := s.redRepo.GetLuckyRecord(uuid)
	if err == nil && record != nil {
		record.IsLucky = true
		// 这里简化处理，实际应该更新数据库
	}
}

// GetEnvelopeInfo 获取红包信息
func (s *RedEnvelopeService) GetEnvelopeInfo(uuid string) (*models.RedEnvelope, []models.RedEnvelopeRecord, error) {
	envelope, err := s.redRepo.GetByUUID(uuid)
	if err != nil {
		return nil, nil, ErrEnvelopeNotFound
	}

	records, _ := s.redRepo.GetRecordsByEnvelope(uuid)
	return envelope, records, nil
}
