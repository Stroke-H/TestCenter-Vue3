<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { usePermissionStore } from '@/stores/modules/permissions'
import DashboardPermissionPanel from './DashboardPermissionPanel.vue'
import DefectFieldConfig from '@/views/DefectManagement/components/DefectFieldConfig.vue'
import DefectProjectPermissions from '@/views/DefectManagement/components/DefectProjectPermissions.vue'
import { defectApi } from '@/views/DefectManagement/api'
import type { DefectMeta } from '@/views/DefectManagement/types'
import { ASSISTANT_QUICK_ENTRIES, ASSISTANT_QUICK_ENTRY_LIMIT, getAssistantQuickEntryIds, saveAssistantQuickEntryIds, ASSISTANT_QUICK_ENTRY_CHANGED_EVENT } from '@/config/assistantQuickEntries'
import { onMounted, onBeforeUnmount } from 'vue'

defineOptions({ name: 'PermissionManagement' })
const auth = useAuthStore()
const permissionStore = usePermissionStore()
const isAdmin = computed(() => permissionStore.isPermissionAdmin)
type Tab = 'dashboard' | 'quick' | 'fields' | 'permissions'
const activeTab = ref<Tab>(isAdmin.value ? 'dashboard' : 'quick')
const tabs = computed(() => [
  ...(isAdmin.value ? [{ id: 'dashboard' as Tab, label: '仪表盘管理', icon: Icons.Grid, detail: '用户与模块访问' }] : []),
  { id: 'quick' as Tab, label: '快捷入口配置', icon: Icons.Lightning, detail: '我的常用工具' },
  ...(isAdmin.value ? [
    { id: 'fields' as Tab, label: '字段配置', icon: Icons.SetUp, detail: '缺陷自定义字段' },
    { id: 'permissions' as Tab, label: '权限配置', icon: Icons.Key, detail: '缺陷项目权限' }
  ] : [])
])
const selectedQuickEntryIds = ref<string[]>(getAssistantQuickEntryIds(auth.user?.id || ''))
const groupedQuickEntries = computed(() => ['API 工具', '测试流程工具', 'UI 自动化', '性能测试', '其他拓展'].map(group => ({
  group, items: ASSISTANT_QUICK_ENTRIES.filter(entry => entry.group === group)
})).filter(group => group.items.length))
const meta = ref<DefectMeta | null>(null)
const metaLoading = ref(false)
const metaError = ref(false)
let metaRequest = 0
async function loadMeta() {
  if (!isAdmin.value) return
  const request = ++metaRequest
  metaLoading.value = true
  metaError.value = false
  try {
    const result = await defectApi.meta()
    if (request === metaRequest && isAdmin.value) meta.value = result
  } catch {
    if (request === metaRequest) { metaError.value = true; ElMessage.error('缺陷配置加载失败，请重试') }
  } finally { if (request === metaRequest) metaLoading.value = false }
}
function syncQuickEntries() {
  selectedQuickEntryIds.value = getAssistantQuickEntryIds(auth.user?.id || '')
}
function toggleQuickEntry(entryId: string) {
  if (!auth.user?.id) return
  const exists = selectedQuickEntryIds.value.includes(entryId)
  if (!exists && selectedQuickEntryIds.value.length >= ASSISTANT_QUICK_ENTRY_LIMIT) {
    ElMessage.warning(`最多选择 ${ASSISTANT_QUICK_ENTRY_LIMIT} 个快捷入口`)
    return
  }
  const next = exists ? selectedQuickEntryIds.value.filter(id => id !== entryId) : [...selectedQuickEntryIds.value, entryId]
  try {
    saveAssistantQuickEntryIds(next, auth.user.id)
    selectedQuickEntryIds.value = next
    ElMessage.success('已保存我的快捷入口')
  } catch { ElMessage.error('浏览器存储不可用，配置未保存') }
}
watch(() => auth.user?.id, () => {
  ++metaRequest
  meta.value = null
  metaLoading.value = false
  metaError.value = false
  activeTab.value = isAdmin.value ? 'dashboard' : 'quick'
  syncQuickEntries()
})
watch(activeTab, tab => {
  if (!isAdmin.value && tab !== 'quick') { activeTab.value = 'quick'; return }
  if ((tab === 'fields' || tab === 'permissions') && !meta.value) loadMeta()
})
onMounted(() => {
  window.addEventListener(ASSISTANT_QUICK_ENTRY_CHANGED_EVENT, syncQuickEntries)
  window.addEventListener('storage', syncQuickEntries)
})
onBeforeUnmount(() => {
  window.removeEventListener(ASSISTANT_QUICK_ENTRY_CHANGED_EVENT, syncQuickEntries)
  window.removeEventListener('storage', syncQuickEntries)
})
</script>

<template>
  <main class="settings-hub">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">权限管理</h1>
          <span class="page-badge">Security & Access</span>
        </div>
        <p class="page-desc">管理平台模块访问权限、缺陷数据权限与个人快捷入口偏好</p>
      </div>
    </div>

    <div class="hub-layout">
      <nav class="hub-sidebar" aria-label="权限管理配置分类">
        <button 
          v-for="tab in tabs" 
          :key="tab.id" 
          :class="{ 'is-active': activeTab === tab.id }" 
          :aria-current="activeTab === tab.id ? 'page' : undefined" 
          @click="activeTab = tab.id"
        >
          <div class="tab-icon-box">
            <el-icon><component :is="tab.icon" /></el-icon>
          </div>
          <span class="tab-text">
            <strong>{{ tab.label }}</strong>
            <small>{{ tab.detail }}</small>
          </span>
          <el-icon class="nav-arrow"><Icons.ArrowRight /></el-icon>
        </button>
      </nav>

      <section class="hub-content">
        <DashboardPermissionPanel v-if="isAdmin && activeTab === 'dashboard'" :key="auth.user?.id" />
        <div v-else-if="activeTab === 'quick'" class="personal-config">
          <div class="quick-intro-banner">
            <p class="personal-note">仅配置当前账号 <strong class="user-highlight">{{ auth.user?.username }}</strong> 的快捷入口，保存在当前浏览器，可随时调整。</p>
          </div>
          
          <section class="quick-entry-config">
            <div class="quick-entry-config__header">
              <div>
                <h2>快捷入口偏好</h2>
                <p>选择展示在智能助手对话框底部的常用快捷入口，最多选择 4 个。</p>
              </div>
              <div class="entry-counter">
                <span>{{ selectedQuickEntryIds.length }} / {{ ASSISTANT_QUICK_ENTRY_LIMIT }}</span>
              </div>
            </div>

            <div class="quick-entry-groups">
              <section
                v-for="group in groupedQuickEntries"
                :key="group.group"
                class="quick-entry-group"
              >
                <div class="group-title-row">
                  <span class="group-dot"></span>
                  <h3>{{ group.group }}</h3>
                </div>
                <div class="quick-entry-grid">
                  <button
                    v-for="entry in group.items"
                    :key="entry.id"
                    type="button"
                    class="quick-entry-item"
                    :class="{ 'is-selected': selectedQuickEntryIds.includes(entry.id) }"
                    @click="toggleQuickEntry(entry.id)"
                  >
                    <span class="quick-entry-item__icon" :style="{ background: entry.iconBg, color: entry.iconColor }">
                      <el-icon><component :is="Icons[entry.iconName as keyof typeof Icons]" /></el-icon>
                    </span>
                    <span class="quick-entry-item__content">
                      <strong>{{ entry.name }}</strong>
                      <span>{{ entry.description }}</span>
                    </span>
                    <span class="entry-status-badge">
                      {{ selectedQuickEntryIds.includes(entry.id) ? '已选择' : '点击选择' }}
                    </span>
                  </button>
                </div>
              </section>
            </div>
          </section>
        </div>
        <div v-else-if="isAdmin" v-loading="metaLoading" class="defect-config">
          <el-empty v-if="metaError" description="配置加载失败"><el-button @click="loadMeta">重新加载</el-button></el-empty>
          <template v-else-if="meta">
            <DefectFieldConfig v-if="activeTab === 'fields'" :meta="meta" @changed="loadMeta" />
            <DefectProjectPermissions v-else-if="activeTab === 'permissions'" :meta="meta" />
          </template>
        </div>
      </section>
    </div>
  </main>
</template>

<style scoped>
.settings-hub {
  padding: 24px;
  max-width: 1680px;
  margin: 0 auto;
  color: #1e293b;
}

/* Standard Page Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  letter-spacing: -0.02em;
}

.page-badge {
  font-size: 12px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 3px 10px;
  border-radius: 9999px;
  letter-spacing: 0.02em;
}

.page-desc {
  margin: 6px 0 0;
  font-size: 13px;
  color: #64748b;
}

/* Layout */
.hub-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

/* Hub Sidebar */
.hub-sidebar {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);
  position: sticky;
  top: 20px;
}

.hub-sidebar button {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: #64748b;
  text-align: left;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-icon-box {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #f1f5f9;
  color: #64748b;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.hub-sidebar button:hover {
  background: #f8fafc;
  color: #1e293b;
}

.hub-sidebar button:hover .tab-icon-box {
  background: #e2e8f0;
  color: #0f172a;
}

.hub-sidebar button.is-active {
  background: #eff6ff;
  border-color: #dbeafe;
  color: #2563eb;
}

.hub-sidebar button.is-active .tab-icon-box {
  background: #2563eb;
  color: #ffffff;
  box-shadow: 0 2px 6px rgba(37, 99, 235, 0.25);
}

.hub-sidebar button > .tab-text {
  flex: 1;
  min-width: 0;
}

.hub-sidebar strong {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: inherit;
}

.hub-sidebar small {
  display: block;
  font-size: 11px;
  margin-top: 2px;
  color: #94a3b8;
}

.nav-arrow {
  font-size: 12px;
  color: #cbd5e1;
  transition: transform 0.2s ease;
}

.hub-sidebar button.is-active .nav-arrow {
  color: #2563eb;
  transform: translateX(2px);
}

/* Hub Content */
.hub-content {
  min-width: 0;
}

.personal-config {
  padding: 24px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.quick-intro-banner {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 12px 16px;
  margin-bottom: 24px;
}

.personal-note {
  margin: 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}

.user-highlight {
  color: #2563eb;
  font-weight: 600;
}

.quick-entry-config__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f1f5f9;
}

.quick-entry-config__header h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.quick-entry-config__header p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.entry-counter {
  padding: 6px 14px;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 9999px;
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.quick-entry-groups {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.quick-entry-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.group-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #3b82f6;
}

.quick-entry-group h3 {
  margin: 0;
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.quick-entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 12px;
}

.quick-entry-item {
  min-height: 84px;
  padding: 12px;
  text-align: left;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  grid-template-rows: 1fr auto;
  gap: 8px 12px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.02);
}

.quick-entry-item:hover {
  border-color: #93c5fd;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.08);
  transform: translateY(-1px);
}

.quick-entry-item.is-selected {
  background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%);
  border-color: #3b82f6;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.12);
}

.quick-entry-item__icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.quick-entry-item__content {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.quick-entry-item__content strong {
  color: #0f172a;
  font-size: 13px;
  font-weight: 600;
}

.quick-entry-item__content span {
  color: #64748b;
  font-size: 11px;
  line-height: 1.4;
}

.entry-status-badge {
  grid-column: 2;
  font-size: 11px;
  font-weight: 600;
  color: #94a3b8;
}

.quick-entry-item.is-selected .entry-status-badge {
  color: #2563eb;
}

.defect-config {
  min-height: 300px;
}

.hub-content :deep(.permission-page) {
  padding: 0;
}

.hub-content :deep(.permission-page__eyebrow) {
  display: none;
}

.hub-content :deep(.field-config),
.hub-content :deep(.project-permission) {
  min-width: 0;
}

@media (max-width: 1000px) {
  .hub-layout {
    grid-template-columns: 1fr;
  }
  .hub-sidebar {
    position: static;
    flex-direction: row;
    flex-wrap: wrap;
  }
  .hub-sidebar button {
    flex: 1;
    min-width: 150px;
  }
}

@media (max-width: 600px) {
  .settings-hub {
    padding: 14px;
  }
  .personal-config {
    padding: 16px;
  }
}
</style>
