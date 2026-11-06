# ChangeLog - K6 Integration

## 2026-11-06

### Updated
- **Paged reports / conditional Feishu detail**: 剧集报告及完整错误页改为轻量HTML入口、后端分页（默认50、上限100）、分类/全文搜索及点击加载证据；当前页读取处理标记，共享菜单，完整错误集合精确忽略不变。旧报告URL、语种页URL兼容，保留原始HTML入口和待复核语种证据；读取仅依赖归档，不访问实时运营后台。飞书播放错误保留完整输出；分组/分类/语种/版权按分类明细合计<=30保留原明细，>30摘要化，底部提供动态完整错误链接；免费集数及其他异常不变。超长文本按实际卡片预算无损分段，不再截断播放错误。未重启后台、未发送正式消息。
- **Exact overseas naming marker (v4.7)**: ad出海特殊分组规则由包含出海二字收紧为完整(出海)/（出海）标识。普通剧名含出海、(出海版)等不触发点石出海独占要求，继续按其他适用规则检查；番茄版权及特殊规则冲突策略不变。同步只读规则说明、规则及收集阶段回归；不修改历史报告、不重启后台或启动线上测试。

## 2026-11-05

### Updated
- **Hong Kong IAA media naming (v4.6)**: ad剧名同时完整包含(IAA)、(媒资库)、(香港主体)时，白名单组仅允许TT_IAA_媒资库组（香港主体）；兼容中英文括号、IAA大小写及标识顺序。该分支优先于普通IAA媒资库及美欧，番茄内供与DS优先级不变；无香港主体的普通IAA媒资库规则不变。规则说明及收集阶段回归同步，不改运营后台数据、不启动线上测试。
- **Collection-time metadata checks (v4.5)**: 收集阶段一次完成分组/解锁类型、免费集数、版权和配音分类判断；独立保存并归档初检/复验字段规则结果。k6只合并预计算结果，保留本地Go简介识别结果判定与播放检查；不改变业务规则、分类日期限制、二次确认或飞书报告路径。复验仅计算候选配音及不确定分组，明确字段错误不重复执行。SharedArray改为小型头信息及逐剧记录，正常简介不复制原文，取消各VU重复扫描分组规则；版本/阶段/开关快照不一致明确拒绝。未修改HTML渲染方式、未重启后台、未发起线上任务。
- **Direct explicit-data findings / selective confirmation (v4.4)**: 分组组合不符、unlock_type空/未知、免费集数及版权异常初检直接记入报告，不再入复验队列；同剧因其他原因复验时不重复执行这些明确项，保留初检结果并合并后续配音/语种/抓取异常。仅配音分类、简介语种、播放抓取失败和分组元数据不确定保留确认；元数据恢复后补做此前无法判定的分组检查，避免漏检。传递元数据复验子集并记录重试策略，规则说明同步；不改报告渲染、不自动发起测试。
- **Whitelist-only membership validation (v4.3)**: 剧集命中任一白名单组后，所有分组独占/互斥判断仅考虑白名单组；外部联运等已知非白名单组忽略。保留完整分组元数据核验，无法解析的ID不当作非白名单跳过；筛选归档记录原始/参与/忽略组，失败详情注明已忽略组。38251示例及普通coin/ad、媒资库、番茄版权优先分组均增加回归；说明和ruleScope同步，不启动线上任务。
- **Ad IAA media exclusive rule (v4.2)**: unlock_type=ad且剧名含媒资库与完整(IAA)标识时，要求有且只有TT_IAA_媒资库组。兼容中英文括号及IAA大小写；保留番茄内供最高优先、DS和东南亚的具体分支优先。分组检查不受分类created_at日期限制。规则说明和回归同步，未启动线上任务。
- **Dubbing created_at cutoff (v4.1)**: 仅配音分类（X配/X全剧配音语种及分类）限定created_at>=2026-06-01，按业务+08:00解析；更早、缺失或无效日期跳过该项。播放、简介语种、分组、版权和免费集数不受日期限制。复用严格日期解析，覆盖零点、UTC换算及无效日历边界；规则说明及结构化执行范围同步。
- **Drama Policy v4 / Scope First**: 仅15个指定分组参与；至少命中一个即纳入，完整保留所有实际分组校验。准备阶段过滤范围外剧集，语言准备与播放不触达范围外；无法判定范围明确失败。归档初检/复验筛选清单，移出在线白名单的候选不复验、不视为恢复、不进入最终飞书异常；初检证据保留。
- **Group Rules**: 普通coin允许四组任意非空子集；普通ad仅独占TT-Minis或IAA/漫剧产品组-IAA/网赚三组任意非空组合，禁止混用或额外组。DS媒资库东南亚/普通DS分别绑定分销媒资库香港主体/普通分销媒资库，优先于泛化区域规则；番茄内供copyright值最高优先且独占ShortsWave。保留出海、互动、coin媒资库后缀等现行特殊约束。
- **Copyright / Dubbing**: 补齐copyright及copyright_expire_at获取/合并；版权日期按明确时区或业务+08:00解析，严格晚于检查时间，缺失/无效/过期均异常，新增飞书版权异常分类。X配与X全剧配音核对lang及配音剧标签；取消无X标识的反向检查、内容五部及其他旧豁免。空简介继续跳过，ad免费集数=2，播放检查不变。全部规则重新启用，只读说明同步。
- **Verification**: 本地模拟后台CSV/JSON准备、真实k6初检/复验及回归验证，不启动正式服任务、不重启后台；大批量HTML渲染改造继续暂停。

## 2026-11-04

### Updated
- **Temporary three-check scope**: 暂时仅保留播放链接/章节可用性、简介语种和X配语种/配音分类检查。分组检查暂停，并新增独立解锁类型与免费集数跳过开关；包括空/未知unlock_type和App Group元数据异常，初检与复验均不入队、不上报。HTML执行范围、结构化ruleScope及只读规则说明同步标明暂停；原内容五部仅播放策略不变。规则定义保留，便于恢复；不自动启动线上任务。
- **Business Rules v3 clarification**: 普通ad恢复原合法广告分组白名单，不再强制包含TT-Minis分销组；仅MD-产品组的历史允许方式保留。媒资库目标组映射维持不变，媒资库命名同时含IAA与东南亚时由东南亚优先，coin三种精确组合及其他检查不变。规则说明同步到v3。
- **Phase failure guard**: 初检/复验完成名单独立命名，避免读取上一阶段遗留文件；保留k6脚本/汇总错误的有界诊断，识别退出码为0的handleSummary失败。阶段执行或覆盖失败不再由外层定时任务整轮重跑。HTML大报告渲染改造按用户要求暂停，既有120秒汇总超时风险尚未通过渲染方案解决；未重启服务或新建线上任务。
- **Execution Coverage / Continuation**: 初检与复验的单批 k6 时限从10分钟延长到30分钟。通过跨VU聚合的完成标记生成实际完成名单；Go端核对完整待测集合，仅续跑未完成项，每阶段最多额外两轮，连续两轮无进展或仍未完成则明确失败。续跑候选与异常合并回原报告，新增覆盖数和归档清单；缺少名单、名单不符或续跑规则变更均拒绝判为完成。大量复验ID改用独立JSON文件传递，避免环境变量长度限制。未重启后台或额外发起线上任务。
- **Business Rules v2**: 按确认方案重建分组规则，coin仅允许三种精确组合且不兼容coins；特殊命名独占组覆盖普通组判断。配音命名同时核对lang与配音剧分类，印配=id；取消旧DS、出海直接PASS、番茄及创建日期豁免。新增后台free_chapters准备/缺失标记、ad必须等于2规则和独立报告分类。内容五部仅保留播放检查，其他四项含语种准备均策略跳过。业务异常初检加入复验队列，持续异常才上报。重新开启分组及配音检查开关；本次不自动发起线上任务或修改历史报告。
- **Temporary Rule Scope**: 用户确认暂时跳过分组与配音剧分类校验，新增k6配置开关并默认开启跳过；不执行分组元数据、分组命名、分组/特殊命名绑定解锁策略及配音分类双向判断。保留unlock_type空值/未知值、剧名规范、常规播放健康与简介语种检查；初检及二次确认一致生效。HTML报告注明跳过范围，结构化结果记录ruleScope，原飞书报告路径不变。恢复时将两个skip开关设为false。

## 2026-11-02

### Updated
- **Language Evidence Audit**: 复核今日剧集报告：发现日文中反复内嵌拉丁人名导致混合文字占比误判。仅从文字占比统计中排除被中日韩文字直接夹住的短 TitleCase 拉丁姓名，不建立人名或剧集白名单，实际外语段落继续检测。识别前移除 URL 证据，原文和内容指纹不变；新增真实样本、错误 lang、真实双语、普通词、全大写、URL 与正文相邻等回归。升级缓存版本以在下次检测自动重算；不改变历史报告或其他检查规则。

## 2026-10-30

### Updated
- **Conservative Local Language Screening**: 仅调整简介语种链路：id/ms、es/pt 冲突降为待复核；添加混合文字/语句一致性检查及 it/vi/nl/pl 范围外拒绝候选，阿拉伯语转写经验规则降为候选。待复核不计通过、不自动发送飞书语种异常，HTML 报告展示完整待复核明细；持续不一致注明本地疑似结果。升级缓存版本，旧结果下次执行自动重算。刷新元数据不被当作独立语言验证，不调用外部付费 API。

## 2026-10-27

### Updated
- **Arabic Transliteration Detection**: 新增阿拉伯语拉丁转写特征识别，组合校验专用转写字符、冠词、功能词与文本覆盖比例；证据充足时识别为 ar 后再与 lang 比较，弱转写候选记为无法确定。规则独立于预期 lang 和剧名，升级缓存版本以重算旧识别结果，修复剧集 6adc37876f0b5c8c13d743ce 被误报为印尼语的问题。
- **Language Error Details**: 简介语种异常独立归类为“语种异常”，飞书只显示识别语种与 lang 的简短结论，分类标题提供“查看异常明细”链接；新增按历史报告归档读取的免登录只读明细页，展示完整简介、语种、剧集信息并支持搜索。
- **Feishu Card Size Budget**: 剧集报告按实际消息请求体大小拆分卡片，避免大量异常导致超过飞书大小限制后退化为纯文本。完整简介和内容指纹继续保留在原始报告中，不影响暂时忽略的精确匹配。

## 2026-10-26

### Added
- **Description Language Validation**: 使用 Lingua-Go 本地识别 en、ja、es、id、fr、ko、th、ru、ms、zh、ar、pt、de、tr 共14种语种，比较在线剧集 description 与 lang。空简介或单个 / 跳过，短文本或低置信度记为无法确定；疑似不一致和 lang 字段异常刷新元数据后二次确认，持续异常进入 HTML Other Checks 与飞书“其他异常”。
- **Full Online Coverage**: 简介语种判断先于临时直接 PASS，出海及 DS媒资库东南亚剧集也参与；不受分类检查日期限制。
- **Local Detection Cache**: 按简介内容与检测器版本缓存预测，每轮重新比较最新 lang；手动、定时与复验入口共用准备流程。报告显示检查覆盖量、跳过 / 无法确定数量、缓存命中和识别耗时；元数据未获取成功时明确失败，避免误当空简介放行。

## 2026-09-29

### Updated
- **DS Distribution Media Group Rule**: 新增独立的 `DS媒资库` 命名规则及 `TT_IAA_分销_媒资库` App Group。只有 `cn_name` 带 `DS媒资库` 标识的剧集必须且只能配置这一个分组，并要求 `unlock_type=ad`；普通 `DS` 剧集仍沿用 `TT_IAA_媒资库组` 或 `TT-Minis 分销组` 二选一的既有规则。剧集播放测试的只读规则配置说明同步更新。

## 2026-09-21

### Updated
- **Dianshi Overseas Drama Group Rule**: 新增 `点石出海短剧` App Group，按 IAA 类广告分组执行解锁校验；仅 `unlock_type=ad` 通过，`coin` 或 `coins` 均上报解锁类型异常。
- **Drama Rule Configuration Copy**: 剧集播放测试的只读规则配置按实际执行代码重新整理，逐项列明 IAA、AIGC、TT-Minis、网赚短剧、TT_IAA/TT_IAP 媒资库、MD、ShortsWave 等分组的解锁类型要求，并补充 DS 独占、未识别分组和多分组处理说明。

## 2026-09-18

### Updated
- **DS App Group Rule**: `cn_name` 包含 `DS` 时，App Group 必须有且只有一个，允许 `TT_IAA_媒资库组` 或 `TT-Minis 分销组`。两者同时存在、混入其他分组或出现 `TT_IAP_媒资库组` 时继续上报分组异常。

## 2026-07-23

### Updated
- **DS Exclusive Group Rule**: 当剧集 `cn_name` 包含 `-DS` 时，App Group 必须且只能配置为 `TT-Minis 分销组`；未配置该分组或同时配置其他分组时，剧集播放接口检测会报告分组规则异常。
- **Environment-aware Group Lookup**: App Group 列表接口会跟随当前剧集检测环境，不再使用固定正式服地址；`-DS` 剧集命中专属规则后不再重复执行通用广告分组判断，分组元数据不可用时也不会产生全量“未匹配”误报。

## 2026-04-30

### Updated
- **Unique Drama Report Snapshot**: 修复剧集播放接口测试历史报告互相覆盖的问题。现在每次剧集播放测试完成后，都会基于当次运行生成一份独立的 HTML 报告快照，并将执行记录中的 `reportUrl` 指向这份唯一文件，不再所有历史记录共用同一个 `drama_check_report.html`。
- **Single-Issue Summary Promotion**: 优化剧集播放接口测试报告的异常摘要文案。对于仅命中 1 条异常的剧集，报告摘要现在会直接显示具体异常内容，例如缺失章节或单一计数异常，而不再统一只显示“发现 1 项异常”。

## 2026-03-16

### Updated
- **Dynamic Iteration Logic**: 移除了 `k6_config.json` 中硬编码的 `itersPerVu: 3`。
- **Full Coverage Enhancement**: 激活了 `drama_check_flow.js` 中的自动计算逻辑。现在测试会根据 `drama_info.json` 中的实际剧集总数动态分配每个 VU 的迭代次数（例如 7570 条数据下，30 个 VU 每个执行 253 次），确保 100% 数据覆盖率，同时保持恒定的 30 次并发。

## 2026-03-13

### Added
- **K6 Environment Stub**: 在项目根目录创建了专门用于存放压测类脚本的 `k6-scripts/` 目录。
- **Type Definitions**: 通过 `npm install -D @types/k6` 添加了 K6 的 TypeScript/JavaScript 代码提示与类型推断支持。
- **Template Script**: 添加了 `episode.js` 压测范例脚本，包含了基于 Virtual Users (VU) 的并发连接模拟与基础业务链路断言 (200 OK & JSON Body Validation)。
- **HTML Reporter Integration**: 在 `episode.js` 中引入了 `k6-reporter` 插件，用于在测试结束时输出一份可视化的 `summary.html` 分析报告。

### Go Backend Integration 
- **Server Initialization**: 初始化了新的 Go 模块 (`testcenter-server`)，包含 Gin 路由与 Gorilla WebSocket 依赖。
- **WebSocket Streaming**: 新建 `RunK6TestHandler` 接口，它能利用 `os/exec` 后台执行 `k6 run` 命令，并将进程的标准输出与错误流实时发送到前端的 WebSocket 连接。
- **Static Reports Host**: 通过 `r.StaticFS` 给生成的 K6 压测 HTML 报告提供静态网页浏览能力。

### Vue Frontend Updates
- **Real-time Logging**: 将之前的定时器模拟日志功能切换成了原生的 `WebSocket` 事件侦听，并将收到的真实 K6 控制台字符串即时追加到滚动面板中。
- **Report Visualization**: 在前端引入了对于压测报告完结后的动态视图切换。测试完成后隐藏控制台，内嵌展示调度端生成的报告 iframe (`http://localhost:8080/reports/summary.html`)。
