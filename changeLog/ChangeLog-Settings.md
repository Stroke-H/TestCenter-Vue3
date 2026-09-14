# ChangeLog - Settings (配置中心)

## 2026-09-11

### Changed
- **Settings UI Modernization**: 系统配置中心 4 大模块页面全面完成现代化重塑。
  - **项目配置 (ProjectConfig)**：引入标准 Page Header 与 `.content-card` 容器，表格采用等宽项目代号徽标与标签，优化新增/编辑对话框样式。
  - **设备管理 (DeviceConfig)**：顶部引入测试设备总数、Android、iOS 设备三栏式状态统计与快捷切换卡片，更新操作系统药丸标签与表格布局。
  - **账号配置 (AccountConfig)**：引入可登录账号、飞书已绑定、待绑定飞书统计卡片，表格集成首字母用户头像与绑定状态胶囊。
  - **权限管理 (PermissionManagement)**：重构 Hub 侧边栏导航与快捷入口偏好卡片，卡片点击交互、选中高亮与计数徽标视觉整体对齐。

## 2026-06-09

### Changed
- **Unified Monkey Permission**: 权限管理中的 Monkey 测试收束为一个卡片，统一控制入口、执行、结果查看和无线 ADB 操作，不再单独展示 Monkey 子动作权限。

## 2026-06-02

### Added
- **Monkey Wireless ADB Permissions**: 权限管理新增无线 ADB 配对、连接和断开权限，前端操作入口与后端接口同步校验。
- **Project Tree Permission**: 权限管理新增“项目树”独立权限，仪表盘卡片、路由访问和最近使用入口都会同步受控。

## 2026-05-21

### Added
- **Monkey Test Permission**: 权限管理新增“Monkey 测试”独立权限项，可单独控制仪表盘性能测试分区中的 Monkey 测试入口显示。
- **Monkey Run Permissions**: 权限管理新增 Monkey 查看、启动、停止权限，真实设备执行接口会同步校验后端权限。
- **Device OS Quick Filter**: 测试设备列表的“设备信息”表头新增 iOS / Android 快捷筛选，可与搜索框叠加使用，方便快速区分不同系统设备。

### Changed
- **Element Plus Radio Compatibility**: 设备系统筛选与系统单选控件改用 `value` 绑定，消除 Element Plus 3.0 radio API 弃用警告。

## 2026-05-06

### Added
- **Personal Settings**: 顶部用户菜单新增独立“个人设置”页面，支持当前登录用户修改头像、昵称和邮箱，并在保存后立即同步右上角展示信息。

### Changed
- **User Menu Behavior**: 修复“个人设置”误触发退出登录的问题，个人资料与退出登录改为独立命令流转。
- **Avatar Entry Placeholder**: 个人设置页头像入口暂时改为占位提示，点击“更换头像 / 移除头像”会明确提示功能未开发完成，避免误触半成品流程。

## 2026-04-24

### Added
- **Permission Management**: 系统设置新增“权限管理”页面，支持按平台真实用户配置仪表盘、报告中心、系统设置入口权限。
- **Permission Store**: 前端新增权限状态管理，路由守卫、主菜单、仪表盘卡片均可复用同一套权限判断。
- **Minghong Admin Guard**: `minghong` 作为权限管理员默认拥有并锁定“权限管理”访问权限，避免唯一管理员被误关闭。

### Changed
- **Permission Cards**: 权限项从开关组件调整为固定尺寸小卡片，点击卡片本身切换开启/关闭状态。
- **Permission Autosave**: 权限管理移除“保存权限”按钮，点击权限卡片后立即自动保存；保存失败时回滚本地状态并提示用户。
- **Default Permissions**: 新用户默认关闭权限管理、斗兽棋、小说阅读器、视频播放器等非基础入口，权限管理仅 `minghong` 默认开启。
- **User List Display**: 权限页用户列表只显示平台用户名，去除 user_id 等辅助信息。

## 2026-03-24

### Added
- **Configuration Center**: 集中管理平台核心元数据。
- **Account Configuration**: 提供了当前登录用户的权限概览及基础信息。
- **Device Configuration**: 
    - **Inventory Management**: 实现了测试设备的增删改查。
    - **Capability Matrix**: 支持录入设备的 OS 类型、具体型号及其允许挂载的应用列表 (Allowed Apps)。
- **Project Configuration**: 
    - **Mapping Layer**: 维护了“项目代码”、“项目全称”与“缩写”的多位映射关系。
    - **Dynamic Filtering**: 在全系统（如验收报告）中实现基于此配置的项目自动匹配与补全逻辑。

### Changed
- **UI Interaction**: 
    - 统一采用侧边 Tab 切换模式，优化大型配置集的交互。
    - 列表页集成了基于 Element Plus 的对话框 (Dialog) 实现快捷编辑。
- **Data Safety**: 
    - 项目代码在编辑模式下设为只读，确保底层数据引用链的稳定性。
