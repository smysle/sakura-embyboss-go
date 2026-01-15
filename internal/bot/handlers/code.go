// Package handlers 注册码相关处理器
package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/service"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// GenerateCode /code 生成注册码命令
// 用法: /code <天数> [数量]
func GenerateCode(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send(
			"📝 **生成注册码**\n\n"+
				"用法: `/code <天数> [数量]`\n\n"+
				"示例:\n"+
				"- `/code 30` - 生成 1 个 30 天注册码\n"+
				"- `/code 90 5` - 生成 5 个 90 天注册码\n"+
				"- `/code 365 10` - 生成 10 个年卡注册码",
			tele.ModeMarkdown,
		)
	}

	// 解析天数
	days, err := strconv.Atoi(args[0])
	if err != nil || days <= 0 {
		return c.Send("❌ 无效的天数")
	}

	// 解析数量
	count := 1
	if len(args) >= 2 {
		count, err = strconv.Atoi(args[1])
		if err != nil || count <= 0 || count > 100 {
			return c.Send("❌ 数量应在 1-100 之间")
		}
	}

	// 生成注册码
	codeSvc := service.NewCodeService()
	result, err := codeSvc.GenerateCodes(c.Sender().ID, days, count)
	if err != nil {
		logger.Error().Err(err).Msg("生成注册码失败")
		return c.Send("❌ 生成注册码失败: " + err.Error())
	}

	// 构建回复消息
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ **成功生成 %d 个注册码**\n", result.Count))
	sb.WriteString(fmt.Sprintf("📅 有效期: %d 天\n\n", result.Days))

	for i, code := range result.Codes {
		sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, code))
	}

	return c.Send(sb.String(), keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// CodeStats /codestat 注册码统计命令
func CodeStats(c tele.Context) error {
	codeSvc := service.NewCodeService()

	// 获取全局统计
	stats, err := codeSvc.GetCodeStats(nil)
	if err != nil {
		return c.Send("❌ 获取统计失败")
	}

	text := fmt.Sprintf(
		"📊 **注册码统计**\n\n"+
			"**已使用**: %d\n"+
			"**未使用**: %d\n\n"+
			"**按期限分类 (未使用)**\n"+
			"- 月卡 (30天): %d\n"+
			"- 季卡 (90天): %d\n"+
			"- 半年卡 (180天): %d\n"+
			"- 年卡 (365天): %d",
		stats.Used,
		stats.Unused,
		stats.Mon,
		stats.Sea,
		stats.Half,
		stats.Year,
	)

	return c.Send(text, keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// MyCodeStats /mycode 我的注册码统计
func MyCodeStats(c tele.Context) error {
	codeSvc := service.NewCodeService()
	tgID := c.Sender().ID

	// 获取用户的统计
	stats, err := codeSvc.GetCodeStats(&tgID)
	if err != nil {
		return c.Send("❌ 获取统计失败")
	}

	text := fmt.Sprintf(
		"📊 **我的注册码统计**\n\n"+
			"**已使用**: %d\n"+
			"**未使用**: %d\n\n"+
			"**按期限分类 (未使用)**\n"+
			"- 月卡 (30天): %d\n"+
			"- 季卡 (90天): %d\n"+
			"- 半年卡 (180天): %d\n"+
			"- 年卡 (365天): %d",
		stats.Used,
		stats.Unused,
		stats.Mon,
		stats.Sea,
		stats.Half,
		stats.Year,
	)

	return c.Send(text, keyboards.CloseKeyboard(), tele.ModeMarkdown)
}

// DeleteCodes /delcode 删除未使用的注册码
// 用法: /delcode [天数] 或 /delcode all
func DeleteCodes(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send(
			"📝 **删除注册码**\n\n"+
				"用法:\n"+
				"- `/delcode all` - 删除所有未使用的注册码\n"+
				"- `/delcode 30` - 删除所有 30 天的未使用注册码\n"+
				"- `/delcode 30,90` - 删除 30 天和 90 天的未使用注册码",
			tele.ModeMarkdown,
		)
	}

	codeSvc := service.NewCodeService()
	var deleted int64
	var err error

	if args[0] == "all" {
		deleted, err = codeSvc.DeleteUnusedCodes(nil, nil)
	} else {
		// 解析天数列表
		dayStrs := strings.Split(args[0], ",")
		var days []int
		for _, ds := range dayStrs {
			d, parseErr := strconv.Atoi(strings.TrimSpace(ds))
			if parseErr == nil && d > 0 {
				days = append(days, d)
			}
		}

		if len(days) == 0 {
			return c.Send("❌ 无效的天数参数")
		}

		deleted, err = codeSvc.DeleteUnusedCodes(days, nil)
	}

	if err != nil {
		return c.Send("❌ 删除失败: " + err.Error())
	}

	return c.Send(fmt.Sprintf("✅ 已删除 %d 个未使用的注册码", deleted))
}
