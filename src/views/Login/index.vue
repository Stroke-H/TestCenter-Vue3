<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { User, Lock, ArrowRight } from '@element-plus/icons-vue'
import request from '@/api/request'

const router = useRouter()
const authStore = useAuthStore()

const loginForm = reactive({
  username: '',
  password: ''
})

const loading = ref(false)

const handleLogin = async () => {
  if (!loginForm.username || !loginForm.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }

  loading.value = true
  try {
    const data: any = await request.post('/auth/login', loginForm)
    
    authStore.setUser(data.user)
    ElMessage.success('登录成功')
    router.push('/dashboard')
  } catch (err: any) {
    const errorMsg = err.customMessage || '登录流程出现异常'
    ElMessage({
      message: errorMsg,
      type: 'error',
      duration: 5000,
      showClose: true
    })
  } finally {
    loading.value = false
  }
}

const goToRegister = () => {
  router.push('/register')
}
</script>

<template>
  <div class="login-container">
    <div class="mesh-bg"></div>
    
    <div class="glass-card">
      <div class="login-header">
        <div class="logo">
          <div class="logo-icon">🚀</div>
        </div>
        <h1>TestCenter</h1>
        <p class="subtitle">欢迎回来，请登录您的账号</p>
      </div>

      <el-form :model="loginForm" class="login-form">
        <el-form-item>
          <el-input
            v-model="loginForm.username"
            placeholder="用户名"
            :prefix-icon="User"
            size="large"
          />
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            :prefix-icon="Lock"
            show-password
            size="large"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <div class="form-actions">
          <el-button
            type="primary"
            class="submit-btn"
            :loading="loading"
            @click="handleLogin"
          >
            登 录
            <el-icon class="ml-2"><ArrowRight /></el-icon>
          </el-button>
        </div>
      </el-form>

      <div class="login-footer">
        <span>还没有账号？</span>
        <el-link type="primary" underline="never" @click="goToRegister">立即注册</el-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  background: #0f172a;
}

.mesh-bg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: 
    radial-gradient(at 0% 0%, rgba(99, 102, 241, 0.15) 0px, transparent 50%),
    radial-gradient(at 100% 0%, rgba(139, 92, 246, 0.15) 0px, transparent 50%),
    radial-gradient(at 100% 100%, rgba(217, 70, 239, 0.15) 0px, transparent 50%),
    radial-gradient(at 0% 100%, rgba(59, 130, 246, 0.15) 0px, transparent 50%);
  filter: blur(80px);
  z-index: 1;
}

.glass-card {
  position: relative;
  z-index: 10;
  width: 420px;
  padding: 48px;
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  border-radius: 16px;
  margin-bottom: 20px;
  box-shadow: 0 8px 16px rgba(99, 102, 241, 0.3);
}

.logo-icon {
  font-size: 32px;
}

h1 {
  font-size: 28px;
  font-weight: 800;
  color: #fff;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.subtitle {
  color: #94a3b8;
  font-size: 15px;
}

.login-form :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.05);
  box-shadow: none !important;
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 8px 16px;
  border-radius: 12px;
  transition: all 0.2s;
}

.login-form :deep(.el-input__wrapper.is-focus) {
  border-color: #6366f1;
  background: rgba(255, 255, 255, 0.08);
}

.login-form :deep(.el-input__inner) {
  color: #fff;
  font-size: 16px;
}

.login-form :deep(.el-input__inner::placeholder) {
  color: #64748b;
}

.login-form :deep(.el-input__prefix-icon) {
  color: #94a3b8;
  font-size: 18px;
}

.form-actions {
  margin-top: 32px;
}

.submit-btn {
  width: 100%;
  height: 48px;
  border-radius: 12px;
  font-size: 16px;
  font-weight: 600;
  background: linear-gradient(90deg, #6366f1 0%, #4f46e5 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.submit-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(99, 102, 241, 0.4);
  opacity: 0.9;
}

.login-footer {
  margin-top: 24px;
  text-align: center;
  color: #94a3b8;
  font-size: 14px;
}

.ml-2 { margin-left: 8px; }
</style>
