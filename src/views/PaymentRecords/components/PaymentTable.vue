<script setup lang="ts">
import {
  Calendar,
  CreditCard,
  Edit,
  View,
  Check,
  Warning,
  ArrowDown,
  Loading
} from '@element-plus/icons-vue'
import type { PaymentRecord } from '../types'
import { formatDateTime, formatAmount } from '../types'

const props = defineProps<{
  records: PaymentRecord[]
  total: number
  loading: boolean
  currentPage: number
  pageSize: number
  statusUpdatingId?: string
}>()

const emit = defineEmits<{
  (e: 'update:currentPage', val: number): void
  (e: 'update:pageSize', val: number): void
  (e: 'page-change'): void
  (e: 'view-detail', record: PaymentRecord): void
  (e: 'edit', record: PaymentRecord): void
  (e: 'refund', record: PaymentRecord): void
  (e: 'declaration-change', record: PaymentRecord, status: PaymentRecord['corporate_declaration_status']): void
}>()

function payerInitial(name: string) {
  if (!name) return '?'
  return name.trim().slice(0, 1).toUpperCase()
}

// 抽取消费应用的平台与名称
function parseAppInfo(appName: string) {
  if (!appName) return { name: '—', platform: '' }
  if (appName.includes(' - iOS')) {
    return { name: appName.replace(' - iOS', ''), platform: 'iOS' }
  }
  if (appName.includes(' - Android')) {
    return { name: appName.replace(' - Android', ''), platform: 'Android' }
  }
  return { name: appName, platform: '' }
}

function handleRowClick(row: PaymentRecord) {
  emit('view-detail', row)
}

function declarationOptions(record: PaymentRecord) {
  return (['未发起', '已发起', '申报完成'] as const).filter(status => status !== (record.corporate_declaration_status || '未发起'))
}
</script>

<template>
  <div class="table-card">
    <div class="table-header-bar">
      <div class="heading-left">
        <h3 class="heading-title">支付流水明细</h3>
        <span class="count-badge">{{ total }} 笔流水</span>
      </div>
      <div class="heading-right">
        <span class="table-tip">点击任意数据行可展开详细流转与飞书多维表格信息</span>
      </div>
    </div>

    <el-table
      v-loading="loading"
      :data="records"
      row-key="id"
      class="custom-payment-table"
      highlight-current-row
      @row-click="handleRowClick"
    >
      <!-- 支付时间 -->
      <el-table-column label="支付时间" min-width="170">
        <template #default="{ row }">
          <div class="time-cell">
            <el-icon class="time-icon"><Calendar /></el-icon>
            <span class="time-text">{{ formatDateTime(row.payment_time) }}</span>
          </div>
        </template>
      </el-table-column>

      <!-- 支付人 -->
      <el-table-column label="支付人" min-width="160">
        <template #default="{ row }">
          <div class="payer-cell">
            <div class="avatar-initial">{{ payerInitial(row.payer) }}</div>
            <div class="payer-meta">
              <span class="payer-name">{{ row.payer }}</span>
              <span
                v-if="row.created_by_name && row.created_by_name !== row.payer"
                class="creator-hint"
              >
                录入: {{ row.created_by_name }}
              </span>
            </div>
          </div>
        </template>
      </el-table-column>

      <!-- 支付金额 -->
      <el-table-column label="支付金额" width="140" align="right">
        <template #default="{ row }">
          <div class="amount-cell-wrap">
            <div
              class="amount-display"
              :class="{ 'is-refunded': row.refund_status === '已退款' }"
            >
              {{ formatAmount(row.amount, row.currency || 'USD') }}
            </div>
          </div>
        </template>
      </el-table-column>

      <!-- 支付方式 -->
      <el-table-column label="支付方式" width="120" align="center">
        <template #default="{ row }">
          <span
            v-if="row.payment_method"
            class="method-badge"
            :class="`is-${row.payment_method.toLowerCase()}`"
          >
            {{ row.payment_method }}
          </span>
          <span v-else class="text-muted">—</span>
        </template>
      </el-table-column>

      <!-- 消费应用 -->
      <el-table-column label="消费应用" min-width="180">
        <template #default="{ row }">
          <div class="app-cell">
            <span class="app-name">{{ parseAppInfo(row.consumer_app).name }}</span>
            <span
              v-if="parseAppInfo(row.consumer_app).platform"
              class="platform-tag"
              :class="`is-${parseAppInfo(row.consumer_app).platform.toLowerCase()}`"
            >
              {{ parseAppInfo(row.consumer_app).platform }}
            </span>
          </div>
        </template>
      </el-table-column>

      <!-- 退款状态（核心业务状态，提前至显著位置展示） -->
      <el-table-column label="退款状态" width="120" align="center">
        <template #default="{ row }">
          <button
            v-if="row.refund_status !== '已退款'"
            type="button"
            class="status-pill is-unrefunded is-interactive"
            :disabled="!!statusUpdatingId"
            title="点击变更为已退款"
            @click.stop="emit('refund', row)"
          >
            <el-icon v-if="statusUpdatingId === row.id" class="is-loading"><Loading /></el-icon>
            <span v-else class="status-dot"></span>
            {{ row.refund_status || '未退款' }}
          </button>
          <span
            v-else
            class="status-pill"
            :class="row.refund_status === '已退款' ? 'is-refunded' : 'is-unrefunded'"
          >
            <span class="status-dot"></span>
            {{ row.refund_status || '未退款' }}
          </span>
        </template>
      </el-table-column>

      <!-- 对公申报状态 -->
      <el-table-column label="对公申报状态" width="140" align="center">
        <template #default="{ row }">
          <div v-if="row.corporate_declaration_status !== '申报完成'" class="status-action" @click.stop>
            <el-dropdown trigger="click" :disabled="!!statusUpdatingId" popper-class="payment-status-dropdown" @command="(value: PaymentRecord['corporate_declaration_status']) => emit('declaration-change', row, value)">
              <button type="button" class="status-pill is-interactive" :class="row.corporate_declaration_status === '已发起' ? 'is-started' : 'is-pending'" :disabled="!!statusUpdatingId" title="点击更改申报状态">
                <el-icon v-if="statusUpdatingId === row.id" class="is-loading"><Loading /></el-icon>
                <span v-else class="status-dot"></span>
                {{ row.corporate_declaration_status || '未发起' }}
                <el-icon><ArrowDown /></el-icon>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-for="status in declarationOptions(row)" :key="status" :command="status">{{ status }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
          <span
            v-else
            class="status-pill"
            :class="{
              'is-done': row.corporate_declaration_status === '申报完成',
              'is-started': row.corporate_declaration_status === '已发起',
              'is-pending': !row.corporate_declaration_status || row.corporate_declaration_status === '未发起'
            }"
          >
            <span class="status-dot"></span>
            {{ row.corporate_declaration_status || '未发起' }}
          </span>
        </template>
      </el-table-column>

      <!-- 备注 -->
      <el-table-column label="备注" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="remark-text">{{ row.remark || '—' }}</span>
        </template>
      </el-table-column>

      <!-- 飞书同步（与退款状态位置互换，移至明细后部） -->
      <el-table-column label="飞书同步" width="120" align="center">
        <template #default="{ row }">
          <el-tooltip
            :content="
              row.feishu_sync_error ||
              (row.feishu_synced_at ? `已同步于 ${formatDateTime(row.feishu_synced_at)}` : '待同步多维表格')
            "
            placement="top"
          >
            <span
              class="sync-pill"
              :class="`is-${row.feishu_sync_status || '未同步'}`"
            >
              <el-icon v-if="row.feishu_sync_status === '已同步'"><Check /></el-icon>
              <el-icon v-else-if="row.feishu_sync_status === '同步失败'"><Warning /></el-icon>
              <span v-else class="sync-dot"></span>
              {{ row.feishu_sync_status || '未同步' }}
            </span>
          </el-tooltip>
        </template>
      </el-table-column>

      <!-- 操作 -->
      <el-table-column label="操作" width="130" fixed="right" align="center">
        <template #default="{ row }">
          <div class="actions-group" @click.stop>
            <el-button
              link
              type="primary"
              :icon="View"
              size="small"
              @click="emit('view-detail', row)"
            >
              详情
            </el-button>
            <el-button
              link
              type="primary"
              :icon="Edit"
              size="small"
              @click="emit('edit', row)"
            >
              编辑
            </el-button>
          </div>
        </template>
      </el-table-column>

      <!-- 空状态 -->
      <template #empty>
        <div class="empty-state-box">
          <div class="empty-icon-wrap">
            <el-icon><CreditCard /></el-icon>
          </div>
          <h4>暂无匹配的支付记录</h4>
          <p>请尝试重置筛选条件，或点击右上角登记新的测试/业务支付记录</p>
        </div>
      </template>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-footer">
      <el-pagination
        :current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @update:current-page="(val: number) => { emit('update:currentPage', val); emit('page-change') }"
        @update:page-size="(val: number) => { emit('update:pageSize', val); emit('page-change') }"
      />
    </div>
  </div>
</template>

<style scoped>
.table-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  box-shadow: 0 4px 16px -2px rgba(15, 23, 42, 0.035);
  overflow: hidden;
}

.table-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
  gap: 12px;
}

.heading-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.heading-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.01em;
}

.count-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 10px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.table-tip {
  font-size: 12px;
  color: #94a3b8;
}

.custom-payment-table {
  width: 100%;
  cursor: pointer;
}

.custom-payment-table :deep(.el-table__header th) {
  background: #f8fafc;
  color: #475569;
  font-weight: 600;
  font-size: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #e2e8f0;
}

.custom-payment-table :deep(.el-table__row) {
  transition: background-color 0.15s ease;
}

.custom-payment-table :deep(.el-table__row:hover) {
  background-color: #f8fafc;
}

.time-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #334155;
  font-variant-numeric: tabular-nums;
  font-size: 13px;
}

.time-icon {
  color: #94a3b8;
  font-size: 14px;
}

.payer-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.avatar-initial {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: linear-gradient(135deg, #e0e7ff 0%, #c7d2fe 100%);
  color: #4338ca;
  font-size: 13px;
  font-weight: 700;
  flex-shrink: 0;
}

.payer-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.payer-name {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.creator-hint {
  font-size: 11px;
  color: #94a3b8;
}

.amount-cell-wrap {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.amount-display {
  font-size: 14px;
  font-weight: 700;
  color: #059669;
  font-variant-numeric: tabular-nums;
}

.amount-display.is-refunded {
  color: #94a3b8;
  text-decoration: line-through;
}

.refund-badge {
  display: inline-block;
  font-size: 10px;
  font-weight: 600;
  color: #64748b;
  background: #f1f5f9;
  padding: 1px 6px;
  border-radius: 4px;
}

.method-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  text-transform: capitalize;
}

.method-badge.is-stripe {
  background: #eef2ff;
  color: #4f46e5;
  border: 1px solid #e0e7ff;
}

.method-badge.is-breeze {
  background: #ecfdf5;
  color: #059669;
  border: 1px solid #d1fae5;
}

.method-badge.is-adyen {
  background: #fef3c7;
  color: #d97706;
  border: 1px solid #fde68a;
}

.method-badge.is-paypal {
  background: #e0f2fe;
  color: #0284c7;
  border: 1px solid #bae6fd;
}

.app-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.app-name {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.platform-tag {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
}

.platform-tag.is-ios {
  background: #f1f5f9;
  color: #0f172a;
  border: 1px solid #e2e8f0;
}

.platform-tag.is-android {
  background: #ecfdf5;
  color: #047857;
  border: 1px solid #a7f3d0;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 12px;
}

.status-action { display:inline-flex; }
.status-pill.is-interactive {
  border:0;
  font-family:inherit;
  cursor:pointer;
  transition:box-shadow .18s,filter .18s;
}
.status-pill.is-interactive:hover:not(:disabled) { filter:brightness(.96); box-shadow:0 0 0 3px rgba(59,130,246,.09); }
.status-pill.is-interactive:focus-visible { outline:2px solid #93c5fd; outline-offset:3px; }
.status-pill.is-interactive:disabled { cursor:wait; opacity:.65; }
.status-pill.is-interactive .el-icon { font-size:10px; }
:global(.payment-status-dropdown.el-popper) { border:1px solid #e2e8f0; border-radius:10px; box-shadow:0 10px 30px rgba(15,23,42,.12); }
:global(.payment-status-dropdown .el-dropdown-menu) { padding:6px; border-radius:10px; }
:global(.payment-status-dropdown .el-dropdown-menu__item) { border-radius:6px; padding:7px 14px; font-size:12px; }

.status-pill .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.status-pill.is-done {
  background: #ecfdf5;
  color: #059669;
}
.status-pill.is-done .status-dot {
  background: #10b981;
}

.status-pill.is-started {
  background: #fffbeb;
  color: #d97706;
}
.status-pill.is-started .status-dot {
  background: #f59e0b;
}

.status-pill.is-pending {
  background: #f8fafc;
  color: #64748b;
  border: 1px solid #e2e8f0;
}
.status-pill.is-pending .status-dot {
  background: #94a3b8;
}

.status-pill.is-refunded {
  background: #fff1f2;
  color: #e11d48;
  border: 1px solid #ffe4e6;
}
.status-pill.is-refunded .status-dot {
  background: #f43f5e;
  box-shadow: 0 0 6px rgba(244, 63, 94, 0.4);
}

.status-pill.is-unrefunded {
  background: #f8fafc;
  color: #475569;
  border: 1px solid #e2e8f0;
}
.status-pill.is-unrefunded .status-dot {
  background: #94a3b8;
}

.sync-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 12px;
  cursor: help;
}

.sync-pill.is-已同步 {
  background: #f0fdf4;
  color: #16a34a;
  border: 1px solid #bbf7d0;
}

.sync-pill.is-同步失败 {
  background: #fef2f2;
  color: #dc2626;
  border: 1px solid #fecaca;
}

.sync-pill.is-未同步 {
  background: #f8fafc;
  color: #94a3b8;
  border: 1px solid #e2e8f0;
}

.sync-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #94a3b8;
}

.remark-text {
  font-size: 12px;
  color: #64748b;
}

.text-muted {
  color: #cbd5e1;
}

.actions-group {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.pagination-footer {
  display: flex;
  justify-content: flex-end;
  padding: 16px 20px;
  border-top: 1px solid #f1f5f9;
}

.empty-state-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 50px 20px;
}

.empty-icon-wrap {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 26px;
  margin-bottom: 12px;
}

.empty-state-box h4 {
  margin: 0;
  font-size: 15px;
  color: #1e293b;
}

.empty-state-box p {
  margin: 6px 0 0;
  font-size: 12px;
  color: #94a3b8;
}
</style>
