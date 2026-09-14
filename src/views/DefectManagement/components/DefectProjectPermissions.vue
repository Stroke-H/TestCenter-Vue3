<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { ElMessage } from "element-plus"
import { Key, Lock, Plus, Refresh, User } from "@element-plus/icons-vue"
import { defectApi } from "../api"
import type { DefectMemberDefaultRole, DefectMeta, DefectProjectMember, DefectProjectPermission } from "../types"

const props = defineProps<{ meta: DefectMeta | null }>()
const loading = ref(false)
const saving = ref(false)
const savingDefaults = ref(false)
const search = ref("")
const items = ref<DefectProjectPermission[]>([])
const selectedCode = ref("")
const draftMembers = ref<DefectProjectMember[]>([])
const defaultMembers = ref<DefectMemberDefaultRole[]>([])

const filteredItems = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return keyword ? items.value.filter((item) => `${item.project_code} ${item.project_name}`.toLowerCase().includes(keyword)) : items.value
})
const selected = computed(() => items.value.find((item) => item.project_code === selectedCode.value) || null)
const availableAccounts = computed(() => (props.meta?.accounts || []).filter((account) => !draftMembers.value.some((member) => member.user_id === account.id)))
const defaultRoleByUser = computed(() => new Map(defaultMembers.value.map((member) => [member.user_id, member.default_role])))

const roleOptions = [
  { value: "lead", label: "项目负责人", description: "项目内全部缺陷操作权限与管理" },
  { value: "tester", label: "测试人员", description: "执行缺陷全部步骤，包括确认、解决、验证、关闭和重新激活" },
  { value: "developer", label: "开发人员", description: "查看、编辑负责项和解决缺陷" },
  { value: "product", label: "产品人员", description: "提交、编辑和参与缺陷讨论" },
  { value: "viewer", label: "只读成员", description: "仅查看缺陷列表、历史和附件" }
]

const defaultRoleOptions = roleOptions.filter((role) => ["tester", "developer", "product"].includes(role.value))

function message(error: any, fallback: string) {
  return error?.response?.data?.error || error?.customMessage || fallback
}

async function load() {
  loading.value = true
  try {
    const [permissions, defaults] = await Promise.all([
      defectApi.projectPermissions(),
      defectApi.memberDefaultRoles()
    ])
    items.value = permissions
    defaultMembers.value = defaults
    if (!selectedCode.value && items.value[0]) selectProject(items.value[0])
    else if (selected.value) selectProject(selected.value)
  } catch (error) {
    ElMessage.error(message(error, "项目权限加载失败"))
  } finally {
    loading.value = false
  }
}

function selectProject(item: DefectProjectPermission) {
  selectedCode.value = item.project_code
  draftMembers.value = item.members.map((member) => ({ ...member }))
}

function addMember() {
  const account = availableAccounts.value[0]
  if (!account) {
    ElMessage.info("所有平台账号均已添加")
    return
  }
  draftMembers.value.push({
    user_id: account.id,
    username: account.username,
    nickname: account.nickname,
    role_key: defaultRoleByUser.value.get(account.id) || "tester"
  })
}

function changeMemberUser(index: number, userID: string) {
  const account = props.meta?.accounts.find((item) => item.id === userID)
  const member = draftMembers.value[index]
  if (member && account) Object.assign(member, {
    user_id: account.id,
    username: account.username,
    nickname: account.nickname,
    role_key: defaultRoleByUser.value.get(account.id) || "tester"
  })
}

async function save() {
  if (!selected.value) return
  saving.value = true
  try {
    await defectApi.saveProjectPermission(selected.value.project_code, {
      permission_mode: "open",
      members: draftMembers.value.map((member) => ({ ...member }))
    })
    ElMessage.success("项目缺陷权限已保存")
    await load()
  } catch (error) {
    ElMessage.error(message(error, "项目权限保存失败"))
  } finally {
    saving.value = false
  }
}

async function saveDefaults() {
  savingDefaults.value = true
  try {
    await defectApi.saveMemberDefaultRoles(defaultMembers.value.map((member) => ({ ...member })))
    ElMessage.success("默认成员属性已保存")
  } catch (error) {
    ElMessage.error(message(error, "默认成员属性保存失败"))
  } finally {
    savingDefaults.value = false
  }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="project-permission">
    <header class="permission-header">
      <div class="permission-header__title">
        <span class="permission-header__icon"><el-icon><Key /></el-icon></span>
        <div>
          <strong>项目缺陷成员与权限策略</strong>
          <p>项目数据开放查看，成员按照项目角色获得对应的生命周期操作权限</p>
        </div>
      </div>
      <el-button :icon="Refresh" @click="load">刷新</el-button>
    </header>

    <section class="default-role-section">
      <div class="default-role-title">
        <div>
          <strong>默认成员属性</strong>
          <span>成员加入任意项目时，自动带入这里配置的人员属性，项目内仍可单独调整。</span>
        </div>
        <el-button type="primary" plain :loading="savingDefaults" @click="saveDefaults">保存默认属性</el-button>
      </div>
      <div class="default-role-grid">
        <div v-for="member in defaultMembers" :key="member.user_id" class="default-role-item">
          <span class="member-avatar"><el-icon><User /></el-icon></span>
          <span class="default-role-user">
            <strong>{{ member.nickname || member.username }}</strong>
            <small v-if="member.nickname">{{ member.username }}</small>
          </span>
          <el-select v-model="member.default_role" class="default-role-select">
            <el-option v-for="role in defaultRoleOptions" :key="role.value" :label="role.label" :value="role.value" />
          </el-select>
        </div>
      </div>
    </section>

    <div class="permission-layout">
      <!-- 左侧项目选择导航 -->
      <aside class="project-list">
        <el-input v-model="search" clearable placeholder="搜索项目代码或名称..." />
        <div class="project-items">
          <button
            v-for="item in filteredItems"
            :key="item.project_code"
            class="project-item"
            :class="{ 'is-active': item.project_code === selectedCode }"
            @click="selectProject(item)"
          >
            <span class="project-code">{{ item.project_code }}</span>
            <strong class="project-name">{{ item.project_name }}</strong>
            <em class="project-mode">开放</em>
          </button>
        </div>
      </aside>

      <!-- 右侧权限配置主区 -->
      <main v-if="selected" class="permission-editor">
        <div class="editor-title">
          <div>
            <strong>{{ selected.project_code }} · {{ selected.project_name }}</strong>
            <span>所有登录用户可查看；仅项目成员可按角色执行提交、处理或验证操作</span>
          </div>
          <el-button type="primary" :loading="saving" @click="save">保存权限设置</el-button>
        </div>

        <div class="open-policy-banner">
          <span class="open-policy-badge">开放模式</span>
          <div>
            <strong>项目内容对登录用户开放查看</strong>
            <p>未加入项目的用户为只读；加入成员后，系统自动带入其默认属性并启用相应操作权限。</p>
          </div>
        </div>

        <!-- 成员列表管理 -->
        <section class="members-section">
          <div class="members-title">
            <div>
              <strong>项目成员权限表</strong>
              <span>成员角色控制缺陷提交、编辑、解决、验证等生命周期操作</span>
            </div>
            <el-button :icon="Plus" :disabled="!availableAccounts.length" @click="addMember">添加成员</el-button>
          </div>

          <div v-if="draftMembers.length" class="member-list">
            <div v-for="(member, index) in draftMembers" :key="`${member.user_id}-${index}`" class="member-row">
              <span class="member-avatar"><el-icon><User /></el-icon></span>
              <el-select :model-value="member.user_id" filterable class="member-select" @change="changeMemberUser(index, $event)">
                <el-option
                  v-for="account in props.meta?.accounts || []"
                  :key="account.id"
                  :label="account.nickname ? `${account.nickname} (${account.username})` : account.username"
                  :value="account.id"
                  :disabled="draftMembers.some((entry, entryIndex) => entryIndex !== index && entry.user_id === account.id)"
                />
              </el-select>
              <el-select v-model="member.role_key" class="role-select">
                <el-option v-for="role in roleOptions" :key="role.value" :label="role.label" :value="role.value">
                  <div class="role-option">
                    <strong>{{ role.label }}</strong>
                    <small>{{ role.description }}</small>
                  </div>
                </el-option>
              </el-select>
              <el-button text type="danger" @click="draftMembers.splice(index, 1)">移除</el-button>
            </div>
          </div>

          <div v-else class="member-empty">
            <el-icon><Lock /></el-icon>
            <strong>尚未配置项目成员</strong>
            <span>当前仅管理员可操作，其他登录用户可以只读查看。</span>
          </div>
        </section>

        <!-- 角色权限速查指南 -->
        <div class="role-guide">
          <div v-for="role in roleOptions" :key="role.value" class="role-guide-item">
            <strong>{{ role.label }}</strong>
            <span>{{ role.description }}</span>
          </div>
        </div>
      </main>
    </div>
  </section>
</template>

<style scoped>
.project-permission {
  min-height: 520px;
}

.permission-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding: 16px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.permission-header__title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.permission-header__icon {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  color: #0891b2;
  background: #ecfeff;
  border: 1px solid #a5f3fc;
  font-size: 20px;
}

.permission-header strong {
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
}

.permission-header p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.permission-layout {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  min-height: 540px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.default-role-section {
  margin-bottom: 16px;
  padding: 18px 20px;
  border: 1px solid #dbe5f0;
  border-radius: 14px;
  background: linear-gradient(135deg, #ffffff 0%, #f8fbff 100%);
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.default-role-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.default-role-title > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.default-role-title strong {
  color: #0f172a;
  font-size: 15px;
}

.default-role-title span {
  color: #64748b;
  font-size: 12px;
}

.default-role-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(270px, 1fr));
  gap: 10px;
  margin-top: 16px;
}

.default-role-item {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) 108px;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #e8eef5;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.9);
}

.default-role-user {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.default-role-user strong,
.default-role-user small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.default-role-user strong {
  color: #334155;
  font-size: 13px;
}

.default-role-user small {
  margin-top: 2px;
  color: #94a3b8;
  font-size: 11px;
}

.project-list {
  padding: 16px;
  border-right: 1px solid #e2e8f0;
  background: #f8fafc;
}

.project-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 460px;
  margin-top: 14px;
  overflow: auto;
}

.project-item {
  display: grid;
  grid-template-columns: 60px minmax(0, 1fr) 40px;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}

.project-item:hover {
  background: #f1f5f9;
}

.project-item.is-active {
  background: #eff6ff;
  box-shadow: inset 3px 0 #3b82f6;
}

.project-code {
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.project-name {
  overflow: hidden;
  color: #334155;
  font-size: 13px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-mode {
  padding: 2px 6px;
  border-radius: 4px;
  color: #64748b;
  background: #e2e8f0;
  font-size: 11px;
  font-style: normal;
  text-align: center;
  font-weight: 600;
}

.permission-editor {
  padding: 22px;
}

.editor-title,
.members-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.editor-title > div,
.members-title > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.editor-title strong {
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
}

.editor-title span,
.members-title span {
  color: #64748b;
  font-size: 12px;
}

.open-policy-banner {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 20px;
  padding: 14px 16px;
  border: 1px solid #bfdbfe;
  border-radius: 12px;
  background: #eff6ff;
}

.open-policy-badge {
  flex: 0 0 auto;
  padding: 5px 9px;
  border-radius: 999px;
  color: #1d4ed8;
  background: #dbeafe;
  font-size: 12px;
  font-weight: 700;
}

.open-policy-banner strong {
  color: #1e293b;
  font-size: 14px;
}

.open-policy-banner p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 12px;
}

.members-section {
  margin-top: 18px;
  padding: 18px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
}

.members-title strong {
  color: #1e293b;
  font-size: 14px;
  font-weight: 700;
}

.member-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 16px;
}

.member-row {
  display: grid;
  grid-template-columns: 36px minmax(180px, 1fr) minmax(160px, 0.8fr) 52px;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #eef2f6;
}

.member-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 8px;
  color: #3b82f6;
  background: #eff6ff;
  font-size: 16px;
}

.role-option {
  display: flex;
  flex-direction: column;
}

.role-option strong {
  font-size: 13px;
}

.role-option small {
  color: #94a3b8;
  font-size: 11px;
}

.member-empty {
  display: flex;
  align-items: center;
  flex-direction: column;
  padding: 40px 0;
  color: #94a3b8;
}

.member-empty .el-icon {
  font-size: 26px;
}

.member-empty strong {
  margin-top: 10px;
  color: #475569;
  font-size: 14px;
}

.member-empty span {
  margin-top: 4px;
  font-size: 12px;
}

.role-guide {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 10px;
  margin-top: 16px;
}

.role-guide-item {
  display: flex;
  flex-direction: column;
  padding: 12px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #f1f5f9;
}

.role-guide-item strong {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.role-guide-item span {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .permission-layout {
    grid-template-columns: 240px 1fr;
  }
  .role-guide {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 700px) {
  .permission-layout {
    grid-template-columns: 1fr;
  }
  .project-list {
    border-right: 0;
    border-bottom: 1px solid #e2e8f0;
  }
  .project-items {
    max-height: 180px;
  }
  .member-row {
    grid-template-columns: 34px 1fr;
  }
  .member-row .el-select:nth-of-type(2) {
    grid-column: 2;
  }
  .role-guide {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
