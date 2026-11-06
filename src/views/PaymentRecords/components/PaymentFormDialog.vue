<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { CreditCard } from '@element-plus/icons-vue'
import type { PaymentForm, PaymentRecord } from '../types'
import {
  paymentMethods,
  presetAmounts,
  refundStatuses,
  declarationStatuses,
  consumerApps
} from '../types'

const props = defineProps<{
  visible: boolean
  editingRecord: PaymentRecord | null
  saving: boolean
  defaultPayer: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'submit', form: PaymentForm): void
}>()

const formRef = ref<FormInstance>()
const isEdit = computed(() => Boolean(props.editingRecord?.id))
const amountPreset = ref('3.99')
const consumerAppPreset = ref('ShortsWave - iOS')

const form = ref<PaymentForm>({
  payment_time: '',
  payer: '',
  amount: 3.99,
  payment_method: 'Stripe',
  consumer_app: 'ShortsWave - iOS',
  refund_status: '未退款',
  corporate_declaration_status: '未发起',
  remark: ''
})

const rules: FormRules<PaymentForm> = {
  payment_time: [{ required: true, message: '请选择支付时间', trigger: 'change' }],
  payer: [
    { required: true, message: '请填写支付人', trigger: 'blur' },
    { max: 255, message: '支付人不能超过 255 个字符', trigger: 'blur' }
  ],
  amount: [{ required: true, message: '请填写支付金额', trigger: 'change' }],
  consumer_app: [{ required: true, message: '请选择或填写消费应用', trigger: 'change' }]
}

function toLocalDateTime(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

watch(
  () => props.visible,
  (val) => {
    if (!val) return
    if (props.editingRecord) {
      const rec = props.editingRecord
      form.value = {
        payment_time: toLocalDateTime(new Date(rec.payment_time)),
        payer: rec.payer,
        amount: Number(rec.amount),
        payment_method: rec.payment_method || 'Stripe',
        consumer_app: rec.consumer_app,
        refund_status: rec.refund_status || '未退款',
        corporate_declaration_status: rec.corporate_declaration_status || '未发起',
        remark: rec.remark || ''
      }
      const amountStr = Number(rec.amount).toFixed(2)
      amountPreset.value = (presetAmounts as readonly string[]).includes(amountStr) ? amountStr : 'custom'
      consumerAppPreset.value = (consumerApps as readonly string[]).includes(rec.consumer_app)
        ? rec.consumer_app
        : 'custom'
    } else {
      form.value = {
        payment_time: toLocalDateTime(new Date()),
        payer: props.defaultPayer || '',
        amount: 3.99,
        payment_method: 'Stripe',
        consumer_app: 'ShortsWave - iOS',
        refund_status: '未退款',
        corporate_declaration_status: '未发起',
        remark: ''
      }
      amountPreset.value = '3.99'
      consumerAppPreset.value = 'ShortsWave - iOS'
    }
    formRef.value?.clearValidate()
  }
)

function selectAmountChip(val: string) {
  amountPreset.value = val
  if (val === 'custom') {
    form.value.amount = undefined
  } else {
    form.value.amount = Number(val)
  }
  formRef.value?.clearValidate('amount')
}

function selectAppChip(app: string) {
  consumerAppPreset.value = app
  if (app === 'custom') {
    form.value.consumer_app = ''
  } else {
    form.value.consumer_app = app
  }
  formRef.value?.clearValidate('consumer_app')
}

async function handleConfirm() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  emit('submit', { ...form.value })
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="isEdit ? '编辑支付记录' : '新增支付记录'"
    width="min(660px, calc(100vw - 32px))"
    align-center
    destroy-on-close
    class="payment-modal-dialog"
    @update:model-value="(val: boolean) => emit('update:visible', val)"
  >
    <!-- 头部说明卡片 -->
    <div class="modal-intro-card">
      <div class="intro-icon">
        <el-icon><CreditCard /></el-icon>
      </div>
      <div class="intro-content">
        <strong>{{ isEdit ? '更新流水与对公信息' : '登记新支付流水' }}</strong>
        <p>支付流水将同时保存至本地平台并实时同步至飞书多维表格。已退款记录不会扣减虚拟卡可用额度。</p>
      </div>
    </div>

    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-position="top"
      class="payment-form-body"
    >
      <!-- 分区 1: 交易基础属性 -->
      <div class="form-section-header">
        <span class="section-num">01</span>
        <span class="section-title">支付核心要素</span>
      </div>

      <div class="form-grid-2">
        <el-form-item label="支付时间" prop="payment_time">
          <el-date-picker
            v-model="form.payment_time"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="选择支付发生时间"
            style="width: 100%"
          />
        </el-form-item>

        <el-form-item label="支付人" prop="payer">
          <el-input
            v-model="form.payer"
            maxlength="255"
            placeholder="请输入支付人姓名或账号"
            clearable
          />
        </el-form-item>
      </div>

      <!-- 金额快捷选择器 -->
      <el-form-item label="支付金额 (USD)" prop="amount">
        <div class="chips-selector-group">
          <button
            v-for="amt in presetAmounts"
            :key="amt"
            type="button"
            class="chip-btn"
            :class="{ 'is-selected': amountPreset === amt }"
            @click="selectAmountChip(amt)"
          >
            ${{ amt }}
          </button>
          <button
            type="button"
            class="chip-btn"
            :class="{ 'is-selected': amountPreset === 'custom' }"
            @click="selectAmountChip('custom')"
          >
            自定义金额
          </button>
        </div>

        <div v-if="amountPreset === 'custom'" class="custom-input-anim">
          <el-input-number
            v-model="form.amount"
            :min="0.01"
            :max="999999999999.99"
            :precision="2"
            :step="1"
            controls-position="right"
            placeholder="输入自定义金额 (USD)"
            style="width: 100%"
          />
        </div>
      </el-form-item>

      <div class="form-grid-3">
        <el-form-item label="支付渠道">
          <el-select v-model="form.payment_method" placeholder="选择支付方式" style="width: 100%">
            <el-option
              v-for="method in paymentMethods"
              :key="method"
              :label="method"
              :value="method"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="对公申报状态">
          <el-select v-model="form.corporate_declaration_status" style="width: 100%">
            <el-option
              v-for="status in declarationStatuses"
              :key="status"
              :label="status"
              :value="status"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="退款状态">
          <el-select v-model="form.refund_status" style="width: 100%">
            <el-option
              v-for="status in refundStatuses"
              :key="status"
              :label="status"
              :value="status"
            />
          </el-select>
        </el-form-item>
      </div>

      <!-- 分区 2: 消费应用与业务详情 -->
      <div class="form-section-header">
        <span class="section-num">02</span>
        <span class="section-title">消费应用与补充说明</span>
      </div>

      <el-form-item label="消费业务 / 应用" prop="consumer_app">
        <div class="chips-selector-group app-chips-group">
          <button
            v-for="app in consumerApps"
            :key="app"
            type="button"
            class="chip-btn app-chip"
            :class="{ 'is-selected': consumerAppPreset === app }"
            @click="selectAppChip(app)"
          >
            {{ app }}
          </button>
          <button
            type="button"
            class="chip-btn app-chip"
            :class="{ 'is-selected': consumerAppPreset === 'custom' }"
            @click="selectAppChip('custom')"
          >
            + 其他应用
          </button>
        </div>

        <div v-if="consumerAppPreset === 'custom'" class="custom-input-anim">
          <el-input
            v-model="form.consumer_app"
            maxlength="255"
            placeholder="请输入自定义消费应用名称（例如：AppName - Platform）"
            clearable
          />
        </div>
      </el-form-item>

      <el-form-item label="备注说明 (选填)">
        <el-input
          v-model="form.remark"
          type="textarea"
          :rows="3"
          maxlength="2000"
          show-word-limit
          resize="none"
          placeholder="可在此记录支付用途、测试关联需求、对公报销单号或补充说明..."
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-actions-footer">
        <el-button @click="emit('update:visible', false)">取消</el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="handleConfirm"
        >
          {{ isEdit ? '保存修改' : '确认新增并同步' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.modal-intro-card {
  display: flex;
  align-items: center;
  gap: 14px;
  background: #f0f7ff;
  border: 1px solid #bfdbfe;
  border-radius: 12px;
  padding: 12px 16px;
  margin-bottom: 20px;
}

.intro-icon {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: #2563eb;
  color: #ffffff;
  font-size: 18px;
  flex-shrink: 0;
}

.intro-content strong {
  display: block;
  font-size: 13px;
  color: #1e3a8a;
}

.intro-content p {
  margin: 3px 0 0;
  font-size: 11px;
  color: #3b82f6;
  line-height: 1.4;
}

.form-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 16px 0 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid #f1f5f9;
}

.section-num {
  font-size: 11px;
  font-weight: 800;
  color: #3b82f6;
  background: #eff6ff;
  padding: 1px 6px;
  border-radius: 4px;
}

.section-title {
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.form-grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.form-grid-3 {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 12px;
}

.chips-selector-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
  width: 100%;
}

.chip-btn {
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  cursor: pointer;
  transition: all 0.2s ease;
}

.chip-btn:hover {
  background: #f1f5f9;
  border-color: #cbd5e1;
  color: #1e293b;
}

.chip-btn.is-selected {
  background: #2563eb;
  border-color: #2563eb;
  color: #ffffff;
  box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);
}

.app-chip {
  font-size: 11px;
  padding: 5px 10px;
}

.custom-input-anim {
  width: 100%;
  margin-top: 4px;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.payment-form-body :deep(.el-form-item__label) {
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  margin-bottom: 4px;
}

.dialog-actions-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
}

@media (max-width: 640px) {
  .form-grid-2,
  .form-grid-3 {
    grid-template-columns: 1fr;
    gap: 0;
  }
}
</style>
