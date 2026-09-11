<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Connection, CopyDocument, DataLine, Iphone, Monitor, Refresh, VideoPlay } from '@element-plus/icons-vue'
import { ttminsLogsApi, type TTminsLogInfo, type TTminsLogTarget } from './api'

const info = ref<TTminsLogInfo | null>(null)
const targets = ref<TTminsLogTarget[]>([])
const loading = ref(true)
const refreshing = ref(false)
const lastRefresh = ref<Date | null>(null)
const selectedScriptURL = ref('')
const inspectorURL = ref('')
const inspectingTarget = ref<TTminsLogTarget | null>(null)
let timer: number | undefined

const scriptURL = computed(() => selectedScriptURL.value || info.value?.script_options?.[0]?.url || (info.value ? new URL(info.value.script_path, window.location.origin).href : ''))
const connectedCount = computed(() => targets.value.length)
const inspectingCount = computed(() => targets.value.filter((item) => item.inspecting).length)

async function load(showLoading = false) {
  if (showLoading) refreshing.value = true
  try {
    const [nextInfo, result] = await Promise.all([info.value && !showLoading ? Promise.resolve(info.value) : ttminsLogsApi.info(), ttminsLogsApi.targets()])
    info.value = nextInfo
    const scriptOptions = nextInfo.script_options || []
    if (!selectedScriptURL.value && scriptOptions.length) selectedScriptURL.value = scriptOptions[0]!.url
    targets.value = [...result.targets].sort((a, b) => Date.parse(b.connected_at) - Date.parse(a.connected_at))
    lastRefresh.value = new Date()
  } catch (error: any) {
    if (showLoading || !info.value) ElMessage.error(error?.customMessage || '日志服务状态加载失败')
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); ElMessage.success('地址已复制') }
  catch { ElMessage.error('复制失败，请手动选择地址') }
}

async function inspect(target: TTminsLogTarget) {
  try {
    const result = await ttminsLogsApi.inspect(target.id)
    const wsURL = new URL(result.client_path, window.location.origin)
    wsURL.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const nextInspectorURL = new URL(result.inspector_path, window.location.origin)
    // Chii adds the WebSocket scheme itself. Passing wsURL.href would produce
    // an invalid address such as `ws://ws://host/...` and close immediately.
    const socketQueryName = wsURL.protocol === 'wss:' ? 'wss' : 'ws'
    nextInspectorURL.searchParams.set(socketQueryName, `${wsURL.host}${wsURL.pathname}${wsURL.search}`)
    nextInspectorURL.searchParams.set('rtc', 'false')
    inspectingTarget.value = target
    window.scrollTo({ top: 0, behavior: 'smooth' })
    // Assign last so the iframe is only mounted after the complete one-time URL is ready.
    window.requestAnimationFrame(() => { inspectorURL.value = nextInspectorURL.href })
    window.setTimeout(() => load(), 500)
  } catch (error: any) {
    ElMessage.error(error?.customMessage || '无法打开 Inspect，设备可能已离线')
  }
}

function closeInspector() {
  inspectorURL.value = ''
  inspectingTarget.value = null
  window.setTimeout(() => load(), 200)
}

function reconnectInspector() {
  if (inspectingTarget.value) inspect(inspectingTarget.value)
}

async function disconnect(target: TTminsLogTarget) {
  try {
    await ElMessageBox.confirm(`断开“${target.title || target.id}”的日志连接？`, '断开设备', { type: 'warning', confirmButtonText: '确认断开', cancelButtonText: '取消' })
  } catch {
    return
  }
  try { await ttminsLogsApi.disconnect(target.id); ElMessage.success('设备已断开'); await load() }
  catch (error: any) { ElMessage.error(error?.customMessage || '断开设备失败') }
}

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

function platformLabel(userAgent: string) {
  if (/iphone|ipad|ios/i.test(userAgent)) return 'iOS'
  if (/android/i.test(userAgent)) return 'Android'
  return '小程序设备'
}

function safeTargetURL(value: string) {
  try {
    const candidate = new URL(value)
    return ['http:', 'https:'].includes(candidate.protocol) ? candidate.href : ''
  } catch {
    return ''
  }
}

onMounted(async () => {
  await load()
  timer = window.setInterval(() => load(), 2000)
})
onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })
</script>

<template>
  <main v-loading="loading" class="logs-page">
    <section v-if="inspectorURL" class="inspector-panel">
      <header class="inspector-header">
        <div>
          <el-button circle :icon="ArrowLeft" aria-label="返回设备列表" @click="closeInspector" />
          <span class="inspector-device"><i /><span><strong>{{ inspectingTarget?.title || '未命名设备' }}</strong><small>{{ inspectingTarget?.ip }} · 实时调试中</small></span></span>
        </div>
        <el-button :icon="Refresh" @click="reconnectInspector">重新连接</el-button>
      </header>
      <iframe :src="inspectorURL" title="TTmins DevTools" allow="clipboard-read; clipboard-write" />
    </section>

    <template v-else>
    <section class="hero-panel">
      <div class="hero-copy">
        <span class="eyebrow"><el-icon><DataLine /></el-icon> REAL-TIME REMOTE DEBUGGING</span>
        <h1>TTmins 日志</h1>
        <p>连接 TikTok 小程序测试包，在平台内发现设备并打开完整 DevTools，实时检查 Console、Network 和页面状态。</p>
        <div class="script-address">
          <div><span>Client script URL</span><code>{{ scriptURL || '正在读取…' }}</code></div>
          <el-select v-if="(info?.script_options?.length || 0) > 1" v-model="selectedScriptURL" class="address-select" size="small" aria-label="选择移动端可访问地址">
            <el-option v-for="option in info?.script_options" :key="option.url" :label="option.label" :value="option.url" />
          </el-select>
          <el-button :icon="CopyDocument" :disabled="!scriptURL" @click="copy(scriptURL)">复制地址</el-button>
        </div>
      </div>
      <div class="hero-visual"><span class="signal signal--one" /><span class="signal signal--two" /><div><el-icon><Connection /></el-icon></div><small>{{ connectedCount ? '设备在线' : '等待连接' }}</small></div>
    </section>

    <section class="metrics-grid">
      <article><span class="metric-icon cyan"><el-icon><Iphone /></el-icon></span><div><small>在线设备</small><strong>{{ connectedCount }}</strong><em>自动实时发现</em></div></article>
      <article><span class="metric-icon violet"><el-icon><Monitor /></el-icon></span><div><small>检查会话</small><strong>{{ inspectingCount }}</strong><em>各设备会话独立</em></div></article>
      <article><span class="metric-icon green"><el-icon><VideoPlay /></el-icon></span><div><small>客户端版本</small><strong class="version-value">v{{ info?.version || '—' }}</strong><em>受控构建 · 协议 v1</em></div></article>
    </section>

    <section class="workspace-panel">
      <header><div><span class="section-mark"><el-icon><Iphone /></el-icon></span><div><h2>已连接设备</h2><p>列表每 2 秒自动刷新，点击 Inspect 在当前页打开调试台。</p></div></div><el-button :icon="Refresh" :loading="refreshing" @click="load(true)">刷新</el-button></header>
      <div v-if="targets.length" class="device-list">
        <article v-for="target in targets" :key="target.id" class="device-card">
          <div class="device-avatar"><el-icon><Iphone /></el-icon><span /></div>
          <div class="device-main"><div class="device-title"><strong>{{ target.title || '未命名页面' }}</strong><el-tag size="small" type="success" effect="light" round>在线</el-tag><el-tag v-if="target.inspecting" size="small" type="warning" effect="light" round>检查中</el-tag></div><a v-if="safeTargetURL(target.url)" :href="safeTargetURL(target.url)" target="_blank" rel="noopener noreferrer">{{ target.url }}</a><span v-else class="device-url">{{ target.url }}</span><div class="device-meta"><span>{{ platformLabel(target.user_agent) }}</span><span>{{ target.ip }}</span><span>会话 {{ target.id.slice(0, 8) }}</span><span>连接于 {{ formatTime(target.connected_at) }}</span></div></div>
          <div class="device-actions"><el-button text type="danger" @click="disconnect(target)">断开</el-button><el-button type="primary" @click="inspect(target)">Inspect</el-button></div>
        </article>
      </div>
      <div v-else class="empty-state"><span><el-icon><Connection /></el-icon></span><h3>等待小程序连接</h3><p>复制上方 Client script URL，在手机日志设置中依次点击 Save address 和 Enable。</p></div>
      <footer><span><i /> 服务运行中</span><small>{{ lastRefresh ? `最后刷新 ${lastRefresh.toLocaleTimeString('zh-CN', { hour12: false })}` : '正在同步状态' }}</small></footer>
    </section>

    <section class="guide-panel">
      <header><h2>连接步骤</h2><span>手机和平台地址必须可互相访问</span></header>
      <div class="steps"><article><b>01</b><div><strong>复制脚本地址</strong><p>使用页面顶部选中的局域网或域名地址。</p></div></article><article><b>02</b><div><strong>在小程序中启用</strong><p>粘贴地址后点击 Save address，再点击 Enable。</p></div></article><article><b>03</b><div><strong>打开 Inspect</strong><p>设备出现后进入调试台，在 Console 查看实时日志。</p></div></article></div>
      <div class="notice"><strong>连接提示</strong><span>测试包需要允许当前脚本来源；若更换平台域名、IP 或端口，需要同步检查 TikTok 测试包的 trustedDomains。客户端脚本内容保持原始受控构建，SRI 不变。</span></div>
    </section>
    </template>
  </main>
</template>

<style scoped>
.logs-page{min-height:100%;padding:22px;background:#f5f7fb;color:#172033}.hero-panel{position:relative;display:grid;grid-template-columns:minmax(0,1fr) 260px;min-height:250px;padding:34px 38px;overflow:hidden;border:1px solid #dce8f4;border-radius:22px;background:radial-gradient(circle at 88% 14%,rgba(34,211,238,.16),transparent 28%),linear-gradient(135deg,#fafdff 0%,#f4f9ff 58%,#eefcff 100%);box-shadow:0 15px 36px rgba(34,73,113,.08)}.hero-copy{position:relative;z-index:1;max-width:760px}.eyebrow{display:inline-flex;align-items:center;gap:7px;color:#0891b2;font-size:10px;font-weight:800;letter-spacing:.12em}.hero-copy h1{margin:15px 0 9px;font-size:30px;letter-spacing:-.03em}.hero-copy>p{max-width:680px;margin:0;color:#64748b;font-size:13px;line-height:1.8}.script-address{display:flex;align-items:center;gap:12px;max-width:760px;margin-top:25px;padding:11px 12px 11px 16px;border:1px solid #d8e8f3;border-radius:13px;background:rgba(255,255,255,.84);box-shadow:0 8px 20px rgba(44,84,120,.06)}.script-address>div{display:flex;min-width:0;flex:1;flex-direction:column;gap:4px}.script-address span{color:#94a3b8;font-size:9px;font-weight:700;text-transform:uppercase}.script-address code{overflow:hidden;color:#0f766e;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.hero-visual{position:relative;display:grid;place-items:center;align-content:center}.hero-visual>div{position:relative;z-index:2;display:grid;place-items:center;width:94px;height:94px;border:1px solid rgba(6,182,212,.2);border-radius:30px;color:#0891b2;background:rgba(255,255,255,.8);box-shadow:0 18px 44px rgba(8,145,178,.16);font-size:38px}.hero-visual small{z-index:2;margin-top:15px;color:#0f766e;font-weight:700}.signal{position:absolute;width:145px;height:145px;border:1px solid rgba(6,182,212,.18);border-radius:50%}.signal--two{width:205px;height:205px;opacity:.7}.metrics-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:14px;margin:16px 0}.metrics-grid article{display:flex;align-items:center;gap:14px;padding:17px 19px;border:1px solid #e5eaf1;border-radius:16px;background:#fff}.metric-icon{display:grid;place-items:center;width:43px;height:43px;border-radius:13px;font-size:20px}.metric-icon.cyan{color:#0891b2;background:#ecfeff}.metric-icon.violet{color:#7c3aed;background:#f5f3ff}.metric-icon.green{color:#059669;background:#ecfdf5}.metrics-grid article>div{display:grid;grid-template-columns:auto 1fr;align-items:baseline;column-gap:10px}.metrics-grid small{color:#64748b;font-size:10px}.metrics-grid strong{grid-row:1/3;grid-column:2;font-size:25px}.metrics-grid em{color:#94a3b8;font-size:9px;font-style:normal}.metrics-grid .version-value{font-size:18px}.workspace-panel,.guide-panel{border:1px solid #e5eaf1;border-radius:18px;background:#fff}.workspace-panel>header,.guide-panel>header{display:flex;align-items:center;justify-content:space-between;padding:19px 21px;border-bottom:1px solid #edf1f6}.workspace-panel>header>div{display:flex;align-items:center;gap:11px}.section-mark{display:grid;place-items:center;width:38px;height:38px;border-radius:11px;color:#0891b2;background:#ecfeff}.workspace-panel h2,.guide-panel h2{margin:0;color:#1e293b;font-size:15px}.workspace-panel header p{margin:4px 0 0;color:#94a3b8;font-size:10px}.device-list{padding:5px 20px}.device-card{display:grid;grid-template-columns:48px minmax(0,1fr) auto;align-items:center;gap:14px;padding:16px 2px;border-bottom:1px solid #edf1f6}.device-card:last-child{border-bottom:0}.device-avatar{position:relative;display:grid;place-items:center;width:45px;height:45px;border-radius:14px;color:#0891b2;background:#ecfeff;font-size:21px}.device-avatar span{position:absolute;right:2px;bottom:2px;width:9px;height:9px;border:2px solid #fff;border-radius:50%;background:#10b981}.device-title{display:flex;align-items:center;gap:7px}.device-title strong{color:#334155;font-size:13px}.device-main>a{display:block;max-width:680px;margin-top:4px;overflow:hidden;color:#0891b2;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.device-meta{display:flex;flex-wrap:wrap;gap:15px;margin-top:7px;color:#94a3b8;font-size:9px}.device-meta span+span:before{content:'·';margin-right:15px}.device-actions{display:flex;align-items:center;gap:5px}.empty-state{display:flex;align-items:center;flex-direction:column;padding:53px 20px}.empty-state>span{display:grid;place-items:center;width:58px;height:58px;border-radius:18px;color:#0891b2;background:#ecfeff;font-size:26px}.empty-state h3{margin:14px 0 5px;color:#475569;font-size:13px}.empty-state p{margin:0;color:#94a3b8;font-size:10px}.workspace-panel>footer{display:flex;justify-content:space-between;padding:11px 21px;border-top:1px solid #edf1f6;color:#94a3b8;font-size:9px}.workspace-panel>footer span{display:flex;align-items:center;gap:6px;color:#059669}.workspace-panel>footer i{width:6px;height:6px;border-radius:50%;background:#10b981}.guide-panel{margin-top:16px}.guide-panel>header span{color:#94a3b8;font-size:10px}.steps{display:grid;grid-template-columns:repeat(3,1fr);gap:14px;padding:20px}.steps article{display:flex;gap:12px;padding:15px;border-radius:13px;background:#f8fafc}.steps b{color:#06b6d4;font-size:19px}.steps strong{color:#334155;font-size:11px}.steps p{margin:5px 0 0;color:#94a3b8;font-size:9px;line-height:1.6}.notice{display:flex;gap:12px;margin:0 20px 20px;padding:12px 14px;border:1px solid #fde68a;border-radius:11px;color:#92400e;background:#fffbeb;font-size:10px;line-height:1.6}.notice strong{flex:none}@media(max-width:900px){.hero-panel{grid-template-columns:1fr}.hero-visual{display:none}.metrics-grid,.steps{grid-template-columns:1fr}.device-card{grid-template-columns:42px 1fr}.device-actions{grid-column:2}.logs-page{padding:14px}.hero-panel{padding:25px}.script-address{align-items:stretch;flex-direction:column}}
.device-url{display:block;max-width:680px;margin-top:4px;overflow:hidden;color:#64748b;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.address-select{width:185px;flex:none}.script-address{display:grid;grid-template-columns:minmax(0,1fr) 185px auto;align-items:end;max-width:none}.script-address code{overflow:visible;text-overflow:clip;white-space:normal;word-break:break-all}.inspector-panel{display:flex;height:calc(100vh - 108px);min-height:620px;overflow:hidden;flex-direction:column;border:1px solid #dce4ed;border-radius:18px;background:#fff;box-shadow:0 12px 34px rgba(15,23,42,.08)}.inspector-header{display:flex;min-height:64px;align-items:center;justify-content:space-between;padding:10px 16px;border-bottom:1px solid #e7edf3;background:linear-gradient(180deg,#fff,#f8fafc)}.inspector-header>div{display:flex;align-items:center;gap:13px}.inspector-device{display:flex;align-items:center;gap:9px}.inspector-device>i{width:9px;height:9px;border-radius:50%;background:#10b981;box-shadow:0 0 0 4px rgba(16,185,129,.12)}.inspector-device>span{display:flex;flex-direction:column}.inspector-device strong{color:#263449;font-size:13px}.inspector-device small{margin-top:3px;color:#8a98aa;font-size:9px}.inspector-panel iframe{width:100%;min-height:0;flex:1;border:0;background:#fff}@media(max-width:900px){.script-address{grid-template-columns:1fr}.address-select{width:100%}.inspector-panel{height:calc(100vh - 82px);min-height:520px}}
</style>
