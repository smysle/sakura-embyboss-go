// Package imggen 图片生成模块
package imggen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"

	"github.com/fogleman/gg"
)

// RankData 排行榜数据
type RankData struct {
	Rank      int
	Username  string
	PlayCount int
	WatchTime string // 格式化后的观看时长
}

// LeaderboardConfig 排行榜图片配置
type LeaderboardConfig struct {
	Title       string
	Subtitle    string
	RankType    string // "day" 或 "week"
	Items       []RankData
	GeneratedAt time.Time
}

// 颜色定义
var (
	bgColor       = color.RGBA{25, 25, 35, 255}      // 深色背景
	cardColor     = color.RGBA{35, 35, 50, 255}      // 卡片背景
	goldColor     = color.RGBA{255, 215, 0, 255}     // 金色
	silverColor   = color.RGBA{192, 192, 192, 255}   // 银色
	bronzeColor   = color.RGBA{205, 127, 50, 255}    // 铜色
	textColor     = color.RGBA{255, 255, 255, 255}   // 白色文字
	subTextColor  = color.RGBA{180, 180, 180, 255}   // 灰色文字
	accentColor   = color.RGBA{138, 43, 226, 255}    // 紫色强调
	dayBgColor    = color.RGBA{30, 60, 114, 255}     // 日榜渐变起始
	weekBgColor   = color.RGBA{114, 30, 60, 255}     // 周榜渐变起始
)

// GenerateLeaderboard 生成排行榜图片
func GenerateLeaderboard(cfg LeaderboardConfig) ([]byte, error) {
	// 计算图片尺寸
	width := 600
	headerHeight := 120
	itemHeight := 70
	footerHeight := 50
	padding := 20
	
	itemCount := len(cfg.Items)
	if itemCount > 10 {
		itemCount = 10
	}
	
	height := headerHeight + itemCount*itemHeight + footerHeight + padding*2

	// 创建画布
	dc := gg.NewContext(width, height)

	// 绘制背景渐变
	drawBackground(dc, width, height, cfg.RankType)

	// 绘制标题区域
	drawHeader(dc, width, cfg)

	// 绘制排行榜条目
	startY := float64(headerHeight + padding)
	for i, item := range cfg.Items {
		if i >= 10 {
			break
		}
		drawRankItem(dc, width, startY+float64(i*itemHeight), item)
	}

	// 绘制底部信息
	drawFooter(dc, width, height, cfg.GeneratedAt)

	// 导出为 PNG
	return exportPNG(dc)
}

// drawBackground 绘制背景
func drawBackground(dc *gg.Context, width, height int, rankType string) {
	// 创建渐变背景
	var startColor, endColor color.RGBA
	if rankType == "week" {
		startColor = weekBgColor
		endColor = bgColor
	} else {
		startColor = dayBgColor
		endColor = bgColor
	}

	for y := 0; y < height; y++ {
		t := float64(y) / float64(height)
		r := uint8(float64(startColor.R)*(1-t) + float64(endColor.R)*t)
		g := uint8(float64(startColor.G)*(1-t) + float64(endColor.G)*t)
		b := uint8(float64(startColor.B)*(1-t) + float64(endColor.B)*t)
		dc.SetColor(color.RGBA{r, g, b, 255})
		dc.DrawRectangle(0, float64(y), float64(width), 1)
		dc.Fill()
	}
}

// drawHeader 绘制标题
func drawHeader(dc *gg.Context, width int, cfg LeaderboardConfig) {
	// 标题图标
	iconText := "📊"
	if cfg.RankType == "week" {
		iconText = "📈"
	}

	// 绘制标题
	dc.SetColor(textColor)
	
	// 使用系统默认字体（简化版本，实际生产环境需要加载中文字体）
	titleFontSize := 28.0
	dc.SetColor(textColor)
	
	// 绘制标题文本
	title := fmt.Sprintf("%s %s", iconText, cfg.Title)
	dc.DrawStringAnchored(title, float64(width)/2, 45, 0.5, 0.5)

	// 绘制副标题
	dc.SetColor(subTextColor)
	dc.DrawStringAnchored(cfg.Subtitle, float64(width)/2, 80, 0.5, 0.5)

	// 绘制分隔线
	dc.SetColor(accentColor)
	dc.SetLineWidth(2)
	dc.DrawLine(50, 110, float64(width-50), 110)
	dc.Stroke()

	_ = titleFontSize
}

// drawRankItem 绘制排行榜条目
func drawRankItem(dc *gg.Context, width int, y float64, item RankData) {
	cardX := 20.0
	cardY := y
	cardW := float64(width - 40)
	cardH := 60.0

	// 绘制卡片背景
	dc.SetColor(color.RGBA{cardColor.R, cardColor.G, cardColor.B, 200})
	drawRoundedRect(dc, cardX, cardY, cardW, cardH, 10)
	dc.Fill()

	// 绘制排名
	rankX := cardX + 35
	rankY := cardY + cardH/2

	// 根据排名设置颜色
	var rankColor color.RGBA
	rankEmoji := ""
	switch item.Rank {
	case 1:
		rankColor = goldColor
		rankEmoji = "🥇"
	case 2:
		rankColor = silverColor
		rankEmoji = "🥈"
	case 3:
		rankColor = bronzeColor
		rankEmoji = "🥉"
	default:
		rankColor = subTextColor
		rankEmoji = fmt.Sprintf("%d", item.Rank)
	}

	dc.SetColor(rankColor)
	dc.DrawStringAnchored(rankEmoji, rankX, rankY, 0.5, 0.5)

	// 绘制用户名
	dc.SetColor(textColor)
	dc.DrawStringAnchored(item.Username, cardX+100, rankY-10, 0, 0.5)

	// 绘制播放次数和时长
	dc.SetColor(subTextColor)
	statsText := fmt.Sprintf("播放 %d 次 | %s", item.PlayCount, item.WatchTime)
	dc.DrawStringAnchored(statsText, cardX+100, rankY+12, 0, 0.5)

	// 绘制右侧装饰
	dc.SetColor(accentColor)
	dc.DrawCircle(cardX+cardW-30, rankY, 5)
	dc.Fill()
}

// drawFooter 绘制底部
func drawFooter(dc *gg.Context, width, height int, generatedAt time.Time) {
	dc.SetColor(subTextColor)
	footerText := fmt.Sprintf("生成于 %s | Sakura EmbyBoss", generatedAt.Format("2006-01-02 15:04"))
	dc.DrawStringAnchored(footerText, float64(width)/2, float64(height-25), 0.5, 0.5)
}

// drawRoundedRect 绘制圆角矩形
func drawRoundedRect(dc *gg.Context, x, y, w, h, r float64) {
	dc.DrawRoundedRectangle(x, y, w, h, r)
}

// exportPNG 导出为 PNG
func exportPNG(dc *gg.Context) ([]byte, error) {
	img := dc.Image()
	
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("编码 PNG 失败: %w", err)
	}
	
	return buf.Bytes(), nil
}

// GenerateSimpleLeaderboard 生成简化版排行榜（纯文本风格图片）
func GenerateSimpleLeaderboard(cfg LeaderboardConfig) ([]byte, error) {
	width := 500
	height := 400
	
	dc := gg.NewContext(width, height)
	
	// 纯色背景
	dc.SetColor(bgColor)
	dc.Clear()
	
	// 绘制标题
	dc.SetColor(goldColor)
	dc.DrawStringAnchored(cfg.Title, float64(width)/2, 30, 0.5, 0.5)
	
	// 绘制条目
	startY := 80.0
	lineHeight := 30.0
	
	for i, item := range cfg.Items {
		if i >= 10 {
			break
		}
		
		y := startY + float64(i)*lineHeight
		
		// 排名颜色
		switch item.Rank {
		case 1:
			dc.SetColor(goldColor)
		case 2:
			dc.SetColor(silverColor)
		case 3:
			dc.SetColor(bronzeColor)
		default:
			dc.SetColor(textColor)
		}
		
		line := fmt.Sprintf("%d. %s - %d次 %s", 
			item.Rank, item.Username, item.PlayCount, item.WatchTime)
		dc.DrawString(line, 40, y)
	}
	
	// 底部时间
	dc.SetColor(subTextColor)
	dc.DrawStringAnchored(
		cfg.GeneratedAt.Format("2006-01-02 15:04:05"),
		float64(width)/2, float64(height-20), 0.5, 0.5,
	)
	
	return exportPNG(dc)
}

// CreateTestImage 创建测试图片（验证图片生成功能）
func CreateTestImage() (image.Image, error) {
	dc := gg.NewContext(200, 100)
	dc.SetColor(color.RGBA{100, 150, 200, 255})
	dc.Clear()
	dc.SetColor(color.White)
	dc.DrawStringAnchored("Test Image", 100, 50, 0.5, 0.5)
	return dc.Image(), nil
}
