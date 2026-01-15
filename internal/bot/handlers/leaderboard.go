// Package handlers 排行榜命令处理器
package handlers

import (
	"bytes"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/imggen"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
	tele "gopkg.in/telebot.v3"
)

// LeaderboardHandler 排行榜处理器
type LeaderboardHandler struct {
	service *service.LeaderboardService
}

// NewLeaderboardHandler 创建排行榜处理器
func NewLeaderboardHandler() *LeaderboardHandler {
	return &LeaderboardHandler{
		service: service.NewLeaderboardService(),
	}
}

// HandleDayRank 处理日榜命令 /dayrank
func (h *LeaderboardHandler) HandleDayRank(c tele.Context) error {
	return h.sendRankImage(c, service.RankTypeDay)
}

// HandleWeekRank 处理周榜命令 /weekrank
func (h *LeaderboardHandler) HandleWeekRank(c tele.Context) error {
	return h.sendRankImage(c, service.RankTypeWeek)
}

// HandleRank 处理通用排行命令 /rank [day|week]
func (h *LeaderboardHandler) HandleRank(c tele.Context) error {
	args := c.Args()
	rankType := service.RankTypeDay // 默认日榜

	if len(args) > 0 {
		switch args[0] {
		case "week", "w", "周":
			rankType = service.RankTypeWeek
		case "day", "d", "日":
			rankType = service.RankTypeDay
		}
	}

	return h.sendRankImage(c, rankType)
}

// sendRankImage 发送排行榜图片
func (h *LeaderboardHandler) sendRankImage(c tele.Context, rankType service.RankType) error {
	// 发送"正在生成"提示
	msg, err := c.Bot().Send(c.Chat(), "📊 正在生成排行榜，请稍候...")
	if err != nil {
		logger.Error().Err(err).Msg("发送提示消息失败")
	}

	// 获取排行榜数据
	var result *service.RankResult
	if rankType == service.RankTypeWeek {
		result, err = h.service.GetWeekRank(10)
	} else {
		result, err = h.service.GetDayRank(10)
	}

	if err != nil {
		logger.Error().Err(err).Msg("获取排行榜数据失败")
		return c.Send("❌ 获取排行榜数据失败，请稍后重试")
	}

	// 检查是否有数据
	if len(result.Items) == 0 {
		if msg != nil {
			c.Bot().Delete(msg)
		}
		return c.Send("📊 暂无排行数据")
	}

	// 转换为图片生成格式
	imgConfig := convertToImgConfig(result)

	// 生成图片
	imgData, err := imggen.GenerateLeaderboard(imgConfig)
	if err != nil {
		logger.Error().Err(err).Msg("生成排行榜图片失败")
		// 降级为文本模式
		if msg != nil {
			c.Bot().Delete(msg)
		}
		return c.Send(result.FormatRankText(), tele.ModeMarkdown)
	}

	// 删除提示消息
	if msg != nil {
		c.Bot().Delete(msg)
	}

	// 发送图片
	photo := &tele.Photo{
		File:    tele.FromReader(bytes.NewReader(imgData)),
		Caption: getCaption(rankType),
	}

	return c.Send(photo)
}

// SendRankToChat 发送排行榜到指定群组（供定时任务调用）
func (h *LeaderboardHandler) SendRankToChat(bot *tele.Bot, chatID int64, rankType service.RankType) error {
	chat := &tele.Chat{ID: chatID}

	// 获取排行榜数据
	var result *service.RankResult
	var err error
	if rankType == service.RankTypeWeek {
		result, err = h.service.GetWeekRank(10)
	} else {
		result, err = h.service.GetDayRank(10)
	}

	if err != nil {
		logger.Error().Err(err).Msg("定时任务获取排行榜数据失败")
		return err
	}

	if len(result.Items) == 0 {
		logger.Info().Msg("排行榜无数据，跳过发送")
		return nil
	}

	// 转换为图片生成格式
	imgConfig := convertToImgConfig(result)

	// 生成图片
	imgData, err := imggen.GenerateLeaderboard(imgConfig)
	if err != nil {
		logger.Error().Err(err).Msg("生成排行榜图片失败，使用文本模式")
		_, err = bot.Send(chat, result.FormatRankText(), tele.ModeMarkdown)
		return err
	}

	// 发送图片
	photo := &tele.Photo{
		File:    tele.FromReader(bytes.NewReader(imgData)),
		Caption: getCaption(rankType),
	}

	_, err = bot.Send(chat, photo)
	return err
}

// convertToImgConfig 转换为图片生成配置
func convertToImgConfig(result *service.RankResult) imggen.LeaderboardConfig {
	items := make([]imggen.RankData, len(result.Items))
	for i, item := range result.Items {
		items[i] = imggen.RankData{
			Rank:      item.Rank,
			Username:  item.Username,
			PlayCount: item.PlayCount,
			WatchTime: service.FormatWatchTime(item.WatchTime),
		}
	}

	rankTypeStr := "day"
	if result.Type == service.RankTypeWeek {
		rankTypeStr = "week"
	}

	return imggen.LeaderboardConfig{
		Title:       result.Title,
		Subtitle:    result.StartDate.Format("01-02") + " ~ " + result.EndDate.Format("01-02"),
		RankType:    rankTypeStr,
		Items:       items,
		GeneratedAt: time.Now(),
	}
}

// getCaption 获取图片说明
func getCaption(rankType service.RankType) string {
	if rankType == service.RankTypeWeek {
		return "📈 本周播放排行榜"
	}
	return "📊 今日播放排行榜"
}

// RegisterLeaderboardHandlers 注册排行榜相关命令
func RegisterLeaderboardHandlers(bot *tele.Bot) {
	h := NewLeaderboardHandler()

	bot.Handle("/rank", h.HandleRank)
	bot.Handle("/dayrank", h.HandleDayRank)
	bot.Handle("/weekrank", h.HandleWeekRank)
	
	// 中文别名
	bot.Handle("/日榜", h.HandleDayRank)
	bot.Handle("/周榜", h.HandleWeekRank)
	bot.Handle("/排行", h.HandleRank)

	logger.Info().Msg("排行榜命令已注册: /rank, /dayrank, /weekrank")
}
