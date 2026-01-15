// Package utils 工具函数
package utils

import (
	"crypto/rand"
	"math/big"
	"time"
)

const (
	// 密码字符集
	passwordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// GeneratePassword 生成随机密码
func GeneratePassword(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordChars))))
		if err != nil {
			return "", err
		}
		result[i] = passwordChars[num.Int64()]
	}
	return string(result), nil
}

// ConvertRuntime 转换运行时间（ticks -> 分钟）
func ConvertRuntime(ticks int64) string {
	if ticks <= 0 {
		return "数据缺失"
	}
	minutes := ticks / 600000000
	hours := minutes / 60
	mins := minutes % 60
	if hours > 0 {
		return formatDuration(time.Duration(minutes) * time.Minute)
	}
	return formatMinutes(mins)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return formatHoursMinutes(h, m)
	}
	return formatMinutes(int64(m))
}

func formatHoursMinutes(h, m int) string {
	if m > 0 {
		return formatInt(h) + "小时" + formatInt(m) + "分钟"
	}
	return formatInt(h) + "小时"
}

func formatMinutes(m int64) string {
	return formatInt64(m) + "分钟"
}

func formatInt(n int) string {
	return formatInt64(int64(n))
}

func formatInt64(n int64) string {
	return string(rune(n))
}

// FormatDuration 格式化时长显示
func FormatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return formatDays(days, hours, minutes)
	}
	if hours > 0 {
		return formatHoursMinutes(hours, minutes)
	}
	return formatMinutes(int64(minutes))
}

func formatDays(d, h, m int) string {
	result := formatInt(d) + "天"
	if h > 0 {
		result += formatInt(h) + "小时"
	}
	if m > 0 {
		result += formatInt(m) + "分钟"
	}
	return result
}

// TimeNowCST 获取当前北京时间
func TimeNowCST() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(loc)
}

// ParseTimeCST 解析时间字符串为北京时间
func ParseTimeCST(layout, value string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.ParseInLocation(layout, value, loc)
}

// FormatTimeCST 格式化时间为北京时间字符串
func FormatTimeCST(t time.Time, layout string) string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return t.In(loc).Format(layout)
}

// DaysBetween 计算两个时间之间的天数
func DaysBetween(a, b time.Time) int {
	if a.After(b) {
		a, b = b, a
	}
	return int(b.Sub(a).Hours() / 24)
}

// IsExpired 判断时间是否已过期
func IsExpired(expiryTime time.Time) bool {
	return time.Now().After(expiryTime)
}

// AddDays 增加天数
func AddDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}
