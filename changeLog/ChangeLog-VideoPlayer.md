# ChangeLog - Video Player

## 2026-04-24

### Added
- **Video Player Page**: 新增视频播放器页面，包装 `bbys.app` 视频播放入口。
- **Float Player Mode**: 新增视频播放器浮窗组件和 Pinia store，支持在主布局中以摸鱼浮窗方式显示视频入口。
- **Dashboard Integration**: 视频播放器入口接入仪表盘“其他拓展”分区，并受权限管理控制。

### Changed
- **Navigation Scope**: 视频播放器入口不再作为左侧菜单项展示，减少侧边栏复杂度。
- **Header Simplification**: 视频播放器页面顶部去除“网页包装”“视频播放”和说明文案，只保留“布布影视”标题。
- **Compact Layout**: 缩小页面顶部包装区域高度，提升 BBYS iframe 可视区域占比。
- **Action Button Consistency**: 统一“摸鱼模式”和“新窗口打开”按钮尺寸，避免两个操作入口视觉大小不一致。
