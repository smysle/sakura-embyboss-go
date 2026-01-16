// Package emby Emby API 客户端
package emby

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
	"github.com/smysle/sakura-embyboss-go/pkg/utils"
)

// Client Emby API 客户端
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *resty.Client
	mu         sync.RWMutex
}

var (
	instance *Client
	once     sync.Once
)

// GetClient 获取 Emby 客户端单例
func GetClient() *Client {
	once.Do(func() {
		cfg := config.Get()
		instance = NewClient(cfg.Emby.URL, cfg.Emby.APIKey)
	})
	return instance
}

// NewClient 创建新的 Emby 客户端
func NewClient(baseURL, apiKey string) *Client {
	client := resty.New()
	client.SetTimeout(10 * time.Second)
	client.SetRetryCount(2)
	client.SetRetryWaitTime(1 * time.Second)
	client.SetHeaders(map[string]string{
		"Accept":                "application/json",
		"Content-Type":         "application/json",
		"X-Emby-Token":         apiKey,
		"X-Emby-Client":        "Sakura BOT",
		"X-Emby-Device-Name":   "Sakura BOT",
		"X-Emby-Client-Version": "2.0.0",
		"User-Agent":           "SakuraEmbyBoss/2.0 Go",
	})

	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: client,
	}
}

// APIResult API 返回结果封装
type APIResult struct {
	Success bool
	Data    interface{}
	Error   string
}

// request 发送 HTTP 请求
func (c *Client) request(method, endpoint string, body interface{}) (*APIResult, error) {
	url := c.baseURL + endpoint

	req := c.httpClient.R()
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
	case http.MethodDelete:
		resp, err = req.Delete(url)
	default:
		return nil, fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}

	if err != nil {
		logger.Error().Err(err).Str("url", url).Msg("HTTP 请求失败")
		return &APIResult{Success: false, Error: err.Error()}, err
	}

	statusCode := resp.StatusCode()
	if statusCode == http.StatusOK || statusCode == http.StatusNoContent {
		var data interface{}
		contentType := resp.Header().Get("Content-Type")
		// 检查 Content-Type 是否包含 json（可能是 application/json 或 application/json; charset=utf-8）
		if len(resp.Body()) > 0 && strings.Contains(contentType, "json") {
			if err := json.Unmarshal(resp.Body(), &data); err != nil {
				logger.Warn().Err(err).Str("url", url).Msg("JSON 解析失败，返回原始数据")
				return &APIResult{Success: true, Data: resp.Body()}, nil
			}
		} else if len(resp.Body()) > 0 {
			// 尝试直接解析为 JSON（有些 Emby 服务器可能不设置正确的 Content-Type）
			if err := json.Unmarshal(resp.Body(), &data); err == nil {
				return &APIResult{Success: true, Data: data}, nil
			}
		}
		return &APIResult{Success: true, Data: data}, nil
	}

	errMsg := fmt.Sprintf("HTTP %d: %s", statusCode, string(resp.Body()))
	logger.Warn().Str("url", url).Int("status", statusCode).Msg("API 请求失败")
	return &APIResult{Success: false, Error: errMsg}, fmt.Errorf(errMsg)
}

// CreateUser 创建 Emby 用户
func (c *Client) CreateUser(name string, days int) (*CreateUserResult, error) {
	logger.Info().Str("name", name).Int("days", days).Msg("开始创建 Emby 用户")

	// 1. 创建用户
	result, err := c.request(http.MethodPost, "/emby/Users/New", map[string]string{"Name": name})
	if err != nil || !result.Success {
		return nil, fmt.Errorf("创建用户失败: %v", err)
	}

	userData, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析用户数据")
	}

	userID, ok := userData["Id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("无法获取用户 ID")
	}

	// 2. 生成并设置密码
	password, err := utils.GeneratePassword(8)
	if err != nil {
		return nil, fmt.Errorf("生成密码失败: %v", err)
	}

	if err := c.SetPassword(userID, password); err != nil {
		// 尝试删除已创建的用户
		c.DeleteUser(userID)
		return nil, fmt.Errorf("设置密码失败: %v", err)
	}

	// 3. 设置用户策略
	if err := c.SetUserPolicy(userID, false, false); err != nil {
		logger.Warn().Str("userID", userID).Err(err).Msg("设置用户策略失败")
	}

	// 4. 隐藏额外媒体库
	cfg := config.Get()
	blockedLibs := append(cfg.Emby.BlockedLibs, cfg.Emby.ExtraLibs...)
	if err := c.HideFolders(userID, blockedLibs); err != nil {
		logger.Warn().Str("userID", userID).Err(err).Msg("隐藏媒体库失败")
	}

	expiryDate := time.Now().AddDate(0, 0, days)
	logger.Info().Str("userID", userID).Str("name", name).Msg("成功创建 Emby 用户")

	return &CreateUserResult{
		UserID:     userID,
		Password:   password,
		ExpiryDate: expiryDate,
	}, nil
}

// CreateUserResult 创建用户结果
type CreateUserResult struct {
	UserID     string
	Password   string
	ExpiryDate time.Time
}

// DeleteUser 删除 Emby 用户
func (c *Client) DeleteUser(userID string) error {
	logger.Info().Str("userID", userID).Msg("删除 Emby 用户")

	result, err := c.request(http.MethodDelete, "/emby/Users/"+userID, nil)
	if err != nil || !result.Success {
		return fmt.Errorf("删除用户失败: %v", result.Error)
	}
	return nil
}

// SetPassword 设置用户密码
func (c *Client) SetPassword(userID, password string) error {
	// 先重置密码
	resetData := map[string]interface{}{
		"Id":            userID,
		"ResetPassword": true,
	}
	if _, err := c.request(http.MethodPost, "/emby/Users/"+userID+"/Password", resetData); err != nil {
		return err
	}

	// 设置新密码
	pwdData := map[string]interface{}{
		"Id":    userID,
		"NewPw": password,
	}
	result, err := c.request(http.MethodPost, "/emby/Users/"+userID+"/Password", pwdData)
	if err != nil || !result.Success {
		return fmt.Errorf("设置密码失败: %v", result.Error)
	}
	return nil
}

// ResetPassword 重置密码（设置为空）
func (c *Client) ResetPassword(userID string) error {
	resetData := map[string]interface{}{
		"Id":            userID,
		"ResetPassword": true,
	}
	result, err := c.request(http.MethodPost, "/emby/Users/"+userID+"/Password", resetData)
	if err != nil || !result.Success {
		return fmt.Errorf("重置密码失败: %v", result.Error)
	}
	return nil
}

// SetUserPolicy 设置用户策略
func (c *Client) SetUserPolicy(userID string, isAdmin, isDisabled bool) error {
	policy := c.createPolicy(isAdmin, isDisabled, 2, nil)

	result, err := c.request(http.MethodPost, "/emby/Users/"+userID+"/Policy", policy)
	if err != nil || !result.Success {
		return fmt.Errorf("设置策略失败: %v", result.Error)
	}
	return nil
}

// EnableUser 启用用户
func (c *Client) EnableUser(userID string) error {
	return c.SetUserPolicy(userID, false, false)
}

// DisableUser 禁用用户
func (c *Client) DisableUser(userID string) error {
	return c.SetUserPolicy(userID, false, true)
}

// createPolicy 创建用户策略
func (c *Client) createPolicy(isAdmin, isDisabled bool, streamLimit int, blockedFolders []string) map[string]interface{} {
	if blockedFolders == nil {
		cfg := config.Get()
		blockedFolders = append([]string{"播放列表"}, cfg.Emby.ExtraLibs...)
	}

	return map[string]interface{}{
		"IsAdministrator":                    isAdmin,
		"IsHidden":                           true,
		"IsHiddenRemotely":                   true,
		"IsDisabled":                         isDisabled,
		"EnableRemoteControlOfOtherUsers":   false,
		"EnableSharedDeviceControl":         false,
		"EnableRemoteAccess":                true,
		"EnableLiveTvManagement":            false,
		"EnableLiveTvAccess":                true,
		"EnableMediaPlayback":               true,
		"EnableAudioPlaybackTranscoding":    false,
		"EnableVideoPlaybackTranscoding":    false,
		"EnablePlaybackRemuxing":            false,
		"EnableContentDeletion":             false,
		"EnableContentDownloading":          false,
		"EnableSubtitleDownloading":         false,
		"EnableSubtitleManagement":          false,
		"EnableSyncTranscoding":             false,
		"EnableMediaConversion":             false,
		"EnableAllDevices":                  true,
		"SimultaneousStreamLimit":           streamLimit,
		"BlockedMediaFolders":               blockedFolders,
		"AllowCameraUpload":                 false,
	}
}

// GetUser 获取用户信息
func (c *Client) GetUser(userID string) (*User, error) {
	result, err := c.request(http.MethodGet, "/emby/Users/"+userID, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取用户失败: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析用户数据")
	}

	return parseUser(data), nil
}

// GetUsers 获取所有用户列表
func (c *Client) GetUsers() ([]User, error) {
	result, err := c.request(http.MethodGet, "/emby/Users", nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取用户列表失败: %v", result.Error)
	}

	data, ok := result.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析用户列表")
	}

	var users []User
	for _, item := range data {
		if userData, ok := item.(map[string]interface{}); ok {
			users = append(users, *parseUser(userData))
		}
	}
	return users, nil
}

// GetUserByName 根据用户名获取用户
func (c *Client) GetUserByName(name string) (*User, error) {
	endpoint := fmt.Sprintf("/emby/Users/Query?NameStartsWithOrGreater=%s&api_key=%s", name, c.apiKey)
	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("查询用户失败: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析响应数据")
	}

	items, ok := data["Items"].([]interface{})
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("用户不存在")
	}

	for _, item := range items {
		if userData, ok := item.(map[string]interface{}); ok {
			if userData["Name"] == name {
				return parseUser(userData), nil
			}
		}
	}
	return nil, fmt.Errorf("用户不存在")
}

// User Emby 用户
type User struct {
	ID       string
	Name     string
	Policy   *UserPolicy
	LastSeen *time.Time
}

// UserPolicy 用户策略
type UserPolicy struct {
	IsAdmin         bool
	IsDisabled      bool
	EnableAllFolders bool
	EnabledFolders  []string
	BlockedFolders  []string
}

func parseUser(data map[string]interface{}) *User {
	user := &User{
		ID:   getString(data, "Id"),
		Name: getString(data, "Name"),
	}

	if policy, ok := data["Policy"].(map[string]interface{}); ok {
		user.Policy = &UserPolicy{
			IsAdmin:          getBool(policy, "IsAdministrator"),
			IsDisabled:       getBool(policy, "IsDisabled"),
			EnableAllFolders: getBool(policy, "EnableAllFolders"),
		}

		if folders, ok := policy["EnabledFolders"].([]interface{}); ok {
			for _, f := range folders {
				if s, ok := f.(string); ok {
					user.Policy.EnabledFolders = append(user.Policy.EnabledFolders, s)
				}
			}
		}

		if folders, ok := policy["BlockedMediaFolders"].([]interface{}); ok {
			for _, f := range folders {
				if s, ok := f.(string); ok {
					user.Policy.BlockedFolders = append(user.Policy.BlockedFolders, s)
				}
			}
		}
	}

	return user
}

// GetLibraries 获取媒体库列表
func (c *Client) GetLibraries() (map[string]string, error) {
	endpoint := fmt.Sprintf("/emby/Library/VirtualFolders?api_key=%s", c.apiKey)
	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取媒体库失败: %v", result.Error)
	}

	data, ok := result.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析媒体库数据")
	}

	libs := make(map[string]string)
	for _, item := range data {
		if lib, ok := item.(map[string]interface{}); ok {
			guid := getString(lib, "Guid")
			name := getString(lib, "Name")
			if guid != "" && name != "" {
				libs[guid] = name
			}
		}
	}
	return libs, nil
}

// HideFolders 隐藏指定媒体库
func (c *Client) HideFolders(userID string, folderNames []string) error {
	if len(folderNames) == 0 {
		return nil
	}

	// 获取用户当前策略
	user, err := c.GetUser(userID)
	if err != nil {
		return err
	}

	// 获取要隐藏的媒体库 ID
	libs, err := c.GetLibraries()
	if err != nil {
		return err
	}

	var hideIDs []string
	for guid, name := range libs {
		for _, fn := range folderNames {
			if name == fn {
				hideIDs = append(hideIDs, guid)
				break
			}
		}
	}

	// 更新启用的文件夹列表
	enabledFolders := user.Policy.EnabledFolders
	if user.Policy.EnableAllFolders {
		// 如果启用所有文件夹，先获取所有文件夹
		for guid := range libs {
			enabledFolders = append(enabledFolders, guid)
		}
	}

	// 从启用列表移除要隐藏的
	var newEnabled []string
	for _, f := range enabledFolders {
		hide := false
		for _, h := range hideIDs {
			if f == h {
				hide = true
				break
			}
		}
		if !hide {
			newEnabled = append(newEnabled, f)
		}
	}

	// 更新策略
	updateData := map[string]interface{}{
		"EnableAllFolders":    false,
		"EnabledFolders":      newEnabled,
		"BlockedMediaFolders": folderNames,
	}

	// 先获取现有策略再合并
	result, err := c.request(http.MethodGet, "/emby/Users/"+userID, nil)
	if err != nil || !result.Success {
		return err
	}

	userData := result.Data.(map[string]interface{})
	if policy, ok := userData["Policy"].(map[string]interface{}); ok {
		for k, v := range updateData {
			policy[k] = v
		}
		_, err = c.request(http.MethodPost, "/emby/Users/"+userID+"/Policy", policy)
		return err
	}

	return fmt.Errorf("无法更新用户策略")
}

// ShowFolders 显示指定媒体库
func (c *Client) ShowFolders(userID string, folderNames []string) error {
	if len(folderNames) == 0 {
		return nil
	}

	// 获取媒体库 ID
	libs, err := c.GetLibraries()
	if err != nil {
		return err
	}

	var showIDs []string
	for guid, name := range libs {
		for _, fn := range folderNames {
			if name == fn {
				showIDs = append(showIDs, guid)
				break
			}
		}
	}

	// 获取用户当前策略
	user, err := c.GetUser(userID)
	if err != nil {
		return err
	}

	// 合并启用的文件夹
	enabledSet := make(map[string]bool)
	for _, f := range user.Policy.EnabledFolders {
		enabledSet[f] = true
	}
	for _, f := range showIDs {
		enabledSet[f] = true
	}

	var newEnabled []string
	for f := range enabledSet {
		newEnabled = append(newEnabled, f)
	}

	// 从阻止列表移除
	var newBlocked []string
	for _, b := range user.Policy.BlockedFolders {
		remove := false
		for _, fn := range folderNames {
			if b == fn {
				remove = true
				break
			}
		}
		if !remove {
			newBlocked = append(newBlocked, b)
		}
	}

	// 更新策略
	result, err := c.request(http.MethodGet, "/emby/Users/"+userID, nil)
	if err != nil || !result.Success {
		return err
	}

	userData := result.Data.(map[string]interface{})
	if policy, ok := userData["Policy"].(map[string]interface{}); ok {
		policy["EnableAllFolders"] = false
		policy["EnabledFolders"] = newEnabled
		policy["BlockedMediaFolders"] = newBlocked
		_, err = c.request(http.MethodPost, "/emby/Users/"+userID+"/Policy", policy)
		return err
	}

	return fmt.Errorf("无法更新用户策略")
}

// DisableAllLibraries 禁用用户所有媒体库
func (c *Client) DisableAllLibraries(userID string) error {
	result, err := c.request(http.MethodGet, "/emby/Users/"+userID, nil)
	if err != nil || !result.Success {
		return err
	}

	userData := result.Data.(map[string]interface{})
	if policy, ok := userData["Policy"].(map[string]interface{}); ok {
		policy["EnableAllFolders"] = false
		policy["EnabledFolders"] = []string{}
		_, err = c.request(http.MethodPost, "/emby/Users/"+userID+"/Policy", policy)
		return err
	}

	return fmt.Errorf("无法更新用户策略")
}

// EnableAllLibraries 启用用户所有媒体库
func (c *Client) EnableAllLibraries(userID string) error {
	result, err := c.request(http.MethodGet, "/emby/Users/"+userID, nil)
	if err != nil || !result.Success {
		return err
	}

	userData := result.Data.(map[string]interface{})
	if policy, ok := userData["Policy"].(map[string]interface{}); ok {
		policy["EnableAllFolders"] = true
		policy["EnabledFolders"] = []string{}
		policy["BlockedMediaFolders"] = []string{}
		_, err = c.request(http.MethodPost, "/emby/Users/"+userID+"/Policy", policy)
		return err
	}

	return fmt.Errorf("无法更新用户策略")
}

// GetMediaCounts 获取媒体统计
func (c *Client) GetMediaCounts() (*MediaCounts, error) {
	endpoint := fmt.Sprintf("/emby/Items/Counts?api_key=%s", c.apiKey)
	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取媒体统计失败: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析媒体统计")
	}

	return &MediaCounts{
		Movies:   getInt(data, "MovieCount"),
		Series:   getInt(data, "SeriesCount"),
		Episodes: getInt(data, "EpisodeCount"),
		Songs:    getInt(data, "SongCount"),
	}, nil
}

// MediaCounts 媒体统计
type MediaCounts struct {
	Movies   int
	Series   int
	Episodes int
	Songs    int
}

// FormatText 格式化为文本
func (m *MediaCounts) FormatText() string {
	return fmt.Sprintf(
		"🎬 电影数量：%d\n📽️ 剧集数量：%d\n🎵 音乐数量：%d\n🎞️ 总集数：%d",
		m.Movies, m.Series, m.Songs, m.Episodes,
	)
}

// GetCurrentPlayingCount 获取当前播放用户数
func (c *Client) GetCurrentPlayingCount() (int, error) {
	result, err := c.request(http.MethodGet, "/emby/Sessions", nil)
	if err != nil || !result.Success {
		return -1, fmt.Errorf("获取会话失败: %v", result.Error)
	}

	data, ok := result.Data.([]interface{})
	if !ok {
		return 0, nil
	}

	count := 0
	for _, item := range data {
		if session, ok := item.(map[string]interface{}); ok {
			if session["NowPlayingItem"] != nil {
				count++
			}
		}
	}
	return count, nil
}

// TerminateSession 终止会话
func (c *Client) TerminateSession(sessionID, reason string) error {
	logger.Info().Str("sessionID", sessionID).Str("reason", reason).Msg("终止会话")

	// 停止播放
	c.request(http.MethodPost, "/emby/Sessions/"+sessionID+"/Playing/Stop", nil)

	// 发送消息
	msgData := map[string]interface{}{
		"Text":      "🚫 会话已被终止: " + reason,
		"Header":    "安全警告",
		"TimeoutMs": 10000,
	}
	c.request(http.MethodPost, "/emby/Sessions/"+sessionID+"/Message", msgData)

	return nil
}

// FavoriteItem 收藏项目
type FavoriteItem struct {
	ID       string
	Name     string
	Type     string
	Year     int
	ImageTag string
}

// GetUserFavorites 获取用户收藏列表（分页版本）
func (c *Client) GetUserFavorites(userID string, offset, limit int) ([]FavoriteItem, int, error) {
	if limit <= 0 {
		limit = 20
	}

	endpoint := fmt.Sprintf("/emby/Users/%s/Items?Filters=IsFavorite&StartIndex=%d&Limit=%d&Recursive=true&SortBy=SortName&SortOrder=Ascending", userID, offset, limit)
	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}

	if !result.Success {
		return nil, 0, fmt.Errorf("获取收藏失败: %s", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("响应格式错误")
	}

	totalCount := getInt(data, "TotalRecordCount")

	items, ok := data["Items"].([]interface{})
	if !ok {
		return []FavoriteItem{}, 0, nil
	}

	var favorites []FavoriteItem
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			fav := FavoriteItem{
				ID:   getString(itemMap, "Id"),
				Name: getString(itemMap, "Name"),
				Type: getString(itemMap, "Type"),
				Year: getInt(itemMap, "ProductionYear"),
			}
			favorites = append(favorites, fav)
		}
	}

	return favorites, totalCount, nil
}

// GetUserFavoritesSimple 获取用户收藏列表（简单版本，不分页）
func (c *Client) GetUserFavoritesSimple(userID string, limit int) ([]FavoriteItem, error) {
	favorites, _, err := c.GetUserFavorites(userID, 0, limit)
	return favorites, err
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID               string
	DeviceName       string
	AppName          string
	LastActivityDate string
	RemoteAddr       string
}

// GetUserDevices 获取用户的设备列表（分页版本）
func (c *Client) GetUserDevices(userID string, offset, limit int) ([]DeviceInfo, int, error) {
	// 通过 Sessions 获取该用户的设备
	result, err := c.request(http.MethodGet, "/emby/Sessions", nil)
	if err != nil {
		return nil, 0, err
	}

	if !result.Success {
		return nil, 0, fmt.Errorf("获取会话失败: %s", result.Error)
	}

	sessions, ok := result.Data.([]interface{})
	if !ok {
		return []DeviceInfo{}, 0, nil
	}

	var allDevices []DeviceInfo
	seenDevices := make(map[string]bool)

	for _, session := range sessions {
		if sessionMap, ok := session.(map[string]interface{}); ok {
			sessionUserID := getString(sessionMap, "UserId")
			if sessionUserID != userID {
				continue
			}

			deviceID := getString(sessionMap, "DeviceId")
			if seenDevices[deviceID] {
				continue
			}
			seenDevices[deviceID] = true

			lastActivity := getString(sessionMap, "LastActivityDate")
			if lastActivity != "" {
				// 解析并格式化时间
				if t, err := time.Parse(time.RFC3339, lastActivity); err == nil {
					lastActivity = t.Format("2006-01-02 15:04")
				}
			}

			device := DeviceInfo{
				ID:               deviceID,
				DeviceName:       getString(sessionMap, "DeviceName"),
				AppName:          getString(sessionMap, "Client"),
				LastActivityDate: lastActivity,
				RemoteAddr:       getString(sessionMap, "RemoteEndPoint"),
			}

			allDevices = append(allDevices, device)
		}
	}

	total := len(allDevices)

	// 应用分页
	if offset >= len(allDevices) {
		return []DeviceInfo{}, total, nil
	}

	end := offset + limit
	if end > len(allDevices) {
		end = len(allDevices)
	}

	return allDevices[offset:end], total, nil
}

// GetUserDevicesSimple 获取用户的设备列表（简单版本）
func (c *Client) GetUserDevicesSimple(userID string) ([]DeviceInfo, error) {
	devices, _, err := c.GetUserDevices(userID, 0, 100)
	return devices, err
}

// 工具函数
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

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// AuthenticateUser 验证用户登录
// 返回: (embyID, error)
func (c *Client) AuthenticateUser(username, password string) (string, error) {
	data := map[string]string{
		"Username": username,
	}
	if password != "" && password != "None" {
		data["Pw"] = password
	}

	result, err := c.request(http.MethodPost, "/emby/Users/AuthenticateByName", data)
	if err != nil {
		return "", fmt.Errorf("认证请求失败: %v", err)
	}

	if !result.Success {
		return "", fmt.Errorf("认证失败: %s", result.Error)
	}

	respData, ok := result.Data.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("无法解析认证响应")
	}

	// 从响应中获取用户信息
	if user, ok := respData["User"].(map[string]interface{}); ok {
		if id, ok := user["Id"].(string); ok {
			return id, nil
		}
	}

	return "", fmt.Errorf("认证响应中无用户ID")
}

// GetDeviceByID 通过设备ID获取设备详情
func (c *Client) GetDeviceByID(deviceID string) (*DeviceInfo, error) {
	endpoint := fmt.Sprintf("/emby/Devices/Info?Id=%s&api_key=%s", deviceID, c.apiKey)
	result, err := c.request(http.MethodGet, endpoint, nil)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("获取设备信息失败: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析设备数据")
	}

	device := &DeviceInfo{
		ID:         getString(data, "Id"),
		DeviceName: getString(data, "Name"),
		AppName:    getString(data, "AppName"),
		AppVersion: getString(data, "AppVersion"),
	}

	if lastUsed := getString(data, "DateLastActivity"); lastUsed != "" {
		if t, err := time.Parse(time.RFC3339, lastUsed); err == nil {
			device.LastActivityDate = &t
		}
	}

	return device, nil
}

// SetUserAdminPolicy 设置用户管理员权限
func (c *Client) SetUserAdminPolicy(userID string, isAdmin bool) error {
	// 先获取用户当前策略
	user, err := c.GetUser(userID)
	if err != nil {
		return err
	}

	policy := c.createDefaultPolicy()
	policy["IsAdministrator"] = isAdmin
	if user.Policy != nil {
		policy["IsDisabled"] = user.Policy.IsDisabled
		policy["EnableAllFolders"] = user.Policy.EnableAllFolders
	}

	result, err := c.request(http.MethodPost, "/emby/Users/"+userID+"/Policy", policy)
	if err != nil || !result.Success {
		return fmt.Errorf("设置管理员权限失败: %v", result.Error)
	}

	return nil
}

// ExecuteCustomQuery 执行自定义SQL查询（需要 user_usage_stats 插件）
func (c *Client) ExecuteCustomQuery(sql string, replaceUserID bool) ([][]interface{}, error) {
	endpoint := fmt.Sprintf("/emby/user_usage_stats/submit_custom_query?api_key=%s", c.apiKey)
	
	data := map[string]interface{}{
		"CustomQueryString": sql,
		"ReplaceUserId":     replaceUserID,
	}

	result, err := c.request(http.MethodPost, endpoint, data)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("执行自定义查询失败: %v", result.Error)
	}

	respData, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法解析查询响应")
	}

	results, ok := respData["results"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("查询结果格式错误")
	}

	var rows [][]interface{}
	for _, row := range results {
		if rowData, ok := row.([]interface{}); ok {
			rows = append(rows, rowData)
		}
	}

	return rows, nil
}

// GetUserIPHistory 获取用户的IP和设备历史
func (c *Client) GetUserIPHistory(userID string, days int) ([]AuditResult, error) {
	sql := fmt.Sprintf(`
		SELECT DISTINCT 
			RemoteEndPoint as ip_address,
			DeviceName as device_name,
			ClientName as client_name,
			MAX(DateCreated) as last_seen
		FROM PlaybackActivity 
		WHERE UserId = '%s' 
		AND DateCreated >= date('now', '-%d days')
		GROUP BY RemoteEndPoint, DeviceName, ClientName
		ORDER BY last_seen DESC
		LIMIT 50
	`, userID, days)

	rows, err := c.ExecuteCustomQuery(sql, true)
	if err != nil {
		// 如果插件不可用，返回空结果
		logger.Warn().Err(err).Msg("执行用户IP历史查询失败，可能缺少 user_usage_stats 插件")
		return nil, nil
	}

	var results []AuditResult
	for _, row := range rows {
		if len(row) >= 4 {
			result := AuditResult{}
			if v, ok := row[0].(string); ok {
				result.IPAddress = v
			}
			if v, ok := row[1].(string); ok {
				result.DeviceName = v
			}
			if v, ok := row[2].(string); ok {
				result.ClientName = v
			}
			if v, ok := row[3].(string); ok {
				if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
					result.LastSeen = &t
				}
			}
			results = append(results, result)
		}
	}

	return results, nil
}

