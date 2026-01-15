# 构建阶段
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/embyboss ./cmd/bot

# 运行阶段
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 从构建阶段复制二进制文件
COPY --from=builder /app/embyboss /app/embyboss

# 复制资源文件
COPY --from=builder /app/configs /app/configs

# 创建必要的目录
RUN mkdir -p /app/log /app/db_backup

# 设置权限
RUN chmod +x /app/embyboss

# 暴露端口
EXPOSE 8838

# 启动命令
CMD ["/app/embyboss", "-config", "/app/config.json"]
