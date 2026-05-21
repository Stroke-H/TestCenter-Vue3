# ChangeLog - NovelWriter

## 2026-05-21

### Removed
- **Novel Writer App Removal**: 按当前产品边界移除小说智能生成在 TestCenter 内的前端入口、路由、权限模块、UI 组件、前端 API 封装和 `/api/novel-writer` 后端服务注册。
- **Novel Writer Service Removal**: 移除小说项目、大纲、文风、章节生成、审计和修订相关后端服务文件，保留本地小说阅读器等其他拓展功能不受影响。

## 2026-05-12

### Added
- **Open Novel Writing Skill Assets**: 项目级 `.agents/skills/open-novel-writing` 纳入小说写作 skill 资料，便于后续生成策略和团队协作共享。
- **Chapter Spec Outline**: 大纲生成升级为章节规格生成，新增章节摘要、前后状态、必发事件、张力曲线、关键场景和新增钩子等结构化字段。

### Changed
- **Skill-guided Chapter Generation**: 章节正文生成接入项目级小说写作规则，基于章节规格、前 3 章上下文、事实库、文风画像和长期记忆生成正文，并强化 AI 味红线、动作细节和节奏要求。
- **Chapter Outline Preview**: 分章生成左侧大纲卡片补充展示章节摘要、必发事件和关键场景，方便生成前确认章节规格。
- **Material Character Roles**: 素材图谱添加人物改为角色类型选择，支持男主角、女主角、主要配角、次要配角、重要 NPC 和普通 NPC，并在人物模板中补充性格字段。
- **Conflict Character Selection**: 添加冲突前必须先选择冲突人物；单选人物生成自身冲突，多选人物生成人物间冲突卡片。
- **Material Role Editing**: 人物卡片左上角角色标签支持直接下拉修改，并同步写回素材文本。
- **Material Card Edit Mode**: 人物角色卡编辑完成后切换为展示态，右下角提供编辑按钮，编辑页拆分为角色类型、名称、性格、身份、欲望、弱点独立输入项。
- **Canvas Drag Gesture Support**: 素材图谱节点支持鼠标左键拖动，画布支持 Mac 触摸板两指平移和缩放。
- **Outline Batch Generation**: 大纲生成保持完整章节规格不降级，首屏先返回第 1 章可用大纲，剩余章节在后台按 5/3/1 自适应批次继续生成并展示加载状态。
- **Novel AI Error Detail**: 小说 AI 请求失败时保留后端具体错误，并附带 provider、model、prompt 长度、输出长度和已知上下文规格，便于定位模型、网关或 JSON 截断问题。
- **Outline Background Save Fix**: 后台续生成章节大纲时只更新大纲字段，避免覆盖用户刚生成、审计或修订完成的章节正文。
- **Outline Output Budget Fix**: 后台分批大纲生成提高输出 token 预算，避免单章完整规格因 JSON 被截断导致后续章节生成失败。
- **Book Cover Card Layout**: 小说创作入口项目卡片参考书籍横幅样式，改为三列 32% 紧凑封面卡，底部操作在封面右侧横向排列。

## 2026-05-11

### Added
- **小说智能生成 MVP**: 新增小说创作工作台，支持小说项目创建、素材输入、信息提取、大纲规划、文风画像、分章生成、章节审计、智能修订、人工确认和 Markdown 导出。
- **Novel Writer Backend**: 新增 `/api/novel-writer` 后端接口组，使用 SQL `novel_projects` 表保存项目、素材、结构化事实库、文风画像、章节版本、审计结果和长期记忆。
- **MiMo Writing Flow**: 小说生成链路优先使用 `novel_writing` 能力配置，未配置时回退到现有 MiMo 轻聊/工作聊天能力，避免新增模型配置成为阻塞。

### Changed
- **Dashboard Extension Entry**: 仪表盘“其他拓展”新增“小说智能生成”入口，权限模块新增 `dashboard.novel_writer.visible`，默认仅 `minghong` 开启，其他用户保持关闭。
- **Material Map Workspace**: 素材输入拆分为独立大页面，将人物信息、世界观、重点冲突事件和灵感片段改为思维导图式卡片布局，并把文风参考文本及后续生成流程移动到“文风与生成”分页。
- **Material Flow Semantics**: 素材图谱进一步调整为“世界观基础层 → 多人物节点 → 人物冲突连线”的主流程，并将灵感片段移入独立灵感池，支持拖拽灵感卡片追加到主流程冲突事件。
- **Canvas-first Novel Workspace**: 小说智能生成页移除项目侧边栏和顶部 Hero，改为以当前小说名开头的创作画布，并支持在人物、冲突和灵感卡片内直接编辑内容。
- **Mind-map Canvas Interaction**: 素材图谱改为参考必测流程验证的大画布模式，支持网格画布、空白处拖拽平移、Ctrl/Cmd 滚轮缩放、工具栏缩放复位、节点卡片和 SVG 连线。
- **Novel AI Request Robustness**: 小说写作类 AI 请求前端超时从通用 15 秒提升到 120 秒，生成章节时会自动补齐信息提取和章节大纲，避免无大纲时直接报错。
- **Style Generation Workspace**: 文风参考文本改为按需添加，支持手动输入和 txt 文件上传；未提供参考时由模型按题材和事实库生成默认原创文风画像。
- **Outline Generation Layout**: 生成大纲后将事实卡片和文风画像收纳为右上角悬浮面板，首次生成大纲后默认展开；大纲/结构规划并入分章生成左侧栏，保留每章目标、冲突和钩子说明。
- **Novel Header Simplification**: 小说工作台顶部精简为仅展示当前小说名，移除“小说智能生成”副标题以及新建小说、导出 Markdown 顶部按钮。
- **Style Workspace Detail Tuning**: 小说名上移并放大展示，素材图谱/文风生成切换改为半透明悬浮按钮；文风参考入口并入操作区，标题旁展示“已提供参考/无参考”状态。
- **Chapter Draft Reading Area**: 分章生成区左侧大纲栏与右侧正文预览高度对齐，左侧超出内容支持独立滚动，右侧正文预览区向下扩展约 100px。
- **Novel Project Entry Restored**: 点击仪表盘“小说智能生成”后先进入小说项目列表，支持继续编辑已有小说、新建小说、返回列表，并将 Markdown 导出能力移动到小说卡片操作区。
- **Novel Project Management**: 小说入口页新增显式“编辑”按钮和删除能力，删除前进行二次确认，并补齐 `/api/novel-writer/projects/:id` 删除接口。
- **Material Map Node Editing**: 素材图谱中人物、冲突、灵感节点新增删除操作；灵感卡片只有拖放到人物卡片上才会转成冲突，拖到空白区域不再误生成冲突卡片。
- **Novel Cover Status Badge**: 小说入口卡片阶段状态并入底部信息胶囊区，避免“事实库已生成”等状态在卡片顶部挤压成竖排。
- **Novel Editor Header Cleanup**: 小说编辑页移除当前小说名标题展示，顶部布局改为参考素材图谱视图的居中悬浮切换按钮。
- **Workspace Switch Alignment**: 素材图谱/文风生成切换按钮改为标题行内水平居中，避免固定顶部导致按钮出框。
- **Outline Entry Guidance**: 素材图谱“下一页：文风生成”改为“生成大纲”，进入文风生成页时如未提供文风参考会提示是否使用模型默认文风，取消则自动打开参考文本弹窗。
- **Material Preview Carry-over**: 从素材图谱切到文风生成页时，会将当前人物、世界观、冲突和灵感内容作为事实卡片预览展示，文风画像区域也会显示参考状态占位。
- **Material Preview Merge Fix**: 事实卡片预览改为合并后端事实库与当前素材图谱内容，避免已生成事实库后新增人物或灵感无法在文风生成页展示。
- **Material Preview Naming Fix**: 素材图谱本地预览卡片会从“新人物：名称”等首行提取真实名称，并在切到文风生成页前保存最新素材，避免显示“人物 1/人物 2”或旧数据。
- **Material Preview Deduplication**: 事实卡片合并去重升级为内容归一化匹配，避免人物、冲突、世界观同时展示后端事实库和本地素材预览两份重复数据。
- **Material Preview Compatibility Fix**: 事实卡片合并兼容旧项目中的非数组事实库字段，避免从素材图谱切换到文风生成时报 `source is not iterable`。
- **Insight Floating Button Visibility**: 事实卡片/文风画像悬浮按钮不再依赖章节大纲存在，切到文风生成页即可显示并默认展开。
