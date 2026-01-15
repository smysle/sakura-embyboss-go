// Package service 数据库备份服务
package service

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
)

// BackupService 备份服务
type BackupService struct {
	cfg       *config.Config
	backupDir string
}

// BackupData 备份数据结构
type BackupData struct {
	Version   string           `json:"version"`
	CreatedAt time.Time        `json:"created_at"`
	Emby      []models.Emby    `json:"emby"`
	Codes     []models.Code   `json:"codes"`
	Envelopes []models.RedEnvelope `json:"red_envelopes"`
}

// BackupResult 备份结果
type BackupResult struct {
	Filename  string
	FilePath  string
	Size      int64
	Duration  time.Duration
	Records   int
	Compressed bool
}

// NewBackupService 创建备份服务
func NewBackupService() *BackupService {
	cfg := config.Get()
	backupDir := cfg.Database.BackupDir
	if backupDir == "" {
		backupDir = "./backups"
	}

	// 确保备份目录存在
	os.MkdirAll(backupDir, 0755)

	return &BackupService{
		cfg:       cfg,
		backupDir: backupDir,
	}
}

// Backup 执行备份
func (s *BackupService) Backup(compress bool) (*BackupResult, error) {
	startTime := time.Now()
	db := database.GetDB()

	// 收集数据
	var data BackupData
	data.Version = "1.0"
	data.CreatedAt = time.Now()

	// 备份 Emby 用户
	if err := db.Find(&data.Emby).Error; err != nil {
		return nil, fmt.Errorf("备份 Emby 用户失败: %w", err)
	}

	// 备份注册码
	if err := db.Find(&data.Codes).Error; err != nil {
		return nil, fmt.Errorf("备份注册码失败: %w", err)
	}

	// 备份红包
	if err := db.Find(&data.Envelopes).Error; err != nil {
		return nil, fmt.Errorf("备份红包失败: %w", err)
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	if compress {
		filename = fmt.Sprintf("backup_%s.json.gz", timestamp)
	} else {
		filename = fmt.Sprintf("backup_%s.json", timestamp)
	}
	filePath := filepath.Join(s.backupDir, filename)

	// 序列化
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}

	// 写入文件
	var fileSize int64
	if compress {
		fileSize, err = s.writeCompressed(filePath, jsonData)
	} else {
		fileSize, err = s.writeRaw(filePath, jsonData)
	}
	if err != nil {
		return nil, err
	}

	totalRecords := len(data.Emby) + len(data.Codes) + len(data.Envelopes)

	logger.Info().
		Str("file", filename).
		Int64("size", fileSize).
		Int("records", totalRecords).
		Msg("数据库备份完成")

	return &BackupResult{
		Filename:   filename,
		FilePath:   filePath,
		Size:       fileSize,
		Duration:   time.Since(startTime),
		Records:    totalRecords,
		Compressed: compress,
	}, nil
}

// writeRaw 写入原始 JSON
func (s *BackupService) writeRaw(path string, data []byte) (int64, error) {
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, fmt.Errorf("写入文件失败: %w", err)
	}
	info, _ := os.Stat(path)
	return info.Size(), nil
}

// writeCompressed 写入压缩文件
func (s *BackupService) writeCompressed(path string, data []byte) (int64, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	if _, err := gz.Write(data); err != nil {
		return 0, fmt.Errorf("压缩写入失败: %w", err)
	}

	gz.Close()
	file.Close()

	info, _ := os.Stat(path)
	return info.Size(), nil
}

// Restore 从备份恢复
func (s *BackupService) Restore(filePath string) error {
	// 读取文件
	var data []byte
	var err error

	if filepath.Ext(filePath) == ".gz" {
		data, err = s.readCompressed(filePath)
	} else {
		data, err = os.ReadFile(filePath)
	}
	if err != nil {
		return fmt.Errorf("读取备份文件失败: %w", err)
	}

	// 解析
	var backupData BackupData
	if err := json.Unmarshal(data, &backupData); err != nil {
		return fmt.Errorf("解析备份数据失败: %w", err)
	}

	db := database.GetDB()

	// 恢复 Emby 用户
	for _, emby := range backupData.Emby {
		if err := db.Save(&emby).Error; err != nil {
			logger.Warn().Err(err).Int64("tg", emby.TG).Msg("恢复用户失败")
		}
	}

	// 恢复注册码
	for _, code := range backupData.Codes {
		if err := db.Save(&code).Error; err != nil {
			logger.Warn().Err(err).Str("code", code.Code).Msg("恢复注册码失败")
		}
	}

	logger.Info().
		Int("emby", len(backupData.Emby)).
		Int("codes", len(backupData.Codes)).
		Msg("数据库恢复完成")

	return nil
}

// readCompressed 读取压缩文件
func (s *BackupService) readCompressed(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

// ListBackups 列出所有备份
func (s *BackupService) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:  entry.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	// 按时间倒序
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// BackupInfo 备份信息
type BackupInfo struct {
	Filename  string
	Size      int64
	CreatedAt time.Time
}

// CleanOldBackups 清理旧备份
func (s *BackupService) CleanOldBackups(keepDays int) (int, error) {
	if keepDays <= 0 {
		keepDays = 7 // 默认保留 7 天
	}

	backups, err := s.ListBackups()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays)
	deleted := 0

	for _, backup := range backups {
		if backup.CreatedAt.Before(cutoff) {
			filePath := filepath.Join(s.backupDir, backup.Filename)
			if err := os.Remove(filePath); err != nil {
				logger.Warn().Err(err).Str("file", backup.Filename).Msg("删除旧备份失败")
			} else {
				deleted++
				logger.Debug().Str("file", backup.Filename).Msg("已删除旧备份")
			}
		}
	}

	return deleted, nil
}

// GetLatestBackup 获取最新备份
func (s *BackupService) GetLatestBackup() (*BackupInfo, error) {
	backups, err := s.ListBackups()
	if err != nil {
		return nil, err
	}
	if len(backups) == 0 {
		return nil, nil
	}
	return &backups[0], nil
}

// GetBackupFilePath 获取备份文件完整路径
func (s *BackupService) GetBackupFilePath(filename string) string {
	return filepath.Join(s.backupDir, filename)
}

// FormatSize 格式化文件大小
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
