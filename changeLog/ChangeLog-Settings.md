# ChangeLog - Settings (配置中心)

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
