export interface AssistantQuickEntry {
  id: string
  name: string
  description: string
  iconName: string
  iconColor: string
  iconBg: string
  path?: string
  query?: Record<string, string>
  permissionKey?: string
  group: string
}

export const ASSISTANT_QUICK_ENTRY_LIMIT = 4
export const ASSISTANT_QUICK_ENTRY_STORAGE_KEY = 'assistant_quick_entry_ids'
export const ASSISTANT_QUICK_ENTRY_CHANGED_EVENT = 'assistant-quick-entry-changed'

export const DEFAULT_ASSISTANT_QUICK_ENTRY_IDS = [
  'acceptance-project-tree',
  'common-api',
  'episode-api',
  'monkey-test'
]

export const ASSISTANT_QUICK_ENTRIES: AssistantQuickEntry[] = [
  {
    id: 'common-api',
    name: '通用接口',
    description: '进入执行测试工作台，快速发起通用接口调试与测试',
    iconName: 'Connection',
    iconColor: '#6366f1',
    iconBg: 'rgba(99, 102, 241, 0.1)',
    path: '/com_api_commit',
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'episode-api',
    name: '剧集接口',
    description: '进入剧集播放接口测试，覆盖播放链路与性能验证',
    iconName: 'VideoPlay',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.1)',
    path: '/com_api_commit',
    query: {
      name: '剧集播放接口测试',
      desc: '结合 K6 压测引擎的流媒体播放链路性能测试'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'external-subtitle-test',
    name: '外挂字幕测试',
    description: '进入剧集外挂字幕测试，检查字幕数量、语种、时间轴与文件可用性',
    iconName: 'DocumentChecked',
    iconColor: '#0ea5e9',
    iconBg: 'rgba(14, 165, 233, 0.12)',
    path: '/com_api_commit',
    query: {
      name: '剧集外挂字幕测试',
      desc: '全量检查外挂剧字幕数量、语种、时间轴和字幕文件可用性'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'short-drama-api-test',
    name: '短剧类接口测试',
    description: '聚合短剧项目的登录、剧集、章节与播放链路接口测试',
    iconName: 'VideoCamera',
    iconColor: '#0ea5e9',
    iconBg: 'rgba(14, 165, 233, 0.12)',
    path: '/com_api_commit',
    query: {
      name: '短剧类接口测试',
      desc: '聚合短剧项目的登录、剧集、章节与播放链路接口测试'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'delete-account',
    name: '删除账号',
    description: '删除指定账号数据',
    iconName: 'Delete',
    iconColor: '#ef4444',
    iconBg: 'rgba(239, 68, 68, 0.1)',
    path: '/com_api_commit',
    query: {
      name: '删除账号',
      desc: '删除指定账号数据'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'push-test',
    name: '推送测试',
    description: '测试应用推送功能',
    iconName: 'Promotion',
    iconColor: '#f59e0b',
    iconBg: 'rgba(245, 158, 11, 0.1)',
    path: '/push_test',
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'rest-client',
    name: 'REST 客户端',
    description: '快速 API 调试与测试工具。',
    iconName: 'Setting',
    iconColor: '#6366f1',
    iconBg: 'rgba(99, 102, 241, 0.1)',
    path: '/com_api_commit',
    query: {
      name: 'REST 客户端',
      desc: '快速 API 调试与测试工具。'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'check-duplicate',
    name: '测试剧集是否重复',
    description: '检测集数据中是否存在重复的drama_intld',
    iconName: 'Connection',
    iconColor: '#3b82f6',
    iconBg: 'rgba(59, 130, 246, 0.1)',
    path: '/com_api_commit',
    query: {
      name: '测试剧集是否重复',
      desc: '检测集数据中是否存在重复的drama_intld'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'check-online',
    name: '测试剧集是否上架和隐藏',
    description: '检查剧集的online和is_hidden状态',
    iconName: 'Switch',
    iconColor: '#8b5cf6',
    iconBg: 'rgba(139, 92, 246, 0.1)',
    path: '/com_api_commit',
    query: {
      name: '测试剧集是否上架和隐藏',
      desc: '检查剧集的online和is_hidden状态'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'check-parent',
    name: '测试剧集是否母剧去重',
    description: '检测母剧的去重情况',
    iconName: 'Filter',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.1)',
    path: '/com_api_commit',
    query: {
      name: '测试剧集是否母剧去重',
      desc: '检测母剧的去重情况'
    },
    permissionKey: 'dashboard.com_api_commit.visible',
    group: 'API 工具'
  },
  {
    id: 'test-process',
    name: '必测流程验证',
    description: '管理与跟踪核心业务流程的测试状态（思维导图模式）',
    iconName: 'Tickets',
    iconColor: '#f59e0b',
    iconBg: 'rgba(245, 158, 11, 0.1)',
    path: '/test_process',
    permissionKey: 'dashboard.test_process.visible',
    group: '测试流程工具'
  },
  {
    id: 'acceptance-project-tree',
    name: '项目树',
    description: '项目记录，快速查看测试时间与需求点',
    iconName: 'Share',
    iconColor: '#2563eb',
    iconBg: 'rgba(37, 99, 235, 0.1)',
    path: '/test_process/project_tree',
    permissionKey: 'dashboard.project_tree.visible',
    group: '测试流程工具'
  },
  {
    id: 'sandbox-accounts',
    name: '测试账号管理',
    description: '集中查看与管理测试流程中使用的沙盒账号与数说测试账号',
    iconName: 'User',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.12)',
    path: '/sandbox_accounts',
    permissionKey: 'dashboard.sandbox_accounts.visible',
    group: '测试流程工具'
  },
  {
    id: 'ui-auto',
    name: 'UI 自动化工作台',
    description: '基于 Playwright 引擎的可视化 UI 自动化编排与执行平台',
    iconName: 'Monitor',
    iconColor: '#8b5cf6',
    iconBg: 'rgba(139, 92, 246, 0.1)',
    path: '/ui_auto',
    permissionKey: 'dashboard.ui_auto.visible',
    group: 'UI 自动化'
  },
  {
    id: 'testcase-gen',
    name: '测试用例生成',
    description: '基于需求自动生成覆盖正向、逆向、异常、并发的测试用例',
    iconName: 'Notebook',
    iconColor: '#8b5cf6',
    iconBg: 'rgba(139, 92, 246, 0.1)',
    path: '/testcase_gen/new',
    permissionKey: 'dashboard.testcase_gen.visible',
    group: 'UI 自动化'
  },
  {
    id: 'skillify',
    name: '节点Skill化工具',
    description: '自动化将测试节点转化为可重用的原子 Skill 组件',
    iconName: 'MagicStick',
    iconColor: '#6366f1',
    iconBg: 'rgba(99, 102, 241, 0.1)',
    path: '/skillify',
    permissionKey: 'dashboard.skillify.visible',
    group: 'UI 自动化'
  },
  {
    id: 'web-stress-test',
    name: 'Web前端压测',
    description: '基于 Lighthouse 引擎，深度分析网页性能、可访问性及最佳实践。',
    iconName: 'Odometer',
    iconColor: '#f59e0b',
    iconBg: 'rgba(245, 158, 11, 0.1)',
    path: '/com_api_commit',
    query: {
      name: 'Web前端压测',
      desc: '基于 Lighthouse 引擎，深度分析网页性能、可访问性及最佳实践。'
    },
    permissionKey: 'dashboard.performance.visible',
    group: '性能测试'
  },
  {
    id: 'monkey-test',
    name: 'Monkey测试',
    description: '执行随机事件采样截图，并生成可点击的原子结构全息图谱。',
    iconName: 'Aim',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.1)',
    path: '/com_api_commit',
    query: {
      name: 'Monkey测试',
      desc: '执行随机事件采样截图，并生成可点击的原子结构全息图谱。'
    },
    permissionKey: 'dashboard.monkey_test.visible',
    group: '性能测试'
  },
  {
    id: 'jungle-chess',
    name: '斗兽棋',
    description: '休闲类游戏自动化策略验证（实验性功能）',
    iconName: 'Grid',
    iconColor: '#14b8a6',
    iconBg: 'rgba(20, 184, 166, 0.12)',
    path: '/ui_auto_jungle',
    permissionKey: 'dashboard.jungle.visible',
    group: '其他拓展'
  },
  {
    id: 'novel-reader',
    name: '小说阅读器',
    description: '打开本地 TXT 小说，自动解析章节并进入沉浸式阅读',
    iconName: 'Reading',
    iconColor: '#0f766e',
    iconBg: 'rgba(20, 184, 166, 0.12)',
    path: '/novel_reader',
    permissionKey: 'dashboard.novel_reader.visible',
    group: '其他拓展'
  },
  {
    id: 'video-player',
    name: '视频播放器',
    description: '平台内打开 BBYS，并可切到摸鱼模式悬浮播放',
    iconName: 'VideoCamera',
    iconColor: '#0284c7',
    iconBg: 'rgba(14, 165, 233, 0.12)',
    path: '/video_player',
    permissionKey: 'dashboard.video_player.visible',
    group: '其他拓展'
  },
  {
    id: 'feishu-assistant',
    name: '飞书助手',
    description: '飞书助手入口',
    iconName: 'Service',
    iconColor: '#6366f1',
    iconBg: 'rgba(99, 102, 241, 0.1)',
    path: '/feishu_assistant',
    permissionKey: 'dashboard.feishu_assistant.visible',
    group: '其他拓展'
  }
]

export function getAssistantQuickEntryIds(userId: string) {
  try {
    const raw = localStorage.getItem(`${ASSISTANT_QUICK_ENTRY_STORAGE_KEY}:${userId}`)
    const ids = raw ? JSON.parse(raw) : DEFAULT_ASSISTANT_QUICK_ENTRY_IDS
    if (!Array.isArray(ids)) return DEFAULT_ASSISTANT_QUICK_ENTRY_IDS
    const validIds = new Set(ASSISTANT_QUICK_ENTRIES.map((entry) => entry.id))
    const normalized = ids.filter((id): id is string => typeof id === 'string' && validIds.has(id))
    return [...new Set(normalized)].slice(0, ASSISTANT_QUICK_ENTRY_LIMIT)
  } catch {
    return DEFAULT_ASSISTANT_QUICK_ENTRY_IDS
  }
}

export function saveAssistantQuickEntryIds(ids: string[], userId: string) {
  const validIds = new Set(ASSISTANT_QUICK_ENTRIES.map((entry) => entry.id))
  const nextIds = [...new Set(ids)].filter((id) => validIds.has(id)).slice(0, ASSISTANT_QUICK_ENTRY_LIMIT)
  localStorage.setItem(`${ASSISTANT_QUICK_ENTRY_STORAGE_KEY}:${userId}`, JSON.stringify(nextIds))
  window.dispatchEvent(new CustomEvent(ASSISTANT_QUICK_ENTRY_CHANGED_EVENT, { detail: nextIds }))
}

export function getAssistantQuickEntriesByIds(ids: string[]) {
  const entryMap = new Map(ASSISTANT_QUICK_ENTRIES.map((entry) => [entry.id, entry]))
  return ids.map((id) => entryMap.get(id)).filter((entry): entry is AssistantQuickEntry => Boolean(entry))
}
