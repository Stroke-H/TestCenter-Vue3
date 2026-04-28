import { defineStore } from 'pinia'
import axios from 'axios'

const API_BASE = '/api/playwright'

export interface TestStep {
  id: string
  keyword: string
  args: Record<string, string>
  return_var: string
  description: string
  disabled: boolean
}

export interface PlaywrightCase {
  id: string
  suite_id: string
  name: string
  description: string
  tags: string[]
  variables: Record<string, string>
  steps: TestStep[]
  created_at: string
  updated_at: string
}

export interface PlaywrightSuite {
  id: string
  name: string
  description: string
  variables: Record<string, string>
  setup: TestStep[]
  teardown: TestStep[]
  case_ids: string[]
  skill_suites: string[]
  created_at: string
  updated_at: string
}

export interface BuiltinKeyword {
  name: string
  keyword: string
  group: string
  args: string[]
}

export const usePlaywrightStore = defineStore('playwright', {
  state: () => ({
    suites: [] as PlaywrightSuite[],
    cases: [] as PlaywrightCase[],
    builtinKeywords: [] as BuiltinKeyword[],
    userKeywords: [] as any[],
    loading: false,
    currentSuite: null as PlaywrightSuite | null,
    currentCase: null as PlaywrightCase | null,
  }),

  actions: {
    async fetchSuites() {
      this.loading = true
      try {
        const res = await axios.get(`${API_BASE}/suites`)
        this.suites = res.data
      } finally {
        this.loading = false
      }
    },

    async fetchCases(suiteId?: string) {
      this.loading = true
      try {
        const url = suiteId ? `${API_BASE}/cases?suite_id=${suiteId}` : `${API_BASE}/cases`
        const res = await axios.get(url)
        this.cases = res.data
      } finally {
        this.loading = false
      }
    },

    async fetchBuiltinKeywords() {
      const res = await axios.get(`${API_BASE}/builtin-keywords`)
      this.builtinKeywords = res.data
    },

    async saveSuite(suite: Partial<PlaywrightSuite>) {
      const res = await axios.post(`${API_BASE}/suites`, suite)
      await this.fetchSuites()
      return res.data
    },

    async saveCase(testCase: Partial<PlaywrightCase>) {
      const res = await axios.post(`${API_BASE}/cases`, testCase)
      if (testCase.suite_id) {
        await this.fetchCases(testCase.suite_id)
      }
      return res.data
    },

    async deleteSuite(id: string) {
      await axios.delete(`${API_BASE}/suites/${id}`)
      await this.fetchSuites()
    },

    async deleteCase(id: string) {
      await axios.delete(`${API_BASE}/cases/${id}`)
      this.cases = this.cases.filter(c => c.id !== id)
    },

    async fetchCaseById(id: string) {
      this.loading = true
      try {
        const res = await axios.get(`${API_BASE}/cases/${id}`)
        this.currentCase = res.data
        return res.data
      } finally {
        this.loading = false
      }
    },

    async fetchUserKeywords() {
      try {
        const res = await axios.get(`${API_BASE}/keywords`)
        this.userKeywords = res.data
      } catch (err) {
        console.error('Failed to fetch user keywords', err)
      }
    }
  }
})
