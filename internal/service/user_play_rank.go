// Package service 用户播放榜服务
package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
	"github.com/smysle/sakura-embyboss-go/pkg/utils"
)

// UserPlayRankService 用户播放榜服务
type UserPlayRankService struct {
	embyClient *emby.Client
	embyRepo   *repository.EmbyRepository
	cfg        *config.Config
	bot        *tele.Bot
}

// PlayRecord 播放记录
type PlayRecord struct {
	UserID    string
	UserName  string
	TelegramID int64
	Duration  int64 // 秒
	Level     string
	Points    int // 当前积分
}

// RankEntry 排行榜条目
type RankEntry struct {
	Rank       int
	Name       string
	TelegramID int64
	Duration   int64
	DurationStr string
	Medal      string
	Points     int // 获得的积分奖励
	NewTotal   int // 新的总积分
}

// UserPlayRankResult 用户播放榜结果
type UserPlayRankResult struct {
	Entries        []RankEntry
	TotalPages     int
	Days           int
	PointsAwarded  bool
	AwardedEntries []RankEntry
}

// NewUserPlayRankService 创建用户播放榜服务
func NewUserPlayRankService() *UserPlayRankService {
	return &UserPlayRankService{
		embyClient: emby.GetClient(),
		embyRepo:   repository.NewEmbyRepository(),
		cfg:        config.Get(),
	}
}

// SetBot 设置 Bot 实例
func (s *UserPlayRankService) SetBot(bot *tele.Bot) {
	s.bot = bot
}

// 排行榜积分奖励（前10名）
var rankPoints = []int{1000, 900, 800, 700, 600, 500, 400, 300, 200, 100}

// 排名奖牌
var rankMedals = []string{"🥇", "🥈", "🥉", "🏅"}

// GetUserPlayRank 获取用户播放排行榜
func (s *UserPlayRankService) GetUserPlayRank(days int) (*UserPlayRankResult, error) {
	// 计算日期范围
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	// 获取用户播放统计
	stats, err := s.embyClient.GetAllUsersPlaybackStats(startDate, endDate)
	if err != nil {
		logger.Error().Err(err).Int("days", days).Msg("获取用户播放统计失败")
		return nil, fmt.Errorf("获取用户播放统计失败: %v", err)
	}

	if len(stats) == 0 {
		return &UserPlayRankResult{
			Entries:    []RankEntry{},
			TotalPages: 0,
			Days:       days,
		}, nil
	}

	// 获取数据库中的用户信息
	allUsers, err := s.embyRepo.GetAll()
	if err != nil {
		logger.Warn().Err(err).Msg("获取用户列表失败")
	}

	// 构建用户名到用户信息的映射
	userMap := make(map[string]*struct {
		TG    int64
		Name  string
		Level string
		IV    int
	})
	for _, u := range allUsers {
		if u.Name != nil {
			userMap[*u.Name] = &struct {
				TG    int64
				Name  string
				Level string
				IV    int
			}{
				TG:    u.TG,
				Name:  *u.Name,
				Level: string(u.Lv),
				IV:    u.IV,
			}
		}
	}

	// 构建排行榜条目
	var entries []RankEntry
	for i, stat := range stats {
		rank := i + 1
		medal := s.getMedal(rank)
		
		entry := RankEntry{
			Rank:        rank,
			Name:        stat.UserName,
			Duration:    stat.TotalTime,
			DurationStr: utils.FormatDuration(stat.TotalTime),
			Medal:       medal,
		}

		// 匹配数据库用户
		if userInfo, ok := userMap[stat.UserName]; ok {
			entry.TelegramID = userInfo.TG
			
			// 计算积分奖励（前10名）
			if rank <= 10 {
				entry.Points = rankPoints[rank-1] + int(stat.TotalTime/60) // 排名奖励 + 观看分钟数
			} else {
				entry.Points = int(stat.TotalTime / 60) // 只有观看时长积分
			}
			entry.NewTotal = userInfo.IV + entry.Points
		} else {
			entry.Name = stat.UserName + " (未绑定)"
		}

		entries = append(entries, entry)
	}

	totalPages := int(math.Ceil(float64(len(entries)) / 10))

	return &UserPlayRankResult{
		Entries:    entries,
		TotalPages: totalPages,
		Days:       days,
	}, nil
}

// AwardPoints 发放积分奖励
func (s *UserPlayRankService) AwardPoints(entries []RankEntry) ([]RankEntry, error) {
	var awarded []RankEntry
	var updates []struct {
		TG int64
		IV int
	}

	for _, entry := range entries {
		if entry.TelegramID > 0 && entry.Points > 0 {
			updates = append(updates, struct {
				TG int64
				IV int
			}{
				TG: entry.TelegramID,
				IV: entry.NewTotal,
			})
			awarded = append(awarded, entry)
		}
	}

	if len(updates) == 0 {
		return awarded, nil
	}

	// 批量更新数据库
	for _, u := range updates {
		if err := s.embyRepo.UpdateFields(u.TG, map[string]interface{}{"iv": u.IV}); err != nil {
			logger.Error().Err(err).Int64("tg", u.TG).Msg("更新用户积分失败")
		}
	}

	logger.Info().Int("count", len(awarded)).Msg("成功发放播放榜积分奖励")
	return awarded, nil
}

// GenerateAndSendPlayRank 生成并发送播放榜
func (s *UserPlayRankService) GenerateAndSendPlayRank(days int, awardPoints bool) error {
	if s.bot == nil {
		return fmt.Errorf("bot 未设置")
	}

	// 获取群组 ID
	var chatID int64
	if len(s.cfg.Groups) > 0 {
		chatID = s.cfg.Groups[0]
	}
	if chatID == 0 {
		return fmt.Errorf("未配置群组")
	}

	// 获取排行榜
	result, err := s.GetUserPlayRank(days)
	if err != nil {
		return err
	}

	if len(result.Entries) == 0 {
		chat := &tele.Chat{ID: chatID}
		_, err := s.bot.Send(chat, fmt.Sprintf("🍥 获取过去 %d 天用户播放榜失败，暂无数据", days))
		return err
	}

	// 格式化排行榜文本
	title := fmt.Sprintf("**▎🏆%s %d 天观影榜**\n\n", s.cfg.Ranks.Logo, days)
	text := s.formatRankPage(result.Entries, 1, title)

	// 添加时间戳
	now := time.Now().Format("2006-01-02")
	text += fmt.Sprintf("\n#UPlaysRank %s", now)

	// 发送到群组
	chat := &tele.Chat{ID: chatID}
	_, err = s.bot.Send(chat, text, tele.ModeMarkdown)
	if err != nil {
		logger.Error().Err(err).Msg("发送播放榜失败")
		return err
	}

	// 发放积分奖励
	if awardPoints && s.cfg.Open.UserPlays {
		awarded, err := s.AwardPoints(result.Entries)
		if err != nil {
			logger.Error().Err(err).Msg("发放积分失败")
		} else if len(awarded) > 0 {
			// 发送积分奖励消息
			s.sendAwardMessage(chatID, awarded, days)
		}
	}

	return nil
}

// formatRankPage 格式化排行榜页面
func (s *UserPlayRankService) formatRankPage(entries []RankEntry, page int, title string) string {
	var sb strings.Builder
	sb.WriteString(title)

	start := (page - 1) * 10
	end := start + 10
	if end > len(entries) {
		end = len(entries)
	}

	for _, entry := range entries[start:end] {
		rankCN := s.numberToChinese(entry.Rank)
		
		var userLink string
		if entry.TelegramID > 0 {
			userLink = fmt.Sprintf("[%s](tg://user?id=%d)", entry.Name, entry.TelegramID)
		} else {
			userLink = entry.Name
		}

		sb.WriteString(fmt.Sprintf("%s**第%s名** | %s\n", entry.Medal, rankCN, userLink))
		sb.WriteString(fmt.Sprintf("  观影时长 | %s\n", entry.DurationStr))
	}

	return sb.String()
}

// sendAwardMessage 发送积分奖励消息
func (s *UserPlayRankService) sendAwardMessage(chatID int64, awarded []RankEntry, days int) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**自动将 %d 天观看时长转换为%s**\n\n", days, s.cfg.Money))

	for _, entry := range awarded {
		sb.WriteString(fmt.Sprintf("%s[%s](tg://user?id=%d) 获得了 %d %s奖励\n",
			entry.Medal, entry.Name, entry.TelegramID, entry.Points, s.cfg.Money))
	}

	sb.WriteString(fmt.Sprintf("\n⏱️ 当前时间 - %s", time.Now().Format("2006-01-02")))

	chat := &tele.Chat{ID: chatID}
	text := sb.String()

	// 分段发送（如果太长）
	if len(text) > 4000 {
		chunks := s.splitText(text, 4000)
		for _, chunk := range chunks {
			s.bot.Send(chat, chunk, tele.ModeMarkdown)
		}
	} else {
		s.bot.Send(chat, text, tele.ModeMarkdown)
	}
}

// getMedal 获取排名奖牌
func (s *UserPlayRankService) getMedal(rank int) string {
	if rank <= 3 {
		return rankMedals[rank-1]
	}
	return rankMedals[3]
}

// numberToChinese 数字转中文
func (s *UserPlayRankService) numberToChinese(n int) string {
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	units := []string{"", "十", "百"}

	if n < 10 {
		return digits[n]
	}
	if n < 20 {
		if n == 10 {
			return "十"
		}
		return "十" + digits[n-10]
	}
	if n < 100 {
		tens := n / 10
		ones := n % 10
		if ones == 0 {
			return digits[tens] + "十"
		}
		return digits[tens] + "十" + digits[ones]
	}
	return fmt.Sprintf("%d", n)
}

// splitText 分割文本
func (s *UserPlayRankService) splitText(text string, maxLen int) []string {
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		chunks = append(chunks, text[:maxLen])
		text = text[maxLen:]
	}
	return chunks
}
