<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Avatar, Delete, Picture, UserFilled } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import request from '@/api/request'
import { useAuthStore, type AuthUser } from '@/stores/auth'

interface ProfileForm {
  nickname: string
  email: string
  avatar: string
}

const router = useRouter()
const authStore = useAuthStore()

const loading = ref(false)
const saving = ref(false)
const form = reactive<ProfileForm>({
  nickname: '',
  email: '',
  avatar: ''
})

const currentUser = computed(() => authStore.user)
const displayAvatar = computed(() => form.avatar || currentUser.value?.avatar || '')
const displayName = computed(() => form.nickname || currentUser.value?.nickname || currentUser.value?.username || '当前用户')

function syncForm(user: AuthUser | null) {
  form.nickname = user?.nickname || ''
  form.email = user?.email || ''
  form.avatar = user?.avatar || ''
}

async function ensureCurrentUserLoaded() {
  if (authStore.user) {
    syncForm(authStore.user)
    return
  }

  loading.value = true
  try {
    await authStore.fetchMe()
    syncForm(authStore.user)
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.back()
}

function resetAvatar() {
  ElMessage.info('头像功能暂未开发完成')
}

function showAvatarFeaturePending() {
  ElMessage.info('头像功能暂未开发完成')
}

async function saveProfile() {
  if (!form.nickname.trim()) {
    ElMessage.warning('昵称不能为空')
    return
  }

  saving.value = true
  try {
    const response = await request.put('/auth/profile', {
      nickname: form.nickname.trim(),
      email: form.email.trim(),
      avatar: form.avatar
    }) as { message?: string, user: AuthUser }

    authStore.setUser(response.user)
    syncForm(response.user)
    ElMessage.success(response.message || '个人设置已保存')
  } catch (err) {
    console.error('Failed to save profile', err)
    ElMessage.error('保存个人设置失败')
  } finally {
    saving.value = false
  }
}

onMounted(ensureCurrentUserLoaded)
</script>

<template>
  <div class="profile-settings-page" v-loading="loading">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">个人设置</h1>
          <span class="page-badge">Profile & Identity</span>
        </div>
        <p class="page-desc">维护你在平台内的个人身份信息，更新昵称与联系邮箱，修改后全局同步生效</p>
      </div>
      <div class="header-right">
        <el-button plain :icon="ArrowLeft" class="back-btn" @click="goBack">返回上一页</el-button>
      </div>
    </div>

    <div class="profile-settings-layout">
      <!-- Left Avatar Card -->
      <section class="profile-card profile-card--avatar">
        <div class="avatar-panel">
          <el-avatar
            :size="96"
            :src="displayAvatar || undefined"
            :icon="UserFilled"
            class="avatar-panel__avatar"
          />
          <div class="avatar-panel__meta">
            <div class="avatar-panel__name">{{ displayName }}</div>
            <div class="avatar-panel__sub">账号：{{ currentUser?.username || '未登录用户' }}</div>
          </div>
        </div>

        <div class="avatar-panel__actions">
          <el-button type="primary" plain :icon="Picture" @click="showAvatarFeaturePending">更换头像</el-button>
          <el-button plain :icon="Delete" @click="resetAvatar">移除头像</el-button>
        </div>

        <div class="avatar-panel__tip">
          提示：建议上传清晰方形头像，支持常见图片格式，大小不超过 2MB。
        </div>
      </section>

      <!-- Right Form Card -->
      <section class="profile-card profile-card--form">
        <div class="section-title">
          <el-icon><Avatar /></el-icon>
          <span>基础资料</span>
        </div>

        <el-form label-width="96px" class="profile-form">
          <el-form-item label="登录账号">
            <el-input :model-value="currentUser?.username || ''" disabled />
          </el-form-item>
          <el-form-item label="显示昵称" required>
            <el-input v-model="form.nickname" maxlength="24" show-word-limit placeholder="请输入昵称" />
          </el-form-item>
          <el-form-item label="工作邮箱">
            <el-input v-model="form.email" placeholder="可选，用于接收联系信息与提醒" />
          </el-form-item>
          <el-form-item label="用户唯一 ID">
            <el-input :model-value="currentUser?.id || ''" disabled />
          </el-form-item>
        </el-form>

        <div class="profile-form__footer">
          <el-button @click="syncForm(currentUser)">重置</el-button>
          <el-button type="primary" class="save-btn" :loading="saving" @click="saveProfile">保存设置</el-button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.profile-settings-page {
  padding: 24px;
  max-width: 1680px;
  margin: 0 auto;
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

.back-btn {
  border-radius: 8px;
  font-weight: 500;
}

/* Layout */
.profile-settings-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
}

.profile-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.profile-card--avatar {
  padding: 24px;
  display: flex;
  flex-direction: column;
}

.avatar-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 14px;
}

.avatar-panel__avatar {
  border: 3px solid #eff6ff;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.15);
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: #ffffff;
}

.avatar-panel__name {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.avatar-panel__sub {
  font-size: 13px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.avatar-panel__actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 20px;
}

.avatar-panel__actions :deep(.el-button) {
  border-radius: 8px;
}

.avatar-panel__tip {
  margin-top: 20px;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.6;
  color: #64748b;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
}

.profile-card--form {
  padding: 28px;
}

.section-title {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 24px;
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.section-title .el-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #eff6ff;
  color: #2563eb;
  font-size: 16px;
}

.profile-form {
  max-width: 580px;
}

.profile-form__footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 32px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
}

.save-btn {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
}

@media (max-width: 960px) {
  .profile-settings-layout {
    grid-template-columns: 1fr;
  }
}
</style>
