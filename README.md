# webrtc2nas

Mac 本地 RTSP 摄像头录制与回放工具。

## 依赖

- Go 1.22+
- ffmpeg

## 安装

```bash
go build -o webrtc2nas ./cmd/webrtc2nas
```

## 配置

复制 `config.example.yaml` 为 `config.yaml` 并按需修改。

## 运行

```bash
./webrtc2nas -config config.yaml
```

访问 http://127.0.0.1:8080 查看回放页面。

## 配置说明

| 字段 | 说明 |
|---|---|
| `output_dir` | 录像保存目录 |
| `log_level` | 日志级别：debug / info / warn / error |
| `playback_host` | 回放服务绑定地址，默认 127.0.0.1 |
| `playback_port` | 回放服务端口，默认 8080 |
| `streams[].name` | 摄像头名称，决定子目录名 |
| `streams[].url` | RTSP 拉流地址 |
| `streams[].max_files` | 该摄像头最多保留的片段数 |
| `streams[].segment_minutes` | 每个片段时长（分钟） |
| `streams[].enabled` | 是否启用 |

## 注意事项

- 视频使用 `-c:v copy` 直接复制，CPU 占用最低。
- 音频从 PCMA 转码为 AAC，以兼容 MP4 容器。
- 片段长度可能因关键帧位置略有偏差。
