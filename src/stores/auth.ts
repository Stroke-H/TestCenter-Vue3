import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref<any>(null)

  const isLoggedIn = computed(() => !!token.value)

  function setToken(newToken: string) {
    token.value = newToken
    localStorage.setItem('token', newToken)
  }

  function setUser(newUser: any) {
    user.value = newUser
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  async function fetchMe() {
    if (!token.value) return
    try {
      const res = await fetch('http://localhost:8080/api/auth/me', {
        headers: {
          'Authorization': token.value
        }
      })
      if (res.ok) {
        user.value = await res.json()
      } else {
        logout()
      }
    } catch (err) {
      console.error('Failed to fetch user info', err)
      logout()
    }
  }

  return {
    token,
    user,
    isLoggedIn,
    setToken,
    setUser,
    logout,
    fetchMe
  }
})
