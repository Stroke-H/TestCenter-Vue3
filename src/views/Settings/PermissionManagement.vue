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
    <header class="hub-heading"><span class="hub-heading__icon"><el-icon><Icons.Lock /></el-icon></span><div><h1>权限管理</h1><p>管理平台访问权限与个人快捷入口偏好</p></div></header>
    <div class="hub-layout">
      <nav class="hub-sidebar" aria-label="权限管理配置分类">
        <button v-for="tab in tabs" :key="tab.id" :class="{ 'is-active': activeTab === tab.id }" :aria-current="activeTab === tab.id ? 'page' : undefined" @click="activeTab = tab.id">
          <el-icon><component :is="tab.icon" /></el-icon><span><strong>{{ tab.label }}</strong><small>{{ tab.detail }}</small></span><el-icon class="nav-arrow"><Icons.ArrowRight /></el-icon>
        </button>
      </nav>
      <section class="hub-content">
        <DashboardPermissionPanel v-if="isAdmin && activeTab === 'dashboard'" :key="auth.user?.id" />
        <div v-else-if="activeTab === 'quick'" class="personal-config">
          <p class="personal-note">仅配置当前账号 {{ auth.user?.username }} 的快捷入口，保存在当前浏览器，可随时调整。</p>
            <section class="permission-group quick-entry-config">
              <div class="quick-entry-config__header">
                <div>
                  <h2>快捷入口配置</h2>
                  <p>选择展示在智能助手对话框底部的入口，最多选择四个。</p>
                </div>
                <span>{{ selectedQuickEntryIds.length }}/{{ ASSISTANT_QUICK_ENTRY_LIMIT }}</span>
              </div>
              <div class="quick-entry-groups">
                <section
                  v-for="group in groupedQuickEntries"
                  :key="group.group"
                  class="quick-entry-group"
                >
                  <h3>{{ group.group }}</h3>
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
                      <em>{{ selectedQuickEntryIds.includes(entry.id) ? '已选择' : '可选择' }}</em>
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

.permission-page {
  display: grid;
  gap: 18px;
}

.permission-page__header {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: center;
  padding: 20px 22px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.permission-page__eyebrow {
  margin: 0 0 6px;
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
}

.permission-page__header h1 {
  margin: 0;
  font-size: 28px;
}

.permission-page__subtitle {
  margin: 8px 0 0;
  color: #64748b;
}

.permission-layout {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 18px;
}

.permission-users,
.permission-editor {
  padding: 18px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.permission-users {
  display: grid;
  align-content: start;
  gap: 12px;
}

.permission-users__title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #0f172a;
  font-weight: 700;
}

.permission-user-card {
  display: grid;
  gap: 4px;
  padding: 12px;
  color: #0f172a;
  text-align: left;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  cursor: pointer;
}

.permission-user-card.is-active {
  background: #eff6ff;
  border-color: #60a5fa;
}

.permission-user-card span {
  color: #64748b;
  font-size: 12px;
}

.permission-editor {
  display: grid;
  gap: 18px;
}

.permission-editor__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.permission-editor__identity {
  display: flex;
  align-items: center;
  gap: 10px;
}

.permission-editor__identity span {
  display: block;
  margin-top: 4px;
  color: #64748b;
  font-size: 13px;
}

.permission-editor__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.permission-editor__autosave {
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.permission-groups {
  display: grid;
  gap: 18px;
}

.permission-group {
  display: grid;
  gap: 12px;
}

.permission-group h2 {
  margin: 0;
  font-size: 16px;
  color: #0f172a;
}

.permission-grid {
  display: grid;
  justify-content: flex-start;
  gap: 10px;
  grid-template-columns: repeat(auto-fill, 180px);
}

.permission-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: space-between;
  gap: 6px;
  width: 180px;
  min-height: 90px;
  padding: 11px 12px;
  text-align: left;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.18s ease;
}

.permission-item:hover {
  border-color: #93c5fd;
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.08);
}

.permission-item.is-enabled {
  background: linear-gradient(180deg, #eff6ff 0%, #dbeafe 100%);
  border-color: #60a5fa;
}

.permission-item.is-locked {
  cursor: not-allowed;
}

.permission-item.is-saving {
  border-color: #2563eb;
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.12);
}

.permission-item:disabled {
  cursor: wait;
}

.permission-item.is-locked:disabled {
  cursor: not-allowed;
}

.permission-item__content {
  display: grid;
  gap: 4px;
}

.permission-item__content strong {
  color: #0f172a;
  font-size: 14px;
}

.permission-item__content span {
  color: #64748b;
  font-size: 11px;
}

.permission-item em {
  color: #2563eb;
  font-size: 11px;
  font-style: normal;
  font-weight: 700;
}

.quick-entry-config {
  padding-top: 6px;
  border-top: 1px solid #e5e7eb;
}

.quick-entry-config__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.quick-entry-config__header p {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 13px;
}

.quick-entry-config__header span {
  min-width: 44px;
  padding: 5px 9px;
  color: #2563eb;
  text-align: center;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 800;
}

.quick-entry-groups {
  display: grid;
  gap: 14px;
}

.quick-entry-group {
  display: grid;
  gap: 10px;
}

.quick-entry-group h3 {
  margin: 0;
  color: #475569;
  font-size: 13px;
}

.quick-entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 10px;
}

.quick-entry-item {
  min-height: 88px;
  padding: 11px;
  text-align: left;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  grid-template-rows: 1fr auto;
  gap: 8px 10px;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

.quick-entry-item:hover {
  border-color: #93c5fd;
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.08);
}

.quick-entry-item.is-selected {
  background: linear-gradient(180deg, #eff6ff 0%, #dbeafe 100%);
  border-color: #60a5fa;
}

.quick-entry-item__icon {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
}

.quick-entry-item__content {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.quick-entry-item__content strong {
  color: #0f172a;
  font-size: 14px;
}

.quick-entry-item__content span {
  color: #64748b;
  font-size: 11px;
  line-height: 1.4;
}

.quick-entry-item em {
  grid-column: 2;
  color: #2563eb;
  font-size: 11px;
  font-style: normal;
  font-weight: 800;
}

@media (max-width: 1080px) {
  .permission-layout {
    grid-template-columns: 1fr;
  }

  .permission-grid {
    grid-template-columns: 1fr;
  }

  .permission-item {
    width: 100%;
  }

  .permission-page__header,
  .permission-editor__top,
  .quick-entry-config__header {
    align-items: flex-start;
    flex-direction: column;
  }
}

.settings-hub{padding:28px;max-width:1680px;margin:0 auto;color:#1e293b}
.hub-heading{display:flex;align-items:center;gap:14px;margin-bottom:24px}
.hub-heading__icon{display:grid;place-items:center;width:46px;height:46px;border-radius:14px;background:#eff6ff;color:#2563eb;font-size:23px}
.hub-heading h1{font-size:24px;margin:0 0 6px}.hub-heading p{margin:0;color:#64748b;font-size:13px}
.hub-layout{display:grid;grid-template-columns:210px minmax(0,1fr);gap:22px;align-items:start}
.hub-sidebar{display:flex;flex-direction:column;gap:8px;padding:10px;border:1px solid #e2e8f0;border-radius:16px;background:#fff;position:sticky;top:20px}
.hub-sidebar button{display:flex;align-items:center;gap:12px;padding:14px 10px;border:0;border-radius:11px;background:transparent;color:#64748b;text-align:left;cursor:pointer;transition:.2s}
.hub-sidebar button:hover{background:#f8fafc}.hub-sidebar button.is-active{background:#eff6ff;color:#2563eb}
.hub-sidebar button>span{flex:1}.hub-sidebar strong,.hub-sidebar small{display:block}.hub-sidebar strong{font-size:14px}.hub-sidebar small{font-size:11px;margin-top:6px;color:#94a3b8}.nav-arrow{font-size:12px}
.hub-content{min-width:0}.personal-config{padding:24px;background:#fff;border:1px solid #e2e8f0;border-radius:16px}.personal-note{margin:0 0 22px;color:#64748b;font-size:13px;line-height:1.7}.defect-config{min-height:300px}
.hub-content :deep(.permission-page){padding:0}.hub-content :deep(.permission-page__eyebrow){display:none}
.hub-content :deep(.field-config),.hub-content :deep(.project-permission){min-width:0}
@media(max-width:1000px){.hub-layout{grid-template-columns:1fr}.hub-sidebar{position:static;flex-direction:row;flex-wrap:wrap}.hub-sidebar button{flex:1;min-width:150px}}
@media(max-width:600px){.settings-hub{padding:14px}.personal-config{padding:16px}}
</style>
