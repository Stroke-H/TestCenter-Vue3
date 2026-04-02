import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

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
        meta: { title: '执行测试', icon: 'VideoPlay' }
      },
      {
        path: 'reports',
        name: 'TestReports',
        component: () => import('@/views/TestReports/index.vue'),
        meta: { title: '测试报告', icon: 'DataAnalysis' }
      },
      {
        path: 'test_process',
        name: 'TestProcessList',
        component: () => import('@/views/TestProcess/List.vue'),
        meta: { title: '流程验证', icon: 'Tickets' }
      },
      {
        path: 'test_process/edit/:id?',
        name: 'TestProcessEditor',
        component: () => import('@/views/TestProcess/index.vue'),
        meta: { title: '编辑流程', hidden: true },
        props: true
      },
      {
        path: 'feishu_assistant',
        name: 'FeishuAssistant',
        component: () => import('@/views/FeishuAssistant/index.vue'),
        meta: { title: '飞书助手', icon: 'Service' }
      },
      {
        path: 'acceptance_reports',
        name: 'AcceptanceReport',
        component: () => import('@/views/AcceptanceReport/index.vue'),
        meta: { title: '验收报告', icon: 'Monitor' }
      },
      {
        path: 'ui_auto_jungle',
        name: 'JungleChess',
        component: () => import('@/views/JungleChess/index.vue'),
        meta: { title: '斗兽棋', hidden: true }
      },
      {
        path: 'ui_auto',
        name: 'UIAutoTest',
        component: () => import('@/views/UIAutoTest/index.vue'),
        meta: { title: 'UI 自动化', icon: 'Monitor' }
      },
      {
        path: 'ui_auto/edit/:id?',
        name: 'UIAutoEditor',
        component: () => import('@/views/UIAutoTest/Editor.vue'),
        meta: { title: '编辑用例', hidden: true },
        props: true
      },
      {
        path: 'ui_auto/run/:id',
        name: 'UIAutoRunner',
        component: () => import('@/views/UIAutoTest/Runner.vue'),
        meta: { title: '执行用例', hidden: true },
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
  
  // 1. 如果去往公开页面（登录/注册），直接放行
  if (to.meta.public) {
    next()
    return
  }

  // 2. 如果未登录，且去往需要权限的页面，重定向到登录
  if (!authStore.isLoggedIn) {
    next('/login')
    return
  }

  // 3. 如果已登录但没有用户信息，尝试获取用户信息
  if (authStore.isLoggedIn && !authStore.user) {
    await authStore.fetchMe()
    // 如果获取失败（token失效），会被 fetchMe 自动调用 logout 并清除 token
    if (!authStore.isLoggedIn) {
      next('/login')
      return
    }
  }

  next()
})

export default router
