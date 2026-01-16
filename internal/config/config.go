// Package config 配置管理模块
package config

import (
	"encoding/json"
	"os"
	"sync"
)

// Config 全局配置结构
type Config struct {
	BotName   string  `json:"bot_name"`
	BotToken  string  `json:"bot_token"`
	Owner     int64   `json:"owner"`
	Groups    []int64 `json:"group"`
	MainGroup string  `json:"main_group"`
	Channel   string  `json:"channel"`
	BotPhoto  string  `json:"bot_photo"`
	Admins    []int64 `json:"admins"`
	Money     string  `json:"money"`

	Emby       EmbyConfig       `json:"emby"`
	Database   DatabaseConfig   `json:"database"`
	Open       OpenConfig       `json:"open"`
	Ranks      RanksConfig      `json:"ranks"`
	Scheduler  SchedulerConfig  `json:"scheduler"`
	Proxy      ProxyConfig      `json:"proxy"`
	MoviePilot MoviePilotConfig `json:"moviepilot"`
	AutoUpdate AutoUpdateConfig `json:"auto_update"`
	API        APIConfig        `json:"api"`
	RedEnvelope RedEnvelopeConfig `json:"red_envelope"`
	AntiChannel AntiChannelConfig `json:"anti_channel"`
	Nezha       NezhaConfig       `json:"nezha"`

	KKGiftDays        int `json:"kk_gift_days"`
	ActivityCheckDays int `json:"activity_check_days"`
	FreezeDays        int `json:"freeze_days"`
}

// EmbyConfig Emby 服务器配置
type EmbyConfig struct {
	APIKey                 string   `json:"api_key"`
	URL                    string   `json:"url"`
	Line                   string   `json:"line"`
	WhitelistLine          *string  `json:"whitelist_line"`
	BlockedLibs            []string `json:"blocked_libs"`
	ExtraLibs              []string `json:"extra_libs"`
	BlockedClients         []string `json:"blocked_clients"`
	TerminateOnFilter      bool     `json:"terminate_session_on_filter"`
	BlockUserOnFilter      bool     `json:"block_user_on_filter"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	Password       string `json:"password"`
	Name           string `json:"name"`
	IsDocker       bool   `json:"is_docker"`
	DockerName     string `json:"docker_name"`
	BackupDir      string `json:"backup_dir"`
	BackupMaxCount int    `json:"backup_max_count"`
}

// OpenConfig 开放注册配置
type OpenConfig struct {
	Status        bool   `json:"status"`
	MaxUsers      int    `json:"max_users"`
	Timing        int    `json:"timing"`
	Temp          int    `json:"temp"`
	Checkin       bool   `json:"checkin"`
	CheckinLevel  string `json:"checkin_level"`
	Exchange      bool   `json:"exchange"`
	Whitelist     bool   `json:"whitelist"`
	Invite        bool   `json:"invite"`
	InviteLevel   string `json:"invite_level"`
	LeaveBan      bool   `json:"leave_ban"`
	UserPlays     bool   `json:"user_plays"`
	LowActivity   bool   `json:"low_activity"`
	InactiveDays  int    `json:"inactive_days"`
	CheckinReward []int  `json:"checkin_reward"`
	ExchangeCost  int    `json:"exchange_cost"`
	WhitelistCost int    `json:"whitelist_cost"`
	InviteCost    int    `json:"invite_cost"`
}

// RanksConfig 排行榜配置
type RanksConfig struct {
	Logo     string `json:"logo"`
	Backdrop bool   `json:"backdrop"`
}

// SchedulerConfig 定时任务配置
type SchedulerConfig struct {
	DayRank       bool `json:"day_rank"`
	WeekRank      bool `json:"week_rank"`
	DayPlayRank   bool `json:"day_play_rank"`
	WeekPlayRank  bool `json:"week_play_rank"`
	CheckExpired  bool `json:"check_expired"`
	LowActivity   bool `json:"low_activity"`
	BackupDB      bool `json:"backup_db"`
	SyncFavorites bool `json:"sync_favorites"` // 同步收藏到数据库

	// 运行时状态（不序列化）
	DayRanksMsgID  int64 `json:"-"`
	WeekRanksMsgID int64 `json:"-"`
	RestartChatID  int64 `json:"-"`
	RestartMsgID   int64 `json:"-"`
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// MoviePilotConfig MoviePilot 配置
type MoviePilotConfig struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Price    int    `json:"price"`
	Level    string `json:"level"`
}

// AutoUpdateConfig 自动更新配置
type AutoUpdateConfig struct {
	Enabled bool   `json:"enabled"`
	GitRepo string `json:"git_repo"`
}

// APIConfig Web API 配置
type APIConfig struct {
	Enabled      bool     `json:"enabled"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	AllowOrigins []string `json:"allow_origins"`
}

// RedEnvelopeConfig 红包配置
type RedEnvelopeConfig struct {
	Enabled      bool `json:"enabled"`
	AllowPrivate bool `json:"allow_private"`
}

// AntiChannelConfig 反皮套人配置
type AntiChannelConfig struct {
	Enabled   bool    `json:"enabled"`
	WhiteList []int64 `json:"white_list"`
}

// NezhaConfig 探针配置
type NezhaConfig struct {
	URL       string `json:"url"`
	Token     string `json:"token"`
	MonitorID string `json:"monitor_id"`
}

var (
	cfg     *Config
	cfgOnce sync.Once
	cfgLock sync.RWMutex
)

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	config.setDefaults()

	cfgLock.Lock()
	cfg = &config
	cfgLock.Unlock()

	return &config, nil
}

// Get 获取全局配置（线程安全）
func Get() *Config {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.Money == "" {
		c.Money = "花币"
	}
	if c.KKGiftDays == 0 {
		c.KKGiftDays = 30
	}
	if c.ActivityCheckDays == 0 {
		c.ActivityCheckDays = 21
	}
	if c.FreezeDays == 0 {
		c.FreezeDays = 5
	}
	if c.Database.Port == 0 {
		c.Database.Port = 3306
	}
	if c.Database.BackupMaxCount == 0 {
		c.Database.BackupMaxCount = 7
	}
	if c.API.Port == 0 {
		c.API.Port = 8838
	}
	if len(c.API.AllowOrigins) == 0 {
		c.API.AllowOrigins = []string{"*"}
	}
	if c.Open.CheckinLevel == "" {
		c.Open.CheckinLevel = "d"
	}
	if c.Open.InviteLevel == "" {
		c.Open.InviteLevel = "b"
	}
	if c.Ranks.Logo == "" {
		c.Ranks.Logo = "SAKURA"
	}
}

// IsAdmin 判断是否是管理员
func (c *Config) IsAdmin(userID int64) bool {
	if userID == c.Owner {
		return true
	}
	for _, admin := range c.Admins {
		if admin == userID {
			return true
		}
	}
	return false
}

// IsOwner 判断是否是 Owner
func (c *Config) IsOwner(userID int64) bool {
	return userID == c.Owner
}

// IsInGroup 判断群组是否在配置中
func (c *Config) IsInGroup(groupID int64) bool {
	for _, g := range c.Groups {
		if g == groupID {
			return true
		}
	}
	return false
}

// configPath 存储配置文件路径
var configPath string

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	return configPath
}

// SetConfigPath 设置配置文件路径
func SetConfigPath(path string) {
	configPath = path
}

// Reload 重新加载配置文件
func Reload() (*Config, error) {
	if configPath == "" {
		return nil, nil
	}
	return Load(configPath)
}

// Update 更新配置（热重载）
func Update(updateFn func(*Config)) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	if cfg == nil {
		return nil
	}

	// 执行更新函数
	updateFn(cfg)

	return nil
}

// SaveConfig 保存当前配置到文件
func SaveConfig() error {
	if configPath == "" {
		return nil
	}

	cfgLock.RLock()
	defer cfgLock.RUnlock()

	if cfg == nil {
		return nil
	}

	return cfg.Save(configPath)
}

// UpdateAndSave 更新配置并保存
func UpdateAndSave(updateFn func(*Config)) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	if cfg == nil {
		return nil
	}

	// 执行更新函数
	updateFn(cfg)

	// 保存到文件
	if configPath != "" {
		return cfg.Save(configPath)
	}

	return nil
}

// AddAdmin 添加管理员
func (c *Config) AddAdmin(userID int64) bool {
	for _, admin := range c.Admins {
		if admin == userID {
			return false // 已经是管理员
		}
	}
	c.Admins = append(c.Admins, userID)
	return true
}

// RemoveAdmin 移除管理员
func (c *Config) RemoveAdmin(userID int64) bool {
	for i, admin := range c.Admins {
		if admin == userID {
			c.Admins = append(c.Admins[:i], c.Admins[i+1:]...)
			return true
		}
	}
	return false
}

// SetOpenStatus 设置开放注册状态
func (c *Config) SetOpenStatus(status bool) {
	c.Open.Status = status
}

// SetCheckinStatus 设置签到功能状态
func (c *Config) SetCheckinStatus(status bool) {
	c.Open.Checkin = status
}

// SetMoviePilotStatus 设置 MoviePilot 功能状态
func (c *Config) SetMoviePilotStatus(status bool) {
	c.MoviePilot.Enabled = status
}

// SetRedEnvelopeStatus 设置红包功能状态
func (c *Config) SetRedEnvelopeStatus(status bool) {
	c.RedEnvelope.Enabled = status
}
