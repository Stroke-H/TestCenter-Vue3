import type { ReportItem } from '@/stores/modules/reports'

export interface MonkeyAtomicNode {
  id: string
  title: string
  event: string
  activity: string
  risk: 'normal' | 'warning' | 'critical' | 'unknown'
  imageUrl?: string
  summary?: string
  evidence?: Array<{
    source: string
    message: string
    timestamp?: string
  }>
  x?: number
  y?: number
}

export interface MonkeyAtomicEdge {
  source: string
  target: string
  event: string
}

export const MONKEY_ATOMIC_DEMO_REPORT_ID = 'MONKEY-DEMO-600-ATOMIC'

export const monkeyAtomicDemoReport: ReportItem = {
  id: MONKEY_ATOMIC_DEMO_REPORT_ID,
  name: 'Monkey 600 节点验收报告原子图 Demo',
  type: 'Monkey 测试',
  status: 'Warning',
  duration: '01:00:00',
  createdAt: '2026-05-27 14:30:00',
  author: 'demo',
  reportUrl: 'monkey-demo://atomic-600',
  environment: 'test',
  analysisResult: [
    'Monkey 验收报告原子图 Demo',
    '节点规模: 600',
    '采样策略: 低精度基础采样 + 异常加密采样',
    '节点颜色: 绿色 normal，黄色 warning，红色 critical，灰色 unknown',
    '用途: 验证大量截图节点下的球形空间模型、节点点击预览与风险分布表达。'
  ].join('\n')
}

const activities = [
  'SplashActivity',
  'HomeActivity',
  'FeedActivity',
  'PlayerActivity',
  'DetailActivity',
  'LoginDialog',
  'PurchaseSheet',
  'SettingsActivity',
  'WebViewActivity',
  'ErrorBoundary',
  'ProfileActivity',
  'SearchActivity',
  'CommentSheet',
  'ShareDialog',
  'ThemeSelector',
  'CacheManager',
  'OfflineDialog',
  'NetworkRetrySheet',
  'EpisodeSelector',
  'PaymentResultActivity'
]

const eventTypes = ['tap', 'swipe', 'back', 'fling', 'input', 'appswitch', 'majornav', 'screencap', 'keyevent', 'pinch']

const createSeededRandom = (seed: string) => {
  let hash = 2166136261
  for (let i = 0; i < seed.length; i++) {
    hash ^= seed.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return () => {
    hash += 0x6D2B79F5
    let value = hash
    value = Math.imul(value ^ (value >>> 15), value | 1)
    value ^= value + Math.imul(value ^ (value >>> 7), value | 61)
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296
  }
}

const createDemoScreenshot = (index: number, activity: string, risk: MonkeyAtomicNode['risk']) => {
  const accentMap: Record<MonkeyAtomicNode['risk'], string> = {
    normal: '#10b981',
    warning: '#f59e0b',
    critical: '#ef4444',
    unknown: '#64748b'
  }
  const accent = accentMap[risk]
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="360" height="720" viewBox="0 0 360 720">
      <rect width="360" height="720" rx="34" fill="#020617"/>
      <rect x="18" y="28" width="324" height="664" rx="28" fill="#f8fafc"/>
      <rect x="38" y="58" width="284" height="56" rx="16" fill="${accent}"/>
      <text x="58" y="93" font-family="Arial" font-size="21" font-weight="700" fill="#ffffff">Screen ${index}</text>
      <rect x="38" y="136" width="284" height="92" rx="18" fill="#e2e8f0"/>
      <rect x="58" y="160" width="176" height="14" rx="7" fill="#94a3b8"/>
      <rect x="58" y="188" width="224" height="12" rx="6" fill="#cbd5e1"/>
      <rect x="58" y="210" width="130" height="12" rx="6" fill="#cbd5e1"/>
      <rect x="38" y="252" width="132" height="132" rx="20" fill="#ffffff"/>
      <rect x="190" y="252" width="132" height="132" rx="20" fill="#ffffff"/>
      <rect x="38" y="408" width="284" height="86" rx="18" fill="#ffffff"/>
      <rect x="38" y="520" width="284" height="82" rx="18" fill="#ffffff"/>
      <circle cx="104" cy="318" r="30" fill="${accent}" opacity="0.82"/>
      <circle cx="256" cy="318" r="30" fill="#3b82f6" opacity="0.72"/>
      <text x="58" y="458" font-family="Arial" font-size="17" font-weight="700" fill="#0f172a">${activity}</text>
      <text x="58" y="566" font-family="Arial" font-size="15" fill="#64748b">risk: ${risk}</text>
    </svg>
  `
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

export const createMonkeyAtomicDemoGraph = () => {
  const random = createSeededRandom('monkey-atomic-demo-600')
  const nodes: MonkeyAtomicNode[] = []

  for (let index = 0; index < 600; index += 1) {
    const activity = activities[Math.floor(random() * activities.length)] || 'UnknownActivity'
    const event = eventTypes[Math.floor(random() * eventTypes.length)] || 'tap'
    let risk: MonkeyAtomicNode['risk'] = 'normal'

    if (index % 149 === 0 && index > 0) {
      risk = 'critical'
    } else if (index % 31 === 0 || random() > 0.91) {
      risk = 'warning'
    } else if (random() > 0.985) {
      risk = 'unknown'
    }

    nodes.push({
      id: `demo-screen-${String(index + 1).padStart(3, '0')}`,
      title: `采样截图 ${index + 1}`,
      event: `${event} #${120 + index * 18 + Math.floor(random() * 12)}`,
      activity,
      risk,
      imageUrl: createDemoScreenshot(index + 1, activity, risk),
      summary: risk === 'critical'
        ? '检测到 Crash/ANR 高风险信号，建议优先回看截图与日志。'
        : risk === 'warning'
          ? '检测到慢响应、空白页或恢复超时迹象。'
          : '常规截图采样节点。'
    })
  }

  const edges: MonkeyAtomicEdge[] = []
  for (let index = 1; index < nodes.length; index += 1) {
    const parentIndex = Math.floor(random() * index)
    const parent = nodes[parentIndex]
    const current = nodes[index]
    if (parent && current) {
      edges.push({
        source: parent.id,
        target: current.id,
        event: eventTypes[Math.floor(random() * eventTypes.length)] || 'tap'
      })
    }

    if (index > 4 && random() > 0.82) {
      const crossTarget = nodes[Math.floor(random() * index)]
      if (crossTarget && current && crossTarget.id !== current.id) {
        edges.push({
          source: current.id,
          target: crossTarget.id,
          event: 'crosspath'
        })
      }
    }
  }

  return { nodes, edges }
}
