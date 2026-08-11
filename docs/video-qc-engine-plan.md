# Video QC Engine 开发计划

> 当前状态：Phase 3 黑屏一级检测已完成，二级亮度与三级边缘确认待开发  
> 创建日期：2026-07-08  
> 适用范围：视频跳帧、黑屏、卡帧、文件完整性、画面异常与音视频同步检测  
> 设计原则：核心检测能力独立于 TestCenter，平台仅作为调用方和展示方

## 1. 目标

新增一套可接入 TestCenter、也可随时拆分为独立平台的视频质量检测能力，暂定名称为 `Video QC Engine`。

核心目标：

- 检测视频文件是否损坏、是否可解析、是否可完整解码。
- 检测黑屏、闪黑、首尾黑、白屏、绿屏等画面异常。
- 检测跳帧、卡帧、冻结、时间戳异常。
- 支持默认快速检测和深度检测两种模式。
- 输出统一 JSON 报告，方便平台展示、导出和后续服务化。
- 核心代码保持独立，避免和 TestCenter 前后端业务强耦合。

## 2. 约束与规则

开发时遵循仓库根目录 `.cursorrules`：

- 前端保持 Vue 3、TypeScript、Vite、Pinia、Vue Router、Element Plus 技术栈。
- 后端保持 Go、Gin、MySQL 技术栈。
- 保持视图层、状态层、服务层、数据层职责分离。
- 新增用户可见入口必须接入权限系统。
- 不在前端页面、报告 URL、WebSocket、iframe、下载链接中硬编码 `localhost`、本机 IP 或临时 IP。
- 前端运行时地址统一使用 `src/utils/runtimeUrl.ts`。
- 后端生成平台链接统一使用 `server/services/platform_url.go`。
- 涉及 Go 后端改动优先运行 `go test ./...`。
- 涉及前端改动优先运行 `npm run build`。
- 功能完成后同步更新对应 ChangeLog。

## 3. 总体架构

```text
TestCenter 平台
  └─ video-qc-adapter
       ├─ 任务创建
       ├─ 进度查询
       ├─ 报告读取
       └─ 前端展示

独立 Video QC Engine
  ├─ video-qc-core      核心检测引擎
  ├─ video-qc-cli       命令行入口
  ├─ video-qc-service   可选 HTTP 服务
  ├─ detectors          检测器插件
  ├─ pipeline           多阶段流水线
  ├─ report             JSON/HTML/Excel 报告
  └─ storage            任务结果和证据文件
```

第一期建议先以 CLI 方式接入 TestCenter：

```bash
video-qc check --input video.mp4 --profile default --output report.json
```

后续可升级为独立服务：

```http
POST /api/video-qc/tasks
GET  /api/video-qc/tasks/:id
GET  /api/video-qc/tasks/:id/report
GET  /api/video-qc/tasks/:id/artifacts/:file
```

## 4. 目录规划

建议先在仓库内以独立目录承载，后续可整体拆出：

```text
video-qc-core/
  cmd/
    video-qc/
  configs/
    default.json
    fast.json
    deep.json
  internal/
    probe/
    decode/
    frame/
    detectors/
      filecheck/
      blackscreen/
      framedrop/
      freeze/
      artifact/
      avsync/
    pipeline/
    report/
    storage/
  pkg/
    schema/
    client/
```

TestCenter 侧仅增加适配层：

```text
server/services/video_qc_service.go
server/models/video_qc_models.go
src/views/VideoQuality/
src/api/videoQuality.ts
```

## 5. 检测分层

### 5.1 第一层：文件完整性检测 File Check

目标：最快判断视频是否损坏。

检测内容：

- `ffprobe` 是否能读取 metadata。
- 是否能读取 duration。
- 是否存在 video stream。
- codec、fps、resolution、bitrate 是否正常。
- audio/video stream 是否完整。

解码检测：

```bash
ffmpeg -v error -i video.mp4 -f null -
```

捕获错误：

- `error while decoding`
- `invalid data`
- `corrupt`
- `non monotonically increasing dts`
- `negative timestamp`
- `timestamp discontinuity`

输出事件类型：

- `file_probe_error`
- `decode_error`
- `timestamp_error`

### 5.2 第二层：黑屏检测 Black Screen

黑屏采用三级检测，避免只依赖 `blackdetect`。

一级：FFmpeg blackdetect 初筛。

```bash
ffmpeg -i video.mp4 -vf blackdetect=d=0.5:pix_th=0.10 -an -f null -
```

二级：逐帧亮度检测。

核心指标：

- `meanY`
- `blackPixelRatio`
- `durationMs`

默认规则：

```text
meanY < 10
blackPixelRatio > 0.95
duration >= 500ms
```

三级：边缘检测防误判。

低亮度但边缘较多时，可能是夜景、黑背景字幕、暗场画面，不直接判事故。

核心指标：

- `edgeRatio`
- `edgeCount`
- `meanY`
- `blackPixelRatio`

输出事件：

- `black_screen`
- `flash_black`
- `head_black`
- `tail_black`

### 5.3 第三层：跳帧检测 Frame Drop

跳帧检测拆成四种算法。

方法一：PTS 检测。

读取每帧 PTS：

```text
expectedDelta = 1 / fps
actualDelta = pts[i] - pts[i - 1]
```

当 `actualDelta > expectedDelta * threshold` 时，判定疑似掉帧。

方法二：帧间差异检测。

使用：

- SSIM
- PSNR
- MSE
- pHash distance

连续多帧几乎一致时，不判跳帧，单独输出为 `freeze`。

方法三：运动矢量检测。

使用 FFmpeg motion vector，检测运动突然消失和恢复的片段。

方法四：光流检测。

默认可使用 OpenCV Farneback；深度模式可预留 RAFT。

检测内容：

- motion magnitude
- motion direction
- flow discontinuity

输出事件：

- `frame_drop`
- `freeze`
- `motion_jump`

### 5.4 第四层：画面异常检测 Visual Artifacts

建议纳入框架，分阶段实现。

检测项：

- 花屏
- 马赛克
- 绿屏
- 白屏
- 亮度突变
- 过暗
- 过曝
- 模糊
- 噪声

基础规则示例：

```text
green_screen:
  greenPixelRatio > threshold
  G >> R and G >> B
  duration >= threshold

white_screen:
  meanY > 245
  whitePixelRatio > 0.95
  duration >= 20 frames
```

### 5.5 第五层：音视频同步 AV Sync

建议作为第二期或深度检测能力。

检测内容：

- audio start pts
- video start pts
- audio duration
- video duration
- audio/video drift

输出事件：

- `av_sync_offset`
- `audio_missing`
- `audio_duration_mismatch`

## 6. 检测模式

### 6.1 默认检测 default

适合平台自动审核。

执行：

- FFprobe
- FFmpeg 完整解码
- timestamp 检查
- blackdetect
- 平均亮度统计
- 边缘确认
- PTS frame drop
- SSIM freeze
- 白屏/绿屏基础检测

### 6.2 深度检测 deep

适合疑似异常视频或人工复核。

执行：

- motion vector
- optical flow
- RAFT 预留
- 花屏/马赛克高级检测
- AV sync
- 更密集抽帧

## 7. 统一报告模型

### 7.1 事件模型

```json
{
  "id": "evt_001",
  "type": "black_screen",
  "category": "visual",
  "severity": "high",
  "startTimeMs": 12120,
  "endTimeMs": 13480,
  "durationMs": 1360,
  "message": "黑屏持续 1.36 秒",
  "confidence": 0.96,
  "method": "blackdetect+luma+edge",
  "evidence": {
    "frameStart": 303,
    "frameEnd": 337,
    "thumbnail": "evidence/evt_001.jpg",
    "metrics": {
      "meanY": 3.1,
      "blackPixelRatio": 0.98,
      "edgeRatio": 0.001
    }
  }
}
```

### 7.2 总报告模型

```json
{
  "taskId": "video-qc-xxx",
  "status": "failed",
  "profile": "default",
  "video": {
    "duration": 123.21,
    "fps": 25,
    "resolution": "1920x1080",
    "codec": "h264"
  },
  "decode": {
    "status": "ok"
  },
  "summary": {
    "score": 72,
    "critical": 1,
    "high": 3,
    "medium": 5,
    "low": 2
  },
  "events": []
}
```

## 8. 任务状态机

```text
queued
  -> probing
  -> decoding
  -> analyzing
  -> reporting
  -> success / failed / canceled
```

要求：

- 每个阶段落盘任务状态。
- 支持失败原因记录。
- 支持任务取消。
- 支持检测日志回放。
- 支持单个 detector 失败但整体报告继续生成。

## 9. 平台集成计划

### 后端

- 新增视频 QC 任务模型。
- 新增任务创建、查询、取消、报告读取 API。
- 通过服务层调用独立 CLI，不在 handler 中写检测逻辑。
- 后续可将 CLI 调用替换为 HTTP 服务调用。

### 前端

- 新增视频质检入口卡片，必须接入权限系统。
- 新增任务创建页。
- 新增任务列表页。
- 新增报告详情页。
- 报告展示按事件类型分组：文件、黑屏、跳帧、卡帧、画面异常、音视频同步。

### 权限

建议权限 Key：

```text
dashboard.video_quality.visible
video_quality.task.create
video_quality.task.read
video_quality.task.cancel
video_quality.report.read
```

## 10. 开发阶段与进度

### Phase 0：方案与计划

- [x] 梳理独立引擎边界。
- [x] 梳理检测分层。
- [x] 梳理报告模型。
- [x] 创建开发计划文档。

状态：已完成  
完成日期：2026-07-08  
验证：已按 `.cursorrules` 读回检查文档结构。

### Phase 1：核心骨架

- [x] 创建 `video-qc-core` 独立目录。
- [x] 创建 CLI 入口。
- [x] 定义 profile 配置结构。
- [x] 定义 report schema。
- [x] 定义 detector 接口。
- [x] 定义 pipeline 上下文和状态机。

状态：已完成  
完成日期：2026-07-08  
验证：

- 已在 `video-qc-core/` 执行 `go fmt ./...`。
- 已在 `video-qc-core/` 执行 `go test ./...`，测试通过。
- 已执行 `go run ./cmd/video-qc check --input sample.mp4 --profile ./configs/default.json --output ./tmp/sample-report.json`，可生成标准 JSON 报告，空事件稳定输出为 `[]`。

调整记录：

- Phase 1 仅完成独立引擎骨架，不接入具体 ffmpeg / ffprobe 检测逻辑。
- `video-qc-core` 当前作为独立 Go module 存在，避免与 TestCenter `server` module 强耦合。

### Phase 2：File Check

- [x] 接入 `ffprobe`。
- [x] 解析 metadata、duration、stream、codec、fps、resolution。
- [x] 接入 `ffmpeg -v error -f null` 解码检测。
- [x] 捕获 decode error 和 timestamp error。
- [x] 输出 `file_probe_error`、`decode_error`、`timestamp_error`。
- [x] 增加最小单元测试或样例测试。

状态：已完成代码落地  
完成日期：2026-07-08  
验证：

- 已在 `video-qc-core/` 执行 `go fmt ./...`。
- 已在 `video-qc-core/` 执行 `go test ./...`，测试通过。
- 已执行 CLI 缺失工具场景验证；当前机器 PATH 中没有 `ffmpeg` / `ffprobe`，CLI 能输出 `file_probe_error` 和 `decode_error` critical 事件，并将报告状态置为 `failed`。
- 已通过 `winget install Gyan.FFmpeg` 安装 FFmpeg 8.1.2，并使用 `--ffmpeg` / `--ffprobe` 覆盖参数完成真实样例验证。
- 已生成 2 秒临时测试视频并执行真实 File Check，能解析 duration、fps、resolution、codec，并完成解码检测。

调整记录：

- profile 增加 `tools.ffprobe` 和 `tools.ffmpeg` 配置，后续可指向内置二进制或部署机路径。
- CLI 增加 `--ffmpeg` 和 `--ffprobe` 参数，用于当前 shell PATH 尚未刷新或平台指定工具路径的场景。
- `decodecheck` 会将 `negative timestamp`、`timestamp discontinuity`、`non monotonically increasing dts`、`non-monotonous dts` 归类为 `timestamp_error`。
- 核心 profile 保持可移植默认值 `ffmpeg` / `ffprobe`，不写死本机安装路径。

### Phase 3：Black Screen

- [x] 接入 FFmpeg blackdetect。
- [x] 实现逐帧平均亮度统计。
- [x] 实现 black pixel ratio。
- [x] 实现 Sobel/Canny 边缘确认。
- [x] 合并黑屏片段。
- [x] 输出黑屏证据截图。
- [x] 支持首尾黑、闪黑分类。

状态：进行中  
完成日期：2026-07-08  
验证：

- 已在 `video-qc-core/` 执行 `go fmt ./...`。
- 已在 `video-qc-core/` 执行 `go test ./...`，测试通过。
- 已生成前 1 秒黑屏、后 1 秒测试图的临时视频，CLI 能输出 `black_screen` 事件。
- 事件包含 `startTimeMs=0`、`endTimeMs=1000`、`durationMs=1000`、`method=ffmpeg blackdetect`、`confidence=0.70`。

调整记录：

- 核心引擎 JSON message 暂统一使用 ASCII 文本，避免 Windows PowerShell 编码导致报告 JSON 被破坏；平台展示层后续负责中文文案映射。
- 黑屏一级检测当前只基于 `blackdetect`，后续继续补逐帧亮度、black pixel ratio、边缘确认、截图证据、首尾黑和闪黑分类。

### Phase 4：Freeze 与 PTS Frame Drop

- [x] 读取每帧 PTS。
- [x] 检测 PTS gap。
- [ ] 实现 SSIM/MSE 基础帧间差异。
- [x] 检测 freeze event。
- [ ] 区分 freeze 与 frame drop。
- [ ] 输出事件置信度。

状态：未开始

### Phase 5：画面异常基础检测

- [x] 实现白屏检测。
- [x] 实现绿屏检测。
- [x] 实现亮度突变检测。
- [ ] 预留花屏/马赛克指标。
- [x] 输出 visual artifact 事件。

状态：未开始

### Phase 6：平台后端适配

- [x] 新增 Go service。
- [x] 新增 API handler。
- [x] 新增任务状态持久化。
- [x] 新增报告文件读取接口。
- [x] 接入权限校验。
- [x] 避免写死 localhost/IP。

状态：未开始

### Phase 7：平台前端页面

- [x] 新增视频质检入口卡片。
- [x] 接入权限系统。
- [x] 新增任务创建页。
- [x] 新增任务列表页。
- [ ] 新增报告详情页。
- [ ] 支持事件筛选、时间轴查看、证据截图查看。

状态：未开始

### Phase 8：深度检测

- [ ] motion vector 检测。
- [ ] optical flow 检测。
- [x] AV sync 检测。
- [ ] 花屏/马赛克高级检测。

状态：未开始

### Phase 9：验证与交付

- [ ] 准备正常视频样例。
- [ ] 准备黑屏样例。
- [ ] 准备卡帧样例。
- [ ] 准备损坏视频样例。
- [ ] 准备时间戳异常样例。
- [ ] 运行 CLI 验证。
- [ ] 运行后端测试。
- [ ] 运行前端构建。
- [ ] 更新 ChangeLog。

状态：未开始

## 11. 风险与待确认

- `ffmpeg` / `ffprobe` 是否随项目分发，还是要求部署机预安装。
- OpenCV/Farneback 用 Go 实现、Python 实现，还是独立二进制工具实现，需要在 Phase 1 前确认。
- 检测结果是否需要入 MySQL，还是先以报告文件落盘。
- 视频来源是本地文件、远程 URL，还是平台已有资源地址。
- 深度检测计算量较大，需要任务队列和并发限制。

## 12. 每阶段更新规则

每完成一部分功能，必须更新本文件：

- 将对应 checklist 从 `[ ]` 改为 `[x]`。
- 更新阶段状态。
- 记录完成日期。
- 记录验证方式和结果。
- 如发现方案变化，在对应章节追加“调整记录”。

### Phase 3 进度更新：黑屏二级与三级确认

更新时间：2026-07-08

已完成：
- 接入 FFmpeg `signalstats`，统计黑屏片段内逐帧平均亮度 `meanY`。
- 接入 FFmpeg `blackframe`，统计黑屏片段内黑像素比例 `blackPixelRatio`。
- 接入 `edgedetect + signalstats`，输出近似边缘强度 `edgeRatio`，用于降低暗场/夜景误判风险。
- 黑屏事件 method 升级为 `blackdetect+luma+blackframe+edge`，证据指标包含 `meanY`、`blackPixelRatio`、`edgeRatio`。
- 默认 `blackMeanY` 调整为 24，以兼容常见视频范围内纯黑 Y 值约为 16 的编码特性。

验证：
- `go fmt ./...` 已执行。
- `go test ./...` 已通过。
- 使用 FFmpeg 生成前 1 秒黑屏、后 1 秒测试画面的视频样例，CLI 可输出 `black_screen` 事件，指标为 `meanY=16`、`blackPixelRatio=1`、`edgeRatio=0`、`confidence=0.95`。

待开发：
- 黑屏片段合并。
- 黑屏证据截图。
- 首黑、尾黑、闪黑分类。
### Phase 3 进度更新：黑屏片段合并、截图与分类

更新时间：2026-07-08

已完成：
- 相邻黑屏片段支持按短间隔合并，避免同一事故被拆成多个事件。
- 黑屏事件支持分类为 `head_black`、`tail_black`、`flash_black`、`black_screen`。
- 当传入 `--work-dir` 时，自动生成黑屏证据截图到 `evidence/` 目录，并在报告中写入 `evidence.thumbnail` 相对路径。

验证：
- `go fmt ./...` 已执行。
- `go test ./...` 已通过。
- 使用 FFmpeg 生成前 1.2 秒黑屏、后 1 秒测试画面的视频样例，CLI 输出 `head_black` 事件，并生成 `evidence/head_black_001.jpg`。

Phase 3 状态：主体完成。后续如需要可以再增强截图策略、片段合并阈值配置化和更多边缘误判样例集。
### Phase 4 进度更新：PTS Frame Drop 与 Freeze 基础检测

更新时间：2026-07-08

已完成：
- 接入 `ffprobe` 逐帧 PTS 读取，输出 `FramePTS` 列表。
- 新增 `framedrop` detector，根据 `ptsGapRatio` 检测 PTS gap，并输出 `frame_drop` 事件。
- 新增 `freeze` detector，接入 FFmpeg `freezedetect`，输出 `freeze` 事件。
- 兼容 FFmpeg 8.1.2 中 `freezedetect` 的分行日志格式：`freeze_duration` 和 `freeze_end` 可以分开输出。
- CLI pipeline 已接入 `framedrop` 和 `freeze`。

验证：
- `go fmt ./...` 已执行。
- `go test ./...` 已通过。
- 使用 FFmpeg 生成 1 秒动态画面 + 2.2 秒静止画面 + 1 秒动态画面的样例，CLI 输出 `freeze_001`，时间为 `1000ms~3200ms`，持续 `2200ms`，置信度 `0.86`。
- FrameDrop 事件生成逻辑已通过单元测试覆盖，真实 PTS 异常素材仍需在样例集阶段补齐。

待开发：
- 实现 SSIM/MSE 基础帧间差异检测。
- 区分 freeze 与 frame drop 的综合判定逻辑。
- 准备真实 PTS 异常视频样例，并做 CLI 验证。
### Phase 5/6/7 进度更新：轻量画面异常与平台入口

更新时间：2026-07-08

已完成：
- 新增 `visualartifact` detector，基于 FFmpeg `signalstats` 输出 `white_screen` 与 `brightness_jump` 事件。
- 白屏阈值调整为 `whiteMeanY=232`，兼容常见 limited range 视频中纯白 Y 值约为 235 的编码特性。
- 新增后端视频质检同步任务接口：`POST /api/video-qc/tasks`。
- 新增报告读取接口：`GET /api/video-qc/tasks/:taskId/report`。
- 新增证据文件读取接口：`GET /api/video-qc/tasks/:taskId/artifacts/*file`。
- 后端通过独立 `video-qc-core` CLI 调用核心引擎，平台只作为适配层。
- 新增前端 `VideoQuality` 页面，支持输入服务器可访问的视频路径或 URL，并展示评分、视频信息、事件与证据图。
- 仪表盘 API 工具区域新增“视频质检”卡片，接入 `dashboard.video_quality.visible` 权限。

验证：
- `video-qc-core`: `go fmt ./...` 与 `go test ./...` 已通过。
- `server`: `go test ./... -run TestVideoQCDoesNotExist` 已通过，用于 Windows 下避开既有 `/usr/bin/true` 测试假设并验证全包编译。
- `npm run build` 已通过。
- 使用 FFmpeg 生成 1 秒动态画面 + 1.2 秒白屏 + 1.2 秒蓝屏样例，CLI 输出 `white_screen_001` 和 `brightness_jump` 事件。

待开发：
- 平台异步任务队列与任务列表。
- 浏览器上传视频文件能力。
- 绿屏、马赛克、花屏等更细画面异常。
- SSIM/MSE 冻结检测和 freeze/frame_drop 综合判定。
- motion vector、optical flow、AV sync 深度检测。
### Phase 5/6/7 进度更新：上传、历史任务与绿屏检测

更新时间：2026-07-08

已完成：
- 后端新增 `GET /api/video-qc/tasks`，用于读取历史任务。
- 后端新增 `POST /api/video-qc/upload`，用于上传视频文件并返回服务器本地输入路径。
- 前端 `VideoQuality` 页面新增视频上传、历史任务刷新和点击历史报告回看。
- `signalstats` 解析扩展 `UAVG`、`VAVG`。
- `visualartifact` detector 新增 `green_screen` 事件。

验证：
- `video-qc-core`: `go fmt ./...` 与 `go test ./...` 已通过。
- `server`: `gofmt` 已针对视频质检相关文件执行，`go test ./... -run TestVideoQCDoesNotExist` 已通过。
- `frontend`: `npm run build` 已通过。
- 使用 FFmpeg 生成 1 秒动态画面 + 1.2 秒绿色画面 + 1 秒动态画面的样例，CLI 输出 `green_screen_001`，指标为 `meanY=80`、`meanU=91`、`meanV=81`。

待开发：
- 后端异步队列、任务取消和运行中进度。
- SSIM/MSE 帧间差异检测。
- 马赛克/花屏/模糊检测。
- AV Sync、motion vector、optical flow 深度检测。
### Phase 6/8 进度更新：异步任务与 AV Sync 基础检测

更新时间：2026-07-08

已完成：
- 后端视频质检支持异步执行：创建任务时可传入 `async=true`。
- 新增任务状态接口：`GET /api/video-qc/tasks/:taskId/status`。
- 新增任务取消接口：`POST /api/video-qc/tasks/:taskId/cancel`。
- 任务元信息落盘到 `report/video_qc/<taskId>/task.json`。
- 前端 `VideoQuality` 页面新增后台执行开关、状态轮询和取消按钮。
- 新增 `avsync` detector，deep profile 下通过 ffprobe stream timing 检测音视频起始时间差和时长差。
- Pipeline 分析阶段支持同时执行 frame detector 和 audio detector。

验证：
- `video-qc-core`: `go fmt ./...` 与 `go test ./...` 已通过。
- `server`: `gofmt` 已针对视频质检相关文件执行，`go test ./... -run TestVideoQCDoesNotExist` 已通过。
- `frontend`: `npm run build` 已通过。
- 使用 FFmpeg 生成音频延迟约 0.7 秒的视频样例，deep profile 输出 `av_sync_001`，`startOffsetMs≈676`。

待开发：
- SSIM/MSE 帧间差异检测。
- 马赛克/花屏/模糊检测。
- motion vector 与 optical flow 深度跳帧检测。
- 更完整的样例集和批量回归。

### Phase 6/7/8 进度更新：帧间差异、深度运动异常与批量回归

更新时间：2026-07-09

已完成：
- `signalstats` 解析扩展 `YDIF`、`YMIN`、`YMAX`，用于帧间差异、亮度范围和疑似花屏/块状异常分析。
- `visualartifact` detector 新增 `freeze_similarity`，作为 SSIM/MSE 级别的轻量替代方案，基于连续低 `YDIF` 检测相似帧卡顿。
- Pipeline 新增事件归并逻辑：当 FFmpeg `freezedetect` 已输出 `freeze` 时，自动去掉高度重叠的 `freeze_similarity`，避免同一段卡顿重复报错。
- `visualartifact` detector 新增 `blur_low_detail`，通过 `edgedetect + signalstats` 的边缘强度检测低细节/疑似模糊画面。
- `visualartifact` detector 新增 `artifact_spike`，基于连续多帧高 `YDIF + luma range` 检测疑似花屏、块状异常、异常纹理突变。
- `artifact_spike` 已收紧为连续多帧才触发，避免正常场景切换产生单帧误报。
- 新增 `deepmotion` detector，在 deep profile 中基于运动信号突变输出 `motion_jump`，作为后续 motion vector/optical flow 的轻量深度检测入口。
- 新增 `scripts/run_regression.ps1`，可在 Windows 上批量生成黑屏、白屏、绿屏、卡顿、音画偏移样例并运行 CLI 回归。

验证：
- `video-qc-core`: `go fmt ./...` 与 `go test ./...` 已通过。
- 批量回归脚本已通过，当前样例输出：
  - `black`: `head_black`, `freeze`, `brightness_jump`, `blur_low_detail`
  - `white`: `freeze`, `white_screen`, `freeze_similarity`, `brightness_jump`, `blur_low_detail`
  - `green`: `freeze`, `green_screen`
  - `freeze`: `freeze`
  - `avsync` deep profile: `av_sync_offset`

当前完成度：
- 默认检测链路已覆盖：文件完整性、解码错误、时间轴错误、黑屏、首尾黑、白屏、绿屏、亮度突变、卡顿、相似帧冻结、PTS 跳帧、低细节/疑似模糊、基础疑似花屏/块状异常。
- 深度检测链路已覆盖：AV Sync 基础偏移、运动突变 `motion_jump`。
- 平台链路已覆盖：仪表盘入口、视频上传、同步/异步任务、取消任务、历史任务、报告展示、证据图访问。

剩余增强：
- 真正的 motion vector 导出分析仍可继续增强，用于降低跳帧误判。
- 真正的 optical flow/RAFT 深度算法仍作为高成本离线增强项保留，目前已先落地轻量 `deepmotion` 版本。
- 后续需要补充真实生产问题视频样例集，用于进一步校准阈值。

### 解耦状态更新：保留独立核心，移除平台入口

更新时间：2026-07-09

已完成：
- 按最新方向撤下平台内视频质检入口，不再保留仪表盘卡片、前端路由、前端页面、前端 API、后端 `/api/video-qc` 路由和平台权限项。
- 保留 `video-qc-core` 独立 Go module，检测能力仍通过 CLI 入口 `cmd/video-qc` 承载。
- CLI profile 支持内置 `default`、`fast`、`deep`，也支持继续传入外部 profile JSON。
- CLI 工具查找逻辑兼容 Windows 与 macOS：Windows 可查找同目录 `ffmpeg.exe`，macOS 可查找同目录 `ffmpeg`。
- 本次不再保留打包脚本和发布产物；后续如果需要 M1 Mac 二进制，可从 `video-qc-core` 直接按 `GOOS=darwin GOARCH=arm64` 构建。

验证：
- 平台目录扫描无 `video-qc`、`video_quality`、`VideoQuality` 残留引用。
- `video-qc-core`: `go fmt ./...` 与 `go test ./...` 已通过。
- `server`: `gofmt` 已针对入口/权限文件执行，`go test ./... -run TestVideoQCDoesNotExist` 已通过。
- `frontend`: `npm run build` 已通过。
