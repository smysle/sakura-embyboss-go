// Package emby Emby 工具函数测试
package emby

import (
	"testing"
)

func TestFormatWatchTime(t *testing.T) {
	tests := []struct {
		seconds  int64
		expected string
	}{
		{0, "0分钟"},
		{30, "0分钟"},
		{60, "1分钟"},
		{90, "1分钟"},
		{3600, "1小时0分钟"},
		{3660, "1小时1分钟"},
		{7200, "2小时0分钟"},
		{7320, "2小时2分钟"},
		{86400, "24小时0分钟"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatWatchTime(tt.seconds)
			if got != tt.expected {
				t.Errorf("FormatWatchTime(%d) = %v, want %v", tt.seconds, got, tt.expected)
			}
		})
	}
}

func TestSanitizeSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"it's", "it''s"},
		{"test; DROP TABLE", "test DROP TABLE"},
		{"test--comment", "testcomment"},
		{"test/*comment*/", "testcomment"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeSQL(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeSQL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
