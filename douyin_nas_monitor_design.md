# 抖音指定用户视频更新自动下载器设计文档

> 项目名称建议：`douyin-nas-monitor`  
> 适用环境：飞牛 NAS、绿联 NAS、通用 Linux、Docker / Docker Compose  
> 技术方向：Go + yt-dlp + SQLite + Docker Compose  
> 文档版本：v1.0

---

## 1. 项目背景

用户希望在自己的 NAS 上部署一个自动化工具，用于监控指定的抖音用户主页。当指定用户发布新视频时，程序自动检测更新，并按照预设分辨率下载视频到 NAS 的指定目录。

本项目面向个人本地自动化备份场景，重点支持：

- 使用自己的抖音账号 Cookie。
- 指定用户主页监控。
- 每个用户单独设置下载分辨率。
- 自动跳过已下载视频。
- 支持飞牛 NAS / 绿联 NAS 的 Docker 环境部署。
- 支持 NAS 定时任务或容器常驻运行。

---

## 2. 合规边界

本项目必须遵守以下边界：

1. 只下载用户自己账号可正常访问、且有权保存的视频。
2. 不支持下载私密视频、付费内容、无权限内容。
3. 不绕过登录、验证码、风控、地区限制或平台权限限制。
4. 不逆向抖音 App 签名。
5. 不开发破解接口、无水印解析接口或批量搬运工具。
6. 下载功能优先交给 `yt-dlp`，本项目只做本地调度、配置、记录和自动化。
7. Cookie 由用户自行导出并保存在本地 `cookies.txt` 中，程序不得把 Cookie 输出到日志。

---

## 3. 项目目标

### 3.1 核心目标

实现一个可以部署在 NAS 上的自动化程序：

```text
读取配置文件
    ↓
加载 Cookie
    ↓
遍历指定抖音用户主页
    ↓
调用 yt-dlp 检测并下载新视频
    ↓
按用户设置的分辨率保存
    ↓
写入 archive 和 SQLite
    ↓
输出日志 / 可选通知
```

### 3.2 支持的运行环境

项目必须兼容：

- 飞牛 NAS Docker 环境
- 绿联 NAS Docker 环境
- 通用 Linux 服务器
- Docker Compose
- NAS 系统自带计划任务
- 命令行手动执行

### 3.3 不写死 NAS 路径

不要使用固定路径，例如：

```text
/volume1/docker/xxx
/volume1/video/xxx
```

所有宿主机路径都通过 `docker-compose.yaml` 配置。

容器内部统一使用：

```text
/app/config.yaml
/app/cookies.txt
/app/data
/app/logs
/downloads
```

---

## 4. 功能范围

### 4.1 MVP 功能

第一版必须实现：

1. 读取 `config.yaml`。
2. 读取本地 `cookies.txt`。
3. 指定多个抖音用户主页。
4. 每个用户支持单独设置分辨率：
   - `best`
   - `1080`
   - `720`
   - `480`
5. 调用 `yt-dlp` 下载视频。
6. 使用 `--download-archive` 避免重复下载。
7. 使用 SQLite 记录下载历史。
8. 支持单次运行模式。
9. 支持常驻运行模式。
10. 支持日志文件。
11. 支持失败重试。
12. 支持用户之间下载间隔。
13. 支持 Docker / Docker Compose 部署。

### 4.2 第二阶段功能

后续可以扩展：

1. Web 管理页面。
2. 企业微信 / 钉钉 / Bark / Telegram 通知。
3. Cookie 失效提醒。
4. 下载失败重试队列。
5. 下载完成后自动整理目录。
6. 视频元数据保存。
7. 按用户分目录。
8. 按日期分目录。
9. 关注列表同步。

### 4.3 暂不实现

第一版不实现：

1. 自动登录抖音。
2. 自动绕过验证码。
3. 自动绕过风控。
4. 解析私密内容。
5. 抖音 App 协议逆向。
6. 无水印破解下载。
7. 批量采集全站内容。

---

## 5. 总体架构

```text
+-------------------------+
|       config.yaml       |
+-----------+-------------+
            |
            v
+-------------------------+
|      Go 主程序          |
|  douyin-nas-monitor     |
+-----------+-------------+
            |
            v
+-------------------------+
|      用户监控模块        |
+-----------+-------------+
            |
            v
+-------------------------+
|     yt-dlp 调用模块      |
+-----------+-------------+
            |
            v
+-------------------------+
|   视频下载 / archive     |
+-----------+-------------+
            |
            v
+-------------------------+
| SQLite 下载记录数据库    |
+-----------+-------------+
            |
            v
+-------------------------+
| 日志 / 可选 Webhook 通知 |
+-------------------------+
```

---

## 6. 项目目录结构

```text
douyin-nas-monitor/
├── cmd/
│   └── douyin-nas-monitor/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── downloader/
│   │   └── ytdlp.go
│   ├── monitor/
│   │   └── monitor.go
│   ├── storage/
│   │   └── sqlite.go
│   ├── notify/
│   │   └── webhook.go
│   └── logger/
│       └── logger.go
├── config.yaml
├── cookies.txt.example
├── Dockerfile
├── docker-compose.yaml
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

---

## 7. 配置文件设计

### 7.1 config.yaml 示例

```yaml
app:
  mode: once
  interval_minutes: 120
  sleep_between_users_seconds: 30
  log_file: /app/logs/douyin-monitor.log
  database: /app/data/douyin-monitor.db
  cookies_file: /app/cookies.txt
  archive_file: /app/data/archive.txt
  default_save_dir: /downloads
  yt_dlp_path: yt-dlp
  timeout_seconds: 1800

下载:
  merge_output_format: mp4
  output_template: "%(uploader)s/%(upload_date)s-%(title).80s-%(id)s.%(ext)s"
  retries: 3

users:
  - name: 用户A
    url: "https://www.douyin.com/user/xxxxxxxx"
    enabled: true
    quality: 1080
    save_dir: ""

  - name: 用户B
    url: "https://www.douyin.com/user/yyyyyyyy"
    enabled: true
    quality: 720
    save_dir: ""

notify:
  enabled: false
  type: generic
  webhook_url: ""
```

> 注意：上方 `下载` 字段如果实际开发时为了兼容 YAML 和 Go 结构体，建议改为英文 `download`。

### 7.2 推荐正式字段

正式开发建议使用英文键名：

```yaml
app:
  mode: once
  interval_minutes: 120
  sleep_between_users_seconds: 30
  log_file: /app/logs/douyin-monitor.log
  database: /app/data/douyin-monitor.db
  cookies_file: /app/cookies.txt
  archive_file: /app/data/archive.txt
  default_save_dir: /downloads
  yt_dlp_path: yt-dlp
  timeout_seconds: 1800

download:
  merge_output_format: mp4
  output_template: "%(uploader)s/%(upload_date)s-%(title).80s-%(id)s.%(ext)s"
  retries: 3

users:
  - name: 用户A
    url: "https://www.douyin.com/user/xxxxxxxx"
    enabled: true
    quality: 1080
    save_dir: ""

  - name: 用户B
    url: "https://www.douyin.com/user/yyyyyyyy"
    enabled: true
    quality: 720
    save_dir: ""

notify:
  enabled: false
  type: generic
  webhook_url: ""
```

---

## 8. 分辨率规则设计

### 8.1 quality 配置

支持以下值：

| quality | 含义 |
|---|---|
| best | 下载可用的最高画质 |
| 1080 | 下载 1080P 以内最高画质 |
| 720 | 下载 720P 以内最高画质 |
| 480 | 下载 480P 以内最高画质 |

### 8.2 yt-dlp format 映射

```text
best = bestvideo+bestaudio/best
1080 = bv*[height<=1080]+ba/b[height<=1080]/best
720  = bv*[height<=720]+ba/b[height<=720]/best
480  = bv*[height<=480]+ba/b[height<=480]/best
```

### 8.3 降级规则

如果某个视频没有 1080P，使用：

```text
bv*[height<=1080]+ba/b[height<=1080]/best
```

它会优先选择 1080P 以内最高清晰度，如果没有合适格式，则回退到 `best`。

---

## 9. yt-dlp 调用设计

### 9.1 基础命令

```bash
yt-dlp \
  --cookies /app/cookies.txt \
  --download-archive /app/data/archive.txt \
  --no-overwrites \
  --continue \
  --merge-output-format mp4 \
  -f "bv*[height<=1080]+ba/b[height<=1080]/best" \
  -o "%(uploader)s/%(upload_date)s-%(title).80s-%(id)s.%(ext)s" \
  -P "/downloads" \
  "https://www.douyin.com/user/xxxxxxxx"
```

### 9.2 参数说明

| 参数 | 作用 |
|---|---|
| `--cookies` | 使用用户自己的 Cookie |
| `--download-archive` | 记录已下载内容，避免重复下载 |
| `--no-overwrites` | 不覆盖已有文件 |
| `--continue` | 断点续传 |
| `--merge-output-format mp4` | 合并为 mp4 |
| `-f` | 指定分辨率格式 |
| `-o` | 指定文件命名模板 |
| `-P` | 指定保存目录 |

### 9.3 Go 调用要求

Go 程序使用 `os/exec` 调用 `yt-dlp`：

1. 使用 `context.WithTimeout` 控制超时。
2. 捕获 `stdout` 和 `stderr`。
3. 日志记录执行结果。
4. 不输出 Cookie 内容。
5. 失败时返回错误，不影响其他用户继续执行。
6. 支持重试。

---

## 10. SQLite 数据库设计

### 10.1 表名

```text
downloads
```

### 10.2 表结构

```sql
CREATE TABLE IF NOT EXISTS downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT,
    user_url TEXT,
    video_id TEXT UNIQUE,
    title TEXT,
    file_path TEXT,
    quality TEXT,
    status TEXT,
    error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 10.3 status 状态

| status | 含义 |
|---|---|
| pending | 等待下载 |
| success | 下载成功 |
| failed | 下载失败 |
| skipped | 已存在或已下载 |

### 10.4 去重策略

第一层去重：

```text
yt-dlp --download-archive archive.txt
```

第二层记录：

```text
SQLite downloads 表
```

实际去重以 `archive.txt` 为主，SQLite 用于状态查询、日志分析和后续扩展。

---

## 11. 运行模式设计

### 11.1 单次运行模式

适合 NAS 定时任务。

```bash
douyin-nas-monitor --config /app/config.yaml --once
```

执行流程：

```text
启动程序
  ↓
读取配置
  ↓
检查依赖
  ↓
遍历用户
  ↓
下载新视频
  ↓
退出程序
```

### 11.2 常驻运行模式

适合 Docker 容器长期运行。

```bash
douyin-nas-monitor --config /app/config.yaml --daemon
```

执行流程：

```text
启动程序
  ↓
每隔 interval_minutes 执行一次检测
  ↓
循环运行
```

### 11.3 配置检查模式

```bash
douyin-nas-monitor --config /app/config.yaml --check
```

检查内容：

1. 配置文件是否存在。
2. Cookie 文件是否存在。
3. 数据目录是否可写。
4. 日志目录是否可写。
5. `yt-dlp` 是否可用。
6. `ffmpeg` 是否可用。
7. 用户配置是否有效。

---

## 12. 日志设计

### 12.1 日志文件

默认路径：

```text
/app/logs/douyin-monitor.log
```

### 12.2 日志内容

需要记录：

```text
程序启动
配置加载成功
Cookie 文件检测成功
yt-dlp 检测成功
开始处理用户：用户A
用户主页 URL
用户分辨率设置
下载开始
下载完成
无新视频
下载失败
重试次数
程序结束
```

### 12.3 日志安全要求

不得记录：

```text
cookies.txt 内容
完整 Cookie 字符串
敏感请求头
用户账号敏感信息
```

---

## 13. 通知模块设计

### 13.1 通知开关

```yaml
notify:
  enabled: false
  type: generic
  webhook_url: ""
```

### 13.2 通知场景

可选通知：

1. 下载成功。
2. 下载失败。
3. Cookie 疑似失效。
4. yt-dlp 不存在。
5. ffmpeg 不存在。
6. 所有用户检测完成。

### 13.3 通用 Webhook Payload

```json
{
  "title": "抖音视频下载完成",
  "user": "用户A",
  "quality": "1080",
  "status": "success",
  "file_path": "/downloads/用户A/20260513-标题-id.mp4",
  "time": "2026-05-13 10:30:00"
}
```

---

## 14. Dockerfile 设计

### 14.1 构建要求

1. 使用 Go 多阶段构建。
2. 运行镜像安装：
   - `yt-dlp`
   - `ffmpeg`
   - `ca-certificates`
   - `python3`
   - `pip`
3. 默认启动单次运行模式。

### 14.2 Dockerfile 示例

```dockerfile
FROM golang:1.22 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/douyin-nas-monitor ./cmd/douyin-nas-monitor

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    python3 \
    python3-pip \
    ffmpeg \
    sqlite3 \
    && pip3 install --break-system-packages -U "yt-dlp[default]" \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/douyin-nas-monitor /usr/local/bin/douyin-nas-monitor

CMD ["douyin-nas-monitor", "--config", "/app/config.yaml", "--once"]
```

---

## 15. Docker Compose 设计

### 15.1 通用 docker-compose.yaml

```yaml
services:
  douyin-monitor:
    build: .
    container_name: douyin-monitor
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./cookies.txt:/app/cookies.txt
      - ./data:/app/data
      - ./logs:/app/logs
      - /mnt/media/douyin:/downloads
    command: douyin-nas-monitor --config /app/config.yaml --once
    restart: "no"
```

### 15.2 飞牛 / 绿联 NAS 路径说明

`/mnt/media/douyin` 只是示例路径。

在飞牛 NAS 或绿联 NAS 上，需要改成自己的实际媒体目录，例如：

```yaml
volumes:
  - ./config.yaml:/app/config.yaml
  - ./cookies.txt:/app/cookies.txt
  - ./data:/app/data
  - ./logs:/app/logs
  - /你的NAS媒体目录/douyin:/downloads
```

### 15.3 推荐宿主机目录结构

```text
douyin-monitor/
├── docker-compose.yaml
├── config.yaml
├── cookies.txt
├── data/
│   ├── archive.txt
│   └── douyin-monitor.db
└── logs/
    └── douyin-monitor.log
```

---

## 16. NAS 定时任务设计

### 16.1 单次运行命令

适合飞牛 NAS / 绿联 NAS 的计划任务：

```bash
cd /你的NAS路径/douyin-monitor && docker compose run --rm douyin-monitor
```

或者：

```bash
docker compose -f /你的NAS路径/douyin-monitor/docker-compose.yaml run --rm douyin-monitor
```

### 16.2 推荐执行频率

| 场景 | 推荐频率 |
|---|---|
| 少量用户 | 每 1-2 小时一次 |
| 普通用户 | 每 2-6 小时一次 |
| 用户较多 | 每 6-12 小时一次 |
| 大量用户 | 不建议高频检测 |

不建议每分钟检测，避免触发平台风控。

### 16.3 常驻运行方式

如果希望容器一直运行：

```yaml
command: douyin-nas-monitor --config /app/config.yaml --daemon
restart: unless-stopped
```

---

## 17. 错误处理设计

### 17.1 Cookie 不存在

错误提示：

```text
cookies.txt 不存在，请先导出自己的抖音账号 Cookie，并保存到 /app/cookies.txt
```

### 17.2 Cookie 失效

常见表现：

```text
需要 fresh cookies
登录状态失效
无法访问用户主页
```

处理方式：

1. 日志记录 Cookie 可能失效。
2. 可选发送通知。
3. 不自动登录。
4. 用户手动更新 cookies.txt。

### 17.3 yt-dlp 不存在

错误提示：

```text
yt-dlp 未安装或不可执行，请检查镜像构建或手动安装 yt-dlp
```

### 17.4 ffmpeg 不存在

错误提示：

```text
ffmpeg 未安装，可能无法合并音视频，请安装 ffmpeg
```

### 17.5 某个用户下载失败

处理方式：

1. 记录错误。
2. 按配置重试。
3. 失败后继续处理下一个用户。
4. 不让单个用户失败影响全局任务。

---

## 18. 安全设计

### 18.1 Cookie 安全

要求：

1. `cookies.txt` 不提交 Git。
2. `.gitignore` 必须包含：

```gitignore
cookies.txt
data/
logs/
*.db
archive.txt
```

3. 日志不得输出 Cookie 内容。
4. Docker 镜像内不内置 Cookie。
5. Cookie 只通过 volume 挂载进入容器。

### 18.2 访问频率控制

要求：

1. 用户之间增加 sleep。
2. 支持配置检测间隔。
3. 支持失败重试间隔。
4. 不做高频刷接口行为。

---

## 19. 命令行参数设计

```bash
douyin-nas-monitor --config /app/config.yaml --once
```

支持参数：

| 参数 | 说明 |
|---|---|
| `--config` | 指定配置文件路径 |
| `--once` | 单次运行 |
| `--daemon` | 常驻运行 |
| `--check` | 检查配置和依赖 |
| `--version` | 输出版本信息 |

默认行为：

```text
如果没有指定 --daemon，默认按 --once 执行。
```

---

## 20. 下载文件命名设计

默认模板：

```text
%(uploader)s/%(upload_date)s-%(title).80s-%(id)s.%(ext)s
```

保存效果：

```text
/downloads/
└── 作者名/
    ├── 20260513-视频标题-738xxxx.mp4
    └── 20260512-视频标题-737xxxx.mp4
```

优点：

1. 按作者自动分目录。
2. 文件名包含日期。
3. 文件名包含标题。
4. 文件名包含视频 ID，方便去重和排查。

---

## 21. 开发模块说明

### 21.1 config 模块

负责：

1. 读取 YAML。
2. 校验字段。
3. 提供默认值。
4. 检查用户配置。

### 21.2 downloader 模块

负责：

1. 构造 yt-dlp 命令。
2. 映射分辨率参数。
3. 执行下载。
4. 超时控制。
5. 重试控制。
6. 返回执行结果。

### 21.3 monitor 模块

负责：

1. 遍历用户列表。
2. 判断用户是否启用。
3. 调用 downloader。
4. 控制用户之间的 sleep。
5. 汇总执行结果。

### 21.4 storage 模块

负责：

1. 初始化 SQLite。
2. 创建 downloads 表。
3. 写入下载记录。
4. 更新下载状态。
5. 查询历史记录。

### 21.5 notify 模块

负责：

1. 发送通用 Webhook。
2. 后续扩展企业微信、钉钉、Bark。

### 21.6 logger 模块

负责：

1. 输出控制台日志。
2. 写入日志文件。
3. 控制日志级别。
4. 过滤敏感信息。

---

## 22. README 需要包含的内容

README.md 必须包含：

1. 项目介绍。
2. 合规说明。
3. 功能列表。
4. 目录结构。
5. 如何准备 `cookies.txt`。
6. 如何编辑 `config.yaml`。
7. 如何设置不同用户不同分辨率。
8. 如何 Docker Compose 部署。
9. 飞牛 NAS 部署示例。
10. 绿联 NAS 部署示例。
11. NAS 定时任务命令。
12. 常驻运行方式。
13. 如何查看日志。
14. 如何更新 yt-dlp。
15. 常见问题。

---

## 23. 常见问题设计

### 23.1 没有下载新视频

可能原因：

1. 没有新视频。
2. 视频已在 archive.txt 中。
3. Cookie 失效。
4. 用户主页无法访问。
5. yt-dlp 暂时不支持当前页面结构。

### 23.2 1080P 下载不到

可能原因：

1. 原视频没有 1080P。
2. 当前账号只能访问较低清晰度。
3. yt-dlp 获取到的格式有限。

程序会根据 format 规则自动降级。

### 23.3 Cookie 失效怎么办

处理方式：

1. 重新从浏览器导出 Cookie。
2. 覆盖 NAS 上的 `cookies.txt`。
3. 重新执行容器。

### 23.4 如何更新 yt-dlp

如果使用 Docker 镜像：

```bash
docker compose build --no-cache
```

如果在宿主机运行：

```bash
python3 -m pip install -U "yt-dlp[default]"
```

---

## 24. 验收标准

项目完成后，需要满足：

1. 可以成功读取 `config.yaml`。
2. 可以检测 `cookies.txt` 是否存在。
3. 可以检查 `yt-dlp` 是否可用。
4. 可以检查 `ffmpeg` 是否可用。
5. 可以处理多个用户。
6. 可以按用户设置不同分辨率。
7. 可以下载新视频。
8. 可以跳过已下载视频。
9. 可以生成日志文件。
10. 可以生成 SQLite 数据库。
11. 可以在 Docker Compose 中运行。
12. 可以在飞牛 NAS / 绿联 NAS 中通过计划任务执行。
13. 单个用户失败不会中断整个任务。
14. 日志不会泄露 Cookie。

---

## 25. 推荐开发提示词

可以把下面这段交给 AI 编程工具生成项目代码：

```text
请用 Go 语言开发一个 douyin-nas-monitor 项目。

这是一个适合飞牛 NAS、绿联 NAS 和通用 Linux Docker 环境部署的抖音指定用户视频更新自动下载器。

要求：
1. 使用 config.yaml 配置。
2. 使用用户自己的 cookies.txt。
3. 指定多个抖音用户主页。
4. 每个用户支持单独设置分辨率：best、1080、720、480。
5. 调用 yt-dlp 下载视频。
6. 使用 --download-archive 避免重复下载。
7. 使用 SQLite 记录下载历史。
8. 支持 --once 单次运行。
9. 支持 --daemon 常驻运行。
10. 支持 --check 检查配置和依赖。
11. 支持 Dockerfile 和 docker-compose.yaml。
12. 不要写死群晖路径，必须兼容飞牛 NAS、绿联 NAS 和通用 Docker Compose。
13. 不要实现绕过登录、验证码、风控、私密权限、付费限制、App 签名逆向或无水印破解接口。
14. 本项目只做本地自动化调度，实际下载交给 yt-dlp。

请输出完整项目代码、README、配置示例、Dockerfile、docker-compose.yaml 和部署说明。
```

---

## 26. 最终定位

本项目不是破解下载器，而是：

```text
基于用户自己 Cookie 和 yt-dlp 的 NAS 本地自动化视频备份调度器
```

更准确的中文名称：

```text
飞牛 / 绿联 NAS 抖音指定用户分辨率自动下载器
```

英文名称：

```text
douyin-nas-monitor
```

