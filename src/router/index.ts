// 路由配置 — 使用 Vue Router 4 的 createRouter + createWebHistory
import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

// 路由表定义
const routes: RouteRecordRaw[] = [
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
      }
    ]
  }
]

// 创建路由实例
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  // 平滑滚动
  scrollBehavior: () => ({ top: 0 })
})

export default router
