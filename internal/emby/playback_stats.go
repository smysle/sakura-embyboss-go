// Package emby Emby 播放统计 API
// 此模块对接 Emby 的 playback_reporting 或 user_usage_stats 插件
package emby

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// PlaybackStats 播放统计
type PlaybackStats struct {
	UserID      string
	UserName    string
	PlayCount   int
	TotalTime   int64 // 秒
	LastPlayed  *time.Time
	TopItems    []PlayedItem
}

// PlayedItem 播放的项目
type PlayedItem struct {
	ItemID   string
	ItemName string
	Type     string // Movie, Episode, etc.
	Count    int
	Duration int64 // 秒
}

// UserUsageStats 用户使用统计（来自 user_usage_stats 插件）
type UserUsageStats struct {
	Date           string  `json:"date"`
	UserID         string  `json:"user_id"`
	UserName       string  `json:"user_name"`
	TotalPlayCount int     `json:"total_play_count"`
	TotalPlayTime  float64 `json:"total_play_time"` // 分钟
}

// GetUserPlaybackStats 获取用户播放统计（使用 Emby 原生 API）
func (c *Client) GetUserPlaybackStats(userID string, days int) (*PlaybackStats, error) {
	// 获取用户播放的项目
	endpoint := fmt.Sprintf("/emby/Users/%s/Items?Recursive=true&Filters=IsPlayed&SortBy=DatePlayed&SortOrder=Descending&Limit=50&api_key=%s",
		userID, c.apiKey)

	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取播放记录失败: %v", err)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析数据")
	}

	items, ok := data["Items"].([]interface{})
	if !ok {
		return &PlaybackStats{UserID: userID}, nil
	}

	stats := &PlaybackStats{
		UserID:    userID,
		PlayCount: len(items),
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	playCount := 0

	for _, item := range items {
		if itemData, ok := item.(map[string]interface{}); ok {
			// 获取播放日期
			if dateStr := getString(itemData, "LastPlayedDate"); dateStr != "" {
				if playedDate, err := time.Parse(time.RFC3339, dateStr); err == nil {
					if playedDate.After(cutoff) {
						playCount++
						// 估算播放时长（使用 RunTimeTicks）
						if ticks, ok := itemData["RunTimeTicks"].(float64); ok {
							stats.TotalTime += int64(ticks / 10000000) // Ticks to seconds
						}
					}
				}
			}

			// 收集播放项目
			if len(stats.TopItems) < 5 {
				stats.TopItems = append(stats.TopItems, PlayedItem{
					ItemID:   getString(itemData, "Id"),
					ItemName: getString(itemData, "Name"),
					Type:     getString(itemData, "Type"),
				})
			}
		}
	}

	stats.PlayCount = playCount
	return stats, nil
}

// GetAllUsersPlaybackStats 获取所有用户的播放统计
func (c *Client) GetAllUsersPlaybackStats(startDate, endDate time.Time) ([]PlaybackStats, error) {
	logger.Debug().
		Time("start", startDate).
		Time("end", endDate).
		Msg("获取所有用户播放统计")

	// 首先尝试使用 user_usage_stats 插件 API
	stats, err := c.getUserUsageStatsFromPlugin(startDate, endDate)
	if err == nil && len(stats) > 0 {
		return stats, nil
	}

	logger.Debug().Msg("user_usage_stats 插件不可用，使用原生 API")

	// 回退到原生 API
	return c.getUserPlaybackStatsNative(startDate, endDate)
}

// getUserUsageStatsFromPlugin 从 user_usage_stats 插件获取统计
func (c *Client) getUserUsageStatsFromPlugin(startDate, endDate time.Time) ([]PlaybackStats, error) {
	// user_usage_stats 插件端点
	// GET /user_usage_stats/user_activity?days=7
	days := int(endDate.Sub(startDate).Hours() / 24)
	if days <= 0 {
		days = 1
	}

	endpoint := fmt.Sprintf("/user_usage_stats/user_activity?days=%d&api_key=%s", days, c.apiKey)
	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("插件 API 不可用: %v", err)
	}

	// 解析插件返回的数据
	data, ok := result.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析插件数据")
	}

	statsMap := make(map[string]*PlaybackStats)

	for _, item := range data {
		if row, ok := item.(map[string]interface{}); ok {
			userID := getString(row, "user_id")
			if userID == "" {
				continue
			}

			if _, exists := statsMap[userID]; !exists {
				statsMap[userID] = &PlaybackStats{
					UserID:   userID,
					UserName: getString(row, "user_name"),
				}
			}

			stats := statsMap[userID]
			stats.PlayCount += getInt(row, "play_count")
			// 插件返回的是分钟
			if minutes, ok := row["total_time"].(float64); ok {
				stats.TotalTime += int64(minutes * 60)
			}
		}
	}

	// 转换为切片并排序
	var statsList []PlaybackStats
	for _, s := range statsMap {
		statsList = append(statsList, *s)
	}

	sort.Slice(statsList, func(i, j int) bool {
		return statsList[i].TotalTime > statsList[j].TotalTime
	})

	return statsList, nil
}

// getUserPlaybackStatsNative 使用原生 API 获取播放统计
func (c *Client) getUserPlaybackStatsNative(startDate, endDate time.Time) ([]PlaybackStats, error) {
	// 获取所有用户
	users, err := c.GetUsers()
	if err != nil {
		return nil, err
	}

	days := int(endDate.Sub(startDate).Hours() / 24)
	if days <= 0 {
		days = 1
	}

	var allStats []PlaybackStats

	for _, user := range users {
		if user.Policy != nil && user.Policy.IsDisabled {
			continue // 跳过禁用用户
		}

		stats, err := c.GetUserPlaybackStats(user.ID, days)
		if err != nil {
			logger.Debug().Err(err).Str("user", user.Name).Msg("获取用户统计失败")
			continue
		}

		stats.UserName = user.Name
		if stats.PlayCount > 0 {
			allStats = append(allStats, *stats)
		}
	}

	// 按观看时长排序
	sort.Slice(allStats, func(i, j int) bool {
		return allStats[i].TotalTime > allStats[j].TotalTime
	})

	return allStats, nil
}

// GetPlaybackReport 获取播放报告（从 playback_reporting 插件）
func (c *Client) GetPlaybackReport(startDate, endDate time.Time) ([]PlaybackReportItem, error) {
	// playback_reporting 插件端点
	start := startDate.Format("2006-01-02")
	end := endDate.Format("2006-01-02")

	endpoint := fmt.Sprintf("/playback_reporting/session_list?StartDate=%s&EndDate=%s&api_key=%s",
		start, end, c.apiKey)

	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取播放报告失败: %v", err)
	}

	data, ok := result.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析报告数据")
	}

	var items []PlaybackReportItem
	for _, item := range data {
		if row, ok := item.(map[string]interface{}); ok {
			items = append(items, PlaybackReportItem{
				UserID:       getString(row, "UserId"),
				UserName:     getString(row, "UserName"),
				ItemName:     getString(row, "ItemName"),
				ItemType:     getString(row, "ItemType"),
				PlayDuration: getInt(row, "PlayDuration"),
				PlayCount:    getInt(row, "PlayCount"),
			})
		}
	}

	return items, nil
}

// PlaybackReportItem 播放报告项
type PlaybackReportItem struct {
	UserID       string
	UserName     string
	ItemName     string
	ItemType     string
	PlayDuration int // 秒
	PlayCount    int
}

// GetUserRanking 获取用户播放排行
func (c *Client) GetUserRanking(startDate, endDate time.Time, limit int) ([]RankingItem, error) {
	stats, err := c.GetAllUsersPlaybackStats(startDate, endDate)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}

	var ranking []RankingItem
	for i, s := range stats {
		if i >= limit {
			break
		}
		ranking = append(ranking, RankingItem{
			Rank:      i + 1,
			UserID:    s.UserID,
			UserName:  s.UserName,
			PlayCount: s.PlayCount,
			WatchTime: s.TotalTime,
		})
	}

	return ranking, nil
}

// RankingItem 排行项
type RankingItem struct {
	Rank      int
	UserID    string
	UserName  string
	PlayCount int
	WatchTime int64 // 秒
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

// AuditResult 审计结果
type AuditResult struct {
	UserID        string
	Username      string
	DeviceName    string
	ClientName    string
	RemoteAddress string
	LastActivity  time.Time
	ActivityCount int
}

// GetUsersByIP 根据 IP 地址查询使用该 IP 的用户信息
func (c *Client) GetUsersByIP(ipAddress string, days int) ([]AuditResult, error) {
	return c.executeAuditQuery("RemoteAddress", ipAddress, days, true)
}

// GetUsersByDeviceName 根据设备名关键词查询用户
func (c *Client) GetUsersByDeviceName(deviceKeyword string, days int) ([]AuditResult, error) {
	return c.executeAuditQuery("DeviceName", deviceKeyword, days, false)
}

// GetUsersByClientName 根据客户端名关键词查询用户
func (c *Client) GetUsersByClientName(clientKeyword string, days int) ([]AuditResult, error) {
	return c.executeAuditQuery("ClientName", clientKeyword, days, false)
}

// GetUserActivityByName 根据用户名查询活动记录
func (c *Client) GetUserActivityByName(username string, days int) ([]AuditResult, error) {
	// 首先获取用户 ID
	user, err := c.GetUserByName(username)
	if err != nil {
		return nil, fmt.Errorf("用户不存在: %w", err)
	}

	return c.executeAuditQueryByUserID(user.ID, days)
}

// executeAuditQuery 执行审计查询
func (c *Client) executeAuditQuery(field, value string, days int, exactMatch bool) ([]AuditResult, error) {
	// 构建 SQL 查询
	var whereClause string
	if exactMatch {
		// 精确匹配（用于 IP 地址）
		whereClause = fmt.Sprintf("%s = '%s'", field, sanitizeSQL(value))
	} else {
		// 模糊匹配（用于设备名、客户端名）
		whereClause = fmt.Sprintf("%s LIKE '%%%s%%'", field, sanitizeSQL(value))
	}

	sql := fmt.Sprintf(`
		SELECT DISTINCT UserId, DeviceName, ClientName, RemoteAddress,
			   MAX(DateCreated) AS LastActivity, COUNT(*) AS ActivityCount
		FROM PlaybackActivity 
		WHERE %s`, whereClause)

	if days > 0 {
		sql += fmt.Sprintf(" AND DateCreated >= datetime('now', '-%d days')", days)
	}

	sql += " GROUP BY UserId, DeviceName, ClientName, RemoteAddress ORDER BY LastActivity DESC"

	return c.executeAuditSQL(sql)
}

// executeAuditQueryByUserID 根据用户 ID 查询活动
func (c *Client) executeAuditQueryByUserID(userID string, days int) ([]AuditResult, error) {
	sql := fmt.Sprintf(`
		SELECT UserId, DeviceName, ClientName, RemoteAddress,
			   MAX(DateCreated) AS LastActivity, COUNT(*) AS ActivityCount
		FROM PlaybackActivity 
		WHERE UserId = '%s'`, sanitizeSQL(userID))

	if days > 0 {
		sql += fmt.Sprintf(" AND DateCreated >= datetime('now', '-%d days')", days)
	}

	sql += " GROUP BY DeviceName, ClientName, RemoteAddress ORDER BY LastActivity DESC"

	return c.executeAuditSQL(sql)
}

// executeAuditSQL 执行审计 SQL 查询
func (c *Client) executeAuditSQL(sql string) ([]AuditResult, error) {
	// 通过 user_usage_stats 插件提交自定义 SQL 查询
	endpoint := fmt.Sprintf("/emby/user_usage_stats/submit_custom_query?api_key=%s", c.apiKey)
	payload := map[string]interface{}{
		"CustomQueryString": sql,
		"ReplaceUserId":     false,
	}

	result, err := c.request("POST", endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("查询失败")
	}

	// 解析返回数据
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析响应数据")
	}

	results, ok := data["results"].([]interface{})
	if !ok {
		// 尝试直接作为数组解析
		if arrData, ok := result.Data.([]interface{}); ok {
			results = arrData
		} else {
			return nil, fmt.Errorf("无法解析查询结果")
		}
	}

	var auditResults []AuditResult

	// 创建用户名缓存
	userCache := make(map[string]string)

	for _, row := range results {
		rowData, ok := row.([]interface{})
		if !ok || len(rowData) < 6 {
			continue
		}

		userID := fmt.Sprintf("%v", rowData[0])

		// 获取用户名（带缓存）
		username := userCache[userID]
		if username == "" {
			if user, err := c.GetUserByID(userID); err == nil {
				username = user.Name
			} else {
				username = "未知用户"
			}
			userCache[userID] = username
		}

		// 解析最后活动时间
		var lastActivity time.Time
		if timeStr, ok := rowData[4].(string); ok {
			lastActivity, _ = time.Parse("2006-01-02 15:04:05", timeStr)
		}

		// 解析活动次数
		activityCount := 0
		if count, ok := rowData[5].(float64); ok {
			activityCount = int(count)
		}

		auditResults = append(auditResults, AuditResult{
			UserID:        userID,
			Username:      username,
			DeviceName:    fmt.Sprintf("%v", rowData[1]),
			ClientName:    fmt.Sprintf("%v", rowData[2]),
			RemoteAddress: fmt.Sprintf("%v", rowData[3]),
			LastActivity:  lastActivity,
			ActivityCount: activityCount,
		})
	}

	return auditResults, nil
}

// sanitizeSQL 清理 SQL 输入，防止注入
func sanitizeSQL(input string) string {
	// 替换危险字符
	input = strings.ReplaceAll(input, "'", "''")
	input = strings.ReplaceAll(input, ";", "")
	input = strings.ReplaceAll(input, "--", "")
	input = strings.ReplaceAll(input, "/*", "")
	input = strings.ReplaceAll(input, "*/", "")
	return input
}
