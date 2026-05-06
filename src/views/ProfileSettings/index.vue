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
    <div class="profile-settings-header">
      <div>
        <div class="profile-settings-header__eyebrow">个人设置</div>
        <h1 class="profile-settings-header__title">维护你的平台身份信息</h1>
        <p class="profile-settings-header__desc">支持更新头像、昵称和联系邮箱，保存后右上角信息会立即同步。</p>
      </div>
      <el-button plain :icon="ArrowLeft" @click="goBack">返回</el-button>
    </div>

    <div class="profile-settings-layout">
      <section class="profile-card profile-card--avatar">
        <div class="avatar-panel">
          <el-avatar
            :size="104"
            :src="displayAvatar || undefined"
            :icon="UserFilled"
            class="avatar-panel__avatar"
          />
          <div class="avatar-panel__meta">
            <div class="avatar-panel__name">{{ displayName }}</div>
            <div class="avatar-panel__sub">{{ currentUser?.username || '未登录用户' }}</div>
          </div>
        </div>

        <div class="avatar-panel__actions">
          <el-button type="primary" plain :icon="Picture" @click="showAvatarFeaturePending">更换头像</el-button>
          <el-button plain :icon="Delete" @click="resetAvatar">移除头像</el-button>
        </div>

        <div class="avatar-panel__tip">
          建议上传清晰方形头像，支持常见图片格式，大小不超过 2MB。
        </div>
      </section>

      <section class="profile-card profile-card--form">
        <div class="section-title">
          <el-icon><Avatar /></el-icon>
          <span>基础资料</span>
        </div>

        <el-form label-width="92px" class="profile-form">
          <el-form-item label="用户名">
            <el-input :model-value="currentUser?.username || ''" disabled />
          </el-form-item>
          <el-form-item label="昵称">
            <el-input v-model="form.nickname" maxlength="24" show-word-limit placeholder="请输入昵称" />
          </el-form-item>
          <el-form-item label="邮箱">
            <el-input v-model="form.email" placeholder="可选，用于接收联系信息" />
          </el-form-item>
          <el-form-item label="用户 ID">
            <el-input :model-value="currentUser?.id || ''" disabled />
          </el-form-item>
        </el-form>

        <div class="profile-form__footer">
          <el-button @click="syncForm(currentUser)">重置</el-button>
          <el-button type="primary" :loading="saving" @click="saveProfile">保存设置</el-button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.profile-settings-page {
  padding: 24px;
  min-height: 100%;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.08), transparent 28%),
    linear-gradient(180deg, #f8fbff 0%, #f3f6fb 100%);
}

.profile-settings-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 24px;
}

.profile-settings-header__eyebrow {
  font-size: 13px;
  font-weight: 600;
  color: #2563eb;
  margin-bottom: 8px;
}

.profile-settings-header__title {
  margin: 0;
  font-size: 30px;
  font-weight: 800;
  color: #0f172a;
}

.profile-settings-header__desc {
  margin: 10px 0 0;
  font-size: 14px;
  color: #64748b;
  line-height: 1.7;
}

.profile-settings-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
}

.profile-card {
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 24px;
  box-shadow: 0 18px 45px rgba(15, 23, 42, 0.06);
}

.profile-card--avatar {
  padding: 24px;
}

.avatar-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 14px;
}

.avatar-panel__avatar {
  border: 4px solid rgba(59, 130, 246, 0.12);
  box-shadow: 0 12px 30px rgba(59, 130, 246, 0.18);
}

.avatar-panel__name {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}

.avatar-panel__sub {
  font-size: 14px;
  color: #64748b;
}

.avatar-panel__actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 22px;
}

.avatar-panel__tip {
  margin-top: 16px;
  padding: 12px 14px;
  font-size: 13px;
  line-height: 1.7;
  color: #64748b;
  background: #f8fafc;
  border-radius: 14px;
}

.profile-card--form {
  padding: 28px;
}

.section-title {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 24px;
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
}

.section-title .el-icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.profile-form {
  max-width: 620px;
}

.profile-form :deep(.el-input__wrapper) {
  min-height: 44px;
  border-radius: 12px;
}

.profile-form__footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 28px;
}

@media (max-width: 960px) {
  .profile-settings-layout {
    grid-template-columns: 1fr;
  }

  .profile-settings-header {
    flex-direction: column;
  }
}
</style>
