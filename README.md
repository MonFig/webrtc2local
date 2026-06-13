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
