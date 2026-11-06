<script setup lang="ts">
import { Lock, Setting } from '@element-plus/icons-vue'

defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

// 只读说明，对照 drama_business_rules.js / drama_check_flow.js（2026-11-04）。
// 判定规则变更时同步更新此说明；此组件不参与执行配置。
const sections = [
  { title: '检测范围 · 2026-11-06-v4.7', note: '命中任一白名单组即纳入；分组判定仅考虑白名单组，非白名单组忽略', rows: [
    ['白名单', 'ShortsWave组、小程序组、mangogo组、CoCoShort产品组、TT_IAP_媒资库组、TT-Minis分销组、点石出海短剧、TT_IAA_媒资库组、TT_IAA_媒资库组（香港主体）、海外互动短剧、IAA组、漫剧产品组-IAA、网赚短剧、TT_IAA_分销_媒资库、TT_IAA_分销_媒资库（香港主体）。'],
    ['入口过滤', '完全不命中白名单的剧直接过滤，不执行播放、语种或其他规则，不进入复验。分组元数据不足以判定范围时明确失败，不默认为过滤或通过。'],
    ['混合分组', '命中白名单后，忽略外部联运组等非白名单分组；原始、参与及忽略的组均保留在筛选归档。白名单组之间仍执行互斥与独占规则，无法解析的ID不能直接当作非白名单忽略。'],
    ['检测范围', '当前环境的全部在线且显示剧集；常规播放、章节连续性、数量、可用性及转换状态检查保持正常。'],
    ['旧规则取消', '内容五部仅播放豁免取消，不按旧DS、出海或番茄剧名直接放行。媒资库+IAA按下方新规则执行；创建日期仅限定配音分类，不豁免分组等其他检查。'],
  ] },
  { title: '分组与解锁类型', note: '仅支持coin/ad；所有独占、互斥条件仅针对白名单组，非白名单组不影响判定', rows: [
    ['coin普通组合', 'ShortsWave、小程序、mangogo、CoCoShort四组的任意非空子集，共15种组合；不能夹带其他白名单组。'],
    ['coin + (媒资库)后缀', '仅当后缀为完整(媒资库)时，要求有且只有TT_IAP_媒资库组。'],
    ['普通ad', '仅TT-Minis分销组；或IAA、漫剧产品组-IAA、网赚短剧三组的任意非空组合。两类互斥，不得夹带其他白名单组；外部联运等非白名单组忽略。'],
    ['ad + (出海)', '命名完整包含(出海)或（出海）标识时，有且只有点石出海短剧。仅含出海二字、(出海版)等不触发此特殊分组规则，按其他适用规则判断。'],
    ['-DS媒资库(东南亚)', '有且只有TT_IAA_分销_媒资库（香港主体）；优先于普通DS和泛化媒资库区域规则。'],
    ['-DS媒资库', '非东南亚分支，有且只有TT_IAA_分销_媒资库。'],
    ['媒资库 + (东南亚)', '非DS分支，有且只有TT_IAA_媒资库组（香港主体）。'],
    ['ad + (IAA) + (媒资库) + (香港主体)', '同时完整包含这三个标识时，白名单组中有且只有TT_IAA_媒资库组（香港主体），优先于普通IAA媒资库和美欧分支。兼容中英文括号、IAA大小写及标识顺序；番茄内供及DS规则优先级不变。'],
    ['媒资库 + (美欧)', '有且只有TT_IAA_媒资库组。'],
    ['ad + 媒资库 + (IAA)', '未命中香港主体等更具体分支时，白名单组中有且只有TT_IAA_媒资库组，不得同时进入其他白名单组。番茄内供最高优先，DS、东南亚及上述香港主体规则仍优先；兼容中英文括号。'],
    ['互动', '命名含互动时，有且只有海外互动短剧。'],
    ['番茄内供最高优先', 'copyright值为番茄内供时，有且只有ShortsWave组；覆盖其他特殊命名及普通解锁分组要求，不以剧名含番茄代替版权字段。'],
    ['重叠与数据缺失', '明确的分组冲突初检直接记入报告，不复验。已命中白名单但存在额外未知分组ID时复验元数据；恢复后补做此前无法判断的分组校验。范围无法确定时先明确失败，不丢弃、不静默通过。'],
  ] },
  { title: '配音分类', note: '仅检查created_at为2026-06-01（含）及之后的剧集', rows: [
    ['创建日期范围', '按业务时区+08:00判断：2026-06-01零点及之后才执行X配/X全剧配音的语种和分类校验；更早、日期缺失或无法解析时仅跳过分类检查，其他检查正常执行。'],
    ['双项校验', '识别X配及(X全剧配音)，核对标识语种与lang，并要求marketing_position包含配音剧。语种和分类都错时分别记录。'],
    ['语种映射', '英=en、日=ja、西=es、印/印尼=id、法=fr、韩=ko、泰=th、俄=ru、马来=ms、中文=zh、阿=ar、葡=pt、德=de、土=tr。兼容完整语言名称、AI前缀及地区后缀。'],
    ['冲突与范围', '多个不同配音标识时上报；明确配音标签缺少映射时记录待核验，不猜语种。不再仅因有配音剧分类但无X配标识而反向报错。'],
  ] },
  { title: '简介语种', note: '继续使用本地识别，不接付费API', rows: [
    ['检查方式', '简介语种与lang比对；简介为空或只有一个/时跳过。明确冲突刷新数据后仍异常则上报语种异常。'],
    ['保守复核', 'id/ms、es/pt歧义、混合语种、范围外语言及拉丁转写候选保留待复核，不当作通过或自动业务错误。人名和URL防干扰保留，中文不区分简繁。'],
    ['证据与缓存', '模型分数不是准确率，刷新数据不是独立语言验证。缓存只保存文本预测，每轮重新比较最新lang；HTML保留待复核原文，飞书提供异常详情链接。'],
  ] },
  { title: '版权有效期', note: '缺失、无效或过期均由初检直接记入版权异常，不进入二次确认', rows: [
    ['版权字段', '检查copyright_expire_at严格晚于实际检查时间；相等也视为过期。报告保留截止及检查时间。'],
    ['日期解析', '优先使用接口明确时区；后台无时区日期按Asia/Shanghai（+08:00）解析，拒绝无效日历日期，不依赖电脑时区。'],
  ] },
  { title: '免费集数', note: '检查后台配置free_chapters，不按播放URL数量推算', rows: [
    ['ad免费集数', 'unlock_type=ad时，free_chapters必须为2；coin不执行此项。'],
    ['字段异常', 'free_chapters缺失、null或格式异常不当作0，也不能通过；与数值不等于2分别记录，初检直接进入报告，不复验。'],
    ['报告归类', '新增免费集数异常分类、计数和详情，HTML及飞书报告均保留。'],
  ] },
  { title: '二次确认与报告', note: '历史报告不回写', rows: [
    ['执行分工', '收集阶段完成分组、解锁类型、免费集数、版权和配音分类判断，并单独保存规则结果。简介由Go本地模型识别，k6负责简介语种结果判定及播放检查，并合并收集阶段的异常，不重复计算字段规则。'],
    ['明确异常直接记录', '分组组合不符、unlock_type空/未知、免费集数和版权异常以初检结果直接进入最终报告，不因此触发复验；即使同剧因其他原因复验，也不重复验证这些已明确项。报告完成后统一发送。'],
    ['保留二次确认', '配音分类、简介语种、720p接口抓取失败和无法判断的App Group元数据仍复验。元数据恢复后补做此前未能完成的分组判断。移出在线白名单的候选单独记录，不复验、不计为恢复。'],
    ['发送与记录', '新增版权异常分类；保留分组、分类、语种、免费集数和其他异常分类、详情链接与精确忽略。记录白名单过滤及实际完成数量。'],
    ['卡片与明细展示', '播放异常始终完整输出；分组、分类、语种、版权四类的分类明细数量合计不超过30条时保留原明细，超过30条仅展示分类摘要并提供完整错误链接。同剧跨分类分别计数。HTML按页加载，详情点击获取，暂时忽略及数据问题标记保留；免费集数及其他异常沿用原发送方式。'],
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
        <div><strong>当前筛选与检查规则</strong><p>规则说明版本：2026-11-06-v4.7 · 展示现行规则，不支持修改。</p></div>
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
