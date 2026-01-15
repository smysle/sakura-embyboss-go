// Package logger 日志模块
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

// Init 初始化日志
func Init(debug bool) {
	// 设置时区为 Asia/Shanghai
	loc, _ := time.LoadLocation("Asia/Shanghai")
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().In(loc)
	}

	// 控制台输出
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    false,
	}

	// 多输出：控制台 + 文件
	var writers []io.Writer
	writers = append(writers, consoleWriter)

	// 创建日志文件
	if err := os.MkdirAll("log", 0755); err == nil {
		logFile, err := os.OpenFile(
			"log/embyboss.log",
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err == nil {
			writers = append(writers, logFile)
		}
	}

	multi := zerolog.MultiLevelWriter(writers...)

	// 设置日志级别
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)

	Logger = zerolog.New(multi).With().Timestamp().Caller().Logger()
	log.Logger = Logger
}

// Debug 调试日志
func Debug() *zerolog.Event {
	return Logger.Debug()
}

// Info 信息日志
func Info() *zerolog.Event {
	return Logger.Info()
}

// Warn 警告日志
func Warn() *zerolog.Event {
	return Logger.Warn()
}

// Error 错误日志
func Error() *zerolog.Event {
	return Logger.Error()
}

// Fatal 致命错误日志
func Fatal() *zerolog.Event {
	return Logger.Fatal()
}
