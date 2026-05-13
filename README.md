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
- 支持 Docker / Docker Compose 部署。

## 快速开始

复制配置文件：

```bash
cp config.yaml.example config.yaml
```

把浏览器导出的 Cookie 保存为：

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

## Docker Compose

先准备：

```bash
cp config.yaml.example config.yaml
```

编辑 `docker-compose.yaml` 中的下载目录映射，把 `./downloads:/downloads` 改成你的 NAS 媒体目录，例如：

```yaml
- /你的NAS媒体目录/douyin:/downloads
```

构建并单次运行：

```bash
docker compose run --rm douyin-monitor
```

常驻运行时，把 compose 中的 command 和 restart 改成：

```yaml
command: douyin-nas-monitor --config /app/config.yaml --daemon
restart: unless-stopped
```

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
