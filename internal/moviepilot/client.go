// Package moviepilot MoviePilot API 客户端
package moviepilot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// Client MoviePilot API 客户端
type Client struct {
	baseURL     string
	username    string
	password    string
	accessToken string
	httpClient  *resty.Client
	mu          sync.RWMutex
}

var (
	instance *Client
	once     sync.Once
)

// GetClient 获取 MoviePilot 客户端单例
func GetClient() *Client {
	once.Do(func() {
		cfg := config.Get()
		if cfg.MoviePilot.Enabled {
			instance = NewClient(cfg.MoviePilot.URL, cfg.MoviePilot.Username, cfg.MoviePilot.Password)
		}
	})
	return instance
}

// NewClient 创建新的 MoviePilot 客户端
func NewClient(baseURL, username, password string) *Client {
	client := resty.New()
	client.SetTimeout(30 * time.Second)
	client.SetRetryCount(2)
	client.SetRetryWaitTime(3 * time.Second)

	return &Client{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: client,
	}
}

// IsEnabled 检查 MoviePilot 是否启用
func IsEnabled() bool {
	cfg := config.Get()
	return cfg.MoviePilot.Enabled
}

// Login 登录获取 Token
func (c *Client) Login() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	loginURL := fmt.Sprintf("%s/api/v1/login/access-token", c.baseURL)
	
	resp, err := c.httpClient.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"username": c.username,
			"password": c.password,
		}).
		Post(loginURL)

	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if accessToken, ok := result["access_token"].(string); ok {
		tokenType, _ := result["token_type"].(string)
		c.accessToken = fmt.Sprintf("%s %s", tokenType, accessToken)
		logger.Info().Msg("MoviePilot 登录成功")
		return nil
	}

	return fmt.Errorf("登录失败: %v", result)
}

// request 发送请求
func (c *Client) request(method, endpoint string, body interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	url := c.baseURL + endpoint

	req := c.httpClient.R().
		SetHeader("Authorization", token).
		SetHeader("Content-Type", "application/json")

	if body != nil {
		req.SetBody(body)
	}

	var resp *resty.Response
	var err error

	switch method {
	case http.MethodGet:
		resp, err = req.Get(url)
	case http.MethodPost:
		resp, err = req.Post(url)
	default:
		return nil, fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	// Token 过期，重新登录
	if resp.StatusCode() == http.StatusUnauthorized || resp.StatusCode() == http.StatusForbidden {
		logger.Warn().Msg("MoviePilot Token 过期，重新登录")
		if err := c.Login(); err != nil {
			return nil, err
		}
		return c.request(method, endpoint, body)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		// 尝试解析为数组
		var arr []interface{}
		if json.Unmarshal(resp.Body(), &arr) == nil {
			return map[string]interface{}{"data": arr}, nil
		}
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result, nil
}

// SearchResult 搜索结果
type SearchResult struct {
	Title        string  `json:"title"`
	Year         string  `json:"year"`
	Type         string  `json:"type"`
	ResourcePix  string  `json:"resource_pix"`
	VideoEncode  string  `json:"video_encode"`
	AudioEncode  string  `json:"audio_encode"`
	ResourceTeam string  `json:"resource_team"`
	Seeders      int     `json:"seeders"`
	Size         int64   `json:"size"`
	SizeGB       float64 `json:"size_gb"`
	Labels       string  `json:"labels"`
	Description  string  `json:"description"`
	TorrentInfo  map[string]interface{} `json:"torrent_info"`
}

// Search 搜索资源
func (c *Client) Search(keyword string) ([]SearchResult, error) {
	if keyword == "" {
		return nil, fmt.Errorf("关键词不能为空")
	}

	encoded := url.QueryEscape(keyword)
	endpoint := fmt.Sprintf("/api/v1/search/title?keyword=%s", encoded)

	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	success, _ := result["success"].(bool)
	if !success {
		return nil, fmt.Errorf("搜索失败")
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("解析搜索结果失败")
	}

	var results []SearchResult
	for _, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		metaInfo, _ := itemMap["meta_info"].(map[string]interface{})
		torrentInfo, _ := itemMap["torrent_info"].(map[string]interface{})

		seeders := 0
		if s, ok := torrentInfo["seeders"].(float64); ok {
			seeders = int(s)
		} else if s, ok := torrentInfo["seeders"].(string); ok {
			fmt.Sscanf(s, "%d", &seeders)
		}

		var size int64
		if s, ok := torrentInfo["size"].(float64); ok {
			size = int64(s)
		} else if s, ok := torrentInfo["size"].(string); ok {
			fmt.Sscanf(s, "%d", &size)
		}

		results = append(results, SearchResult{
			Title:        getString(metaInfo, "title"),
			Year:         getString(metaInfo, "year"),
			Type:         getString(metaInfo, "type"),
			ResourcePix:  getString(metaInfo, "resource_pix"),
			VideoEncode:  getString(metaInfo, "video_encode"),
			AudioEncode:  getString(metaInfo, "audio_encode"),
			ResourceTeam: getString(metaInfo, "resource_team"),
			Seeders:      seeders,
			Size:         size,
			SizeGB:       float64(size) / (1024 * 1024 * 1024),
			Labels:       getString(torrentInfo, "labels"),
			Description:  getString(torrentInfo, "description"),
			TorrentInfo:  torrentInfo,
		})
	}

	// 按做种数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Seeders > results[j].Seeders
	})

	logger.Info().Str("keyword", keyword).Int("count", len(results)).Msg("MoviePilot 搜索完成")
	return results, nil
}

// AddDownload 添加下载任务
func (c *Client) AddDownload(torrentInfo map[string]interface{}) (string, error) {
	if torrentInfo == nil {
		return "", fmt.Errorf("种子信息不能为空")
	}

	// 兼容 MP v2 API
	param := map[string]interface{}{
		"torrent_in": torrentInfo,
	}
	for k, v := range torrentInfo {
		param[k] = v
	}

	result, err := c.request(http.MethodPost, "/api/v1/download/add", param)
	if err != nil {
		return "", err
	}

	success, _ := result["success"].(bool)
	if !success {
		return "", fmt.Errorf("添加下载失败: %v", result)
	}

	data, _ := result["data"].(map[string]interface{})
	downloadID, _ := data["download_id"].(string)

	logger.Info().Str("download_id", downloadID).Msg("MoviePilot 下载任务添加成功")
	return downloadID, nil
}

// DownloadTask 下载任务
type DownloadTask struct {
	DownloadID string  `json:"download_id"`
	State      string  `json:"state"`
	Progress   float64 `json:"progress"`
	LeftTime   string  `json:"left_time"`
}

// GetDownloadTasks 获取下载任务列表
func (c *Client) GetDownloadTasks() ([]DownloadTask, error) {
	result, err := c.request(http.MethodGet, "/api/v1/download?name=下载", nil)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		return nil, nil
	}

	var tasks []DownloadTask
	for _, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		progress, _ := itemMap["progress"].(float64)

		tasks = append(tasks, DownloadTask{
			DownloadID: getString(itemMap, "hash"),
			State:      getString(itemMap, "state"),
			Progress:   progress,
			LeftTime:   getString(itemMap, "left_time"),
		})
	}

	return tasks, nil
}

// GetTransferStatus 获取转移状态
func (c *Client) GetTransferStatus(title, downloadID string) (bool, error) {
	encoded := url.QueryEscape(title)
	endpoint := fmt.Sprintf("/api/v1/history/transfer?title=%s&page=1&count=50", encoded)

	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}

	success, _ := result["success"].(bool)
	if !success {
		return false, nil
	}

	data, _ := result["data"].(map[string]interface{})
	list, _ := data["list"].([]interface{})

	for _, item := range list {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if getString(itemMap, "download_hash") == downloadID {
			return getBool(itemMap, "status"), nil
		}
	}

	return false, nil
}

// FormatSearchResult 格式化搜索结果
func (r *SearchResult) FormatText(index int) string {
	text := fmt.Sprintf("📦 资源编号: `%d`\n", index)
	text += fmt.Sprintf("标题：%s\n", r.Title)

	if r.Year != "" {
		text += fmt.Sprintf("年份：%s\n", r.Year)
	}

	typeStr := r.Type
	if typeStr == "" || typeStr == "未知" {
		typeStr = "电影"
	}
	text += fmt.Sprintf("类型：%s\n", typeStr)

	if r.Size > 0 {
		text += fmt.Sprintf("大小：%.2f GB\n", r.SizeGB)
	}

	if r.Labels != "" {
		text += fmt.Sprintf("标签：%s\n", r.Labels)
	}

	text += fmt.Sprintf("做种数：%d\n", r.Seeders)

	var mediaInfo []string
	if r.ResourcePix != "" {
		mediaInfo = append(mediaInfo, r.ResourcePix)
	}
	if r.VideoEncode != "" {
		mediaInfo = append(mediaInfo, r.VideoEncode)
	}
	if r.AudioEncode != "" {
		mediaInfo = append(mediaInfo, r.AudioEncode)
	}
	if len(mediaInfo) > 0 {
		text += fmt.Sprintf("媒体信息：%s\n", joinStrings(mediaInfo, " | "))
	}

	return text
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
