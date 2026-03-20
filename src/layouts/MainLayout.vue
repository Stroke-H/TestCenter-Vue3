<script setup lang="ts">
// MainLayout — 主布局组件
// 包含顶部导航栏 + 可折叠侧边菜单 + 主内容区
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import {
  Fold,
  Expand,
  Odometer,
  Setting,
  Document,
  DataAnalysis,
  Monitor
} from '@element-plus/icons-vue'

// 路由实例
const router = useRouter()
// 当前路由
const route = useRoute()
// 应用状态
const appStore = useAppStore()

// 侧边栏宽度：折叠时 64px，展开时 220px
const sidebarWidth = computed(() => (appStore.sidebarCollapsed ? '64px' : '220px'))

// 当前激活的菜单项
const activeMenu = computed(() => route.path)

// 菜单项配置
const menuItems = [
  { path: '/dashboard', title: '仪表盘', icon: Odometer },
  { path: '/reports', title: '测试报告', icon: DataAnalysis },
  { path: '/testcases', title: '用例管理', icon: Document },
  { path: '/tasks', title: '任务中心', icon: Monitor },
  { path: '/settings', title: '系统设置', icon: Setting }
]

// 菜单点击导航
const handleMenuSelect = (path: string) => {
  router.push(path)
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
        <el-menu-item
          v-for="item in menuItems"
          :key="item.path"
          :index="item.path"
        >
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ item.title }}</template>
        </el-menu-item>
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
          <el-tag type="success" effect="dark" round size="small">
            在线
          </el-tag>
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

.sidebar-menu .el-menu-item {
  height: 48px;
  line-height: 48px;
  margin: 2px 8px;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.sidebar-menu .el-menu-item:hover {
  background: #f1f5f9 !important;
}

.sidebar-menu .el-menu-item.is-active {
  background: #eef2ff !important;
  color: #3b82f6 !important;
  font-weight: 600;
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
  gap: 16px;
}

/* ==================== 主内容区 ==================== */
.layout-main-container {
  flex-direction: column;
  background: #f5f6fa;
}

.layout-content {
  padding: 24px;
  background: #f5f6fa;
  overflow-y: auto;
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
