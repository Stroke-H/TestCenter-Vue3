import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePermissionStore } from '@/stores'

// 路由表定义
const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login/index.vue'),
    meta: { title: '登录', hidden: true, public: true }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register/index.vue'),
    meta: { title: '注册', hidden: true, public: true }
  },
  {
    path: '/',
    name: 'Root',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard/index.vue'),
        meta: { title: '仪表盘', icon: 'Odometer' }
      },
      {
        path: 'com_api_commit',
        name: 'ComApiCommit',
        component: () => import('@/views/ComApiCommit/index.vue'),
        meta: { title: '执行测试', icon: 'VideoPlay', permissionKey: 'dashboard.com_api_commit.visible' }
      },
      {
        path: 'reports',
        name: 'TestReports',
        component: () => import('@/views/TestReports/index.vue'),
        meta: { title: '测试报告', icon: 'DataAnalysis', permissionKey: 'reports.test_reports.visible' }
      },
      {
        path: 'test_process',
        name: 'TestProcessList',
        component: () => import('@/views/TestProcess/List.vue'),
        meta: { title: '流程验证', icon: 'Tickets', permissionKey: 'dashboard.test_process.visible' }
      },
      {
        path: 'test_process/edit/:id?',
        name: 'TestProcessEditor',
        component: () => import('@/views/TestProcess/index.vue'),
        meta: { title: '编辑流程', hidden: true, permissionKey: 'dashboard.test_process.visible' },
        props: true
      },
      {
        path: 'test_process/project_tree',
        name: 'ProjectTree',
        component: () => import('@/views/TestProcess/ProjectTree/index.vue'),
        meta: { title: '项目树', hidden: true, permissionKey: 'dashboard.project_tree.visible' }
      },
      {
        path: 'feishu_assistant',
        name: 'FeishuAssistant',
        component: () => import('@/views/FeishuAssistant/index.vue'),
        meta: { title: '飞书助手', icon: 'Service', permissionKey: 'dashboard.feishu_assistant.visible' }
      },
      {
        path: 'skillify',
        name: 'NodeSkillify',
        component: () => import('@/views/TestProcess/SkillifyTool/index.vue'),
        meta: { title: '节点 Skill 化', icon: 'MagicStick', permissionKey: 'dashboard.skillify.visible' }
      },
      {
        path: 'sandbox_accounts',
        name: 'SandboxAccounts',
        component: () => import('@/views/TestProcess/SandboxAccounts/index.vue'),
        meta: { title: '沙盒账号管理', hidden: true, permissionKey: 'dashboard.sandbox_accounts.visible' }
      },
      {
        path: 'acceptance_reports',
        name: 'AcceptanceReport',
        component: () => import('@/views/AcceptanceReport/index.vue'),
        meta: { title: '验收报告', icon: 'Monitor', permissionKey: 'reports.acceptance_reports.visible' }
      },
      {
        path: 'profile',
        name: 'ProfileSettings',
        component: () => import('@/views/ProfileSettings/index.vue'),
        meta: { title: '个人设置', hidden: true }
      },
      {
        path: 'ui_auto_jungle',
        name: 'JungleChess',
        component: () => import('@/views/JungleChess/index.vue'),
        meta: { title: '斗兽棋', hidden: true, permissionKey: 'dashboard.jungle.visible' }
      },
      {
        path: 'novel_reader',
        name: 'NovelReader',
        component: () => import('@/views/NovelReader/index.vue'),
        meta: { title: '小说阅读器', hidden: true, permissionKey: 'dashboard.novel_reader.visible' }
      },
      {
        path: 'video_player',
        name: 'VideoPlayer',
        component: () => import('@/views/VideoPlayer/index.vue'),
        meta: { title: '视频播放器', hidden: true, permissionKey: 'dashboard.video_player.visible' }
      },
      {
        path: 'ui_auto',
        name: 'UIAutoTest',
        component: () => import('@/views/UIAutoTest/index.vue'),
        meta: { title: 'UI 自动化', icon: 'Monitor', permissionKey: 'dashboard.ui_auto.visible' }
      },
      {
        path: 'testcase_gen',
        name: 'TestCaseGenRoot',
        component: () => import('@/views/TestCaseGen/Layout.vue'),
        meta: { title: '测试用例生成', icon: 'Notebook', permissionKey: 'dashboard.testcase_gen.visible' },
        redirect: '/testcase_gen/list',
        children: [
          {
            path: 'list',
            name: 'TestCaseGen',
            component: () => import('@/views/TestCaseGen/List.vue'),
            meta: { title: '用例生成历史', permissionKey: 'dashboard.testcase_gen.visible' }
          },
          {
            path: 'new',
            name: 'TestCaseGenerator',
            component: () => import('@/views/TestCaseGen/Generator.vue'),
            meta: { title: '智能生成用例', hidden: true, permissionKey: 'dashboard.testcase_gen.visible' }
          },
          {
            path: 'view/:id',
            name: 'TestCaseView',
            component: () => import('@/views/TestCaseGen/Generator.vue'),
            meta: { title: '查看用例详情', hidden: true, permissionKey: 'dashboard.testcase_gen.visible' },
            props: true
          }
        ]
      },
      {
        path: 'ui_auto/edit/:id?',
        name: 'UIAutoEditor',
        component: () => import('@/views/UIAutoTest/Editor.vue'),
        meta: { title: '编辑用例', hidden: true, permissionKey: 'dashboard.ui_auto.visible' },
        props: true
      },
      {
        path: 'ui_auto/run/:id',
        name: 'UIAutoRunner',
        component: () => import('@/views/UIAutoTest/Runner.vue'),
        meta: { title: '执行用例', hidden: true, permissionKey: 'dashboard.ui_auto.visible' },
        props: true
      },
      {
        path: 'settings',
        name: 'Settings',
        redirect: '/settings/projects',
        meta: { title: '系统设置', icon: 'Setting' },
        children: [
          {
            path: 'projects',
            name: 'ProjectConfig',
            component: () => import('@/views/Settings/ProjectConfig.vue'),
            meta: { title: '项目代码', icon: 'Collection' }
          },
          {
            path: 'devices',
            name: 'DeviceConfig',
            component: () => import('@/views/Settings/DeviceConfig.vue'),
            meta: { title: '测试设备', icon: 'Iphone' }
          }
        ]
      }
    ]
  }
]

const settingsRoute = routes.find((route) => route.name === 'Root')?.children?.find((route) => route.name === 'Settings')
if (settingsRoute && settingsRoute.children) {
  settingsRoute.children.push({
    path: 'accounts',
    name: 'AccountConfig',
    component: () => import('@/views/Settings/AccountConfig.vue'),
    meta: { title: '账号管理', icon: 'User' }
  })
  settingsRoute.children.push({
    path: 'permissions',
    name: 'PermissionManagement',
    component: () => import('@/views/Settings/PermissionManagement.vue'),
    meta: { title: '权限管理', icon: 'Lock', permissionKey: 'settings.permissions.visible', adminOnly: true }
  })
}

// 创建路由实例
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  // 平滑滚动
  scrollBehavior: () => ({ top: 0 })
})

// 导航守卫
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()
  const permissionStore = usePermissionStore()

  if (to.meta.public) {
    next()
    return
  }

  if (authStore.status === 'anonymous') {
    next('/login')
    return
  }

  if (authStore.status === 'checking' || authStore.status === 'unreachable' || !authStore.user) {
    await authStore.fetchMe()
    if (!authStore.isLoggedIn) {
      if (authStore.status === 'unreachable') {
        next()
        return
      }
      next('/login')
      return
    }
  }

  if (authStore.isLoggedIn && !permissionStore.loaded) {
    await permissionStore.fetchCurrentPermissions()
  }

  if (to.meta.adminOnly && !permissionStore.isPermissionAdmin) {
    next('/dashboard')
    return
  }

  if (typeof to.meta.permissionKey === 'string' && !permissionStore.canAccess(to.meta.permissionKey)) {
    next('/dashboard')
    return
  }

  next()
})

export default router
