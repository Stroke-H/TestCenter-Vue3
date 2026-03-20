<script setup lang="ts">
import { ref, onMounted } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'

const router = useRouter()

// ===== 数据结构定义 =====
interface ToolDef {
  id: string
  name: string
  description: string
  iconName: string
  iconColor: string
  iconBg: string
  action?: string
  statusIndicator?: 'toggle-off' | 'toggle-on' | 'dots' | 'ready' | 'on-hold'
  statusText?: string
  extra?: string
}

interface RecentTool {
  id: string
  name: string
  description: string
  iconName: string
  iconColor: string
  iconBg: string
  timestamp: number
}

// ===== 最近使用缓存逻辑 =====
const recentTools = ref<RecentTool[]>([])

onMounted(() => {
  const cached = localStorage.getItem('recent_tools_cache')
  if (cached) {
    try {
      recentTools.value = JSON.parse(cached)
    } catch (e) {}
  }
})

const addToRecent = (tool: ToolDef | RecentTool) => {
  const newTool: RecentTool = {
    id: tool.id,
    name: tool.name,
    description: tool.description,
    iconName: tool.iconName,
    iconColor: tool.iconColor,
    iconBg: tool.iconBg,
    timestamp: Date.now()
  }

  // 过滤掉同名记录并插入到队首
  let list = recentTools.value.filter(t => t.name !== newTool.name)
  list.unshift(newTool)
  
  // 限制最多四个卡片
  if (list.length > 4) {
    list = list.slice(0, 4)
  }
  
  recentTools.value = list
  localStorage.setItem('recent_tools_cache', JSON.stringify(list))
}

const clearHistory = () => {
  recentTools.value = []
  localStorage.removeItem('recent_tools_cache')
}

// 通用跳转与记录逻辑
const handleLaunch = (tool: ToolDef | RecentTool) => {
  addToRecent(tool)
  router.push({
    path: '/com_api_commit',
    query: {
      name: tool.name,
      desc: tool.description
    }
  })
}

// ===== 静态卡片数据列表 =====

// 原 Recently Used 移至 API Tools
const apiTools = ref<ToolDef[]>([
  {
    id: 'episode-playback-test',
    name: '剧集播放接口测试',
    description: '结合 K6 压测引擎的流媒体播放链路性能测试',
    iconName: 'VideoPlay',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.1)',
    statusIndicator: 'ready'
  },
  {
    id: 'delete-account',
    name: '删除账号',
    description: '删除指定账号数据',
    iconName: 'Delete',
    iconColor: '#ef4444',
    iconBg: 'rgba(239, 68, 68, 0.1)',
    statusIndicator: 'ready'
  },
  {
    id: 'push-test',
    name: '推送测试',
    description: '测试应用推送功能',
    iconName: 'Promotion',
    iconColor: '#f59e0b',
    iconBg: 'rgba(245, 158, 11, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'rest-client',
    name: 'REST Client',
    description: 'Fast API debugging and testing utility.',
    iconName: 'Setting',
    iconColor: '#6366f1',
    iconBg: 'rgba(99, 102, 241, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'cypress-runner',
    name: 'Cypress Runner',
    description: 'Execute headless UI automation suites.',
    iconName: 'VideoPlay',
    iconColor: '#ef4444',
    iconBg: 'rgba(239, 68, 68, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'load-generator',
    name: 'Load Generator',
    description: 'Simulate concurrent traffic for stress tests.',
    iconName: 'Loading',
    iconColor: '#3b82f6',
    iconBg: 'rgba(59, 130, 246, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'query-optimizer',
    name: 'Query Optimizer',
    description: 'Analyze and refactor slow SQL queries.',
    iconName: 'Document',
    iconColor: '#f59e0b',
    iconBg: 'rgba(245, 158, 11, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'check-duplicate',
    name: '测试剧集是否重复',
    description: '检测集数据中是否存在重复的drama_intld',
    iconName: 'Connection',
    iconColor: '#3b82f6',
    iconBg: 'rgba(59, 130, 246, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'check-online',
    name: '测试剧集是否上架和隐藏',
    description: '检查剧集的online和is_hidden状态',
    iconName: 'Switch',
    iconColor: '#8b5cf6',
    iconBg: 'rgba(139, 92, 246, 0.1)',
    statusIndicator: 'on-hold'
  },
  {
    id: 'check-parent',
    name: '测试剧集是否母剧去重',
    description: '检测母剧的去重情况',
    iconName: 'Filter',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.1)',
    statusIndicator: 'on-hold'
  }
])

const perfTools = ref<ToolDef[]>([
  {
    id: 'latency-sim',
    name: 'Latency Simulator',
    description: 'Throttle network for real-world tests.',
    iconName: 'Clock',
    iconColor: '#6b7280',
    iconBg: 'rgba(107, 114, 128, 0.1)',
    statusText: 'Offline'
  },
  {
    id: 'resource-monitor',
    name: 'Resource Monitor',
    description: 'Watch CPU/RAM during test runs.',
    iconName: 'Cpu',
    iconColor: '#3b82f6',
    iconBg: 'rgba(59, 130, 246, 0.1)',
    statusText: 'Monitoring Active'
  }
])
</script>

<template>
  <div class="dashboard">
    <!-- ========== Recently Used ========== -->
    <div class="section" v-if="recentTools.length > 0">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--blue">
            <el-icon :size="14"><component :is="Icons.Clock" /></el-icon>
          </div>
          <h2 class="section-title">Recently Used</h2>
        </div>
        <a href="#" class="clear-link" @click.prevent="clearHistory">Clear History</a>
      </div>

      <div class="card-grid card-grid--4">
        <div
          v-for="tool in recentTools"
          :key="tool.id"
          class="tool-card tool-card--launch"
        >
          <div class="tool-card__top">
            <div class="tool-card__icon" :style="{ background: tool.iconBg }">
              <el-icon :size="22" :color="tool.iconColor">
                <component :is="Icons[tool.iconName as keyof typeof Icons]" />
              </el-icon>
            </div>
          </div>
          <h3 class="tool-card__name">{{ tool.name }}</h3>
          <p class="tool-card__desc">{{ tool.description }}</p>
          <div class="tool-card__footer tool-card__footer--launch">
            <button class="launch-btn" @click="handleLaunch(tool)">
              Launch
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ========== API Tools ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--indigo">
            <el-icon :size="14"><component :is="Icons.Connection" /></el-icon>
          </div>
          <h2 class="section-title">API Tools</h2>
        </div>
      </div>

      <div class="card-grid card-grid--4">
        <div
          v-for="tool in apiTools"
          :key="tool.id"
          class="tool-card"
        >
          <div class="tool-card__top">
            <div class="tool-card__icon" :style="{ background: tool.iconBg }">
              <el-icon :size="20" :color="tool.iconColor">
                <component :is="Icons[tool.iconName as keyof typeof Icons]" />
              </el-icon>
            </div>
          </div>
          <h3 class="tool-card__name">{{ tool.name }}</h3>
          <p class="tool-card__desc">{{ tool.description }}</p>
          <div class="tool-card__footer">
            <div class="status-indicator">
              <span v-if="tool.statusIndicator === 'ready'" class="ready-text">
                Ready
              </span>
              <span v-else-if="tool.statusIndicator === 'on-hold'" class="on-hold-text">
                On hold
              </span>
            </div>
            <a href="#" class="open-link" @click.prevent="handleLaunch(tool)">Open</a>
          </div>
        </div>
      </div>

    </div>
 
    <!-- ========== Test Process Tools ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--orange">
            <el-icon :size="14"><component :is="Icons.Tickets" /></el-icon>
          </div>
          <h2 class="section-title">Test Process Tools</h2>
        </div>
      </div>

      <div class="card-grid card-grid--4">
        <div class="tool-card">
          <div class="tool-card__top">
            <div class="tool-card__icon" style="background: rgba(245, 158, 11, 0.1)">
              <el-icon :size="20" color="#f59e0b"><component :is="Icons.Tickets" /></el-icon>
            </div>
          </div>
          <h3 class="tool-card__name">必测流程验证</h3>
          <p class="tool-card__desc">管理与跟踪核心业务流程的测试状态（思维导图模式）</p>
          <div class="tool-card__footer">
            <span class="ready-text">V1.0</span>
            <a href="#" class="open-link" @click.prevent="router.push('/test_process')">Open</a>
          </div>
        </div>
      </div>
    </div>

    <!-- ========== UI Automation ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--teal">
            <el-icon :size="14"><component :is="Icons.Monitor" /></el-icon>
          </div>
          <h2 class="section-title">UI Automation</h2>
        </div>
      </div>

      <div class="card-grid card-grid--4">
        <div class="tool-card tool-card--coming-soon">
          <div class="tool-card__top">
            <div class="tool-card__icon" style="background: rgba(107,114,128,0.08)">
              <el-icon :size="20" color="#9ca3af"><component :is="Icons.Setting" /></el-icon>
            </div>
          </div>
          <h3 class="tool-card__name" style="color: #9ca3af;">敬请期待</h3>
          <p class="tool-card__desc">UI自动化工具即将上线</p>
        </div>
      </div>
    </div>

    <!-- ========== Performance ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--green">
            <el-icon :size="14"><component :is="Icons.Cpu" /></el-icon>
          </div>
          <h2 class="section-title">Performance</h2>
        </div>
      </div>

      <div class="card-grid card-grid--4">
        <div
          v-for="tool in perfTools"
          :key="tool.id"
          class="tool-card"
        >
          <div class="tool-card__top">
            <div class="tool-card__icon" :style="{ background: tool.iconBg }">
              <el-icon :size="20" :color="tool.iconColor">
                <component :is="Icons[tool.iconName as keyof typeof Icons]" />
              </el-icon>
            </div>
          </div>
          <h3 class="tool-card__name">{{ tool.name }}</h3>
          <p class="tool-card__desc">{{ tool.description }}</p>
          <div class="tool-card__footer">
            <span class="extra-text">{{ tool.statusText }}</span>
            <a href="#" class="open-link" @click.prevent="handleLaunch(tool)">Open</a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ==================== 页面容器 ==================== */
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 32px;
  padding: 8px 0;
}

/* ==================== 分区头部 ==================== */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.section-icon--blue { background: #3b82f6; }
.section-icon--indigo { background: #6366f1; }
.section-icon--orange { background: #f59e0b; }
.section-icon--teal { background: #14b8a6; }
.section-icon--green { background: #10b981; }

.section-title {
  font-size: 17px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
  letter-spacing: -0.2px;
}

.clear-link {
  font-size: 13px;
  color: #3b82f6;
  font-weight: 500;
  text-decoration: none;
  transition: color 0.2s;
}

.clear-link:hover {
  color: #2563eb;
}

/* ==================== 卡片网格 ==================== */
.card-grid {
  display: grid;
  gap: 16px;
}

.card-grid--4 {
  grid-template-columns: repeat(4, 1fr);
}

/* ==================== 工具卡片 ==================== */
.tool-card {
  background: #ffffff;
  border: 1px solid #f0f0f0;
  border-radius: 14px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: box-shadow 0.25s ease, transform 0.2s ease;
  cursor: default;
  position: relative;
}

.tool-card:hover {
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.06);
  transform: translateY(-2px);
}

.tool-card--coming-soon {
  opacity: 0.6;
}

/* 卡片顶部：图标 + 状态 */
.tool-card__top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 4px;
}

.tool-card__icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* 状态标签 */
.status-tag {
  font-size: 11px !important;
  font-weight: 600 !important;
  letter-spacing: 0.3px;
  border: none !important;
  padding: 0 10px !important;
  height: 22px !important;
}

/* 卡片名称 */
.tool-card__name {
  font-size: 15px;
  font-weight: 650;
  color: #1e293b;
  margin: 0;
  line-height: 1.3;
}

/* 卡片描述 */
.tool-card__desc {
  font-size: 12.5px;
  color: #94a3b8;
  margin: 0;
  line-height: 1.5;
  flex: 1;
}

/* 卡片底部 */
.tool-card__footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
  padding-top: 0;
}

.tool-card__footer--launch {
  margin-top: 4px;
  padding-top: 12px;
  border-top: 1px solid #f5f5f5;
}

/* Launch 按钮 */
.launch-btn {
  width: 100%;
  padding: 9px 0;
  background: #f8f9fb;
  border: 1px solid #eef0f3;
  border-radius: 10px;
  font-size: 13.5px;
  font-weight: 600;
  color: #475569;
  cursor: pointer;
  transition: all 0.2s ease;
  letter-spacing: 0.1px;
}

.launch-btn:hover {
  background: #eef2ff;
  color: #3b82f6;
  border-color: #c7d2fe;
}

/* Open 链接 */
.open-link {
  font-size: 13px;
  font-weight: 600;
  color: #3b82f6;
  text-decoration: none;
  transition: color 0.2s;
}

.open-link:hover {
  color: #2563eb;
}

/* Extra 文本 */
.extra-text {
  font-size: 12px;
  color: #94a3b8;
}

/* Ready 文本 */
.ready-text {
  font-size: 11px;
  font-weight: 700;
  color: #10b981;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.on-hold-text {
  font-size: 11px;
  font-weight: 700;
  color: #ef4444;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* ==================== 状态指示器 ==================== */
.status-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* Toggle 开关 */
.toggle-icon {
  display: flex;
  align-items: center;
  gap: 4px;
}

.toggle-track {
  width: 28px;
  height: 16px;
  background: #e2e8f0;
  border-radius: 8px;
  position: relative;
  transition: background 0.2s;
}

.toggle-track.active {
  background: #3b82f6;
}

.toggle-knob {
  width: 12px;
  height: 12px;
  background: #fff;
  border-radius: 50%;
  position: absolute;
  top: 2px;
  left: 2px;
  transition: transform 0.2s;
  box-shadow: 0 1px 3px rgba(0,0,0,0.15);
}

.toggle-knob.on {
  transform: translateX(12px);
}

/* 状态圆点 */
.status-dots {
  display: flex;
  align-items: center;
  gap: 5px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.dot--red { background: #ef4444; }
.dot--gray { background: #d1d5db; }
.dot--green { background: #10b981; }

/* ==================== 响应式 ==================== */
@media (max-width: 1100px) {
  .card-grid--4 {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .card-grid--4 {
    grid-template-columns: 1fr;
  }
  .dashboard {
    gap: 24px;
  }
}
</style>
