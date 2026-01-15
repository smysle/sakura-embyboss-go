// Package handlers 审计命令处理器
package handlers

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/emby"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// AuditIP /auditip 根据 IP 地址审计用户活动
// 用法: /auditip <IP地址> [天数]
func AuditIP(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send(
			"🔍 **IP 审计**\n\n"+
				"用法: `/auditip <IP地址> [天数]`\n\n"+
				"示例:\n"+
				"- `/auditip 192.168.1.100` - 查询所有时间\n"+
				"- `/auditip 192.168.1.100 30` - 查询最近 30 天",
			tele.ModeMarkdown,
		)
	}

	ipAddress := args[0]

	// 验证 IP 地址格式
	if net.ParseIP(ipAddress) == nil {
		return c.Send("❌ 无效的 IP 地址格式，请输入有效的 IPv4 或 IPv6 地址")
	}

	// 解析天数
	days := 0
	if len(args) > 1 {
		var err error
		days, err = strconv.Atoi(args[1])
		if err != nil || days < 0 {
			return c.Send("❌ 无效的天数")
		}
	}

	c.Send("⏳ 正在查询...")

	client := emby.GetClient()
	results, err := client.GetUsersByIP(ipAddress, days)
	if err != nil {
		logger.Error().Err(err).Str("ip", ipAddress).Msg("IP 审计查询失败")
		return c.Send("❌ 查询失败: " + err.Error())
	}

	if len(results) == 0 {
		return c.Send(fmt.Sprintf("📋 未找到使用 IP `%s` 的用户记录", ipAddress), tele.ModeMarkdown)
	}

	// 构建报告
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 **IP 审计报告**\n\n**IP**: `%s`\n", ipAddress))
	if days > 0 {
		sb.WriteString(fmt.Sprintf("**时间范围**: 最近 %d 天\n", days))
	}
	sb.WriteString(fmt.Sprintf("**匹配用户**: %d 人\n\n", len(results)))

	for i, r := range results {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... 还有 %d 条记录", len(results)-20))
			break
		}
		sb.WriteString(fmt.Sprintf(
			"%d. **%s**\n"+
				"   设备: %s | 客户端: %s\n"+
				"   活动次数: %d | 最后活动: %s\n\n",
			i+1, r.Username,
			r.DeviceName, r.ClientName,
			r.ActivityCount, r.LastActivity.Format("2006-01-02 15:04"),
		))
	}

	return c.Send(sb.String(), keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// AuditDevice /auditdevice 根据设备名审计用户
// 用法: /auditdevice <设备名关键词> [天数]
func AuditDevice(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send(
			"🔍 **设备审计**\n\n"+
				"用法: `/auditdevice <设备名关键词> [天数]`\n\n"+
				"示例:\n"+
				"- `/auditdevice Chrome` - 查询 Chrome 设备\n"+
				"- `/auditdevice iPhone 7` - 查询最近 7 天的 iPhone",
			tele.ModeMarkdown,
		)
	}

	deviceKeyword := args[0]

	// 解析天数
	days := 0
	if len(args) > 1 {
		var err error
		days, err = strconv.Atoi(args[len(args)-1])
		if err != nil {
			// 如果最后一个参数不是数字，把它也当作关键词的一部分
			deviceKeyword = strings.Join(args, " ")
			days = 0
		} else {
			// 排除最后的天数参数
			deviceKeyword = strings.Join(args[:len(args)-1], " ")
		}
	}

	c.Send("⏳ 正在查询...")

	client := emby.GetClient()
	results, err := client.GetUsersByDeviceName(deviceKeyword, days)
	if err != nil {
		logger.Error().Err(err).Str("device", deviceKeyword).Msg("设备审计查询失败")
		return c.Send("❌ 查询失败: " + err.Error())
	}

	if len(results) == 0 {
		return c.Send(fmt.Sprintf("📋 未找到使用设备 `%s` 的用户记录", deviceKeyword), tele.ModeMarkdown)
	}

	// 构建报告
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 **设备审计报告**\n\n**设备关键词**: `%s`\n", deviceKeyword))
	if days > 0 {
		sb.WriteString(fmt.Sprintf("**时间范围**: 最近 %d 天\n", days))
	}
	sb.WriteString(fmt.Sprintf("**匹配用户**: %d 人\n\n", len(results)))

	for i, r := range results {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... 还有 %d 条记录", len(results)-20))
			break
		}
		sb.WriteString(fmt.Sprintf(
			"%d. **%s**\n"+
				"   设备: %s | 客户端: %s\n"+
				"   IP: %s | 活动次数: %d\n\n",
			i+1, r.Username,
			r.DeviceName, r.ClientName,
			r.RemoteAddress, r.ActivityCount,
		))
	}

	return c.Send(sb.String(), keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// AuditClient /auditclient 根据客户端名审计用户
// 用法: /auditclient <客户端名关键词> [天数]
func AuditClient(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send(
			"🔍 **客户端审计**\n\n"+
				"用法: `/auditclient <客户端名关键词> [天数]`\n\n"+
				"示例:\n"+
				"- `/auditclient Emby` - 查询 Emby 客户端\n"+
				"- `/auditclient Infuse 30` - 查询最近 30 天的 Infuse",
			tele.ModeMarkdown,
		)
	}

	clientKeyword := args[0]

	// 解析天数
	days := 0
	if len(args) > 1 {
		var err error
		days, err = strconv.Atoi(args[len(args)-1])
		if err != nil {
			clientKeyword = strings.Join(args, " ")
			days = 0
		} else {
			clientKeyword = strings.Join(args[:len(args)-1], " ")
		}
	}

	c.Send("⏳ 正在查询...")

	client := emby.GetClient()
	results, err := client.GetUsersByClientName(clientKeyword, days)
	if err != nil {
		logger.Error().Err(err).Str("client", clientKeyword).Msg("客户端审计查询失败")
		return c.Send("❌ 查询失败: " + err.Error())
	}

	if len(results) == 0 {
		return c.Send(fmt.Sprintf("📋 未找到使用客户端 `%s` 的用户记录", clientKeyword), tele.ModeMarkdown)
	}

	// 构建报告
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 **客户端审计报告**\n\n**客户端关键词**: `%s`\n", clientKeyword))
	if days > 0 {
		sb.WriteString(fmt.Sprintf("**时间范围**: 最近 %d 天\n", days))
	}
	sb.WriteString(fmt.Sprintf("**匹配用户**: %d 人\n\n", len(results)))

	for i, r := range results {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... 还有 %d 条记录", len(results)-20))
			break
		}
		sb.WriteString(fmt.Sprintf(
			"%d. **%s**\n"+
				"   设备: %s | 客户端: %s\n"+
				"   IP: %s | 活动次数: %d\n\n",
			i+1, r.Username,
			r.DeviceName, r.ClientName,
			r.RemoteAddress, r.ActivityCount,
		))
	}

	return c.Send(sb.String(), keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// UserIP 查询指定用户的 IP 信息
// 通过 /start userip-<username> 触发
func UserIP(c tele.Context, username string) error {
	c.Send("⏳ 正在查询用户 IP 信息...")

	client := emby.GetClient()
	results, err := client.GetUserActivityByName(username, 30)
	if err != nil {
		logger.Error().Err(err).Str("username", username).Msg("用户 IP 查询失败")
		return c.Send("❌ 查询失败: " + err.Error())
	}

	if len(results) == 0 {
		return c.Send(fmt.Sprintf("📋 未找到用户 `%s` 的活动记录", username), tele.ModeMarkdown)
	}

	// 统计 IP 使用情况
	ipStats := make(map[string]struct {
		Count      int
		LastActive time.Time
		Devices    map[string]bool
	})

	for _, r := range results {
		if stat, exists := ipStats[r.RemoteAddress]; exists {
			stat.Count++
			if r.LastActivity.After(stat.LastActive) {
				stat.LastActive = r.LastActivity
			}
			stat.Devices[r.DeviceName] = true
			ipStats[r.RemoteAddress] = stat
		} else {
			ipStats[r.RemoteAddress] = struct {
				Count      int
				LastActive time.Time
				Devices    map[string]bool
			}{
				Count:      1,
				LastActive: r.LastActivity,
				Devices:    map[string]bool{r.DeviceName: true},
			}
		}
	}

	// 构建报告
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("👤 **用户 IP 信息**\n\n**用户名**: `%s`\n", username))
	sb.WriteString(fmt.Sprintf("**使用 IP 数**: %d 个\n\n", len(ipStats)))

	i := 0
	for ip, stat := range ipStats {
		i++
		if i > 10 {
			sb.WriteString(fmt.Sprintf("\n... 还有 %d 个 IP", len(ipStats)-10))
			break
		}

		devices := make([]string, 0, len(stat.Devices))
		for d := range stat.Devices {
			devices = append(devices, d)
		}

		sb.WriteString(fmt.Sprintf(
			"**%s**\n"+
				"  活动次数: %d | 最后活动: %s\n"+
				"  设备: %s\n\n",
			ip,
			stat.Count, stat.LastActive.Format("01-02 15:04"),
			strings.Join(devices, ", "),
		))
	}

	return c.Send(sb.String(), keyboards.CloseKeyboard(), tele.ModeMarkdown)
}
