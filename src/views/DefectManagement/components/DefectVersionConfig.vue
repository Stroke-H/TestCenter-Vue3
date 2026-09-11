<script setup lang="ts">
import { reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { defectApi } from '../api'
import type { DefectMeta } from '../types'
const props = defineProps<{ meta: DefectMeta | null }>()
const emit = defineEmits<{ changed: [] }>()
const drafts = reactive<Record<string, { online: string; testing: string }>>({})
const dirty = reactive<Record<string, boolean>>({})
const saving = reactive<Record<string, boolean>>({})
watch(() => props.meta, (meta) => {
  for (const project of meta?.projects || []) {
    const code = project.project_code
    if (dirty[code] || saving[code]) continue
    const stages = meta?.project_version_stages?.[code]
    drafts[code] = { online: stages?.online || '', testing: stages?.testing || '' }
  }
}, { immediate: true })
async function save(code: string) {
  const draft = drafts[code]
  if (!draft) return
  saving[code] = true
  try {
    await defectApi.saveVersions(code, { online: draft.online.trim(), testing: draft.testing.trim() })
    dirty[code] = false
    ElMessage.success(code + ' 版本已保存')
    emit('changed')
  } catch (error: any) { ElMessage.error(error?.response?.data?.error || '版本号保存失败') }
  finally { saving[code] = false }
}
</script>

<template>
  <article class="version-config">
    <header><strong>版本号管理</strong><p>按项目设置当前线上版本与提测版本，保存后同步至缺陷版本选项。</p></header>
    <div class="version-table">
      <div class="version-head"><span>项目</span><span>当前线上版本</span><span>提测版本</span><span /></div>
      <div v-for="project in meta?.projects || []" :key="project.id" class="version-row">
        <div class="project-info"><strong>{{ project.project_code }}</strong><span>{{ project.project_name }}</span><small v-if="!meta?.project_version_stages?.[project.project_code]?.online && !meta?.project_version_stages?.[project.project_code]?.testing && meta?.project_versions?.[project.project_code]?.length">原版本：{{ meta.project_versions[project.project_code]?.join('、') }}（待指定用途）</small><small v-if="meta?.project_version_stages?.[project.project_code]?.rule">{{ meta.project_version_stages[project.project_code]?.rule }}</small></div>
        <template v-if="drafts[project.project_code]">
          <el-input v-model="drafts[project.project_code]!.online" class="online-input" :disabled="saving[project.project_code]" maxlength="128" placeholder="当前线上版本" :aria-label="project.project_code + ' 当前线上版本'" @input="dirty[project.project_code] = true" />
          <el-input v-model="drafts[project.project_code]!.testing" class="testing-input" :disabled="saving[project.project_code]" maxlength="128" placeholder="提测版本" :aria-label="project.project_code + ' 提测版本'" @input="dirty[project.project_code] = true" />
          <el-button type="primary" plain :disabled="!dirty[project.project_code]" :loading="saving[project.project_code]" @click="save(project.project_code)">保存</el-button>
        </template>
      </div>
      <el-empty v-if="!meta?.projects?.length" description="暂无项目" :image-size="64" />
    </div>
    <p class="version-tip">版本号可留空；两项相同时自动合并为一个选项。更新版本不会改写历史缺陷记录。</p>
  </article>
</template>

<style scoped>
.version-config{padding:19px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.version-config header>strong{font-size:15px;color:#172033}.version-config p{font-size:11px;color:#94a3b8;line-height:1.7;margin:5px 0}.version-table{margin-top:17px}.version-head,.version-row{display:grid;grid-template-columns:minmax(220px,1.5fr) minmax(150px,1fr) minmax(150px,1fr) 70px;gap:18px;align-items:center}.version-head{padding:10px 12px;background:#f8fafc;border-radius:8px;color:#64748b;font-size:11px}.version-row{padding:14px 12px;border-bottom:1px solid #eef2f7}.version-row:last-child{border-bottom:0}.project-info{display:flex;flex-direction:column;gap:4px;min-width:0}.project-info strong{font-size:12px;color:#2563eb}.project-info span{font-size:12px;color:#475569;overflow-wrap:anywhere}.project-info small{font-size:10px;color:#94a3b8}.online-input :deep(.el-input__wrapper){background:#f4fbf7}.testing-input :deep(.el-input__wrapper){background:#f5f8ff}.version-tip{padding-top:10px}@media(max-width:760px){.version-head{display:none}.version-row{grid-template-columns:1fr 1fr;gap:12px}.project-info{grid-column:1/-1}.version-row>.el-button{justify-self:end;grid-column:2}}
</style>
