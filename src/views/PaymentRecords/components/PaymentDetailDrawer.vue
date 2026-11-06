<script setup lang="ts">
import { ElMessage } from 'element-plus'
import {
  Edit,
  DocumentCopy,
  Check,
  Warning
} from '@element-plus/icons-vue'
import type { PaymentRecord } from '../types'
import { formatDateTime, formatAmount } from '../types'

const props = defineProps<{
  visible: boolean
  record: PaymentRecord | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'edit', record: PaymentRecord): void
}>()

function copyText(text: string, label = '内容') {
  if (!text) return
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(`已复制 ${label}`)
  }).catch(() => {
    ElMessage.info('复制失败，请手动选中复制')
  })
}
</script>

<template>
  <el-drawer
    :model-value="visible"
    title="支付流水详情"
    size="520px"
    destroy-on-close
    class="payment-detail-drawer"
    @update:model-value="(val: boolean) => emit('update:visible', val)"
  >
    <div v-if="record" class="drawer-content">
      <!-- 顶部主金额展台 -->
      <div class="amount-hero-banner">
        <div class="hero-label">支付流水金额</div>
        <div class="hero-amount-wrap">
          <span
            class="hero-amount"
            :class="{ 'is-refunded': record.refund_status === '已退款' }"
          >
            {{ formatAmount(record.amount, record.currency || 'USD') }}
          </span>
          <span v-if="record.refund_status === '已退款'" class="hero-refund-tag">
            已退款
          </span>
        </div>

        <div class="hero-tags-row">
          <span
            v-if="record.payment_method"
            class="method-pill"
            :class="`is-${record.payment_method.toLowerCase()}`"
          >
            {{ record.payment_method }}
          </span>
          <span
            class="declaration-pill"
            :class="{
              'is-done': record.corporate_declaration_status === '申报完成',
              'is-started': record.corporate_declaration_status === '已发起',
              'is-pending': !record.corporate_declaration_status || record.corporate_declaration_status === '未发起'
            }"
          >
            {{ record.corporate_declaration_status || '未发起申报' }}
          </span>
        </div>
      </div>

      <!-- 核心信息卡片 -->
      <div class="detail-section-card">
        <h4 class="section-title">交易核心信息</h4>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">支付时间</span>
            <span class="info-value">{{ formatDateTime(record.payment_time) }}</span>
          </div>

          <div class="info-item">
            <span class="info-label">支付人</span>
            <div class="user-inline-value">
              <span class="user-avatar">{{ (record.payer || '?').charAt(0).toUpperCase() }}</span>
              <span class="info-value font-medium">{{ record.payer }}</span>
            </div>
          </div>

          <div class="info-item">
            <span class="info-label">消费业务应用</span>
            <span class="info-value app-name-highlight">{{ record.consumer_app }}</span>
          </div>

          <div class="info-item">
            <span class="info-label">退款状态</span>
            <span
              class="info-value"
              :class="record.refund_status === '已退款' ? 'text-emerald' : 'text-slate'"
            >
              {{ record.refund_status || '未退款' }}
              <small v-if="record.refund_status === '已退款'" class="sub-tip">（已返还可用额度）</small>
            </span>
          </div>
        </div>
      </div>

      <!-- 飞书同步卡片 -->
      <div class="detail-section-card">
        <div class="section-title-row">
          <h4 class="section-title">飞书多维表格同步</h4>
          <span
            class="sync-status-tag"
            :class="`is-${record.feishu_sync_status || '未同步'}`"
          >
            <el-icon v-if="record.feishu_sync_status === '已同步'"><Check /></el-icon>
            <el-icon v-else-if="record.feishu_sync_status === '同步失败'"><Warning /></el-icon>
            {{ record.feishu_sync_status || '未同步' }}
          </span>
        </div>

        <div class="info-grid">
          <div class="info-item full-width">
            <span class="info-label">飞书 Record ID</span>
            <div v-if="record.feishu_record_id" class="id-copy-row">
              <code class="record-id-code">{{ record.feishu_record_id }}</code>
              <el-button
                link
                type="primary"
                :icon="DocumentCopy"
                size="small"
                @click="copyText(record.feishu_record_id, '飞书记录 ID')"
              >
                复制
              </el-button>
            </div>
            <span v-else class="text-muted">暂未生成飞书记录 ID</span>
          </div>

          <div class="info-item">
            <span class="info-label">同步时间</span>
            <span class="info-value">{{ formatDateTime(record.feishu_synced_at) }}</span>
          </div>
        </div>

        <!-- 报错信息展示 -->
        <div v-if="record.feishu_sync_error" class="sync-error-box">
          <div class="error-header">
            <el-icon><Warning /></el-icon>
            <span>飞书多维表格同步失败原因</span>
          </div>
          <pre class="error-pre">{{ record.feishu_sync_error }}</pre>
        </div>
      </div>

      <!-- 系统审计与备注卡片 -->
      <div class="detail-section-card">
        <h4 class="section-title">登记来源与业务备注</h4>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">平台创建人</span>
            <span class="info-value">{{ record.created_by_name || '—' }}</span>
          </div>

          <div class="info-item">
            <span class="info-label">创建时间</span>
            <span class="info-value">{{ formatDateTime(record.created_at) }}</span>
          </div>

          <div class="info-item full-width">
            <span class="info-label">备注说明</span>
            <div class="remark-box">
              {{ record.remark || '无附加备注' }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="drawer-actions-footer">
        <el-button @click="emit('update:visible', false)">关闭</el-button>
        <el-button
          v-if="record"
          type="primary"
          :icon="Edit"
          @click="emit('edit', record)"
        >
          编辑记录
        </el-button>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-bottom: 20px;
}

.amount-hero-banner {
  background: linear-gradient(135deg, #1e293b 0%, #0f172a 100%);
  border-radius: 14px;
  padding: 20px;
  color: #ffffff;
}

.hero-label {
  font-size: 11px;
  font-weight: 600;
  color: #94a3b8;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.hero-amount-wrap {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin: 6px 0 12px;
}

.hero-amount {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #38bdf8;
  font-variant-numeric: tabular-nums;
}

.hero-amount.is-refunded {
  color: #94a3b8;
  text-decoration: line-through;
}

.hero-refund-tag {
  font-size: 11px;
  font-weight: 700;
  color: #cbd5e1;
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 8px;
  border-radius: 6px;
}

.hero-tags-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.method-pill {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
}

.method-pill.is-stripe {
  background: rgba(99, 102, 241, 0.25);
  color: #a5b4fc;
}

.method-pill.is-breeze {
  background: rgba(16, 185, 129, 0.25);
  color: #6ee7b7;
}

.method-pill.is-adyen {
  background: rgba(245, 158, 11, 0.25);
  color: #fcd34d;
}

.method-pill.is-paypal {
  background: rgba(2, 132, 199, 0.25);
  color: #7dd3fc;
}

.declaration-pill {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.1);
  color: #cbd5e1;
}

.declaration-pill.is-done {
  background: rgba(16, 185, 129, 0.25);
  color: #6ee7b7;
}

.declaration-pill.is-started {
  background: rgba(245, 158, 11, 0.25);
  color: #fcd34d;
}

.detail-section-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
}

.section-title {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title-row .section-title {
  margin: 0;
}

.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-item.full-width {
  grid-column: 1 / -1;
}

.info-label {
  font-size: 11px;
  color: #64748b;
  font-weight: 500;
}

.info-value {
  font-size: 13px;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.user-inline-value {
  display: flex;
  align-items: center;
  gap: 6px;
}

.user-avatar {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: #e0e7ff;
  color: #4338ca;
  font-size: 11px;
  font-weight: 700;
}

.app-name-highlight {
  font-weight: 600;
  color: #2563eb;
}

.text-emerald {
  color: #059669;
  font-weight: 600;
}

.text-slate {
  color: #475569;
}

.sub-tip {
  font-size: 11px;
  color: #94a3b8;
  font-weight: normal;
}

.sync-status-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
}

.sync-status-tag.is-已同步 {
  background: #ecfdf5;
  color: #059669;
}

.sync-status-tag.is-同步失败 {
  background: #fef2f2;
  color: #dc2626;
}

.sync-status-tag.is-未同步 {
  background: #f1f5f9;
  color: #94a3b8;
}

.id-copy-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.record-id-code {
  font-family: monospace;
  font-size: 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  padding: 3px 8px;
  border-radius: 6px;
  color: #334155;
}

.sync-error-box {
  margin-top: 12px;
  padding: 10px 12px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
}

.error-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  color: #dc2626;
  margin-bottom: 6px;
}

.error-pre {
  margin: 0;
  font-family: monospace;
  font-size: 11px;
  color: #991b1b;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 120px;
  overflow-y: auto;
}

.remark-box {
  background: #f8fafc;
  border: 1px solid #f1f5f9;
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  color: #475569;
  line-height: 1.5;
  min-height: 48px;
}

.text-muted {
  font-size: 12px;
  color: #94a3b8;
}

.drawer-actions-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
