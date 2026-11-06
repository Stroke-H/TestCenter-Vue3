<script setup lang="ts">
import { computed } from 'vue'
import {
  Search,
  RefreshRight,
  Filter,
  Calendar
} from '@element-plus/icons-vue'
import type { PaymentFilters } from '../types'
import { paymentMethods, declarationStatuses } from '../types'

const props = defineProps<{
  filters: PaymentFilters
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'search'): void
  (e: 'reset'): void
  (e: 'update:filters', val: PaymentFilters): void
}>()

// 快速预设标签
type QuickTab = 'all' | 'unrefunded' | 'refunded' | 'pending_declaration' | 'declared' | 'sync_failed'

const activeQuickTab = computed<QuickTab>(() => {
  if (props.filters.feishu_sync_status === '同步失败') return 'sync_failed'
  if (props.filters.refund_status === '已退款') return 'refunded'
  if (props.filters.refund_status === '未退款' && !props.filters.corporate_declaration_status) return 'unrefunded'
  if (props.filters.corporate_declaration_status === '未发起' || props.filters.corporate_declaration_status === '已发起') return 'pending_declaration'
  if (props.filters.corporate_declaration_status === '申报完成') return 'declared'
  if (
    !props.filters.refund_status &&
    !props.filters.corporate_declaration_status &&
    !props.filters.feishu_sync_status
  ) {
    return 'all'
  }
  return 'all'
})

function selectQuickTab(tab: QuickTab) {
  const next = { ...props.filters, page: 1 }
  if (tab === 'all') {
    next.refund_status = ''
    next.corporate_declaration_status = ''
    next.feishu_sync_status = ''
  } else if (tab === 'unrefunded') {
    next.refund_status = '未退款'
    next.corporate_declaration_status = ''
    next.feishu_sync_status = ''
  } else if (tab === 'refunded') {
    next.refund_status = '已退款'
    next.corporate_declaration_status = ''
    next.feishu_sync_status = ''
  } else if (tab === 'pending_declaration') {
    next.refund_status = ''
    next.corporate_declaration_status = '已发起'
    next.feishu_sync_status = ''
  } else if (tab === 'declared') {
    next.refund_status = ''
    next.corporate_declaration_status = '申报完成'
    next.feishu_sync_status = ''
  } else if (tab === 'sync_failed') {
    next.refund_status = ''
    next.corporate_declaration_status = ''
    next.feishu_sync_status = '同步失败'
  }
  emit('update:filters', next)
  emit('search')
}

// 日期快速预设
function setDateShortcut(type: 'today' | 'week' | 'month' | 'clear') {
  const next = { ...props.filters, page: 1 }
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const formatDate = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

  if (type === 'today') {
    const todayStr = formatDate(now)
    next.date_range = [todayStr, todayStr]
  } else if (type === 'week') {
    const start = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
    next.date_range = [formatDate(start), formatDate(now)]
  } else if (type === 'month') {
    const start = new Date(now.getFullYear(), now.getMonth(), 1)
    next.date_range = [formatDate(start), formatDate(now)]
  } else {
    next.date_range = []
  }
  emit('update:filters', next)
  emit('search')
}

const activeFiltersCount = computed(() => {
  let count = 0
  if (props.filters.search) count++
  if (props.filters.payment_method) count++
  if (props.filters.refund_status) count++
  if (props.filters.corporate_declaration_status) count++
  if (props.filters.feishu_sync_status) count++
  if (props.filters.date_range && props.filters.date_range.length > 0) count++
  return count
})
</script>

<template>
  <div class="filter-panel">
    <!-- 顶部快捷切换药丸 Tab -->
    <div class="quick-tabs-bar">
      <div class="tabs-scroll-wrap">
        <button
          type="button"
          class="quick-tab-pill"
          :class="{ 'is-active': activeQuickTab === 'all' }"
          @click="selectQuickTab('all')"
        >
          全部记录
        </button>
        <button
          type="button"
          class="quick-tab-pill"
          :class="{ 'is-active': activeQuickTab === 'unrefunded' }"
          @click="selectQuickTab('unrefunded')"
        >
          未退款支出
        </button>
        <button
          type="button"
          class="quick-tab-pill"
          :class="{ 'is-active': activeQuickTab === 'refunded' }"
          @click="selectQuickTab('refunded')"
        >
          已退款订单
        </button>
        <button
          type="button"
          class="quick-tab-pill"
          :class="{ 'is-active': activeQuickTab === 'pending_declaration' }"
          @click="selectQuickTab('pending_declaration')"
        >
          对公申报中
        </button>
        <button
          type="button"
          class="quick-tab-pill"
          :class="{ 'is-active': activeQuickTab === 'declared' }"
          @click="selectQuickTab('declared')"
        >
          申报完成
        </button>
        <button
          type="button"
          class="quick-tab-pill pill-danger"
          :class="{ 'is-active': activeQuickTab === 'sync_failed' }"
          @click="selectQuickTab('sync_failed')"
        >
          同步异常
        </button>
      </div>

      <div class="quick-date-shortcuts">
        <span class="date-label"><el-icon><Calendar /></el-icon>快捷周期:</span>
        <button type="button" class="date-chip" @click="setDateShortcut('today')">今日</button>
        <button type="button" class="date-chip" @click="setDateShortcut('week')">近7天</button>
        <button type="button" class="date-chip" @click="setDateShortcut('month')">本月</button>
      </div>
    </div>

    <!-- 筛选条件网格 -->
    <div class="filter-controls-row">
      <div class="filter-field search-field">
        <el-input
          v-model="filters.search"
          clearable
          :prefix-icon="Search"
          placeholder="搜索支付人、消费应用、备注或创建人..."
          @keyup.enter="emit('search')"
          @clear="emit('search')"
        />
      </div>

      <div class="filter-field date-field">
        <el-date-picker
          v-model="filters.date_range"
          type="daterange"
          value-format="YYYY-MM-DD"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          range-separator="至"
          clearable
          @change="emit('search')"
        />
      </div>

      <div class="filter-field select-field">
        <el-select
          v-model="filters.payment_method"
          clearable
          placeholder="支付方式"
          @change="emit('search')"
        >
          <el-option
            v-for="method in paymentMethods"
            :key="method"
            :label="method"
            :value="method"
          >
            <div class="select-option-item">
              <span class="method-color-dot" :class="`is-${method.toLowerCase()}`"></span>
              <span>{{ method }}</span>
            </div>
          </el-option>
        </el-select>
      </div>

      <div class="filter-field select-field">
        <el-select
          v-model="filters.corporate_declaration_status"
          clearable
          placeholder="申报状态"
          @change="emit('search')"
        >
          <el-option
            v-for="status in declarationStatuses"
            :key="status"
            :label="status"
            :value="status"
          />
        </el-select>
      </div>

      <div class="filter-actions">
        <el-button
          type="primary"
          :icon="Search"
          :loading="loading"
          @click="emit('search')"
        >
          查询
        </el-button>
        <el-button
          :icon="RefreshRight"
          @click="emit('reset')"
        >
          重置
        </el-button>
      </div>
    </div>

    <!-- 活跃条件提示栏（有过滤时显示） -->
    <div v-if="activeFiltersCount > 0" class="active-filters-bar">
      <div class="active-summary">
        <el-icon><Filter /></el-icon>
        <span>已启用 {{ activeFiltersCount }} 项筛选条件</span>
      </div>
      <el-button
        type="primary"
        link
        size="small"
        @click="emit('reset')"
      >
        清空全部筛选
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.filter-panel {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 18px 20px;
  margin-bottom: 20px;
  box-shadow: 0 4px 16px -2px rgba(15, 23, 42, 0.03);
}

.quick-tabs-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 14px;
  margin-bottom: 16px;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.tabs-scroll-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}

.tabs-scroll-wrap::-webkit-scrollbar {
  display: none;
}

.quick-tab-pill {
  padding: 5px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.quick-tab-pill:hover {
  background: #f1f5f9;
  color: #334155;
  border-color: #cbd5e1;
}

.quick-tab-pill.is-active {
  background: #2563eb;
  border-color: #2563eb;
  color: #ffffff;
  font-weight: 600;
  box-shadow: 0 2px 6px rgba(37, 99, 235, 0.25);
}

.quick-tab-pill.pill-danger.is-active {
  background: #ef4444;
  border-color: #ef4444;
  color: #ffffff;
  box-shadow: 0 2px 6px rgba(239, 68, 68, 0.25);
}

.quick-date-shortcuts {
  display: flex;
  align-items: center;
  gap: 6px;
}

.date-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #94a3b8;
}

.date-chip {
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 500;
  color: #475569;
  background: #f1f5f9;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s ease;
}

.date-chip:hover {
  background: #e2e8f0;
  color: #1e293b;
}

.filter-controls-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.search-field {
  flex: 1 1 260px;
  min-width: 220px;
}

.date-field {
  flex: 1 1 270px;
  min-width: 240px;
}

.select-field {
  flex: 0 1 160px;
  min-width: 130px;
}

.select-option-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.method-color-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.method-color-dot.is-stripe {
  background: #6366f1;
}

.method-color-dot.is-breeze {
  background: #10b981;
}

.method-color-dot.is-adyen {
  background: #f59e0b;
}

.method-color-dot.is-paypal {
  background: #0284c7;
}

.filter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.active-filters-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed #e2e8f0;
}

.active-summary {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #2563eb;
  font-weight: 500;
}

@media (max-width: 900px) {
  .filter-actions {
    margin-left: 0;
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
