// Package models 数据模型测试
package models

import (
	"testing"
	"time"
)

func TestEmby_HasEmbyAccount(t *testing.T) {
	tests := []struct {
		name     string
		embyID   *string
		expected bool
	}{
		{"有 Emby ID", strPtr("abc123"), true},
		{"空字符串", strPtr(""), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emby{EmbyID: tt.embyID}
			if got := e.HasEmbyAccount(); got != tt.expected {
				t.Errorf("HasEmbyAccount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEmby_IsExpired(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	future := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name     string
		ex       *time.Time
		expected bool
	}{
		{"已过期", &past, true},
		{"未过期", &future, false},
		{"未设置过期时间", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emby{Ex: tt.ex}
			if got := e.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEmby_IsBanned(t *testing.T) {
	tests := []struct {
		name     string
		level    UserLevel
		expected bool
	}{
		{"封禁用户", LevelE, true},
		{"普通用户", LevelD, false},
		{"白名单用户", LevelA, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emby{Lv: tt.level}
			if got := e.IsBanned(); got != tt.expected {
				t.Errorf("IsBanned() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEmby_IsWhitelist(t *testing.T) {
	tests := []struct {
		name     string
		level    UserLevel
		expected bool
	}{
		{"白名单用户", LevelA, true},
		{"普通用户", LevelD, false},
		{"封禁用户", LevelE, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emby{Lv: tt.level}
			if got := e.IsWhitelist(); got != tt.expected {
				t.Errorf("IsWhitelist() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEmby_GetLevelName(t *testing.T) {
	tests := []struct {
		level    UserLevel
		contains string
	}{
		{LevelA, "白名单"},
		{LevelB, "高级"},
		{LevelC, "普通"},
		{LevelD, "基础"},
		{LevelE, "封禁"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			e := &Emby{Lv: tt.level}
			name := e.GetLevelName()
			if len(name) == 0 {
				t.Error("GetLevelName() 返回空字符串")
			}
		})
	}
}

func TestEmby_DaysUntilExpiry(t *testing.T) {
	future7 := time.Now().Add(7 * 24 * time.Hour)
	past3 := time.Now().Add(-3 * 24 * time.Hour)

	tests := []struct {
		name      string
		ex        *time.Time
		minExpect int
		maxExpect int
	}{
		{"7天后过期", &future7, 6, 7},
		{"3天前过期", &past3, -4, -3},
		{"未设置", nil, -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emby{Ex: tt.ex}
			days := e.DaysUntilExpiry()
			if days < tt.minExpect || days > tt.maxExpect {
				t.Errorf("DaysUntilExpiry() = %v, want between %v and %v", days, tt.minExpect, tt.maxExpect)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
