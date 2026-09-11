<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Lock, User } from '@element-plus/icons-vue'
import { PERMISSION_MODULES, usePermissionStore } from '@/stores/modules/permissions'

defineOptions({ name: 'DashboardPermissionPanel' })

const permissionStore = usePermissionStore()
const loading = shallowRef(false)
const selectedUserId = shallowRef('')
const draftPermissions = shallowRef<Record<string, boolean>>({})
const savingPermissionKey = shallowRef('')

const users = computed(() => permissionStore.permissionList)
const selectedUser = computed(() => {
  return users.value.find((item) => item.user_id === selectedUserId.value) || null
})
const groupedModules = computed(() => {
  return ['仪表盘', '缺陷管理', '报告中心', '系统设置'].map((group) => ({
    group,
    items: PERMISSION_MODULES.filter((item) => item.group === group && item.key !== 'settings.permissions.visible')
  }))
})
const selectedUserIsPermissionAdmin = computed(() => selectedUser.value?.username?.trim().toLowerCase() === 'minghong')

function syncDraftPermissions() {
  draftPermissions.value = { ...(selectedUser.value?.permissions || {}) }
}

async function fetchData() {
  loading.value = true
  try {
    await permissionStore.fetchPermissionList()
    const firstUser = users.value[0]
    if (!selectedUserId.value && firstUser) {
      selectedUserId.value = firstUser.user_id
    }
    if (selectedUserId.value && !users.value.some((item) => item.user_id === selectedUserId.value)) {
      selectedUserId.value = firstUser?.user_id || ''
    }
    syncDraftPermissions()
  } finally {
    loading.value = false
  }
}

function selectUser(userId: string) {
  selectedUserId.value = userId
  syncDraftPermissions()
}

async function togglePermission(permissionKey: string) {
  const user = selectedUser.value
  if (!user || savingPermissionKey.value) {
    return
  }
  if (selectedUserIsPermissionAdmin.value && permissionKey === 'settings.permissions.visible') {
    return
  }
  const previousPermissions = { ...draftPermissions.value }
  const nextPermissions = {
    ...draftPermissions.value,
    [permissionKey]: !(draftPermissions.value[permissionKey] !== false)
  }
  if (selectedUserIsPermissionAdmin.value) {
    nextPermissions['settings.permissions.visible'] = true
  }

  savingPermissionKey.value = permissionKey
  draftPermissions.value = {
    ...nextPermissions
  }

  try {
    await permissionStore.saveUserPermissions(user.user_id, nextPermissions)
    ElMessage.success(`已更新 ${user.username} 的权限`)
    if (selectedUserId.value === user.user_id) {
      syncDraftPermissions()
    }
  } catch {
    draftPermissions.value = previousPermissions
    ElMessage.error('权限更新失败，已恢复为修改前状态')
  } finally {
    if (savingPermissionKey.value === permissionKey) {
      savingPermissionKey.value = ''
    }
  }
}

async function resetCurrentUserPermissions() {
  if (!selectedUser.value) return
  try {
    await ElMessageBox.confirm(`确认重置 ${selectedUser.value.username} 的权限吗？`, '重置权限', {
      type: 'warning',
      confirmButtonText: '重置',
      cancelButtonText: '取消'
    })
    await permissionStore.resetUserPermissions(selectedUser.value.user_id)
    ElMessage.success('已恢复默认权限')
    await fetchData()
  } catch {
    // noop
  }
}

onMounted(fetchData)
</script>

<template>
  <main class="permission-page" v-loading="loading">
    <header class="permission-page__header">
      <div>
        <p class="permission-page__eyebrow">仅管理员可见</p>
        <h1>仪表盘管理</h1>
        <p class="permission-page__subtitle">管理用户可访问的仪表盘模块、报告入口和系统权限。</p>
      </div>
      <el-button :icon="Refresh" @click="fetchData">刷新</el-button>
    </header>

    <section class="permission-layout">
      <aside class="permission-users">
        <div class="permission-users__title">
          <el-icon><User /></el-icon>
          <span>用户列表</span>
        </div>
        <button
          v-for="item in users"
          :key="item.user_id"
          type="button"
          class="permission-user-card"
          :class="{ 'is-active': item.user_id === selectedUserId }"
          @click="selectUser(item.user_id)"
        >
          <strong>{{ item.username }}</strong>
        </button>
      </aside>

      <section class="permission-editor">
        <template v-if="selectedUser">
          <div class="permission-editor__top">
            <div class="permission-editor__identity">
              <el-icon><Lock /></el-icon>
              <div>
                <strong>{{ selectedUser.username }}</strong>
                <span>按模块控制访问权限</span>
              </div>
            </div>
            <div class="permission-editor__actions">
              <span class="permission-editor__autosave">点击卡片后自动保存</span>
              <el-button @click="resetCurrentUserPermissions">恢复默认</el-button>
            </div>
          </div>

          <div class="permission-groups">
            <section v-for="group in groupedModules" :key="group.group" class="permission-group">
              <h2>{{ group.group }}</h2>
              <div class="permission-grid">
                <button
                  v-for="item in group.items"
                  :key="item.key"
                  type="button"
                  class="permission-item"
                  :class="{
                    'is-enabled': draftPermissions[item.key] !== false,
                    'is-locked': selectedUserIsPermissionAdmin && item.key === 'settings.permissions.visible',
                    'is-saving': savingPermissionKey === item.key
                  }"
                  :disabled="Boolean(savingPermissionKey) || (selectedUserIsPermissionAdmin && item.key === 'settings.permissions.visible')"
                  @click="togglePermission(item.key)"
                >
                  <div class="permission-item__content">
                    <strong>{{ item.title }}</strong>
                    <span>{{ item.description }}</span>
                  </div>
                  <em>{{
                    savingPermissionKey === item.key
                      ? '保存中'
                      : selectedUserIsPermissionAdmin && item.key === 'settings.permissions.visible'
                      ? '始终开启'
                      : draftPermissions[item.key] !== false ? '已开启' : '已关闭'
                  }}</em>
                </button>
              </div>
            </section>


          </div>
        </template>
      </section>
    </section>
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
</style>
