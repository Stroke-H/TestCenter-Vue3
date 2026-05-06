import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import request from '@/api/request'

export type AuthStatus = 'checking' | 'authenticated' | 'anonymous' | 'unreachable'
export interface AuthUser {
  id: string
  username: string
  nickname: string
  email?: string
  avatar?: string
}

const sessionHintKey = 'testcenter_session_hint'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const hasSessionHint = ref(localStorage.getItem(sessionHintKey) === '1')
  const status = ref<AuthStatus>(hasSessionHint.value ? 'checking' : 'anonymous')
  const token = computed(() => user.value?.id || '')
  const isLoggedIn = computed(() => status.value === 'authenticated')

  function setUser(newUser: AuthUser) {
    user.value = newUser
    status.value = 'authenticated'
    hasSessionHint.value = true
    localStorage.setItem(sessionHintKey, '1')
  }

  function patchUser(partial: Partial<AuthUser>) {
    if (!user.value) return
    user.value = {
      ...user.value,
      ...partial
    }
  }

  function markAnonymous() {
    user.value = null
    status.value = 'anonymous'
    hasSessionHint.value = false
    localStorage.removeItem(sessionHintKey)
  }

  function markUnreachable() {
    status.value = 'unreachable'
  }

  async function logout() {
    try {
      await request.post('/auth/logout')
    } catch (err) {
      console.warn('Failed to revoke server session', err)
    } finally {
      markAnonymous()
    }
  }

  async function fetchMe() {
    if (!hasSessionHint.value && status.value === 'anonymous') return
    status.value = 'checking'
    try {
      const res = await request.get('/auth/me') as AuthUser
      user.value = res
      status.value = 'authenticated'
      hasSessionHint.value = true
      localStorage.setItem(sessionHintKey, '1')
    } catch (err: any) {
      console.error('Failed to fetch user info', err)
      if (err?.response?.status === 401) {
        markAnonymous()
        return
      }
      markUnreachable()
    }
  }

  return {
    token,
    user,
    status,
    hasSessionHint,
    isLoggedIn,
    setUser,
    patchUser,
    markAnonymous,
    markUnreachable,
    logout,
    fetchMe
  }
})
