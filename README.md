# mediago

A streaming media downloader written in Go, supporting HLS and DASH protocols.

## Features

- **HLS** — Master/media playlist parsing, AES-128 decryption, BYTERANGE
- **DASH** — SegmentTemplate, SegmentList, SegmentBase, Timeline
- **Concurrent download** — Goroutine pool with configurable thread count
- **Live recording** — Playlist refresh, segment deduplication, duration limit
- **Auto stream selection** — Pick best quality video + audio
- **Merge** — Binary concat (fMP4) / FFmpeg concat (TS → MP4)
- **Custom headers, proxy, retry** — For restricted content and unstable networks

## Install

```bash
go install github.com/caorushizi/mediago-core/cmd/mediago@latest
```

Or build from source:

```bash
git clone https://github.com/caorushizi/mediago-core.git
cd mediago-core
task build
```

## Usage

```bash
# Basic download
mediago "https://example.com/video.m3u8"

# Specify output
mediago "https://example.com/video.m3u8" -d ./output -n "my_video"

# Custom headers
mediago "https://example.com/video.m3u8" \
  -H "Referer: https://example.com" \
  -H "Cookie: token=xxx"

# 16 threads + proxy
mediago "https://example.com/video.m3u8" -t 16 --proxy socks5://127.0.0.1:1080

# Auto select best quality
mediago "https://example.com/master.m3u8" --auto-select

# DASH
mediago "https://example.com/manifest.mpd" --auto-select

# Live recording for 1 hour
mediago "https://example.com/live.m3u8" --live-duration 01:00:00

# Download only, skip merge
mediago "https://example.com/video.m3u8" --no-merge
```

## CLI Reference

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--save-dir` | `-d` | `.` | Save directory |
| `--save-name` | `-n` | auto | Output filename |
| `--tmp-dir` | | system temp | Temporary directory |
| `--header` | `-H` | | Custom HTTP header (repeatable) |
| `--proxy` | | | Proxy URL (http/socks5) |
| `--timeout` | | `30` | HTTP timeout in seconds |
| `--thread-count` | `-t` | `8` | Concurrent threads |
| `--retry-count` | `-r` | `3` | Retry per segment |
| `--auto-select` | | `false` | Auto select best quality |
| `--select-video` | `-sv` | | Video stream filter |
| `--select-audio` | `-sa` | | Audio stream filter |
| `--no-merge` | | `false` | Skip merge step |
| `--del-after-done` | | `true` | Delete temp files |
| `--ffmpeg-path` | | `ffmpeg` | Path to ffmpeg |
| `--binary-merge` | | `false` | Force binary concat |
| `--key` | | | Decryption key (HEX, repeatable) |
| `--custom-hls-method` | | | Force encryption method |
| `--custom-hls-key` | | | Force HLS key (HEX) |
| `--custom-hls-iv` | | | Force HLS IV (HEX) |
| `--live` | | auto | Force live mode |
| `--live-duration` | | unlimited | Recording duration (HH:mm:ss) |
| `--live-wait-time` | | auto | Playlist refresh interval (sec) |
| `--log-level` | | `info` | Log level |
| `--no-log` | | `false` | Disable logging |

## Architecture

```
mediago [url]
    │
    ├─ Detect protocol (HLS / DASH)
    │
    ├─ Parse manifest
    │   ├─ HLS: master playlist → media playlist → segments
    │   └─ DASH: MPD → periods → adaptation sets → segments
    │
    ├─ Select streams (auto / manual)
    │
    ├─ Download segments (concurrent HTTP)
    │
    ├─ Decrypt (AES-128-CBC if encrypted)
    │
    ├─ Merge
    │   ├─ fMP4: binary concat (init + segments)
    │   └─ TS: ffmpeg -c copy
    │
    └─ Cleanup temp files
```

```
cmd/mediago/          CLI entry point
internal/
├── parser/
│   ├── hls/          HLS playlist parsing
│   └── dash/         DASH MPD parsing
├── downloader/       Concurrent HTTP download engine
├── crypto/           AES-128 decryption
├── merger/           Binary concat + FFmpeg merge
├── pipeline/         Orchestration + live recording
└── model/            Shared data types
```

## Development

Requires [Go](https://go.dev/) 1.21+ and [Task](https://taskfile.dev/).

```bash
task build        # Build binary to bin/mediago
task run          # Run directly
task test         # Run all tests
task fmt          # Format code
task lint         # Run linter
```

## Disclaimer

This software is for educational and research purposes only. See [DISCLAIMER.md](DISCLAIMER.md) for details.

## License

[MIT](LICENSE)
