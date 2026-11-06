# ChangeLog - Dashboard

## 2026-11-03

### Updated
- **Delete Account Method Tabs**: 删号标题下新增UID与request head双页签，分别展示后台查询或请求头填写入口。UID模式必填UID且精确匹配，request head模式不读取残留UID；两种方式切换时独立暂存请求配置、项目及环境，查询/执行期间禁止切换，原人工确认保留。
- **Delete Account UID Lookup**: 删号卡片新增可选UID及只读查询带入。填写UID后必须重新选择项目；服务端使用已配置运营后台账号进行pwd_login，携带x-token精确查询app和UID，日期固定从2025-01-01至访问当天（北京时间）。仅白名单映射设备、广告ID、版本及设备环境字段，不复用后台session/refresh token。带入两张独立Case并清理旧设备标识；执行前重新查询，匿名登录返回用户ID与目标UID不一致时禁止进入删除确认，原人工确认步骤保留。
- **Custom Domain Isolation**: 自定义客户端域名与运营后台查询环境分离，后台账号不会发送到自定义域名；UID模式下允许选择/输入项目名，缺少项目或有效设备标识时不执行。无预设客户端域名的项目需明确填写实际域名。不改变短剧接口或其他工具行为。

## 2026-10-27

### Updated
- **Payment Quick Status Actions**: 支付流水“未退款”支持点击确认变更为已退款；“未发起 / 已发起”支持点击选择其他申报状态，更新后同步飞书多维表格并刷新余额和筛选结果。已退款与申报完成保持普通点击打开详情。
- **Status-only Update**: 新增只更新指定状态的接口，校验原状态并锁定记录，避免快捷操作覆盖金额等其他字段；同步失败时保留本地状态和失败原因。

## 2026-10-14

### Added
- **Payment Records**: 左侧导航在仪表盘下新增“支付记录”，提供支付记录列表、新增记录、关键词/日期/支付方式筛选、分页与累计/本月金额概览；记录持久化保存支付时间、支付人、金额、支付方式、消费应用、备注及平台创建人。
- **Payment Record Feishu Sync**: 新增或编辑支付记录时自动创建或更新指定飞书多维表格行，首次使用会初始化支付字段；平台保留同步状态和错误，飞书临时异常不会阻断本地记录落库。

### Changed
- **Virtual Card Balance Overview**: 支付记录页面第三张概览卡改为“虚拟卡余额”，当前暂定展示 500 美元可用余额。
- **Payment Provider Options**: 支付方式选项统一调整为 `Stripe`、`Breeze`、`Adyen` 和 `Paypal`，新增记录默认选择 `Stripe`。
- **Payment Amount Presets**: 新增支付记录的金额改为 `$3.99`、`$4.99`、`$9.99`、`$19.99` 四档快捷选择，并保留“自定义”金额输入。
- **Consumer App Selection**: “交易 / 订单号”改为“消费应用”，提供 ShortsWave、Mangogo、CocoShorts 的 iOS/Android 双端选项及自定义输入。
- **Payment Record Maintenance**: 支付记录支持编辑，并新增“退款状态”和“对公支付申报状态”；虚拟卡余额按 500 美元初始额度扣减未退款记录金额，已退款记录自动恢复余额。

## 2026-07-02

### Added
- **Windows Migration Guide**: 新增 Windows 迁移部署说明和 PowerShell 辅助脚本，支持从 `windows` 分支拉取代码后，一键还原私有配置包并按固定路径启动前后端服务。
- **Windows Codex Handoff README**: 新增 Windows 端 Codex 接手说明，明确迁移包还原、启动、验证和隐私文件禁止提交规则，降低迁移后接手成本。

## 2026-06-26

### Added
- **Assistant Hover Fan Menu**: 智能助手悬浮按钮支持鼠标停留 0.5 秒后展开更紧凑的快捷入口菜单，图标独立成块、文案透明底显示；入口改为三枚上排水平对齐、一枚左下补位的布局，上排最右图标右边框与智能助手按钮右边框对齐，并将上排与左下入口收近到距离智能助手约 10px。

## 2026-05-28

### Changed
- **Project Tree Entry Rename**: 仪表盘“测试流程工具”分区中的“验收项目树”入口更名为“项目树”。
- **Project Tree Card Copy**: 项目树卡片文案调整为“项目记录，快速查看测试时间与需求点”，并自动迁移最近使用里的旧名称缓存。

## 2026-05-27

### Added
- **Acceptance Project Tree Entry**: 仪表盘“测试流程工具”分区新增“验收项目树”入口，用于快速进入验收报告项目树视图。

## 2026-05-26

### Added
- **Monkey Hologram Demo Entry**: 仪表盘“性能测试”分区新增“Monkey测试”入口，进入执行页后可配置包名、设备、事件数、throttle、截图间隔和 seed，模拟 `adb shell monkey`、`logcat` 与 `screencap` 管线，并在执行完成后生成可点击预览截图的原子结构全息图谱。

### Changed
- **Monkey 3D Space Model**: Monkey 图谱从 CSS 伪 3D 升级为 Three.js WebGL 空间模型，支持拖拽旋转、节点点击拾取、风险节点发光、中心 App 标识、球面节点分布、标签缩放与截图预览联动。

## 2026-04-28

### Changed
- **TestCase Card Routing**: 仪表盘“测试用例生成”卡片改为直接跳转到“智能测试用例生成”页面，不再默认进入历史记录页。

## 2026-04-24

### Added
- **Video Player Entry**: 在仪表盘“其他拓展”分区新增视频播放器入口，支持进入视频播放页面与摸鱼浮窗模式。
- **Permission-aware Cards**: 仪表盘卡片接入权限配置，API 工具、流程验证、测试账号管理、UI 自动化、用例生成、节点 Skill 化、性能测试、斗兽棋、小说阅读器、视频播放器等入口会按用户权限显示或隐藏。

### Changed
- **Sidebar Cleanup**: 视频播放器不再出现在左侧侧边栏，只保留仪表盘入口。
- **Report Center Menu**: 报告中心菜单与用例报告入口按权限配置统一控制。

## 2026-03-30

### Added
- **Web前端压测入口**: 在性能测试分区首位新增了“Web前端压测”工具卡片，支持快速发起 Lighthouse 性能审计。
- **Card Optimization**: 优化了性能测试分区的卡片顺序，将高频使用的 Web 压测工具调整至靠左第一个位置。

## 2026-03-24

### Added
- **Global AI Assistant v2**: 实现了常驻右下角的悬浮 AI 助手，支持全局唤起。
- **Interactive Report Editor**: 新增了全局弹窗式验收报告编辑器，支持在 AI 生成后进行实时编辑、确认与持久化存储。
- **UI Avoidance Logic**: 在编辑器打开时，悬浮助手会自动淡出缩放以避免视觉遮挡及交互冲突。

## 2026-03-23

### Added
- **Feishu Assistant Dashboard**: 将原“测试用例管理”重构为“飞书助手监控大盘”。
- **Session Cycles Tracker**: 实现了会话周期追踪表，聚合显示聊天频次、最后活跃时间及会话状态。
- **Interactive Chat Drawer**: 新增了会话详情抽屉，支持查看完整上下文历史及 AI 实时回复联调。
- **Bot Subsystem Overview**: 在大盘顶部增加了 Bot 状态看板（总用户数、活跃会话等）。

## 2026-03-19

### Added
- **Account Deletion Parameter**: 在“删除账号”工具页面新增了“填入参数”输入框，支持用户在执行操作前输入必要的参数。
- **Auto Project Matching**: 实现了参数自动解析功能，当输入包含 `app` 标识的参数串时，系统会自动匹配并切换“项目”字段（支持 NovelNova 和 ShortsWave）。
- **Anonymous Login Proxy**: 实现了账号删除工具的匿名登录功能。通过 Go 后端代理绕过 CORS，并支持全量 Header 透传，登录结果直接以格式化 JSON 形式在前端弹窗展示。
- **Interactive Deletion Flow**: 完善了注销账号的交互流程。登录后会提取并高亮显示 `user_id` 和 `user_name` 进行二次确认；确认后会自动使用最新的 `session_token` 完成注销请求，并将结果实时输出至日志区。
- **Multi-Env Support**: 支持了 ShortsWave 和 NovelNova 的多环境切换（测试服/正式服），系统会根据“测试服务器”下拉框自动映射对应的 IP 或域名。

## 2026-03-13

### Changed
- **UI Redesign**: 按照参照图要求，将深色主题仪表盘彻底重构为浅色工具启动台（Light-themed Tool Launcher），引入分类网格布局、互动卡片阴影动画和 Element Plus 主题定制。
- **Card Categories**: 将全量卡片重新组织为：`Recently Used`、`API Tools`、`UI Automation` 以及 `Performance`。
- **Responsive Layout**: 取消了 Dashboard 中最大宽度 1200px 的限制，现实现自适应宽度填满内容区。

### Added
- **Dynamic Recently Used Module**: 实现了在 `Recently Used` 下只动态显示最新执行过的最多四个功能卡片（初始不使用时隐藏该区块）。
- **LocalStorage State Caching**: 为 Recently Used 增加了浏览器本地缓存，功能使用顺序在刷新后能保持一致并提供 "Clear History" 清理功能。
- **Navigation Linking**: 所有的卡片动作按钮（Launch、Open 等）已经成功挂载全局路由 (`router.push`)，跳转至 `/com_api_commit` 页面，并传递 `name` 与 `desc`。
- **New API Tool Card**: 在 API Tools 分区首位插入了新的 `剧集播放接口测试` 工具卡片，用于作为后续 K6 性能测试页面的入口。
