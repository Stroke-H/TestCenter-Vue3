<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowLeft,
  Check,
  Connection,
  CopyDocument,
  Link,
  RefreshRight,
  SwitchButton,
  UserFilled
} from '@element-plus/icons-vue'
import request from '@/api/request'

interface OAuthStatus {
  status: 'not_authorized' | 'pending' | 'authorized' | 'expired' | 'failed' | string
  authorized: boolean
  auto_refresh?: boolean
  feishu_user_key?: string
  feishu_name?: string
  feishu_email?: string
  expires_at?: string
  last_error?: string
  session_id?: string
  user_code?: string
  verification_uri?: string
  verification_uri_complete?: string
  poll_interval?: number
}

const router = useRouter()
const loading = ref(false)
const authorizing = ref(false)
const status = ref<OAuthStatus>({ status: 'not_authorized', authorized: false })
let pollTimer: number | undefined

const isAuthorized = computed(() => status.value.authorized && status.value.status === 'authorized')
const isPending = computed(() => status.value.status === 'pending')
const statusLabel = computed(() => {
  if (isAuthorized.value) return '已完成独立授权'
  if (isPending.value) return '等待飞书确认'
  if (status.value.status === 'expired') return '授权已失效'
  if (status.value.status === 'failed') return '授权失败'
  return '尚未授权'
})

const statusDescription = computed(() => {
  if (isAuthorized.value) return '此后同步缺陷时，将使用你自己的飞书项目身份创建记录。'
  if (isPending.value) return '请在打开的飞书页面完成确认，本页面会自动更新授权结果。'
  if (status.value.status === 'expired') return '访问凭证无法继续刷新，请重新完成一次飞书授权。'
  return '授权后，你提交的缺陷会以你的飞书身份同步到对应项目空间。'
})

function errorMessage(error: any, fallback: string) {
  return error?.customMessage || error?.response?.data?.error || fallback
}

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
  }).format(date)
}

function stopPolling() {
  if (pollTimer !== undefined) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

function schedulePoll(interval = status.value.poll_interval || 5) {
  stopPolling()
  pollTimer = window.setTimeout(pollAuthorization, Math.max(2, interval) * 1000)
}

async function loadStatus() {
  loading.value = true
  try {
    status.value = await request.get('/feishu-project/oauth/status') as OAuthStatus
    if (status.value.status === 'pending' && status.value.session_id) {
      authorizing.value = true
      schedulePoll(status.value.poll_interval)
    }
  } catch (error) {
    ElMessage.error(errorMessage(error, '读取飞书项目授权状态失败'))
  } finally {
    loading.value = false
  }
}

async function pollAuthorization() {
  if (!status.value.session_id || !isPending.value) return
  try {
    const result = await request.get('/feishu-project/oauth/device/status', {
      params: { session_id: status.value.session_id }
    }) as OAuthStatus
    status.value = result
    if (result.authorized) {
      authorizing.value = false
      ElMessage.success(`飞书项目授权成功${result.feishu_name ? `：${result.feishu_name}` : ''}`)
      return
    }
    if (result.status === 'pending') {
      schedulePoll(result.poll_interval)
      return
    }
    authorizing.value = false
  } catch (error) {
    authorizing.value = false
    ElMessage.error(errorMessage(error, '查询飞书项目授权结果失败'))
  }
}

async function startAuthorization() {
  stopPolling()
  const authorizationWindow = window.open('', '_blank')
  authorizing.value = true
  try {
    const result = await request.post('/feishu-project/oauth/device/start') as OAuthStatus
    status.value = result
    const target = result.verification_uri_complete || result.verification_uri
    if (target && authorizationWindow) {
      authorizationWindow.location.href = target
    } else if (target) {
      window.open(target, '_blank', 'noopener,noreferrer')
    } else {
      authorizationWindow?.close()
    }
    schedulePoll(result.poll_interval)
  } catch (error) {
    authorizationWindow?.close()
    authorizing.value = false
    ElMessage.error(errorMessage(error, '发起飞书项目授权失败'))
  }
}

async function copyCode() {
  const code = status.value.user_code || ''
  if (!code) return
  try {
    await navigator.clipboard.writeText(code)
    ElMessage.success('授权码已复制')
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = code
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
    ElMessage.success('授权码已复制')
  }
}

function openAuthorizationPage() {
  const target = status.value.verification_uri_complete || status.value.verification_uri
  if (target) window.open(target, '_blank', 'noopener,noreferrer')
}

async function disconnect() {
  try {
    await ElMessageBox.confirm(
      '解除后，你后续提交的缺陷将无法以本人身份同步到飞书项目，确定解除吗？',
      '解除飞书项目授权',
      { type: 'warning', confirmButtonText: '确认解除', cancelButtonText: '取消' }
    )
    await request.delete('/feishu-project/oauth/authorization')
    stopPolling()
    status.value = { status: 'not_authorized', authorized: false }
    ElMessage.success('已解除飞书项目授权')
  } catch (error: any) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(errorMessage(error, '解除授权失败'))
  }
}

onMounted(loadStatus)
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="oauth-page" v-loading="loading">
    <header class="page-header">
      <div>
        <div class="title-row">
          <h1>飞书项目授权</h1>
          <span class="title-badge">Personal OAuth</span>
        </div>
        <p>绑定你的飞书项目身份，让缺陷记录显示真实创建人，并继承你在项目空间中的权限。</p>
      </div>
      <el-button plain :icon="ArrowLeft" @click="router.back()">返回上一页</el-button>
    </header>

    <main class="auth-layout">
      <section class="auth-card hero-card" :class="{ 'hero-card--active': isAuthorized }">
        <div class="hero-icon">
          <el-icon><Connection /></el-icon>
          <span v-if="isAuthorized" class="success-dot"><el-icon><Check /></el-icon></span>
        </div>
        <div class="hero-copy">
          <div class="status-line">
            <h2>{{ statusLabel }}</h2>
            <el-tag :type="isAuthorized ? 'success' : isPending ? 'warning' : 'info'" round effect="light">
              {{ isAuthorized ? '身份可用' : isPending ? '确认中' : '待绑定' }}
            </el-tag>
          </div>
          <p>{{ statusDescription }}</p>
        </div>
        <div class="hero-actions">
          <el-button
            v-if="!isAuthorized && !isPending"
            type="primary"
            :icon="Link"
            :loading="authorizing"
            @click="startAuthorization"
          >授权飞书项目</el-button>
          <el-button v-if="isAuthorized" plain :icon="RefreshRight" :loading="authorizing" @click="startAuthorization">
            重新授权
          </el-button>
          <el-button v-if="isAuthorized" type="danger" plain :icon="SwitchButton" @click="disconnect">
            解除授权
          </el-button>
        </div>
      </section>

      <section v-if="isPending" class="auth-card pending-card">
        <div class="section-heading">
          <div>
            <span class="eyebrow">DEVICE AUTHORIZATION</span>
            <h3>在飞书中完成身份确认</h3>
          </div>
          <span class="pulse-status"><i></i>正在等待授权</span>
        </div>
        <div class="authorization-code-block">
          <span>授权码</span>
          <strong>{{ status.user_code }}</strong>
          <el-button text :icon="CopyDocument" @click="copyCode">复制</el-button>
        </div>
        <div class="pending-actions">
          <el-button type="primary" :icon="Link" @click="openAuthorizationPage">打开飞书确认页</el-button>
          <span>若页面没有自动填写，请在确认页输入上方授权码。</span>
        </div>
      </section>

      <section v-if="isAuthorized" class="auth-card identity-card">
        <div class="section-heading">
          <div>
            <span class="eyebrow">AUTHORIZED IDENTITY</span>
            <h3>当前授权身份</h3>
          </div>
        </div>
        <div class="identity-content">
          <div class="identity-avatar"><el-icon><UserFilled /></el-icon></div>
          <div class="identity-main">
            <strong>{{ status.feishu_name || '飞书用户' }}</strong>
            <span>{{ status.feishu_email || '未返回邮箱' }}</span>
          </div>
          <dl>
            <div><dt>飞书 User Key</dt><dd>{{ status.feishu_user_key || '—' }}</dd></div>
            <div><dt>当前令牌到期</dt><dd>{{ formatDate(status.expires_at) }}</dd></div>
            <div><dt>长期授权</dt><dd>{{ status.auto_refresh ? '自动续期已开启' : '未开启' }}</dd></div>
          </dl>
        </div>
      </section>

      <section v-if="status.last_error" class="error-card">
        <strong>授权状态异常</strong>
        <span>{{ status.last_error }}</span>
      </section>

      <section class="info-grid">
        <article>
          <span class="step">01</span>
          <div><h4>账号一对一绑定</h4><p>授权凭证只属于当前平台账号，不会被其他用户共用。</p></div>
        </article>
        <article>
          <span class="step">02</span>
          <div><h4>以本人身份创建</h4><p>缺陷同步时使用提交人的 OAuth 身份，飞书项目中的创建人保持一致。</p></div>
        </article>
        <article>
          <span class="step">03</span>
          <div><h4>权限严格校验</h4><p>若本人无空间权限或未完成授权，同步会明确失败并保留平台缺陷记录。</p></div>
        </article>
      </section>
    </main>
  </div>
</template>

<style scoped>
.oauth-page { max-width: 1440px; margin: 0 auto; padding: 24px; color: #0f172a; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 24px; margin-bottom: 24px; }
.title-row { display: flex; align-items: center; gap: 12px; }
.title-row h1 { margin: 0; font-size: 24px; letter-spacing: -.02em; }
.title-badge { padding: 4px 10px; border: 1px solid #dbeafe; border-radius: 999px; background: #eff6ff; color: #2563eb; font-size: 11px; font-weight: 700; letter-spacing: .05em; }
.page-header p { margin: 7px 0 0; color: #64748b; font-size: 13px; }
.auth-layout { display: grid; gap: 18px; }
.auth-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 16px; box-shadow: 0 8px 30px rgba(15, 23, 42, .045); }
.hero-card { display: flex; align-items: center; gap: 20px; padding: 28px; background: linear-gradient(135deg, #fff 35%, #f8fbff 100%); }
.hero-card--active { border-color: #bbf7d0; background: linear-gradient(135deg, #fff 35%, #f0fdf4 100%); }
.hero-icon { position: relative; display: grid; place-items: center; width: 62px; height: 62px; flex: 0 0 62px; border-radius: 18px; color: #2563eb; background: #eff6ff; font-size: 30px; }
.hero-card--active .hero-icon { color: #16a34a; background: #dcfce7; }
.success-dot { position: absolute; right: -5px; bottom: -5px; display: grid; place-items: center; width: 23px; height: 23px; color: #fff; background: #22c55e; border: 3px solid #fff; border-radius: 50%; font-size: 12px; }
.hero-copy { min-width: 0; flex: 1; }
.status-line { display: flex; align-items: center; gap: 10px; }
.status-line h2 { margin: 0; font-size: 20px; }
.hero-copy p { margin: 7px 0 0; color: #64748b; font-size: 14px; }
.hero-actions { display: flex; align-items: center; gap: 8px; }
.section-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.section-heading h3 { margin: 4px 0 0; font-size: 17px; }
.eyebrow { color: #3b82f6; font-size: 10px; font-weight: 800; letter-spacing: .12em; }
.pending-card, .identity-card { padding: 24px 28px; }
.pulse-status { display: flex; align-items: center; gap: 8px; color: #b45309; font-size: 12px; }
.pulse-status i { width: 8px; height: 8px; border-radius: 50%; background: #f59e0b; box-shadow: 0 0 0 5px #fef3c7; }
.authorization-code-block { display: flex; align-items: center; min-height: 76px; padding: 0 18px; border: 1px dashed #bfdbfe; border-radius: 13px; background: #f8fbff; }
.authorization-code-block > span { color: #64748b; font-size: 13px; }
.authorization-code-block strong { margin-left: 28px; flex: 1; color: #1d4ed8; font: 700 28px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .1em; }
.pending-actions { display: flex; align-items: center; gap: 14px; margin-top: 16px; }
.pending-actions span { color: #94a3b8; font-size: 12px; }
.identity-content { display: grid; grid-template-columns: 52px minmax(160px, .7fr) 1.5fr; align-items: center; gap: 16px; }
.identity-avatar { display: grid; place-items: center; width: 52px; height: 52px; border-radius: 15px; background: #ecfdf5; color: #16a34a; font-size: 25px; }
.identity-main { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.identity-main strong { font-size: 16px; }
.identity-main span { overflow: hidden; color: #64748b; font-size: 12px; text-overflow: ellipsis; }
.identity-content dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin: 0; }
.identity-content dl div { padding: 12px 14px; border-radius: 10px; background: #f8fafc; }
.identity-content dt { color: #94a3b8; font-size: 11px; }
.identity-content dd { overflow: hidden; margin: 5px 0 0; color: #334155; font-size: 12px; text-overflow: ellipsis; }
.error-card { display: flex; align-items: flex-start; gap: 14px; padding: 15px 18px; border: 1px solid #fecaca; border-radius: 12px; background: #fff7f7; color: #b91c1c; font-size: 13px; }
.error-card span { color: #7f1d1d; }
.info-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; }
.info-grid article { display: flex; gap: 14px; min-height: 92px; padding: 18px; border: 1px solid #e2e8f0; border-radius: 14px; background: #fff; }
.step { color: #93c5fd; font-size: 12px; font-weight: 800; }
.info-grid h4 { margin: 0 0 7px; font-size: 14px; }
.info-grid p { margin: 0; color: #64748b; font-size: 12px; line-height: 1.65; }
@media (max-width: 900px) {
  .hero-card { align-items: flex-start; flex-wrap: wrap; }
  .hero-actions { width: 100%; padding-left: 82px; }
  .identity-content { grid-template-columns: 52px 1fr; }
  .identity-content dl { grid-column: 1 / -1; }
  .info-grid { grid-template-columns: 1fr; }
}
@media (max-width: 640px) {
  .oauth-page { padding: 16px; }
  .page-header { align-items: flex-start; }
  .hero-card { padding: 20px; }
  .hero-actions { padding-left: 0; }
  .authorization-code-block strong { margin-left: 14px; font-size: 20px; }
  .pending-actions { align-items: flex-start; flex-direction: column; }
  .identity-content dl { grid-template-columns: 1fr; }
}
</style>
