# Smarticky Notes - 环境变量配置

## 数据目录配置

Smarticky 支持通过环境变量配置数据存储目录，方便 Docker 部署和数据持久化。

### 环境变量

- `SMARTICKY_DATA_DIR`: 数据目录路径（默认: `./data`）

### 默认目录结构

```
data/
├── smarticky.db          # SQLite 数据库
└── uploads/
    ├── avatars/          # 用户头像
    └── attachments/      # 便签附件
```

### 使用方法

#### 1. 本地运行（默认配置）

```bash
./smarticky.exe
```

数据将存储在程序目录下的 `data/` 文件夹。

#### 2. 自定义数据目录

```bash
# Windows
set SMARTICKY_DATA_DIR=D:\my-notes-data
smarticky.exe

# Linux/Mac
export SMARTICKY_DATA_DIR=/var/lib/smarticky
./smarticky
```

#### 3. Docker 部署

**Dockerfile 示例:**

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o smarticky ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/smarticky .
COPY --from=builder /app/web ./web

# 创建数据目录
RUN mkdir -p /data

# 设置环境变量
ENV SMARTICKY_DATA_DIR=/data

EXPOSE 8080

CMD ["./smarticky"]
```

**docker-compose.yml 示例:**

```yaml
version: '3.8'

services:
  smarticky:
    build: .
    ports:
      - "8080:8080"
    environment:
      - SMARTICKY_DATA_DIR=/data
    volumes:
      - smarticky-data:/data
    restart: unless-stopped

volumes:
  smarticky-data:
```

#### 4. Docker 运行命令

```bash
# 使用 volume 持久化数据
docker run -d \
  --name smarticky \
  -p 8080:8080 \
  -e SMARTICKY_DATA_DIR=/data \
  -v smarticky-data:/data \
  smarticky:latest

# 使用宿主机目录
docker run -d \
  --name smarticky \
  -p 8080:8080 \
  -e SMARTICKY_DATA_DIR=/data \
  -v /path/on/host:/data \
  smarticky:latest
```

### 数据备份

备份数据时，只需要备份整个数据目录：

```bash
# 备份
tar -czf smarticky-backup-$(date +%Y%m%d).tar.gz data/

# 恢复
tar -xzf smarticky-backup-20231128.tar.gz
```

### 迁移数据

1. 停止 Smarticky 服务
2. 复制 `data/` 目录到新位置
3. 设置 `SMARTICKY_DATA_DIR` 环境变量指向新位置
4. 启动 Smarticky 服务

### 注意事项

- 确保数据目录有足够的磁盘空间
- 数据目录需要有读写权限
- Docker 部署时建议使用 volume 而不是 bind mount 以获得更好的性能
- 定期备份数据目录
