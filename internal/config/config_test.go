// Package config 配置模块测试
package config

import (
	"testing"
)

func TestConfig_IsAdmin(t *testing.T) {
	cfg := &Config{
		Owner:  12345,
		Admins: []int64{11111, 22222},
	}

	tests := []struct {
		name     string
		userID   int64
		expected bool
	}{
		{"Owner 是管理员", 12345, true},
		{"Admin 是管理员", 11111, true},
		{"Admin2 是管理员", 22222, true},
		{"普通用户不是管理员", 99999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.IsAdmin(tt.userID); got != tt.expected {
				t.Errorf("IsAdmin(%d) = %v, want %v", tt.userID, got, tt.expected)
			}
		})
	}
}

func TestConfig_IsOwner(t *testing.T) {
	cfg := &Config{
		Owner: 12345,
	}

	if !cfg.IsOwner(12345) {
		t.Error("IsOwner(12345) 应该返回 true")
	}

	if cfg.IsOwner(99999) {
		t.Error("IsOwner(99999) 应该返回 false")
	}
}

func TestConfig_AddAdmin(t *testing.T) {
	cfg := &Config{
		Admins: []int64{11111},
	}

	// 添加新管理员
	if !cfg.AddAdmin(22222) {
		t.Error("AddAdmin(22222) 应该返回 true")
	}

	if len(cfg.Admins) != 2 {
		t.Errorf("管理员数量应该是 2，实际是 %d", len(cfg.Admins))
	}

	// 重复添加
	if cfg.AddAdmin(22222) {
		t.Error("AddAdmin(22222) 重复添加应该返回 false")
	}
}

func TestConfig_RemoveAdmin(t *testing.T) {
	cfg := &Config{
		Admins: []int64{11111, 22222, 33333},
	}

	// 移除存在的管理员
	if !cfg.RemoveAdmin(22222) {
		t.Error("RemoveAdmin(22222) 应该返回 true")
	}

	if len(cfg.Admins) != 2 {
		t.Errorf("管理员数量应该是 2，实际是 %d", len(cfg.Admins))
	}

	// 检查顺序
	if cfg.Admins[0] != 11111 || cfg.Admins[1] != 33333 {
		t.Error("移除后管理员列表不正确")
	}

	// 移除不存在的管理员
	if cfg.RemoveAdmin(99999) {
		t.Error("RemoveAdmin(99999) 应该返回 false")
	}
}

func TestConfig_IsInGroup(t *testing.T) {
	cfg := &Config{
		Groups: []int64{-100001, -100002},
	}

	if !cfg.IsInGroup(-100001) {
		t.Error("IsInGroup(-100001) 应该返回 true")
	}

	if cfg.IsInGroup(-100099) {
		t.Error("IsInGroup(-100099) 应该返回 false")
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()

	if cfg.Money != "花币" {
		t.Errorf("默认 Money 应该是 '花币'，实际是 '%s'", cfg.Money)
	}

	if cfg.KKGiftDays != 30 {
		t.Errorf("默认 KKGiftDays 应该是 30，实际是 %d", cfg.KKGiftDays)
	}

	if cfg.ActivityCheckDays != 21 {
		t.Errorf("默认 ActivityCheckDays 应该是 21，实际是 %d", cfg.ActivityCheckDays)
	}

	if cfg.FreezeDays != 5 {
		t.Errorf("默认 FreezeDays 应该是 5，实际是 %d", cfg.FreezeDays)
	}

	if cfg.Database.Port != 3306 {
		t.Errorf("默认数据库端口应该是 3306，实际是 %d", cfg.Database.Port)
	}

	if cfg.API.Port != 8838 {
		t.Errorf("默认 API 端口应该是 8838，实际是 %d", cfg.API.Port)
	}
}
