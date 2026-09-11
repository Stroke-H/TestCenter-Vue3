<script setup lang="ts">
import { Lock, Setting } from '@element-plus/icons-vue'

defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

// 只读说明，对照 prepare_data.js / drama_check_flow.js（2026-09-07）。
// 判定规则变更时同步更新此说明；此组件不参与执行配置。
const sections = [
  { title: '检测范围', note: '分类日期限制不影响其他检查', rows: [
    ['剧集来源', '获取当前执行环境返回的在线剧集 ID，并补充在线剧集元数据及 App Group 列表。'],
    ['分类检查日期', '仅检查 created_at 日期为 2026-06-01 及之后的剧集；日期缺失或无法提取时跳过分类检查。'],
    ['番茄命名', 'cn_name 包含“番茄”时跳过分类检查；播放、分组、解锁类型等检查继续执行。'],
  ] },
  { title: '分组与解锁类型', note: '多分组同时命中时，仍需满足相应限制', rows: [
    ['广告解锁允许组', 'IAA组、漫剧产品组-IAA、AIGC组、内容二组-觉醒纪元、TT-Minis 分销组、网赚短剧、TT_IAA_媒资库组允许 ad。仅属于 MD-产品组时也允许 ad。'],
    ['IAA 标识', 'cn_name 或分组名称命中 IAA 时，coin / coins 判为异常。'],
    ['网赚短剧 / TT_IAA_媒资库组', '要求 ad；coin / coins 判为异常。'],
    ['TT_IAP_媒资库组', '要求 coins，兼容 coin；即使同时存在广告组，ad 也判为异常。IAA 与 IAP 按最接近的标准组名区分。'],
    ['ShortsWave', 'cn_name 未命中 DS 时，分组命中 ShortsWave 且解锁类型为 ad 则报错。'],
    ['DS 标识', 'cn_name 命中 DS 时，必须且只能属于 TT-Minis 分销组。忽略 DS 大小写和外围命名格式差异。'],
    ['其他广告分组', '非 DS 剧集使用 ad，但没有命中广告允许组、也不满足仅 MD-产品组例外时，判为异常。'],
  ] },
  { title: '分类与命名', note: '命名问题与业务规则分别检查', rows: [
    ['配音分类双向校验', 'cn_name 带 X配 标识时，marketing_position 必须为“配音剧”；分类为“配音剧”时，cn_name 也必须包含配音标识。'],
    ['配音标识识别', '支持西配、葡配、印配等主要语言/国家简称与全称，也支持“语配”“配音”及 1–4 位英文字母配音标识。AI印配等嵌入名称中的标识同样识别。'],
    ['命名轻微差异', 'IAA、ShortsWave 大小写及已知标准组名的相近命名会记录命名异常，并继续按对应规则检查；命名问题进入报告“其他异常”。'],
  ] },
  { title: '播放健康检查', note: '720p 优先，必要时使用 540p 补充判断', rows: [
    ['章节状态', '检查章节 online 与 update_status；未上线或 update_status > 1 视为出错，上线但未完成转换则记录转换异常。健康章节要求 online=1 且 update_status=1。'],
    ['连续性与数量', '检查有效章节 Index 最小值到最大值之间的缺口，并比较返回数量或健康章节数量与接口标称 total。'],
    ['分辨率回退', '720p 无健康异常且 total 与返回条目数一致时结束播放检查；否则查询 540p，按 Index 合并判断，任一分辨率健康即可补足该集。'],
    ['计数异常去重', '合并后健康集数与两种分辨率的较大 total 比较；已有章节异常时不再额外叠加计数异常。540p 请求失败时沿用 720p 检查结果。'],
  ] },
  { title: '二次确认与上报', note: '命名、分类和明确的分组错误直接记录', rows: [
    ['元数据获取失败', 'App Group 元数据首次不可用时进入全局二次确认，二次仍失败则上报元数据异常。'],
    ['空值 / 未知解锁类型', '有效值为 ad、coin、coins（忽略大小写）；空值或未知值首次进入二次确认，二次仍异常则上报。'],
    ['720p 抓取失败', '首次请求失败进入二次确认，二次仍失败则上报接口抓取失败。'],
    ['暂时忽略', '检测报告仍记录错误；已标记“暂时忽略”的完全相同剧集与错误详情，在飞书报告中不再重复提醒。细节变化后重新提醒。'],
  ] },
]
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="剧集播放测试 · 规则配置"
    width="min(800px, calc(100vw - 32px))"
    align-center
    append-to-body
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="drama-rule-view">
      <div class="drama-rule-view__intro">
        <span class="drama-rule-view__icon"><el-icon><Setting /></el-icon></span>
        <div><strong>当前筛选与检查规则</strong><p>规则说明版本：2026-09-07 · 展示现行规则，不支持修改。</p></div>
        <span class="drama-rule-view__readonly"><el-icon><Lock /></el-icon>只读</span>
      </div>
      <div class="drama-rule-view__scroll" tabindex="0" aria-label="规则详情">
        <section v-for="(section, index) in sections" :key="section.title" class="drama-rule-view__section">
          <header><span>{{ String(index + 1).padStart(2, '0') }}</span><h3>{{ section.title }}</h3></header>
          <p class="drama-rule-view__note">{{ section.note }}</p>
          <dl>
            <div v-for="row in section.rows" :key="row[0]" class="drama-rule-view__row">
              <dt>{{ row[0] }}</dt><dd>{{ row[1] }}</dd>
            </div>
          </dl>
        </section>
      </div>
    </div>
    <template #footer><el-button type="primary" @click="emit('update:modelValue', false)">知道了</el-button></template>
  </el-dialog>
</template>

<style scoped>
.drama-rule-view { color: #334155; }
.drama-rule-view__intro { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border: 1px solid #dbeafe; border-radius: 10px; background: #eff6ff; margin-bottom: 16px; }
.drama-rule-view__icon { display: grid; place-items: center; width: 36px; height: 36px; flex-shrink: 0; border-radius: 9px; background: #dbeafe; color: #2563eb; font-size: 20px; }
.drama-rule-view__intro strong { font-size: 14px; color: #1e293b; }
.drama-rule-view__intro p { margin: 5px 0 0; font-size: 12px; color: #64748b; line-height: 1.5; }
.drama-rule-view__readonly { display: inline-flex; align-items: center; gap: 4px; margin-left: auto; white-space: nowrap; color: #2563eb; font-size: 12px; }
.drama-rule-view__scroll { max-height: 56vh; overflow-y: auto; padding-right: 6px; }
.drama-rule-view__section { margin-bottom: 16px; border: 1px solid #e2e8f0; border-radius: 10px; overflow: hidden; }
.drama-rule-view__section:last-child { margin-bottom: 0; }
.drama-rule-view__section header { display: flex; align-items: center; gap: 9px; padding: 14px 16px 0; }
.drama-rule-view__section header span { color: #3b82f6; font: 600 12px monospace; }
.drama-rule-view__section h3 { margin: 0; font-size: 14px; color: #0f172a; }
.drama-rule-view__note { margin: 6px 16px 12px; color: #64748b; font-size: 12px; }
.drama-rule-view__section dl { margin: 0; }
.drama-rule-view__row { display: grid; grid-template-columns: 170px minmax(0, 1fr); gap: 16px; padding: 12px 16px; border-top: 1px solid #f1f5f9; font-size: 12px; line-height: 1.7; }
.drama-rule-view__row:nth-child(odd) { background: #f8fafc; }
.drama-rule-view__row dt { font-weight: 600; color: #475569; overflow-wrap: anywhere; }
.drama-rule-view__row dd { margin: 0; overflow-wrap: anywhere; }
@media (max-width: 600px) { .drama-rule-view__row { grid-template-columns: 1fr; gap: 4px; } .drama-rule-view__intro { padding: 12px; } }
</style>
