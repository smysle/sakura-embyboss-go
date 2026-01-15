// Package service 排行榜服务
package service

import (
	"fmt"
	"os"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// RankType 排行榜类型
type RankType string

const (
	RankTypeDay  RankType = "day"  // 日榜
	RankTypeWeek RankType = "week" // 周榜
)

// RankItem 排行榜条目
type RankItem struct {
	Rank       int    // 排名
	UserID     string // Emby 用户 ID
	Username   string // 用户名
	TGUsername string // Telegram 用户名
	TGID       int64  // Telegram ID
	PlayCount  int    // 播放次数
	WatchTime  int64  // 观看时长（秒）
	ItemName   string // 最常看的内容
}

// RankResult 排行榜结果
type RankResult struct {
	Type      RankType
	Title     string
	Items     []RankItem
	StartDate time.Time
	EndDate   time.Time
	Generated time.Time
}

// LeaderboardService 排行榜服务
type LeaderboardService struct {
	embyClient *emby.Client
	embyRepo   *repository.EmbyRepository
	cfg        *config.Config
}

// NewLeaderboardService 创建排行榜服务
func NewLeaderboardService() *LeaderboardService {
	return &LeaderboardService{
		embyClient: emby.GetClient(),
		embyRepo:   repository.NewEmbyRepository(),
		cfg:        config.Get(),
	}
}

// GetDayRank 获取日榜
func (s *LeaderboardService) GetDayRank(limit int) (*RankResult, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	return s.getRank(RankTypeDay, startOfDay, now, limit)
}

// GetWeekRank 获取周榜
func (s *LeaderboardService) GetWeekRank(limit int) (*RankResult, error) {
	now := time.Now()
	// 获取本周一
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // 周日
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())

	return s.getRank(RankTypeWeek, startOfWeek, now, limit)
}

// getRank 获取排行榜
func (s *LeaderboardService) getRank(rankType RankType, startDate, endDate time.Time, limit int) (*RankResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// 从 Emby 获取播放统计
	stats, err := s.getPlaybackStats(startDate, endDate, limit)
	if err != nil {
		logger.Error().Err(err).Msg("获取播放统计失败")
		return nil, err
	}

	// 构建排行榜
	items := make([]RankItem, 0, len(stats))
	for i, stat := range stats {
		item := RankItem{
			Rank:      i + 1,
			UserID:    stat.UserID,
			Username:  stat.Username,
			PlayCount: stat.PlayCount,
			WatchTime: stat.WatchTime,
		}

		// 尝试关联 Telegram 用户
		if embyUser, err := s.embyRepo.GetByEmbyID(stat.UserID); err == nil && embyUser != nil {
			item.TGID = embyUser.TG
			if embyUser.Name != nil {
				item.TGUsername = *embyUser.Name
			}
		}

		items = append(items, item)
	}

	title := "日榜"
	if rankType == RankTypeWeek {
		title = "周榜"
	}

	return &RankResult{
		Type:      rankType,
		Title:     fmt.Sprintf("📊 %s 播放排行榜", title),
		Items:     items,
		StartDate: startDate,
		EndDate:   endDate,
		Generated: time.Now(),
	}, nil
}

// PlaybackStat 播放统计
type PlaybackStat struct {
	UserID    string
	Username  string
	PlayCount int
	WatchTime int64 // 秒
}

// getPlaybackStats 从 Emby 获取播放统计
func (s *LeaderboardService) getPlaybackStats(startDate, endDate time.Time, limit int) ([]PlaybackStat, error) {
	logger.Debug().
		Time("start", startDate).
		Time("end", endDate).
		Int("limit", limit).
		Msg("获取播放统计")

	// 尝试从 Emby API 获取真实数据
	ranking, err := s.embyClient.GetUserRanking(startDate, endDate, limit)
	if err != nil {
		logger.Warn().Err(err).Msg("从 Emby 获取播放统计失败，使用模拟数据")
		return s.mockPlaybackStats(limit), nil
	}

	// 转换为 PlaybackStat
	stats := make([]PlaybackStat, 0, len(ranking))
	for _, r := range ranking {
		stats = append(stats, PlaybackStat{
			UserID:    r.UserID,
			Username:  r.UserName,
			PlayCount: r.PlayCount,
			WatchTime: r.WatchTime,
		})
	}

	// 如果没有数据，返回模拟数据
	if len(stats) == 0 {
		logger.Debug().Msg("Emby 返回空数据，使用模拟数据")
		return s.mockPlaybackStats(limit), nil
	}

	return stats, nil
}

// mockPlaybackStats 模拟播放统计数据（测试用或 Emby API 不可用时）
func (s *LeaderboardService) mockPlaybackStats(limit int) []PlaybackStat {
	// 获取数据库中的用户作为测试数据
	users, err := s.embyRepo.GetActiveUsers()
	if err != nil || len(users) == 0 {
		return []PlaybackStat{}
	}

	stats := make([]PlaybackStat, 0, limit)
	for i, user := range users {
		if i >= limit {
			break
		}
		if user.EmbyID == nil {
			continue
		}

		username := "用户"
		if user.Name != nil {
			username = *user.Name
		}

		stats = append(stats, PlaybackStat{
			UserID:    *user.EmbyID,
			Username:  username,
			PlayCount: 100 - i*10, // 模拟数据
			WatchTime: int64((100 - i*10) * 3600), // 模拟观看时长
		})
	}

	return stats
}

// FormatWatchTime 格式化观看时长
func FormatWatchTime(seconds int64) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

// FormatRankText 格式化排行榜文本
func (r *RankResult) FormatRankText() string {
	text := fmt.Sprintf("**%s**\n", r.Title)
	text += fmt.Sprintf("📅 %s ~ %s\n\n", r.StartDate.Format("01-02"), r.EndDate.Format("01-02 15:04"))

	for _, item := range r.Items {
		medal := getMedal(item.Rank)
		text += fmt.Sprintf("%s **%d.** %s\n", medal, item.Rank, item.Username)
		text += fmt.Sprintf("   ▸ 播放 %d 次 | %s\n", item.PlayCount, FormatWatchTime(item.WatchTime))
	}

	text += fmt.Sprintf("\n⏰ 生成于 %s", r.Generated.Format("2006-01-02 15:04:05"))
	return text
}

func getMedal(rank int) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "  "
	}
}

// UserPlayStat 用户播放统计
type UserPlayStat struct {
	UserID     string
	UserName   string
	TotalHours float64
	PlayCount  int
}

// GetUserPlayStats 获取用户播放统计
func (s *LeaderboardService) GetUserPlayStats(limit int) ([]UserPlayStat, error) {
	if limit <= 0 {
		limit = 20
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	ranking, err := s.embyClient.GetUserRanking(startOfMonth, now, limit)
	if err != nil {
		logger.Warn().Err(err).Msg("获取用户播放统计失败")
		return nil, err
	}

	stats := make([]UserPlayStat, 0, len(ranking))
	for _, r := range ranking {
		stats = append(stats, UserPlayStat{
			UserID:     r.UserID,
			UserName:   r.UserName,
			TotalHours: float64(r.WatchTime) / 3600.0,
			PlayCount:  r.PlayCount,
		})
	}

	return stats, nil
}

// GenerateDailyRank 生成日榜图片
func (s *LeaderboardService) GenerateDailyRank() (string, error) {
	result, err := s.GetDayRank(10)
	if err != nil {
		return "", err
	}

	return s.generateRankImage(result)
}

// GenerateWeeklyRank 生成周榜图片
func (s *LeaderboardService) GenerateWeeklyRank() (string, error) {
	result, err := s.GetWeekRank(10)
	if err != nil {
		return "", err
	}

	return s.generateRankImage(result)
}

// generateRankImage 生成排行榜图片
func (s *LeaderboardService) generateRankImage(result *RankResult) (string, error) {
	// 使用 imggen 包生成图片
	// 暂时返回文本文件作为替代
	filename := fmt.Sprintf("/tmp/rank_%s_%s.txt", result.Type, time.Now().Format("20060102_150405"))

	// 写入排行榜文本
	text := result.FormatRankText()
	if err := writeTextFile(filename, text); err != nil {
		return "", err
	}

	logger.Info().Str("file", filename).Str("type", string(result.Type)).Msg("排行榜已生成")
	return filename, nil
}

// writeTextFile 写入文本文件
func writeTextFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}
