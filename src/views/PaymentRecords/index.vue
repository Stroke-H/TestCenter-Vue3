<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  CreditCard,
  Plus,
  Refresh
} from '@element-plus/icons-vue'
import request from '@/api/request'
import { useAuthStore } from '@/stores/auth'
import type {
  PaymentRecord,
  PaymentSummary,
  PaymentListResponse,
  PaymentForm,
  PaymentFilters,
  PaymentStatusField
} from './types'
import PaymentSummaryCards from './components/PaymentSummaryCards.vue'
import PaymentFilterBar from './components/PaymentFilterBar.vue'
import PaymentTable from './components/PaymentTable.vue'
import PaymentFormDialog from './components/PaymentFormDialog.vue'
import PaymentDetailDrawer from './components/PaymentDetailDrawer.vue'

defineOptions({ name: 'PaymentRecords' })

const authStore = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const statusUpdatingID = ref('')

const records = ref<PaymentRecord[]>([])
const total = ref(0)
const summary = ref<PaymentSummary>({
  count: 0,
  total_amount: 0,
  current_month_count: 0,
  current_month_amount: 0,
  available_balance: 500
})

const filters = reactive<PaymentFilters>({
  search: '',
  payment_method: '',
  refund_status: '',
  corporate_declaration_status: '',
  feishu_sync_status: '',
  date_range: [],
  page: 1,
  page_size: 20
})

// 弹窗与抽屉控制
const dialogVisible = ref(false)
const detailVisible = ref(false)
const editingRecord = ref<PaymentRecord | null>(null)
const selectedRecord = ref<PaymentRecord | null>(null)

const defaultPayer = computed(() => {
  return authStore.user?.nickname || authStore.user?.username || ''
})

function errorMessage(error: any, fallback: string) {
  return error?.response?.data?.error || error?.customMessage || fallback
}

async function loadRecords() {
  loading.value = true
  try {
    const response = (await request.get('/payment-records', {
      params: {
        search: filters.search.trim() || undefined,
        payment_method: filters.payment_method || undefined,
        refund_status: filters.refund_status || undefined,
        corporate_declaration_status: filters.corporate_declaration_status || undefined,
        feishu_sync_status: filters.feishu_sync_status || undefined,
        date_start: filters.date_range?.[0] || undefined,
        date_end: filters.date_range?.[1] || undefined,
        page: filters.page,
        page_size: filters.page_size
      }
    })) as PaymentListResponse

    records.value = Array.isArray(response.items) ? response.items : []
    total.value = Number(response.total || 0)

    const responseSummary = response.summary as Partial<PaymentSummary> | undefined
    summary.value = {
      count: Number(responseSummary?.count || 0),
      total_amount: Number(responseSummary?.total_amount || 0),
      current_month_count: Number(responseSummary?.current_month_count || 0),
      current_month_amount: Number(responseSummary?.current_month_amount || 0),
      available_balance: Number(responseSummary?.available_balance ?? 500)
    }

    // 若当前抽屉开着，更新当前选中对象数据
    if (selectedRecord.value) {
      const updated = records.value.find((r) => r.id === selectedRecord.value?.id)
      if (updated) selectedRecord.value = updated
    }
  } catch (error) {
    ElMessage.error(errorMessage(error, '加载支付流水记录失败'))
  } finally {
    loading.value = false
  }
}

function searchRecords() {
  filters.page = 1
  loadRecords()
}

function resetFilters() {
  filters.search = ''
  filters.payment_method = ''
  filters.refund_status = ''
  filters.corporate_declaration_status = ''
  filters.feishu_sync_status = ''
  filters.date_range = []
  filters.page = 1
  loadRecords()
}

function handleQuickFilter(type: 'pending_declaration' | 'refunded' | 'all') {
  resetFilters()
  if (type === 'pending_declaration') {
    filters.corporate_declaration_status = '已发起'
  } else if (type === 'refunded') {
    filters.refund_status = '已退款'
  }
  loadRecords()
}

function openCreate() {
  editingRecord.value = null
  dialogVisible.value = true
}

function openEdit(record: PaymentRecord) {
  editingRecord.value = { ...record }
  dialogVisible.value = true
}

function openDetail(record: PaymentRecord) {
  selectedRecord.value = record
  detailVisible.value = true
}

function openEditFromDetail(record: PaymentRecord) {
  detailVisible.value = false
  openEdit(record)
}

async function saveQuickStatus(record: PaymentRecord, field: PaymentStatusField, status: string) {
  const response = await request.patch(`/payment-records/${record.id}/status`, {
    field, status, expected_status: record[field] || (field === 'refund_status' ? '未退款' : '未发起')
  }, { timeout: 45000 }) as { record: PaymentRecord; feishu_sync_success: boolean }
  records.value = records.value.map(item => item.id === record.id ? response.record : item)
  if (selectedRecord.value?.id === record.id) selectedRecord.value = response.record
  if (response.feishu_sync_success) {
    ElMessage.success(`已变更为${status}，并同步飞书多维表格`)
  } else {
    ElMessage.warning('状态已保存，但同步飞书多维表格失败，可在详情查看原因')
  }
  await loadRecords()
}

async function confirmRefund(record: PaymentRecord) {
  if (statusUpdatingID.value || record.refund_status === '已退款') return
  statusUpdatingID.value = record.id
  try {
    await ElMessageBox.confirm(`是否将 ${record.payer} 的这笔 ${record.consumer_app} 支付记录变更为“已退款”？确认后将同步飞书多维表格，并重新计算虚拟卡余额。`, '变更退款状态', {
      confirmButtonText: '确认已退款', cancelButtonText: '取消', type: 'warning'
    })
    await saveQuickStatus(record, 'refund_status', '已退款')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error, '更新退款状态失败'))
  } finally {
    statusUpdatingID.value = ''
  }
}

async function changeDeclarationStatus(record: PaymentRecord, status: PaymentRecord['corporate_declaration_status']) {
  if (statusUpdatingID.value || record.corporate_declaration_status === '申报完成') return
  statusUpdatingID.value = record.id
  try {
    await saveQuickStatus(record, 'corporate_declaration_status', status)
  } catch (error) {
    ElMessage.error(errorMessage(error, '更新申报状态失败'))
  } finally {
    statusUpdatingID.value = ''
  }
}

async function submitPayment(formData: PaymentForm) {
  if (!formData.amount || formData.amount <= 0) {
    ElMessage.warning('支付金额必须大于 0')
    return
  }
  saving.value = true
  try {
    let feishuSyncSucceeded = true
    const payload = {
      payment_time: formData.payment_time,
      payer: formData.payer.trim(),
      amount: formData.amount,
      payment_method: formData.payment_method,
      consumer_app: formData.consumer_app.trim(),
      refund_status: formData.refund_status,
      corporate_declaration_status: formData.corporate_declaration_status,
      remark: formData.remark.trim()
    }

    if (editingRecord.value?.id) {
      const response = (await request.put(
        `/payment-records/${editingRecord.value.id}`,
        payload
      )) as { feishu_sync_success?: boolean }
      feishuSyncSucceeded = response.feishu_sync_success !== false
      if (!feishuSyncSucceeded) {
        ElMessage.warning('本地记录已更新，但同步飞书多维表格失败，可在详情中查看原因')
      } else {
        ElMessage.success('支付记录已更新并成功同步飞书')
      }
    } else {
      const response = (await request.post('/payment-records', payload)) as {
        feishu_sync_success?: boolean
      }
      feishuSyncSucceeded = response.feishu_sync_success !== false
      if (!feishuSyncSucceeded) {
        ElMessage.warning('本地记录已新增，但同步飞书多维表格失败，可在详情中查看原因')
      } else {
        ElMessage.success('支付记录已新增并成功同步飞书')
      }
      filters.page = 1
    }

    dialogVisible.value = false
    await loadRecords()
  } catch (error) {
    ElMessage.error(
      errorMessage(
        error,
        editingRecord.value?.id ? '更新支付记录失败' : '新增支付记录失败'
      )
    )
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadRecords()
})
</script>

<template>
  <main class="payment-records-page">
    <!-- 顶部标题与操作栏 -->
    <header class="page-top-header">
      <div class="header-left">
        <div class="header-icon-box">
          <el-icon><CreditCard /></el-icon>
        </div>
        <div class="header-text">
          <div class="breadcrumb-pill">FINANCE & EXPENSE / 财务与结算</div>
          <h1 class="page-title">支付记录中心</h1>
          <p class="page-subtitle">
            集中登记平台订阅与测试支付流水，实时同步飞书多维表格与企业对公申报
          </p>
        </div>
      </div>

      <div class="header-right-actions">
        <el-button
          :icon="Refresh"
          :loading="loading"
          class="refresh-btn"
          @click="loadRecords"
        >
          刷新数据
        </el-button>
        <el-button
          type="primary"
          :icon="Plus"
          class="create-btn"
          @click="openCreate"
        >
          新增支付记录
        </el-button>
      </div>
    </header>

    <!-- 4 卡片统计概览 -->
    <PaymentSummaryCards
      :summary="summary"
      :loading="loading"
      @quick-filter="handleQuickFilter"
    />

    <!-- 筛选面板 -->
    <PaymentFilterBar
      v-model:filters="filters"
      :loading="loading"
      @search="searchRecords"
      @reset="resetFilters"
    />

    <!-- 数据明细表格 -->
    <PaymentTable
      :records="records"
      :total="total"
      :loading="loading"
      :status-updating-id="statusUpdatingID"
      v-model:current-page="filters.page"
      v-model:page-size="filters.page_size"
      @page-change="loadRecords"
      @view-detail="openDetail"
      @edit="openEdit"
      @refund="confirmRefund"
      @declaration-change="changeDeclarationStatus"
    />

    <!-- 新增 / 编辑弹窗 -->
    <PaymentFormDialog
      v-model:visible="dialogVisible"
      :editing-record="editingRecord"
      :saving="saving"
      :default-payer="defaultPayer"
      @submit="submitPayment"
    />

    <!-- 详情抽屉 -->
    <PaymentDetailDrawer
      v-model:visible="detailVisible"
      :record="selectedRecord"
      @edit="openEditFromDetail"
    />
  </main>
</template>

<style scoped>
.payment-records-page {
  min-height: 100%;
  padding: 24px 32px 40px;
  background: #f8fafc;
  color: #0f172a;
  box-sizing: border-box;
}

.page-top-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-icon-box {
  display: grid;
  place-items: center;
  width: 50px;
  height: 50px;
  border-radius: 14px;
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  color: #ffffff;
  font-size: 24px;
  box-shadow: 0 8px 20px -4px rgba(37, 99, 235, 0.35);
  flex-shrink: 0;
}

.breadcrumb-pill {
  font-size: 11px;
  font-weight: 800;
  color: #2563eb;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 3px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 800;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: -0.02em;
}

.page-subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: #64748b;
  line-height: 1.4;
}

.header-right-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.refresh-btn {
  background: #ffffff;
  border-color: #e2e8f0;
  color: #334155;
  font-weight: 500;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.refresh-btn:hover {
  background: #f8fafc;
  border-color: #cbd5e1;
  color: #0f172a;
}

.create-btn {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  border: none;
  font-weight: 600;
  padding: 8px 18px;
  box-shadow: 0 4px 14px -2px rgba(37, 99, 235, 0.35);
  transition: all 0.2s ease;
}

.create-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 18px -2px rgba(37, 99, 235, 0.45);
}

@media (max-width: 768px) {
  .payment-records-page {
    padding: 16px 14px 28px;
  }

  .page-top-header {
    flex-direction: column;
    align-items: stretch;
    gap: 16px;
  }

  .header-right-actions {
    width: 100%;
  }

  .header-right-actions .el-button {
    flex: 1;
  }
}
</style>
