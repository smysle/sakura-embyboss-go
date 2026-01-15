# Sakura EmbyBoss Go

🌸 Telegram Bot for Emby Server Management - Go 重构版

[![Build](https://github.com/smysle/sakura-embyboss-go/actions/workflows/docker.yml/badge.svg)](https://github.com/smysle/sakura-embyboss-go/actions/workflows/docker.yml)

## ✨ 功能特性

- 🎫 **注册码系统** - 生成和管理注册码
- ✅ **签到系统** - 每日签到获取积分
- 🧧 **红包功能** - 发红包和抢红包
- 📊 **排行榜** - 自动生成播放排行榜图片
- 💾 **自动备份** - 定时备份数据库
- 👥 **用户管理** - 完整的用户生命周期管理

## 🚀 快速开始

### Docker 部署（推荐）

1. 克隆仓库：
```bash
git clone https://github.com/smysle/sakura-embyboss-go.git
cd sakura-embyboss-go
```

2. 创建配置文件：
```bash
cp configs/config.example.json config.json
# 编辑 config.json 填入你的配置
```

3. 启动服务：
```bash
docker-compose up -d
```

### 手动编译

```bash
# 下载依赖
go mod tidy

# 编译
go build -o embyboss ./cmd/bot

# 运行
./embyboss -config config.json
```

## ⚙️ 配置说明

```json
{
  "bot_token": "your_telegram_bot_token",
  "bot_name": "EmbyBot",
  "owner_id": 123456789,
  "group_id": -1001234567890,
  "admins": [123456789],
  "emby": {
    "url": "http://your-emby-server:8096",
    "api_key": "your_emby_api_key"
  },
  "db": {
    "host": "localhost",
    "port": 3306,
    "user": "emby",
    "password": "emby123",
    "database": "emby"
  }
}
```

## 📋 命令列表

### 用户命令
| 命令 | 说明 |
|------|------|
| `/start` | 开启用户面板 |
| `/myinfo` | 查看个人状态 |
| `/checkin` | 每日签到 |
| `/rank` | 查看排行榜 |
| `/red <金额> <个数>` | 发红包 |

### 管理员命令
| 命令 | 说明 |
|------|------|
| `/code <天数> [数量]` | 生成注册码 |
| `/kk <用户>` | 查看用户信息 |
| `/score <用户> <+/-积分>` | 调整积分 |
| `/renew <用户> <天数>` | 续期 |

### Owner 命令
| 命令 | 说明 |
|------|------|
| `/config` | 配置面板 |
| `/backup_db` | 手动备份数据库 |
| `/proadmin <用户ID>` | 添加管理员 |

## 🏗️ 项目结构

```
sakura-embyboss-go/
├── cmd/bot/           # 主程序入口
├── internal/
│   ├── bot/           # Telegram Bot
│   │   ├── handlers/  # 命令处理器
│   │   └── middleware/# 中间件
│   ├── config/        # 配置管理
│   ├── database/      # 数据库层
│   ├── emby/          # Emby API 客户端
│   ├── scheduler/     # 定时任务
│   ├── service/       # 业务逻辑
│   └── web/           # Web API
├── pkg/
│   ├── imggen/        # 图片生成
│   ├── logger/        # 日志
│   └── utils/         # 工具函数
└── configs/           # 配置文件示例
```

## 📝 License

MIT License
