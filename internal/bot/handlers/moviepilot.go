// Package handlers MoviePilot 点播命令处理器
package handlers

import (
	"fmt"
	"math"
	"strconv"
	"sync"

	tele "gopkg.in/telebot.v3"

	"github.com/smysle/sakura-embyboss-go/internal/bot/keyboards"
	"github.com/smysle/sakura-embyboss-go/internal/bot/session"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/internal/database/repository"
	"github.com/smysle/sakura-embyboss-go/internal/moviepilot"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

const mpItemsPerPage = 10

// 用户搜索数据缓存
var (
	userSearchData = make(map[int64]*MPSearchSession)
	searchDataLock sync.RWMutex
)

// MPSearchSession 搜索会话
type MPSearchSession struct {
	Keyword     string
	Results     []moviepilot.SearchResult
	CurrentPage int
	TotalPages  int
}

// HandleDownloadCenter 处理点播中心回调
func HandleDownloadCenter(c tele.Context) error {
	cfg := config.Get()
	if !cfg.MoviePilot.Enabled {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 管理员未开启点播功能",
			ShowAlert: true,
		})
	}

	c.Respond(&tele.CallbackResponse{Text: "🔍 点播中心"})
	return c.Edit("🔍 欢迎进入点播中心\n\n请选择操作：", keyboards.DownloadCenterKeyboard())
}

// HandleSearchResource 处理搜索资源
func HandleSearchResource(c tele.Context) error {
	cfg := config.Get()
	if !cfg.MoviePilot.Enabled {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 管理员未开启点播功能",
			ShowAlert: true,
		})
	}

	// 检查用户权限
	embyUser, err := repository.NewEmbyRepository().GetByTG(c.Sender().ID)
	if err != nil {
		return c.Edit("⚠️ 数据库没有您的记录，请先 /start 录入")
	}

	if embyUser.Lv != models.LevelA && embyUser.Lv != models.LevelB {
		return c.Edit("🫡 您没有权限使用此功能")
	}

	// 检查白名单限制
	if cfg.MoviePilot.Level == "a" && embyUser.Lv != models.LevelA {
		return c.Edit("🫡 此功能仅限白名单用户使用")
	}

	c.Respond(&tele.CallbackResponse{Text: "🔍 请输入资源名称"})

	// 设置等待输入状态
	session.GetManager().SetState(c.Sender().ID, session.StateMoviePilotSearch)

	money := cfg.Money
	if money == "" {
		money = "花币"
	}

	return c.Edit(fmt.Sprintf(
		"🎬 **点播中心**\n\n"+
			"当前点播费用: 1GB 消耗 %d %s\n"+
			"您当前拥有: %d %s\n\n"+
			"请在 120s 内发送您想点播的资源名称\n"+
			"退出请点 /cancel",
		cfg.MoviePilot.Price, money,
		embyUser.Iv, money,
	), tele.ModeMarkdown)
}

// HandleMoviePilotSearchInput 处理 MoviePilot 搜索输入
func HandleMoviePilotSearchInput(c tele.Context) error {
	keyword := c.Text()
	userID := c.Sender().ID

	// 清除状态
	session.GetManager().ClearSession(userID)

	c.Send("🔍 正在搜索，请稍候...")

	// 搜索 MoviePilot
	mpClient := moviepilot.GetClient()
	if mpClient == nil {
		return c.Send("❌ MoviePilot 服务未配置")
	}

	results, err := mpClient.Search(keyword)
	if err != nil {
		logger.Error().Err(err).Str("keyword", keyword).Msg("MoviePilot 搜索失败")
		return c.Send("❌ 搜索失败: " + err.Error())
	}

	if len(results) == 0 {
		return c.Send("🤷‍♂️ 没有找到相关资源")
	}

	// 保存搜索结果
	totalPages := int(math.Ceil(float64(len(results)) / float64(mpItemsPerPage)))
	searchDataLock.Lock()
	userSearchData[userID] = &MPSearchSession{
		Keyword:     keyword,
		Results:     results,
		CurrentPage: 1,
		TotalPages:  totalPages,
	}
	searchDataLock.Unlock()

	// 发送第一页结果
	return sendMPSearchResults(c, userID, 1)
}

// sendMPSearchResults 发送搜索结果
func sendMPSearchResults(c tele.Context, userID int64, page int) error {
	searchDataLock.RLock()
	sess, exists := userSearchData[userID]
	searchDataLock.RUnlock()

	if !exists {
		return c.Send("❌ 搜索会话已过期，请重新搜索")
	}

	// 计算分页
	startIdx := (page - 1) * mpItemsPerPage
	endIdx := startIdx + mpItemsPerPage
	if endIdx > len(sess.Results) {
		endIdx = len(sess.Results)
	}

	pageItems := sess.Results[startIdx:endIdx]

	// 发送每个资源信息
	for i, item := range pageItems {
		idx := startIdx + i + 1
		text := item.FormatText(idx)
		c.Send(text, tele.ModeMarkdown)
	}

	// 发送分页控制
	paginationText := fmt.Sprintf(
		"📋 第 %d/%d 页 | 共 %d 个资源\n\n"+
			"请发送资源编号进行下载\n"+
			"退出请点 /cancel",
		page, sess.TotalPages, len(sess.Results),
	)

	// 设置等待选择状态
	session.GetManager().SetState(userID, session.StateMoviePilotSelectMedia)

	return c.Send(paginationText, keyboards.MPSearchPageKeyboard(page > 1, page < sess.TotalPages))
}

// HandleMPPagePrev 上一页
func HandleMPPagePrev(c tele.Context) error {
	userID := c.Sender().ID

	searchDataLock.RLock()
	sess, exists := userSearchData[userID]
	searchDataLock.RUnlock()

	if !exists {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 搜索会话已过期",
			ShowAlert: true,
		})
	}

	if sess.CurrentPage <= 1 {
		return c.Respond(&tele.CallbackResponse{Text: "已经是第一页"})
	}

	sess.CurrentPage--
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("📃 加载第 %d 页", sess.CurrentPage)})

	return sendMPSearchResults(c, userID, sess.CurrentPage)
}

// HandleMPPageNext 下一页
func HandleMPPageNext(c tele.Context) error {
	userID := c.Sender().ID

	searchDataLock.RLock()
	sess, exists := userSearchData[userID]
	searchDataLock.RUnlock()

	if !exists {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 搜索会话已过期",
			ShowAlert: true,
		})
	}

	if sess.CurrentPage >= sess.TotalPages {
		return c.Respond(&tele.CallbackResponse{Text: "已经是最后一页"})
	}

	sess.CurrentPage++
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("📃 加载第 %d 页", sess.CurrentPage)})

	return sendMPSearchResults(c, userID, sess.CurrentPage)
}

// HandleMPSelectDownload 处理资源选择下载
func HandleMPSelectDownload(c tele.Context) error {
	userID := c.Sender().ID
	indexStr := c.Text()

	// 解析编号
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return c.Send("❌ 请输入有效的资源编号")
	}

	searchDataLock.RLock()
	sess, exists := userSearchData[userID]
	searchDataLock.RUnlock()

	if !exists {
		session.GetManager().ClearSession(userID)
		return c.Send("❌ 搜索会话已过期，请重新搜索")
	}

	if index < 1 || index > len(sess.Results) {
		return c.Send(fmt.Sprintf("❌ 请输入 1-%d 之间的编号", len(sess.Results)))
	}

	result := sess.Results[index-1]
	cfg := config.Get()

	// 计算费用
	needCost := int(math.Ceil(result.SizeGB)) * cfg.MoviePilot.Price

	// 检查用户余额
	embyRepo := repository.NewEmbyRepository()
	embyUser, err := embyRepo.GetByTG(userID)
	if err != nil {
		return c.Send("❌ 获取用户信息失败")
	}

	money := cfg.Money
	if money == "" {
		money = "花币"
	}

	if embyUser.Iv < needCost {
		return c.Send(fmt.Sprintf("❌ %s 不足\n\n此资源需要: %d %s\n您当前拥有: %d %s",
			money, needCost, money, embyUser.Iv, money))
	}

	c.Send("⏳ 正在添加下载任务...")

	// 添加下载任务
	mpClient := moviepilot.GetClient()
	downloadID, err := mpClient.AddDownload(result.TorrentInfo)
	if err != nil {
		logger.Error().Err(err).Msg("添加下载任务失败")
		return c.Send("❌ 添加下载任务失败: " + err.Error())
	}

	// 扣除费用
	embyRepo.UpdateFields(userID, map[string]interface{}{
		"iv": embyUser.Iv - needCost,
	})

	// 清除搜索会话
	searchDataLock.Lock()
	delete(userSearchData, userID)
	searchDataLock.Unlock()
	session.GetManager().ClearSession(userID)

	logger.Info().
		Int64("user", userID).
		Str("title", result.Title).
		Str("download_id", downloadID).
		Int("cost", needCost).
		Msg("MoviePilot 下载任务添加成功")

	return c.Send(fmt.Sprintf(
		"🎉 **下载任务已添加**\n\n"+
			"标题: %s\n"+
			"下载ID: `%s`\n"+
			"消耗: %d %s\n"+
			"剩余: %d %s",
		result.Title,
		downloadID,
		needCost, money,
		embyUser.Iv-needCost, money,
	), tele.ModeMarkdown)
}

// HandleMPCancelSearch 取消搜索
func HandleMPCancelSearch(c tele.Context) error {
	userID := c.Sender().ID

	searchDataLock.Lock()
	delete(userSearchData, userID)
	searchDataLock.Unlock()
	session.GetManager().ClearSession(userID)

	c.Respond(&tele.CallbackResponse{Text: "已取消"})
	return c.Edit("🔍 已取消搜索", keyboards.BackToMemberKeyboard())
}

// HandleViewDownloads 查看下载进度
func HandleViewDownloads(c tele.Context) error {
	cfg := config.Get()
	if !cfg.MoviePilot.Enabled {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ 管理员未开启点播功能",
			ShowAlert: true,
		})
	}

	c.Respond(&tele.CallbackResponse{Text: "📈 查看下载进度"})

	mpClient := moviepilot.GetClient()
	if mpClient == nil {
		return c.Edit("❌ MoviePilot 服务未配置")
	}

	tasks, err := mpClient.GetDownloadTasks()
	if err != nil {
		return c.Edit("❌ 获取下载任务失败: " + err.Error())
	}

	if len(tasks) == 0 {
		return c.Edit("📭 当前没有下载任务")
	}

	text := "📈 **下载任务列表**\n\n"
	for i, task := range tasks {
		if i >= 10 {
			text += fmt.Sprintf("\n... 还有 %d 个任务", len(tasks)-10)
			break
		}

		progressBar := getMPProgressBar(task.Progress)
		stateText := "🔄 下载中"
		if task.State == "completed" {
			stateText = "✅ 已完成"
		} else if task.State == "paused" {
			stateText = "⏸️ 已暂停"
		}

		text += fmt.Sprintf("**%d.** %s\n", i+1, stateText)
		text += fmt.Sprintf("   %s %.1f%%\n", progressBar, task.Progress)
		if task.LeftTime != "" {
			text += fmt.Sprintf("   剩余: %s\n", task.LeftTime)
		}
		text += "\n"
	}

	return c.Edit(text, tele.ModeMarkdown, keyboards.DownloadCenterKeyboard())
}

// getMPProgressBar 生成进度条
func getMPProgressBar(progress float64) string {
	filled := int(progress / 10)
	empty := 10 - filled
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "🟩"
	}
	for i := 0; i < empty; i++ {
		bar += "⬜"
	}
	return bar
}

// RegisterMoviePilotCallbacks 注册 MoviePilot 相关回调
func RegisterMoviePilotCallbacks(bot *tele.Bot) {
	bot.Handle(&tele.Btn{Unique: "download_center"}, HandleDownloadCenter)
	bot.Handle(&tele.Btn{Unique: "get_resource"}, HandleSearchResource)
	bot.Handle(&tele.Btn{Unique: "view_downloads"}, HandleViewDownloads)
	bot.Handle(&tele.Btn{Unique: "mp_prev_page"}, HandleMPPagePrev)
	bot.Handle(&tele.Btn{Unique: "mp_next_page"}, HandleMPPageNext)
	bot.Handle(&tele.Btn{Unique: "mp_cancel"}, HandleMPCancelSearch)

	logger.Info().Msg("MoviePilot 回调已注册")
}
