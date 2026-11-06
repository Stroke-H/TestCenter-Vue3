<script setup lang="ts">
import { computed } from 'vue'
import {
  Calendar,
  Wallet,
  Connection,
  ArrowRight
} from '@element-plus/icons-vue'
import type { PaymentSummary } from '../types'
import { formatAmount } from '../types'

const props = defineProps<{
  summary: PaymentSummary
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'quick-filter', type: 'pending_declaration' | 'refunded' | 'all'): void
}>()

// 虚拟卡初始额度 $500.00
const initialBalance = 500
const balancePercentage = computed(() => {
  const balance = Math.max(0, props.summary.available_balance || 0)
  return Math.min(100, Math.round((balance / initialBalance) * 100))
})

const balanceStatus = computed(() => {
  if (props.summary.available_balance > 200) {
    return { text: '额度充足', tone: 'success' }
  } else if (props.summary.available_balance > 50) {
    return { text: '额度正常', tone: 'warning' }
  } else {
    return { text: '额度偏低', tone: 'danger' }
  }
})

const monthAvgAmount = computed(() => {
  if (!props.summary.current_month_count) return 0
  return props.summary.current_month_amount / props.summary.current_month_count
})
</script>

<template>
  <section class="summary-cards-container">
    <!-- 卡片 1: 虚拟卡可用额度（突出主卡样式） -->
    <div class="stat-card stat-card--balance">
      <div class="balance-card-bg-glow"></div>
      <div class="card-inner">
        <div class="card-top">
          <div class="card-account">
            <div class="card-badge">
              <span class="chip-dot"></span>
              虚拟卡专户
            </div>
            <span class="card-number">•••• •••• •••• ••••</span>
          </div>
          <span class="card-status-pill" :class="`is-${balanceStatus.tone}`">
            {{ balanceStatus.text }}
          </span>
        </div>

        <div class="card-body">
          <span class="label">可用支付余额</span>
          <div class="amount-wrap">
            <h2 class="amount">{{ formatAmount(summary.available_balance) }}</h2>
          </div>
        </div>

        <div class="card-footer-metric">
          <div class="progress-bar-wrap">
            <div class="progress-bar-track">
              <div
                class="progress-bar-fill"
                :style="{ width: `${balancePercentage}%` }"
              ></div>
            </div>
            <span class="progress-label">{{ balancePercentage }}% 可用</span>
          </div>
          <p class="meta-tip">
            初始额度 $500.00 · 已退款记录不计入支出
          </p>
        </div>
      </div>
    </div>

    <!-- 卡片 2: 本月支出 -->
    <div class="stat-card stat-card--standard">
      <div class="card-inner">
        <div class="card-header-row">
          <div class="icon-avatar is-emerald">
            <el-icon><Calendar /></el-icon>
          </div>
          <span class="tag-label">自然月统计</span>
        </div>

        <div class="card-body">
          <span class="label">本月支出总额</span>
          <h2 class="amount amount--emerald">
            {{ formatAmount(summary.current_month_amount) }}
          </h2>
        </div>

        <div class="card-footer-row">
          <div class="sub-stat">
            <span class="sub-value">{{ summary.current_month_count }}</span>
            <span class="sub-label">笔支出</span>
          </div>
          <span class="divider-dot">·</span>
          <div class="sub-stat">
            <span class="sub-value">{{ formatAmount(monthAvgAmount) }}</span>
            <span class="sub-label">均笔支出</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 卡片 3: 累计流水支出 -->
    <div class="stat-card stat-card--standard">
      <div class="card-inner">
        <div class="card-header-row">
          <div class="icon-avatar is-indigo">
            <el-icon><Wallet /></el-icon>
          </div>
          <span class="tag-label">全周期累计</span>
        </div>

        <div class="card-body">
          <span class="label">累计流水总额</span>
          <h2 class="amount amount--indigo">
            {{ formatAmount(summary.total_amount) }}
          </h2>
        </div>

        <div class="card-footer-row">
          <div class="sub-stat">
            <span class="sub-value">{{ summary.count }}</span>
            <span class="sub-label">笔总流水</span>
          </div>
          <span class="divider-dot">·</span>
          <div class="sub-stat">
            <span class="sub-label">含申报与退款流转</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 卡片 4: 快速流转与飞书同步 -->
    <div class="stat-card stat-card--standard stat-card--actions">
      <div class="card-inner">
        <div class="card-header-row">
          <div class="icon-avatar is-violet">
            <el-icon><Connection /></el-icon>
          </div>
          <div class="sync-indicator">
            <span class="indicator-dot"></span>
            <span>飞书表格联动</span>
          </div>
        </div>

        <div class="card-body">
          <span class="label">业务快速筛选</span>
          <div class="quick-actions-list">
            <button
              type="button"
              class="quick-action-btn"
              @click="emit('quick-filter', 'pending_declaration')"
            >
              <span>待对公申报</span>
              <el-icon class="arrow"><ArrowRight /></el-icon>
            </button>
            <button
              type="button"
              class="quick-action-btn"
              @click="emit('quick-filter', 'refunded')"
            >
              <span>已退款订单</span>
              <el-icon class="arrow"><ArrowRight /></el-icon>
            </button>
          </div>
        </div>

        <div class="card-footer-row">
          <span class="info-footnote">
            双向关联飞书多维表格与对公核对流程
          </span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.summary-cards-container {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  position: relative;
  border-radius: 16px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  box-shadow: 0 4px 16px -2px rgba(15, 23, 42, 0.04), 0 2px 6px -1px rgba(15, 23, 42, 0.02);
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  overflow: hidden;
  box-sizing: border-box;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 24px -4px rgba(15, 23, 42, 0.08), 0 4px 10px -2px rgba(15, 23, 42, 0.04);
}

.card-inner {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  height: 100%;
  box-sizing: border-box;
  position: relative;
  z-index: 1;
}

/* 主卡：可用额度卡片（深邃渐变质感） */
.stat-card--balance {
  background: linear-gradient(135deg, #1e293b 0%, #0f172a 100%);
  border: 1px solid #334155;
  color: #ffffff;
}

.balance-card-bg-glow {
  position: absolute;
  top: -40px;
  right: -40px;
  width: 140px;
  height: 140px;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.4) 0%, rgba(99, 102, 241, 0) 70%);
  border-radius: 50%;
  pointer-events: none;
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 600;
  color: #94a3b8;
  letter-spacing: 0.02em;
}

.card-account {
  display: flex;
  flex-direction: column;
  gap: 7px;
  min-width: 0;
}

.card-number {
  color: #e2e8f0;
  font-size: 13px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.06em;
  white-space: nowrap;
  user-select: all;
}

.chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #38bdf8;
  box-shadow: 0 0 8px #38bdf8;
}

.card-status-pill {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 12px;
}

.card-status-pill.is-success {
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.card-status-pill.is-warning {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.card-status-pill.is-danger {
  background: rgba(239, 68, 68, 0.2);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.stat-card--balance .label {
  color: #94a3b8;
  font-size: 12px;
  font-weight: 500;
  margin-bottom: 4px;
}

.stat-card--balance .amount {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.03em;
  margin: 0;
  color: #f8fafc;
  font-variant-numeric: tabular-nums;
}

.progress-bar-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
}

.progress-bar-track {
  flex: 1;
  height: 6px;
  background: rgba(255, 255, 255, 0.12);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background: linear-gradient(90deg, #38bdf8, #818cf8);
  border-radius: 3px;
  transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}

.progress-label {
  font-size: 11px;
  font-weight: 600;
  color: #cbd5e1;
  font-variant-numeric: tabular-nums;
}

.meta-tip {
  margin: 6px 0 0;
  font-size: 11px;
  color: #64748b;
  line-height: 1.4;
}

/* 常规统计卡片 */
.stat-card--standard .card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.icon-avatar {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  font-size: 18px;
}

.icon-avatar.is-emerald {
  background: #ecfdf5;
  color: #059669;
}

.icon-avatar.is-indigo {
  background: #eef2ff;
  color: #4f46e5;
}

.icon-avatar.is-violet {
  background: #f5f3ff;
  color: #7c3aed;
}

.tag-label {
  font-size: 11px;
  font-weight: 500;
  color: #94a3b8;
  background: #f8fafc;
  padding: 3px 8px;
  border-radius: 6px;
}

.stat-card--standard .label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  margin-bottom: 4px;
}

.stat-card--standard .amount {
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin: 0;
  color: #0f172a;
  font-variant-numeric: tabular-nums;
}

.amount--emerald {
  color: #059669;
}

.amount--indigo {
  color: #4338ca;
}

.card-footer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid #f1f5f9;
}

.sub-stat {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.sub-value {
  font-size: 13px;
  font-weight: 700;
  color: #334155;
  font-variant-numeric: tabular-nums;
}

.sub-label {
  font-size: 11px;
  color: #94a3b8;
}

.divider-dot {
  color: #cbd5e1;
  font-size: 12px;
}

/* 卡片 4: 快速操作 */
.sync-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 600;
  color: #059669;
  background: #ecfdf5;
  padding: 3px 8px;
  border-radius: 12px;
}

.indicator-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 6px #10b981;
}

.quick-actions-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 4px;
}

.quick-action-btn {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  color: #334155;
  cursor: pointer;
  transition: all 0.2s ease;
}

.quick-action-btn:hover {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #2563eb;
  transform: translateX(2px);
}

.quick-action-btn .arrow {
  font-size: 12px;
  color: #94a3b8;
  transition: transform 0.2s ease;
}

.quick-action-btn:hover .arrow {
  transform: translateX(2px);
  color: #2563eb;
}

.info-footnote {
  font-size: 11px;
  color: #94a3b8;
}

@media (max-width: 1280px) {
  .summary-cards-container {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 680px) {
  .summary-cards-container {
    grid-template-columns: 1fr;
  }
}
</style>
