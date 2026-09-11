<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Key, Lock, Plus, Refresh, User } from '@element-plus/icons-vue'
import { defectApi } from '../api'
import type { DefectMeta, DefectProjectMember, DefectProjectPermission } from '../types'

const props = defineProps<{ meta: DefectMeta | null }>()
const loading = ref(false)
const saving = ref(false)
const search = ref('')
const items = ref<DefectProjectPermission[]>([])
const selectedCode = ref('')
const draftMode = ref<'open' | 'restricted'>('open')
const draftMembers = ref<DefectProjectMember[]>([])

const filteredItems = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return keyword ? items.value.filter((item) => `${item.project_code} ${item.project_name}`.toLowerCase().includes(keyword)) : items.value
})
const selected = computed(() => items.value.find((item) => item.project_code === selectedCode.value) || null)
const availableAccounts = computed(() => (props.meta?.accounts || []).filter((account) => !draftMembers.value.some((member) => member.user_id === account.id)))

const roleOptions = [
  { value: 'lead', label: '项目负责人', description: '项目内全部缺陷操作' },
  { value: 'tester', label: '测试人员', description: '提交、编辑、验证和重新激活' },
  { value: 'developer', label: '开发人员', description: '查看、编辑负责项和解决缺陷' },
  { value: 'viewer', label: '只读成员', description: '仅查看缺陷、历史和附件' }
]

function message(error: any, fallback: string) { return error?.response?.data?.error || error?.customMessage || fallback }

async function load() {
  loading.value = true
  try {
    items.value = await defectApi.projectPermissions()
    if (!selectedCode.value && items.value[0]) selectProject(items.value[0])
    else if (selected.value) selectProject(selected.value)
  } catch (error) { ElMessage.error(message(error, '项目权限加载失败')) } finally { loading.value = false }
}

function selectProject(item: DefectProjectPermission) {
  selectedCode.value = item.project_code
  draftMode.value = item.permission_mode
  draftMembers.value = item.members.map((member) => ({ ...member }))
}

function addMember() {
  const account = availableAccounts.value[0]
  if (!account) {
    ElMessage.info('所有平台账号都已添加')
    return
  }
  draftMembers.value.push({ user_id: account.id, username: account.username, nickname: account.nickname, role_key: 'viewer' })
}

function changeMemberUser(index: number, userID: string) {
  const account = props.meta?.accounts.find((item) => item.id === userID)
  const member = draftMembers.value[index]
  if (member && account) Object.assign(member, { user_id: account.id, username: account.username, nickname: account.nickname })
}

async function save() {
  if (!selected.value) return
  if (draftMode.value === 'restricted' && !draftMembers.value.some((member) => member.role_key === 'lead')) {
    ElMessage.warning('受限模式至少需要一名项目负责人')
    return
  }
  saving.value = true
  try {
    await defectApi.saveProjectPermission(selected.value.project_code, { permission_mode: draftMode.value, members: draftMembers.value.map((member) => ({ ...member })) })
    ElMessage.success('项目缺陷权限已保存')
    await load()
  } catch (error) { ElMessage.error(message(error, '项目权限保存失败')) } finally { saving.value = false }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="project-permission">
    <header class="permission-header"><div><span><el-icon><Key /></el-icon></span><div><strong>项目级权限</strong><p>成员角色限制操作；受限项目仅指定成员可访问</p></div></div><el-button :icon="Refresh" @click="load">刷新</el-button></header>
    <div class="permission-layout">
      <aside class="project-list">
        <el-input v-model="search" clearable placeholder="搜索项目" />
        <div><button v-for="item in filteredItems" :key="item.project_code" :class="{ 'is-active': item.project_code === selectedCode }" @click="selectProject(item)"><span>{{ item.project_code }}</span><strong>{{ item.project_name }}</strong><em :class="{ 'is-restricted': item.permission_mode === 'restricted' }">{{ item.permission_mode === 'restricted' ? '受限' : '开放' }}</em></button></div>
      </aside>
      <main v-if="selected" class="permission-editor">
        <div class="editor-title"><div><strong>{{ selected.project_code }} · {{ selected.project_name }}</strong><span>配置只影响当前项目的缺陷数据</span></div><el-button type="primary" :loading="saving" @click="save">保存权限</el-button></div>
        <div class="mode-grid"><label :class="{ 'is-active': draftMode === 'open' }"><el-radio v-model="draftMode" value="open"><strong>开放模式</strong></el-radio><p>未指定成员继承平台权限；已指定成员按项目角色限制操作。</p></label><label :class="{ 'is-active': draftMode === 'restricted' }"><el-radio v-model="draftMode" value="restricted"><strong>受限模式</strong></el-radio><p>只有下面指定的项目成员和平台管理员可以访问。</p></label></div>
        <section class="members-section" :class="{ 'is-muted': draftMode === 'open' }">
          <div class="members-title"><div><strong>项目成员</strong><span>{{ draftMode === 'open' ? '已指定成员按角色限制操作，其他用户继承平台权限' : '角色同时控制数据范围和生命周期操作' }}</span></div><el-button :icon="Plus" :disabled="!availableAccounts.length" @click="addMember">添加成员</el-button></div>
          <div v-if="draftMembers.length" class="member-list"><div v-for="(member,index) in draftMembers" :key="`${member.user_id}-${index}`" class="member-row"><span class="member-avatar"><el-icon><User /></el-icon></span><el-select :model-value="member.user_id" filterable @change="changeMemberUser(index, $event)"><el-option v-for="account in props.meta?.accounts || []" :key="account.id" :label="account.nickname ? `${account.nickname} (${account.username})` : account.username" :value="account.id" :disabled="draftMembers.some((entry,entryIndex) => entryIndex !== index && entry.user_id === account.id)" /></el-select><el-select v-model="member.role_key"><el-option v-for="role in roleOptions" :key="role.value" :label="role.label" :value="role.value"><div class="role-option"><strong>{{ role.label }}</strong><small>{{ role.description }}</small></div></el-option></el-select><el-button text type="danger" @click="draftMembers.splice(index,1)">移除</el-button></div></div>
          <div v-else class="member-empty"><el-icon><Lock /></el-icon><strong>尚未配置项目成员</strong><span>切换为受限模式前，请至少添加一名项目负责人。</span></div>
        </section>
        <div class="role-guide"><div v-for="role in roleOptions" :key="role.value"><strong>{{ role.label }}</strong><span>{{ role.description }}</span></div></div>
      </main>
    </div>
  </section>
</template>

<style scoped>
.project-permission{min-height:520px}.permission-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:14px;padding:17px 19px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.permission-header>div{display:flex;align-items:center;gap:11px}.permission-header>div>span{display:grid;place-items:center;width:40px;height:40px;border-radius:12px;color:#0891b2;background:#ecfeff}.permission-header strong{color:#172033;font-size:15px}.permission-header p{margin:4px 0 0;color:#94a3b8;font-size:11px}.permission-layout{display:grid;grid-template-columns:270px minmax(0,1fr);min-height:500px;overflow:hidden;border:1px solid #e8edf5;border-radius:16px;background:#fff}.project-list{padding:15px;border-right:1px solid #e8edf5;background:#fafbfc}.project-list>div{display:flex;flex-direction:column;gap:5px;max-height:440px;margin-top:12px;overflow:auto}.project-list button{display:grid;grid-template-columns:58px minmax(0,1fr) 35px;align-items:center;gap:7px;padding:10px;border:0;border-radius:10px;background:transparent;text-align:left;cursor:pointer}.project-list button:hover{background:#f1f5f9}.project-list button.is-active{background:#eaf3ff;box-shadow:inset 3px 0 #3b82f6}.project-list button>span{color:#2563eb;font-size:10px;font-weight:800}.project-list button>strong{overflow:hidden;color:#475569;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.project-list button>em{padding:3px;border-radius:5px;color:#64748b;background:#e2e8f0;font-size:8px;font-style:normal;text-align:center}.project-list button>em.is-restricted{color:#b45309;background:#fef3c7}.permission-editor{padding:21px}.editor-title,.members-title{display:flex;align-items:center;justify-content:space-between}.editor-title>div,.members-title>div{display:flex;flex-direction:column;gap:4px}.editor-title strong{color:#172033;font-size:15px}.editor-title span,.members-title span{color:#94a3b8;font-size:10px}.mode-grid{display:grid;grid-template-columns:1fr 1fr;gap:11px;margin-top:20px}.mode-grid>label{padding:15px;border:1px solid #e2e8f0;border-radius:12px;cursor:pointer;transition:.18s}.mode-grid>label.is-active{border-color:#93c5fd;background:#f8fbff;box-shadow:0 0 0 2px #dbeafe}.mode-grid strong{color:#334155}.mode-grid p{margin:8px 0 0 24px;color:#94a3b8;font-size:10px;line-height:1.5}.members-section{margin-top:16px;padding:17px;border:1px solid #e8edf5;border-radius:13px}.members-section.is-muted{background:#fafbfc}.members-title strong{color:#334155;font-size:13px}.member-list{display:flex;flex-direction:column;gap:8px;margin-top:15px}.member-row{display:grid;grid-template-columns:34px minmax(180px,1fr) minmax(150px,.7fr) 48px;align-items:center;gap:9px;padding:9px;border-radius:11px;background:#f8fafc}.member-avatar{display:grid;place-items:center;width:32px;height:32px;border-radius:10px;color:#3b82f6;background:#eaf3ff}.role-option{display:flex;flex-direction:column}.role-option small{color:#94a3b8;font-size:9px}.member-empty{display:flex;align-items:center;flex-direction:column;padding:38px 0;color:#94a3b8}.member-empty .el-icon{font-size:23px}.member-empty strong{margin-top:9px;color:#64748b;font-size:12px}.member-empty span{margin-top:4px;font-size:10px}.role-guide{display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin-top:14px}.role-guide>div{display:flex;flex-direction:column;padding:10px;border-radius:9px;background:#f8fafc}.role-guide strong{color:#475569;font-size:10px}.role-guide span{margin-top:4px;color:#94a3b8;font-size:8px;line-height:1.5}@media(max-width:900px){.permission-layout{grid-template-columns:220px 1fr}.role-guide{grid-template-columns:repeat(2,1fr)}}@media(max-width:700px){.permission-layout{grid-template-columns:1fr}.project-list{border-right:0;border-bottom:1px solid #e8edf5}.project-list>div{max-height:180px}.mode-grid{grid-template-columns:1fr}.member-row{grid-template-columns:32px 1fr}.member-row .el-select:nth-of-type(2){grid-column:2}.role-guide{grid-template-columns:1fr 1fr}}
</style>
