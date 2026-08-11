# ChangeLog - Video QC

## 2026-07-09

### Changed
- Decoupled Video QC from the TestCenter platform UI/API layer.
- Removed the platform dashboard entry, frontend route/page/API client, backend `/api/video-qc` route, and platform permission entry.
- Kept `video-qc-core` as the standalone Go module and CLI-oriented codebase.
- Removed standalone packaging scripts and generated release artifacts from the working tree.
- Kept macOS-compatible bundled-tool lookup in the CLI so future M1 builds can find sibling `ffmpeg`/`ffprobe` binaries.

### Added
- Added `YDIF`, `YMIN`, and `YMAX` parsing for FFmpeg `signalstats`.
- Added `freeze_similarity` detection as a lightweight frame-similarity freeze detector.
- Added pipeline event reconciliation to suppress overlapping `freeze_similarity` when `freeze` already covers the same segment.
- Added `blur_low_detail` detection based on `edgedetect + signalstats`.
- Added stricter multi-frame `artifact_spike` detection for possible mosaic/glitch/block artifacts.
- Added `deepmotion` detector for deep-profile motion-jump detection.
- Added Windows regression script `video-qc-core/scripts/run_regression.ps1`.

### Verified
- `video-qc-core`: `go fmt ./...` and `go test ./...` passed.
- Regression script passed with generated black, white, green, freeze, and AV-sync samples.

## 2026-07-08

### Added
- 后端视频质检新增异步执行能力，创建任务时可传入 `async=true`。
- 新增任务状态接口 `GET /api/video-qc/tasks/:taskId/status` 和取消接口 `POST /api/video-qc/tasks/:taskId/cancel`。
- 前端 `VideoQuality` 页面新增后台执行开关、状态轮询和取消按钮。
- 新增 `avsync` detector，deep profile 下通过 ffprobe stream timing 输出 `av_sync_offset` 事件。
- Pipeline 分析阶段支持同时执行 frame detector 和 audio detector。
- 后端视频质检新增 `GET /api/video-qc/tasks` 历史任务接口和 `POST /api/video-qc/upload` 视频上传接口。
- 前端 `VideoQuality` 页面新增视频上传、历史任务刷新和点击历史任务回看报告。
- `signalstats` 解析新增 `UAVG`、`VAVG`，`visualartifact` detector 新增 `green_screen` 事件。
- 新增 `visualartifact` detector，基于 FFmpeg `signalstats` 输出 `white_screen` 与 `brightness_jump` 事件。
- 新增平台后端视频质检接口：`POST /api/video-qc/tasks`、`GET /api/video-qc/tasks/:taskId/report`、`GET /api/video-qc/tasks/:taskId/artifacts/*file`。
- 新增前端视频质检页面 `VideoQuality`，支持输入服务器可访问的视频路径或 URL，并展示评分、视频信息、事件列表、指标和截图证据。
- 仪表盘 API 工具区域新增“视频质检”卡片，接入 `dashboard.video_quality.visible` 权限。
- 默认权限增加 `dashboard.video_quality.visible`，后端默认权限表同步开启该入口。
- 新增 Phase 4 基础检测能力：PTS 跳帧检测器 `framedrop`，通过 `ffprobe` 读取逐帧 PTS 并按 `ptsGapRatio` 输出 `frame_drop` 事件。
- 新增 Freeze 基础检测器 `freeze`，通过 FFmpeg `freezedetect` 输出 `freeze` 事件，记录冻结开始、结束、持续时间和置信度。
- CLI 检测流水线接入 `framedrop` 与 `freeze`，默认 profile 中已开启的 `frameDrop`、`freeze` 开关开始生效。
- 新增 PTS 解析、freeze 日志解析、frame drop 事件生成等相关单元测试。
- 新增黑屏片段合并，避免短间隔黑屏被拆成多个独立事件。
- 新增黑屏事件分类：`head_black`、`tail_black`、`flash_black`、`black_screen`。
- 新增黑屏证据截图能力；当 CLI 传入 `--work-dir` 时，截图会落到 `evidence/` 目录，并写入报告 `evidence.thumbnail`。
- 新增黑屏二级/三级确认能力：通过 FFmpeg `signalstats` 统计 `meanY`，通过 `blackframe` 统计 `blackPixelRatio`，通过 `edgedetect + signalstats` 输出近似 `edgeRatio`。
- 黑屏事件 method 升级为 `blackdetect+luma+blackframe+edge`，并根据亮度、黑像素比例、边缘强度动态调整置信度。
- 新增 `video-qc-core` 独立视频质检引擎骨架，作为后续黑屏、跳帧、卡帧、文件完整性、画面异常和音视频同步检测的独立承载目录。
- 新增 `video-qc` CLI 最小入口，支持 `check` 和 `version` 命令，并可输出统一 JSON 报告结构。
- 新增 `default`、`fast`、`deep` 三套检测 profile 配置，为默认检测和深度检测预留开关与阈值。
- 新增统一 report schema、detector 接口、pipeline 状态机与基础单元测试，确保后续检测器可以插件化接入。
- 新增 File Check 检测器，接入 `ffprobe` 元数据解析和 `ffmpeg -v error -f null` 完整解码检测。
- 新增 `file_probe_error`、`decode_error`、`timestamp_error` 事件输出，报告会在出现 critical/high 事件时标记为 `failed`。
- 新增 `--ffmpeg` 和 `--ffprobe` CLI 覆盖参数，支持平台或当前 shell 指定 FFmpeg 工具路径。
- 新增黑屏一级检测器，接入 FFmpeg `blackdetect`，可输出 `black_screen` 事件及开始时间、结束时间、持续时间和基础指标。

### Notes
- 已用音频延迟约 0.7 秒的视频样例完成 deep profile 验证，报告输出 `av_sync_001`，`startOffsetMs≈676`。
- 异步任务状态会写入 `report/video_qc/<taskId>/task.json`，报告仍写入 `report/video_qc/<taskId>/report.json`。
- 已用 1 秒动态画面 + 1.2 秒绿色画面 + 1 秒动态画面的样例完成 CLI 验证，报告输出 `green_screen_001`，指标为 `meanY=80`、`meanU=91`、`meanV=81`。
- 视频上传文件当前保存到 `report/video_qc/_uploads`，历史任务读取 `report/video_qc` 下已有任务目录。
- 白屏阈值调整为 `whiteMeanY=232`，兼容常见 limited range 视频中纯白 Y 值约为 235 的编码特性。
- 已用 1 秒动态画面 + 1.2 秒白屏 + 1.2 秒蓝屏样例完成 CLI 验证，报告输出 `white_screen_001` 和 `brightness_jump` 事件。
- 后端视频质检接口当前为同步执行第一版，平台通过独立 `video-qc-core` CLI 调用核心引擎，后续可替换为异步队列或独立 HTTP 服务。
- 前端构建 `npm run build` 已通过；后端全包编译验证 `go test ./... -run TestVideoQCDoesNotExist` 已通过。
- Phase 4 已完成代码级验证：`go fmt ./...` 和 `go test ./...` 均通过。
- 已用 1 秒动态画面 + 2.2 秒静止画面 + 1 秒动态画面的样例完成 CLI 验证，报告输出 `freeze_001`，时间为 `1000ms~3200ms`，持续 `2200ms`，置信度 `0.86`。
- FFmpeg 8.1.2 的 `freezedetect` 会将 `freeze_duration` 与 `freeze_end` 分行输出，解析器已兼容该格式和旧的一行格式。
- 已用前 1.2 秒黑屏、后 1 秒测试画面的视频样例完成 CLI 验证，报告输出 `head_black`，并生成 `evidence/head_black_001.jpg`。
- 已用前 1 秒黑屏、后 1 秒测试画面的视频样例完成 CLI 验证，报告输出 `meanY=16`、`blackPixelRatio=1`、`edgeRatio=0`、`confidence=0.95`。
- 已通过 `winget` 安装 FFmpeg 8.1.2，并完成临时样例视频的 File Check 与 blackdetect 验证；当前 shell PATH 未刷新时可使用 CLI 覆盖参数指定工具路径。
- 核心引擎 JSON 文案使用 ASCII，避免 Windows 编码导致报告内容损坏；中文展示文案后续放在平台 UI 层处理。
- 引擎以独立 Go module 形式落地，便于未来从 TestCenter 拆分为单独平台。
