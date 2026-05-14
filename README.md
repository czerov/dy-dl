# douyin-nas-monitor

基于用户自己的 `cookies.txt` 和 `yt-dlp` 的 NAS 本地自动化视频备份调度器。项目只负责编排、配置、日志和记录，实际下载能力交给 `yt-dlp`。

## 合规边界

- 只下载自己账号可正常访问且有权保存的视频。
- 不支持私密、付费或无权限内容。
- 不绕过登录、验证码、风控、地区限制或平台权限。
- 不做 App 签名逆向、破解接口或无水印解析。
- 不把 Cookie 内容写入日志。

## 功能

- 读取 `config.yaml`。
- 使用本地 `cookies.txt`。
- 支持多个抖音用户主页。
- 每个用户可设置 `best`、`1080`、`720`、`480`。
- 调用 `yt-dlp` 下载。
- 使用 `--download-archive` 跳过已下载视频。
- 使用 SQLite 记录下载结果。
- 支持 `--once`、`--daemon`、`--check`。
- 内置 Web 管理台和 HTTP API。
- 支持 Docker / Docker Compose 部署。

## 快速开始

本地直接运行时可以复制配置文件：

```bash
cp config.yaml.example config.yaml
```

如果不使用 Web 管理台，可以把浏览器导出的 Cookie 保存为：

```text
cookies.txt
```

检查配置和依赖：

```bash
go run ./cmd/douyin-nas-monitor --config config.yaml --check
```

本地直接运行时，如果不使用 Docker，请把 `config.yaml` 里的 `/app/...` 和 `/downloads` 改成你的本机路径或相对路径。

单次运行：

```bash
go run ./cmd/douyin-nas-monitor --config config.yaml --once
```

常驻运行：

```bash
go run ./cmd/douyin-nas-monitor --config config.yaml --daemon
```

启动 Web 管理台：

```bash
go run ./cmd/douyin-nas-monitor --config config.yaml --web --addr :3456
```

打开：

```text
http://localhost:3456
```

## Docker Compose

编辑 `docker-compose.yaml` 中的下载目录映射，把 `./downloads:/downloads` 改成你的 NAS 媒体目录，例如：

```yaml
- /你的NAS媒体目录/douyin:/downloads
```

构建并单次运行：

```bash
docker compose run --rm douyin-monitor
```

默认 Compose 会启动 Web 管理台，并暴露 `3456` 端口。

首次启动不需要提前准备 `config.yaml` 或 `cookies.txt`。容器会在 `./data/config.yaml` 自动创建默认配置，CK 可在 Web 管理台的“配置”页面填写，并保存到 `./data/cookies.txt`。

添加用户时，`URL` 填抖音用户主页链接，例如 `https://www.douyin.com/user/MS4wLj...`，不要只填抖音号。也可以填单条视频链接 `https://www.douyin.com/video/数字ID` 用来测试 Cookie 和下载链路。

“发现”页面可以用当前 CK 尝试获取某个主页里的作品、合集和短剧入口，勾选后再下载。抖音 PC 页面经常把真实列表放在浏览器渲染后的动态数据里，这时用“浏览器采集”复制脚本，到已登录的抖音作品/合集/短剧页控制台运行，再把采集结果粘贴回管理台导入选择。

如果只想用 NAS 定时任务单次执行，把 compose 中的 command 和 restart 改成：

```yaml
command: douyin-nas-monitor --config /app/data/config.yaml --once
restart: "no"
```

## Web API

当前内置接口：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/status` | 运行状态 |
| GET | `/api/config` | 读取配置 |
| PUT | `/api/config` | 保存配置 |
| GET | `/api/cookies` | 查看 Cookie 文件状态 |
| PUT | `/api/cookies` | 更新 Cookie 文件内容 |
| POST | `/api/run` | 手动启动一次下载 |
| GET | `/api/check` | 环境检查 |
| GET | `/api/downloads` | 下载历史 |
| POST | `/api/discover` | 尝试从 URL 获取内容列表 |
| POST | `/api/discover/import` | 导入浏览器采集结果或链接列表 |
| POST | `/api/discover/download` | 下载发现页选中的内容 |
| GET | `/api/logs` | 日志尾部 |

## NAS 定时任务

适合飞牛 NAS / 绿联 NAS：

```bash
cd /你的NAS路径/douyin-monitor && docker compose run --rm douyin-monitor
```

推荐频率：

| 场景 | 推荐频率 |
|---|---|
| 少量用户 | 每 1-2 小时 |
| 普通用户 | 每 2-6 小时 |
| 用户较多 | 每 6-12 小时 |

不建议高频检测，避免触发平台风控。

## 配置说明

核心字段见 `config.yaml.example`：

- `app.cookies_file`：Cookie 文件路径。
- `app.archive_file`：`yt-dlp` 下载归档文件。
- `app.default_save_dir`：默认下载目录。
- `app.yt_dlp_path`：`yt-dlp` 可执行文件。
- `download.output_template`：文件命名模板。
- `download.retries`：失败重试次数。
- `users[].quality`：单用户清晰度。
- `users[].save_dir`：单用户保存目录，留空时使用默认目录。

## 日志和数据

- 日志：`logs/douyin-monitor.log`
- 数据库：`data/douyin-monitor.db`
- 归档：`data/archive.txt`

## 更新 yt-dlp

Docker 镜像：

```bash
docker compose build --no-cache
```

宿主机：

```bash
python3 -m pip install -U "yt-dlp[default]"
```

## 常见问题

没有下载新视频时，通常是没有新内容、视频已在 `archive.txt`、Cookie 失效、用户主页不可访问，或 `yt-dlp` 暂时不支持当前页面结构。

1080P 下载不到时，通常是原视频没有 1080P、账号只能访问较低清晰度，或 `yt-dlp` 获取到的格式有限。程序会按配置自动降级。
