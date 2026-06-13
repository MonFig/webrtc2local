# webrtc2nas 录制与回放服务设计文档

**日期**：2026-06-13  
**版本**：v1.0  
**状态**：已确认，待实现

---

## 1. 项目概述

`webrtc2nas` 是一个在 macOS 本地运行的命令行工具，用于把多路 RTSP 摄像头流录制为固定时长的 MP4 文件，并按配置数量循环覆盖。同时内嵌一个轻量 Web 回放服务，方便通过浏览器查看已录制的视频分段。

> 注：项目名沿用 `webrtc2nas`，但实际输入协议为 RTSP。

### 1.1 核心目标

- 支持 1-4 路 RTSP 摄像头同时录制。
- 每 10 分钟生成一个 MP4 文件。
- 每路流可独立配置最大保留文件数，超出后自动删除最旧文件。
- 摄像头断流时无限自动重连。
- 在 macOS 上以低 CPU 方式长期运行。
- 提供简单的 Web 页面回放已录视频。

### 1.2 非目标（本期不做）

- 无缝时间轴拖动播放（HLS/DASH）。
- 实时直播画面。
- 配置热更新。
- macOS `launchd` 后台服务模板。
- 磁盘空间告警。
- 多用户/权限管理。

---

## 2. 运行环境

| 项 | 要求 |
|---|---|
| 操作系统 | macOS（当前目标机器） |
| 运行时依赖 | `ffmpeg`（系统已安装） |
| 开发环境 | Go 1.22+ |
| 摄像头协议 | RTSP |
| 典型视频编码 | H.265 / HEVC |
| 典型音频编码 | PCMA / G.711A |

---

## 3. 架构设计

### 3.1 总体架构

程序是一个单 Go 二进制文件，内部包含两个主要部分：

- **录制器（Recorder）**：读取配置，为每路摄像头启动 ffmpeg 子进程，生成分段 MP4，并负责断线重连与循环删除。
- **回放服务（Playback Server）**：内嵌轻量 HTTP 服务，提供摄像头列表、录像分段列表和 MP4 文件访问。

```
config.yaml
    │
    ▼
webrtc2nas
    ├── Recorder
    │     ├── ffmpeg (living_room)
    │     ├── ffmpeg (bedroom)
    │     └── ...
    │
    └── Playback Server (HTTP)
              └── 浏览器列表回放
```

### 3.2 模块划分

| 模块 | 职责 | 依赖 |
|---|---|---|
| `config` | 读取并校验 YAML 配置文件 | 仅文件系统 |
| `recorder` | 管理所有摄像头录制 goroutine | `config`, `storage`, `ffmpeg` |
| `ffmpeg` | 封装 ffmpeg 命令与进程生命周期 | `config` |
| `storage` | 目录创建、文件命名、循环删除旧文件 | `config` |
| `playback` | HTTP API 服务 | `config`, `storage` |
| `webui` | 内嵌静态前端页面 | `playback` |

---

## 4. 配置文件

### 4.1 文件格式

使用 YAML，路径通过命令行参数指定：

```bash
./webrtc2nas -config config.yaml
```

### 4.2 配置示例

```yaml
output_dir: ./recordings
log_level: info
playback_host: 127.0.0.1
playback_port: 8080

streams:
  - name: living_room
    url: rtsp://homeassistant.local:8554/living_room
    max_files: 144          # 144 × 10min = 24 小时
    segment_minutes: 10
    enabled: true

  - name: bedroom
    url: rtsp://homeassistant.local:8554/bedroom
    max_files: 1008         # 144 × 7 = 7 天
    segment_minutes: 10
    enabled: true
```

### 4.3 配置字段说明

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `output_dir` | string | 是 | - | 录像根目录 |
| `log_level` | string | 否 | `info` | 日志级别：`debug` / `info` / `warn` / `error` |
| `playback_host` | string | 否 | `127.0.0.1` | Web 回放服务绑定地址，设为 `0.0.0.0` 可允许局域网访问 |
| `playback_port` | int | 否 | `8080` | Web 回放服务端口 |
| `streams` | array | 是 | - | 摄像头列表 |
| `streams[].name` | string | 是 | - | 摄像头标识，决定子目录名 |
| `streams[].url` | string | 是 | - | RTSP 拉流地址 |
| `streams[].max_files` | int | 是 | - | 该摄像头最多保留的文件数 |
| `streams[].segment_minutes` | int | 否 | `10` | 每段录像时长（分钟） |
| `streams[].enabled` | bool | 否 | `true` | 是否启用该路录制 |

### 4.4 校验规则

- `name` 必须唯一，且只能包含字母、数字、下划线、连字符。
- `url` 必须以 `rtsp://` 开头。
- `max_files` 必须大于 0。
- `segment_minutes` 必须大于 0 且小于等于 60。

---

## 5. 录制流程

### 5.1 启动流程

1. 读取并校验配置文件。
2. 检查系统是否已安装 `ffmpeg`；未安装则报错退出。
3. 检查 `output_dir` 是否可写；不可写则报错退出。
4. 为每个 `enabled: true` 的摄像头启动独立的录制 goroutine。
5. 启动 Web 回放 HTTP 服务。
6. 阻塞主线程，等待退出信号。

### 5.2 ffmpeg 命令

每个摄像头启动一个 ffmpeg 进程，命令模板如下：

```bash
ffmpeg -loglevel error -rtsp_transport tcp \
  -i rtsp://homeassistant.local:8554/living_room \
  -c:v copy \
  -c:a aac -ar 8000 \
  -f segment \
  -segment_time 600 \
  -reset_timestamps 1 \
  -strftime 1 \
  "./recordings/living_room/%Y-%m-%d/video_%H-%M-%S.mp4"
```

参数说明：

| 参数 | 说明 |
|---|---|
| `-loglevel error` | 仅输出错误，避免 ffmpeg 日志刷屏 |
| `-rtsp_transport tcp` | 强制 TCP 传输，提高稳定性 |
| `-c:v copy` | 视频直接复制，不重新编码，CPU 占用最低 |
| `-c:a aac -ar 8000` | PCMA 音频转码为 AAC；保持 8kHz 采样率，适配对讲音频 |
| `-f segment` | 分段输出模式 |
| `-segment_time 600` | 目标每 600 秒（10 分钟）分一段 |
| `-reset_timestamps 1` | 每段文件时间戳从 0 开始 |
| `-strftime 1` | 文件名按当前时间格式化 |

> **分段精度说明**：配合 `-c:v copy` 时，ffmpeg 会在关键帧处切割，因此实际片段长度可能略大于或小于 10 分钟（通常偏差在几秒到十几秒之间）。文件名仍按分段开始时间生成。

### 5.3 分段与文件组织

文件路径结构：

```
{output_dir}/{stream_name}/{YYYY-MM-DD}/video_HH-MM-SS.mp4
```

示例：

```
recordings/
├── living_room/
│   ├── 2026-06-12/
│   │   ├── video_14-20-00.mp4
│   │   └── video_14-30-00.mp4
│   └── 2026-06-13/
│       ├── video_09-00-00.mp4
│       └── video_09-10-00.mp4
└── bedroom/
    └── ...
```

- 文件名表示该分段的开始时间。
- 分段跨天时，文件仍放在开始日期目录下。

### 5.4 循环删除

- 程序启动时扫描一次该摄像头的所有 MP4 文件，执行清理。
- 之后每 30 秒定时扫描一次，检查文件数量。
- 按文件修改时间排序，保留最新的 `max_files` 个。
- 删除超出数量的最旧文件。
- 若某日期目录变空，则删除该空目录。
- 程序重启后会重新扫描磁盘，继续执行保留策略，不依赖内存状态。

---

## 6. 断线重连与错误处理

### 6.1 断线检测

- ffmpeg 异常退出时，Go 中的 goroutine 收到退出信号。
- 退出码为 0 时（正常结束），不再重启。
- 退出码非 0 或被信号终止时，判定为异常，进入重连流程。

### 6.2 重连策略

- 首次断开立即重试。
- 连续失败时采用指数退避：1s、2s、4s、8s、16s，最大 60s。
- 无限重试，不主动放弃。
- 每次重试打印简洁日志。

### 6.3 错误处理矩阵

| 场景 | 行为 |
|---|---|
| 配置文件不存在或格式错误 | 启动时报错退出 |
| `ffmpeg` 未安装 | 启动时报错退出 |
| `output_dir` 不可写 | 启动时报错退出 |
| 某路流 URL 不可达 | 仅该路无限重试，其他流继续工作 |
| 磁盘满 | ffmpeg 写入失败退出，Go 程序按策略重试，打印警告 |
| 某段文件写入失败 | ffmpeg 退出，重连后从下一时间段继续 |

### 6.4 优雅退出

- 接收到 `SIGINT` / `SIGTERM` 后，程序通知所有 ffmpeg 子进程正常结束（发送 SIGTERM 并等待）。
- 等待超时后强制 kill，避免损坏正在写入的 MP4。

---

## 7. Web 回放服务

### 7.1 API 设计

| 接口 | 方法 | 说明 |
|---|---|---|
| `GET /api/cameras` | GET | 返回摄像头列表 |
| `GET /api/timeline/{camera}` | GET | 返回指定摄像头所有录像分段 |
| `GET /api/video/{camera}/{date}/{filename}` | GET | 返回 MP4 文件 |

### 7.2 响应示例

`GET /api/cameras`

```json
{
  "cameras": [
    {"name": "living_room", "enabled": true},
    {"name": "bedroom", "enabled": true}
  ]
}
```

`GET /api/timeline/living_room`

```json
{
  "camera": "living_room",
  "files": [
    {
      "date": "2026-06-13",
      "filename": "video_09-00-00.mp4",
      "start": "2026-06-13T09:00:00+08:00",
      "end": "2026-06-13T09:10:00+08:00",
      "url": "/api/video/living_room/2026-06-13/video_09-00-00.mp4"
    }
  ]
}
```

### 7.3 前端页面

- 单页应用，内嵌在 Go 二进制中。
- 顶部下拉选择摄像头。
- 主体按日期分组列出 10 分钟分段。
- 点击分段后在 `<video>` 标签中播放。
- 首期不做无缝连续播放，按段独立播放。

---

## 8. 安全与边界

- 回放服务默认绑定 `127.0.0.1`，仅本地访问。
- 可通过配置调整为 `0.0.0.0` 以允许局域网访问。
- 不处理认证授权，依赖网络边界或反向代理。
- 程序只读取配置和写入指定目录，不访问摄像头以外的网络资源。

---

## 9. 测试策略

### 9.1 单元测试

- `config` 模块：配置文件解析与校验。
- `storage` 模块：文件扫描、排序、删除逻辑。
- `ffmpeg` 模块：命令参数生成。

### 9.2 集成测试

- 使用本地测试 RTSP 流（如 ffmpeg 推流或测试文件循环）验证：
  - 是否正确生成 10 分钟分段。
  - 循环删除是否按 `max_files` 生效。
  - 断线后是否能自动重连。
  - Web API 是否返回正确列表。

### 9.3 手动测试

- 连接真实摄像头运行 24 小时，观察 CPU、内存、磁盘占用。
- 验证 H.265 + AAC 在 MP4 中的浏览器播放兼容性。

---

## 10. 依赖与工具

| 依赖 | 用途 | 备注 |
|---|---|---|
| `ffmpeg` | RTSP 拉流、分段、容器封装 | 系统依赖，运行时检查 |
| Go 标准库 | HTTP 服务、进程管理、文件操作 | 无第三方 Web 框架依赖 |
| `gopkg.in/yaml.v3` | YAML 配置解析 | 开发依赖 |

---

## 11. 后续扩展（二期）

- HLS 无缝时间轴回放。
- 实时画面直播。
- 配置文件热更新。
- macOS `launchd` plist 模板。
- 磁盘空间监控与告警。
- 录制计划（按时间段启用/禁用）。

---

## 12. 决策记录

| 决策 | 选择 | 原因 |
|---|---|---|
| 开发语言 | Go | 低 CPU、低内存、单二进制、适合长期服务 |
| 视频编码 | `-c:v copy` | 最低 CPU，满足 H.265 直存 |
| 音频编码 | `-c:a aac` | PCMA 不兼容 MP4，AAC 开销低 |
| 容器格式 | MP4 | 用户指定，浏览器兼容性好 |
| 分段时长 | 10 分钟 | 用户指定，便于管理 |
| 保留策略 | 按文件数循环 | 用户指定，实现简单 |
| 断线处理 | 无限重连 | 用户指定，不丢可恢复时段 |
| 回放方式 | 列表分段播放 | 首期 YAGNI，降低复杂度 |

---

*文档确认日期：2026-06-13*
