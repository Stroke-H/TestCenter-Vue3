export interface PaymentRecord {
  id: string
  payment_time: string
  payer: string
  amount: number
  currency: string
  payment_method: string
  consumer_app: string
  refund_status: '已退款' | '未退款'
  corporate_declaration_status: '未发起' | '已发起' | '申报完成'
  remark: string
  created_by_id: string
  created_by_name: string
  created_at: string
  updated_at: string
  feishu_record_id: string
  feishu_sync_status: '已同步' | '同步失败' | '未同步'
  feishu_sync_error: string
  feishu_synced_at?: string
}

export interface PaymentSummary {
  count: number
  total_amount: number
  current_month_count: number
  current_month_amount: number
  available_balance: number
}

export type PaymentStatusField = 'refund_status' | 'corporate_declaration_status'

export interface PaymentListResponse {
  items: PaymentRecord[]
  total: number
  page: number
  page_size: number
  summary: PaymentSummary
}

export interface PaymentForm {
  payment_time: string
  payer: string
  amount: number | undefined
  payment_method: string
  consumer_app: string
  refund_status: '已退款' | '未退款'
  corporate_declaration_status: '未发起' | '已发起' | '申报完成'
  remark: string
}

export interface PaymentFilters {
  search: string
  payment_method: string
  refund_status: string
  corporate_declaration_status: string
  feishu_sync_status: string
  date_range: string[]
  page: number
  page_size: number
}

export const paymentMethods = ['Stripe', 'Breeze', 'Adyen', 'Paypal'] as const
export const presetAmounts = ['3.99', '4.99', '9.99', '19.99'] as const
export const refundStatuses = ['未退款', '已退款'] as const
export const declarationStatuses = ['未发起', '已发起', '申报完成'] as const

export const consumerApps = [
  'ShortsWave - iOS',
  'ShortsWave - Android',
  'Mangogo - iOS',
  'Mangogo - Android',
  'CocoShorts - iOS',
  'CocoShorts - Android'
] as const

export function methodTone(method: string): '' | 'primary' | 'success' | 'warning' | 'info' | 'danger' {
  switch (method) {
    case 'Stripe':
      return 'primary'
    case 'Breeze':
      return 'success'
    case 'Adyen':
      return 'warning'
    case 'Paypal':
      return 'info'
    default:
      return 'info'
  }
}

export function formatDateTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  }).format(date)
}

export function formatAmount(value: number, currency = 'USD'): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(Number(value || 0))
}
