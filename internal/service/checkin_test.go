// Package service 签到服务测试
package service

import (
	"testing"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
)

func TestCheckinService_hasCheckedInToday(t *testing.T) {
	svc := &CheckinService{}

	tests := []struct {
		name     string
		user     *models.Emby
		now      time.Time
		expected bool
	}{
		{
			name:     "从未签到",
			user:     &models.Emby{TG: 123, Ch: nil},
			now:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name: "今天签到过",
			user: &models.Emby{
				TG: 123,
				Ch: func() *time.Time {
					t := time.Date(2026, 1, 16, 8, 0, 0, 0, time.UTC)
					return &t
				}(),
			},
			now:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "昨天签到过",
			user: &models.Emby{
				TG: 123,
				Ch: func() *time.Time {
					t := time.Date(2026, 1, 15, 20, 0, 0, 0, time.UTC)
					return &t
				}(),
			},
			now:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.hasCheckedInToday(tt.user, tt.now)
			if result != tt.expected {
				t.Errorf("hasCheckedInToday() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckinService_calculateConsecutiveDays(t *testing.T) {
	svc := &CheckinService{}

	tests := []struct {
		name     string
		user     *models.Emby
		now      time.Time
		expected int
	}{
		{
			name:     "首次签到",
			user:     &models.Emby{TG: 123, Ch: nil, Ck: 0},
			now:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			expected: 1,
		},
		{
			name: "连续签到",
			user: &models.Emby{
				TG: 123,
				Ch: func() *time.Time {
					t := time.Date(2026, 1, 15, 20, 0, 0, 0, time.UTC) // 昨天
					return &t
				}(),
				Ck: 5,
			},
			now:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			expected: 6, // 5 + 1
		},
		{
			name: "断签后签到",
			user: &models.Emby{
				TG: 123,
				Ch: func() *time.Time {
					t := time.Date(2026, 1, 14, 20, 0, 0, 0, time.UTC) // 前天
					return &t
				}(),
				Ck: 10,
			},
			now:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			expected: 1, // 断签，重置为 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.calculateConsecutiveDays(tt.user, tt.now)
			if result != tt.expected {
				t.Errorf("calculateConsecutiveDays() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckinService_calculateReward(t *testing.T) {
	svc := &CheckinService{
		cfg: &config.Config{
			Open: config.OpenConfig{
				CheckinReward: []int{1, 10},
			},
		},
	}

	// 测试连续签到加成
	tests := []struct {
		consecutive int
		minExpected int
	}{
		{1, 1},   // 无加成
		{3, 3},   // +2 加成
		{7, 6},   // +5 加成
		{14, 11}, // +10 加成
		{30, 16}, // +15 加成
	}

	for _, tt := range tests {
		t.Run("连续签到加成", func(t *testing.T) {
			reward := svc.calculateReward(tt.consecutive)
			if reward < tt.minExpected {
				t.Errorf("calculateReward(%d) = %v, want >= %v", tt.consecutive, reward, tt.minExpected)
			}
		})
	}
}

func TestCheckinService_isLevelAllowed(t *testing.T) {
	svc := &CheckinService{
		cfg: &config.Config{
			Open: config.OpenConfig{
				CheckinLevel: "d",
			},
		},
	}

	// 封禁用户不能签到
	if svc.isLevelAllowed(models.LevelE) {
		t.Error("封禁用户不应该被允许签到")
	}
}
