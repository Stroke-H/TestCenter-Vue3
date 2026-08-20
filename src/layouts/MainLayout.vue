<script setup lang="ts">
// MainLayout — 主布局组件
// 包含顶部导航栏 + 可折叠侧边菜单 + 主内容区
import { computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAppStore, usePermissionStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import {
  Fold,
  Expand,
  Odometer,
  Setting,
  Service,
  DataAnalysis,
  Monitor,
  Notebook,
  UserFilled,
  SwitchButton,
  Collection,
  Iphone,
  Lock,
  Bell
} from '@element-plus/icons-vue'
import { User as UserMenuIcon } from '@element-plus/icons-vue'

// 路由实例
const router = useRouter()
// 当前路由
const route = useRoute()
// 应用状态
const appStore = useAppStore()
const authStore = useAuthStore()
const permissionStore = usePermissionStore()

import GlobalAssistant from '@/components/GlobalAssistant.vue'
import VideoPlayerFloat from '@/components/VideoPlayerFloat.vue'
import FishReaderFloat from '@/views/NovelReader/components/FishReaderFloat.vue'

// 侧边栏宽度：折叠时 64px，展开时 220px
const sidebarWidth = computed(() => (appStore.sidebarCollapsed ? '64px' : '220px'))

// 当前激活的菜单项
const activeMenu = computed(() => {
  if (route.path.startsWith('/testcase_gen')) {
    return '/testcase_gen/list'
  }
  return route.path
})

interface MenuItem {
  path: string
  title: string
  icon: unknown
  permissionKey?: string
  adminOnly?: boolean
  children?: MenuItem[]
}

function canDisplayMenuItem(item: MenuItem) {
  if (item.adminOnly && !permissionStore.isPermissionAdmin) return false
  if (item.permissionKey && !permissionStore.canAccess(item.permissionKey)) return false
  return true
}

const menuItems = computed<MenuItem[]>(() => {
  const items: MenuItem[] = [
    { path: '/dashboard', title: '仪表盘', icon: Odometer },
    {
      path: '/report_center',
      title: '报告中心',
      icon: DataAnalysis,
      children: [
        { path: '/reports', title: '测试报告', icon: DataAnalysis, permissionKey: 'reports.test_reports.visible' },
        { path: '/acceptance_reports', title: '验收报告', icon: Monitor, permissionKey: 'reports.acceptance_reports.visible' },
        { path: '/testcase_gen/list', title: '用例报告', icon: Notebook, permissionKey: 'reports.testcase_gen.visible' }
      ]
    },
    { path: '/feishu_assistant', title: '飞书助手', icon: Service, permissionKey: 'dashboard.feishu_assistant.visible' },
    {
      path: '/settings',
      title: '系统设置',
      icon: Setting,
      children: [
        { path: '/settings/projects', title: '项目代码', icon: Collection },
        { path: '/settings/devices', title: '测试设备', icon: Iphone },
        { path: '/settings/accounts', title: '账号管理', icon: UserMenuIcon },
        { path: '/settings/permissions', title: '权限管理', icon: Lock, permissionKey: 'settings.permissions.visible', adminOnly: true }
      ]
    }
  ]

  return items.flatMap((item) => {
    if (!item.children) {
      return canDisplayMenuItem(item) ? [item] : []
    }

    const children = item.children.filter(canDisplayMenuItem)
    return children.length > 0 ? [{ ...item, children }] : []
  })
})

onMounted(async () => {
  if (authStore.isLoggedIn) {
    await permissionStore.fetchCurrentPermissions()
  }
})

// 菜单点击导航
const handleMenuSelect = (path: string) => {
  router.push(path)
}

const handleUserCommand = async (command: string) => {
	if (command === 'todos') {
		router.push('/my-todos')
		return
	}

  if (command === 'profile') {
    router.push('/profile')
    return
  }

  if (command === 'logout') {
    await authStore.logout()
    router.push('/login')
  }
}
</script>

<template>
  <el-container class="main-layout">
    <!-- 侧边栏 -->
    <el-aside :width="sidebarWidth" class="layout-sidebar">
      <!-- Logo 区域 -->
      <div class="sidebar-logo">
        <div class="logo-icon">
          <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle cx="16" cy="16" r="14" stroke="currentColor" stroke-width="2" />
            <path d="M10 16L15 21L22 11" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </div>
        <transition name="fade">
          <span v-show="!appStore.sidebarCollapsed" class="logo-text">TestCenter</span>
        </transition>
      </div>

      <!-- 导航菜单 -->
      <el-menu
        :default-active="activeMenu"
        :collapse="appStore.sidebarCollapsed"
        :collapse-transition="true"
        class="sidebar-menu"
        background-color="transparent"
        text-color="#64748b"
        active-text-color="#3b82f6"
        @select="handleMenuSelect"
      >
        <template v-for="item in menuItems" :key="item.path">
          <!-- 子菜单 -->
          <el-sub-menu v-if="item.children" :index="item.path">
            <template #title>
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.title }}</span>
            </template>
            <el-menu-item
              v-for="child in item.children"
              :key="child.path"
              :index="child.path"
            >
              <el-icon><component :is="child.icon" /></el-icon>
              <template #title>{{ child.title }}</template>
            </el-menu-item>
          </el-sub-menu>

          <!-- 普通菜单 -->
          <el-menu-item v-else :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <!-- 右侧内容区 -->
    <el-container class="layout-main-container">
      <!-- 顶部导航栏 -->
      <el-header class="layout-header">
        <div class="header-left">
          <!-- 折叠按钮 -->
          <el-icon
            class="collapse-btn"
            :size="20"
            @click="appStore.toggleSidebar"
          >
            <component :is="appStore.sidebarCollapsed ? Expand : Fold" />
          </el-icon>
          <!-- 面包屑 -->
          <el-breadcrumb separator="/" class="header-breadcrumb">
            <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item>{{ route.meta.title }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <el-tag v-if="authStore.isLoggedIn" type="success" effect="dark" round size="small" class="mr-4">
            在线
          </el-tag>
          
          <el-dropdown trigger="click" @command="handleUserCommand">
            <div class="user-profile">
              <el-avatar :size="32" :src="authStore.user?.avatar || undefined" :icon="UserFilled" class="user-avatar" />
              <span class="username">{{ authStore.user?.nickname || authStore.user?.username || '用户' }}</span>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
				<el-dropdown-item command="todos" :icon="Bell">我的待办</el-dropdown-item>
                <el-dropdown-item command="profile" :icon="Setting">个人设置</el-dropdown-item>
                <el-dropdown-item divided command="logout" :icon="SwitchButton">
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 主内容区 -->
      <el-main class="layout-content">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>

    <!-- 全局智能助手 -->
    <VideoPlayerFloat />
    <FishReaderFloat />
    <GlobalAssistant />
  </el-container>
</template>

<style scoped>
/* ==================== 布局整体 ==================== */
.main-layout {
  height: 100vh;
  overflow: hidden;
}

/* ==================== 侧边栏 ==================== */
.layout-sidebar {
  background: #ffffff;
  border-right: 1px solid #f0f0f0;
  display: flex;
  flex-direction: column;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

/* Logo */
.sidebar-logo {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
  border-bottom: 1px solid #f0f0f0;
  flex-shrink: 0;
}

.logo-icon {
  width: 32px;
  height: 32px;
  color: #3b82f6;
  flex-shrink: 0;
}

.logo-icon svg {
  width: 100%;
  height: 100%;
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
  white-space: nowrap;
  letter-spacing: 0.5px;
}

/* 菜单样式 */
.sidebar-menu {
  border-right: none;
  padding: 12px 0;
  flex: 1;
}

.sidebar-menu :deep(.el-menu-item),
.sidebar-menu :deep(.el-sub-menu__title) {
  height: 48px;
  line-height: 48px;
  margin: 2px 8px;
  padding: 0 14px !important;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.sidebar-menu :deep(.el-menu-item:hover),
.sidebar-menu :deep(.el-sub-menu__title:hover) {
  background: #f1f5f9 !important;
}

.sidebar-menu :deep(.el-menu-item.is-active) {
  background: #eef2ff !important;
  color: #3b82f6 !important;
  font-weight: 600;
}

.sidebar-menu :deep(.el-menu-item .el-icon),
.sidebar-menu :deep(.el-sub-menu__title .el-icon:first-child) {
  width: 20px;
  margin-right: 12px;
  flex-shrink: 0;
}

.sidebar-menu :deep(.el-sub-menu .el-menu-item) {
  padding-left: 38px !important;
}

.sidebar-menu :deep(.el-sub-menu__icon-arrow) {
  right: 14px;
}

/* ==================== 顶部导航 ==================== */
.layout-header {
  height: 60px;
  background: #ffffff;
  backdrop-filter: blur(12px);
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.collapse-btn {
  cursor: pointer;
  color: #64748b;
  transition: color 0.2s, transform 0.2s;
}

.collapse-btn:hover {
  color: #3b82f6;
  transform: scale(1.1);
}

.header-breadcrumb {
  font-size: 14px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 20px;
}

.user-profile {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 8px;
  transition: background 0.2s;
}

.user-profile:hover {
  background: #f1f5f9;
}

.username {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
}

.user-avatar {
  background: #eef2ff;
  color: #6366f1;
}

.mr-4 { margin-right: 16px; }

/* ==================== 主内容区 ==================== */
.layout-main-container {
  flex-direction: column;
  background: #f5f6fa;
}

.layout-content {
  padding: 24px;
  background: #f5f6fa;
  overflow-y: auto;
  scrollbar-gutter: stable;
}

/* ==================== 过渡动画 ==================== */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.page-fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.page-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
