# ChangeLog - Settings (配置中心)

## 2026-04-24

### Added
- **Permission Management**: 系统设置新增“权限管理”页面，支持按平台真实用户配置仪表盘、报告中心、系统设置入口权限。
- **Permission Store**: 前端新增权限状态管理，路由守卫、主菜单、仪表盘卡片均可复用同一套权限判断。
- **Minghong Admin Guard**: `minghong` 作为权限管理员默认拥有并锁定“权限管理”访问权限，避免唯一管理员被误关闭。

### Changed
- **Permission Cards**: 权限项从开关组件调整为固定尺寸小卡片，点击卡片本身切换开启/关闭状态。
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
