// Package database 数据库初始化
package database

import (
	"fmt"
	"time"

	"github.com/smysle/sakura-embyboss-go/internal/config"
	"github.com/smysle/sakura-embyboss-go/internal/database/models"
	"github.com/smysle/sakura-embyboss-go/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)

	// 配置 GORM
	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层 sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接池失败: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	// 自动迁移表结构
	if err := autoMigrate(db); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	DB = db
	logger.Info().Msg("数据库连接成功")
	return nil
}

// autoMigrate 自动迁移表结构
func autoMigrate(db *gorm.DB) error {
	// 核心表 - 必须迁移
	coreTables := []interface{}{
		&models.Emby{},
		&models.Code{},
		&models.RedEnvelope{},
		&models.RedEnvelopeRecord{},
	}

	if err := db.AutoMigrate(coreTables...); err != nil {
		return err
	}

	// 可选表 - 如果已存在则跳过，不存在则创建
	optionalTables := []interface{}{
		&models.Favorites{},
		&models.RequestRecord{},
	}

	for _, table := range optionalTables {
		tableName := ""
		switch table.(type) {
		case *models.Favorites:
			tableName = "favorites"
		case *models.RequestRecord:
			tableName = "request_records"
		}

		// 检查表是否已存在
		if !db.Migrator().HasTable(tableName) {
			if err := db.AutoMigrate(table); err != nil {
				logger.Warn().Str("table", tableName).Err(err).Msg("可选表迁移失败，跳过")
			}
		} else {
			logger.Debug().Str("table", tableName).Msg("表已存在，跳过迁移")
		}
	}

	return nil
}

// Close 关闭数据库连接
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}
