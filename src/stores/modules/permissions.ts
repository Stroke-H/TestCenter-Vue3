import { computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import request from '@/api/request'
import { useAuthStore } from '@/stores/auth'

export interface PermissionModule {
  key: string
  title: string
  description: string
  group: '仪表盘' | '报告中心' | '系统设置'
}

export interface UserPermissionRecord {
  user_id: string
  username: string
  permissions: Record<string, boolean>
  updated_at?: string
}

export const PERMISSION_MODULES: PermissionModule[] = [
  { key: 'dashboard.com_api_commit.visible', title: 'API 工具', description: '执行测试类 API 工具入口', group: '仪表盘' },
  { key: 'dashboard.test_process.visible', title: '流程验证', description: '必测流程验证入口', group: '仪表盘' },
  { key: 'dashboard.project_tree.visible', title: '项目树', description: '按项目和版本查看验收记录树入口', group: '仪表盘' },
  { key: 'dashboard.sandbox_accounts.visible', title: '测试账号管理', description: '沙盒账号与测试账号入口', group: '仪表盘' },
  { key: 'dashboard.ui_auto.visible', title: 'UI 自动化', description: 'UI 自动化工作台入口', group: '仪表盘' },
  { key: 'dashboard.testcase_gen.visible', title: '测试用例生成', description: '测试用例生成入口', group: '仪表盘' },
  { key: 'dashboard.skillify.visible', title: '节点 Skill 化', description: 'Skill 化工具入口', group: '仪表盘' },
  { key: 'dashboard.performance.visible', title: '性能测试', description: 'Web 前端压测等性能工具入口', group: '仪表盘' },
  { key: 'dashboard.monkey_test.visible', title: 'Monkey 测试', description: 'Monkey 测试与截图 3D 图谱入口', group: '仪表盘' },
  { key: 'monkey.run.view', title: '查看 Monkey 结果', description: '查看 Monkey 设备、运行记录、日志和截图', group: '仪表盘' },
  { key: 'monkey.run.start', title: '启动 Monkey 测试', description: '允许在 Android 设备上启动真实 Monkey 测试', group: '仪表盘' },
  { key: 'monkey.run.stop', title: '停止 Monkey 测试', description: '允许停止正在运行的 Monkey 测试', group: '仪表盘' },
  { key: 'monkey.device.wireless_pair', title: '配对无线 ADB', description: '允许使用手机无线调试配对码建立 ADB 配对', group: '仪表盘' },
  { key: 'monkey.device.wireless_connect', title: '连接无线 ADB', description: '允许连接或重新连接已配对的无线 Android 设备', group: '仪表盘' },
  { key: 'monkey.device.wireless_disconnect', title: '断开无线 ADB', description: '允许主动断开无线 Android 设备', group: '仪表盘' },
  { key: 'dashboard.jungle.visible', title: '斗兽棋', description: '斗兽棋入口', group: '仪表盘' },
  { key: 'dashboard.novel_reader.visible', title: '小说阅读器', description: '小说阅读器入口', group: '仪表盘' },
  { key: 'dashboard.video_player.visible', title: '视频播放器', description: '视频播放与摸鱼模式入口', group: '仪表盘' },
  { key: 'dashboard.feishu_assistant.visible', title: '飞书助手', description: '飞书助手入口', group: '仪表盘' },
  { key: 'reports.test_reports.visible', title: '测试报告', description: '测试报告入口', group: '报告中心' },
  { key: 'reports.acceptance_reports.visible', title: '验收报告', description: '验收报告入口', group: '报告中心' },
  { key: 'reports.testcase_gen.visible', title: '用例报告', description: '用例报告入口', group: '报告中心' },
  { key: 'settings.permissions.visible', title: '权限管理', description: '权限管理页入口', group: '系统设置' }
]

function isPermissionAdminUsername(username?: string) {
  return String(username || '').trim().toLowerCase() === 'minghong'
}

function defaultPermissions(username?: string) {
  const defaults = Object.fromEntries(PERMISSION_MODULES.map((item) => [item.key, true])) as Record<string, boolean>
  defaults['dashboard.jungle.visible'] = false
  defaults['dashboard.novel_reader.visible'] = false
  defaults['dashboard.video_player.visible'] = false
  defaults['settings.permissions.visible'] = isPermissionAdminUsername(username)
  return defaults
}

function mergePermissions(source?: Record<string, boolean>, username?: string) {
  const merged = {
    ...defaultPermissions(username),
    ...(source || {})
  }
  if (isPermissionAdminUsername(username)) {
    merged['settings.permissions.visible'] = true
  }
  return merged
}

export const usePermissionStore = defineStore('permissions', () => {
  const authStore = useAuthStore()
  const currentPermissions = shallowRef<Record<string, boolean>>(defaultPermissions(authStore.user?.username))
  const permissionList = shallowRef<UserPermissionRecord[]>([])
  const loaded = shallowRef(false)

  const isPermissionAdmin = computed(() => {
    return isPermissionAdminUsername(authStore.user?.username)
  })

  const groupedModules = computed(() => {
    return ['仪表盘', '报告中心', '系统设置'].map((group) => ({
      group,
      items: PERMISSION_MODULES.filter((item) => item.group === group)
    }))
  })

  function canAccess(permissionKey?: string) {
    if (!permissionKey) return true
    return currentPermissions.value[permissionKey] !== false
  }

  async function fetchCurrentPermissions() {
    if (!authStore.isLoggedIn) {
      currentPermissions.value = defaultPermissions(authStore.user?.username)
      loaded.value = false
      return
    }
    try {
      const res = await request.get('/config/permissions/me') as UserPermissionRecord
      currentPermissions.value = mergePermissions(res.permissions, res.username || authStore.user?.username)
      loaded.value = true
    } catch (error) {
      console.error('Failed to fetch current permissions', error)
      currentPermissions.value = defaultPermissions(authStore.user?.username)
      loaded.value = false
    }
  }

  async function fetchPermissionList() {
    const res = await request.get('/config/permissions') as UserPermissionRecord[]
    permissionList.value = Array.isArray(res)
      ? res.map((item) => ({
          ...item,
          permissions: mergePermissions(item.permissions, item.username)
        }))
      : []
  }

  async function saveUserPermissions(userId: string, permissions: Record<string, boolean>) {
    await request.post('/config/permissions', {
      user_id: userId,
      permissions
    })
    await Promise.all([fetchPermissionList(), fetchCurrentPermissions()])
  }

  async function resetUserPermissions(userId: string) {
    await request.delete(`/config/permissions/${userId}`)
    await Promise.all([fetchPermissionList(), fetchCurrentPermissions()])
  }

  return {
    currentPermissions,
    permissionList,
    loaded,
    isPermissionAdmin,
    groupedModules,
    canAccess,
    fetchCurrentPermissions,
    fetchPermissionList,
    saveUserPermissions,
    resetUserPermissions
  }
})
