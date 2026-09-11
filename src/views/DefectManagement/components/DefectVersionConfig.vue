<script setup lang="ts">
import { reactive, watch } from "vue"
import { ElMessage } from "element-plus"
import { defectApi } from "../api"
import type { DefectMeta } from "../types"

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
    drafts[code] = { online: stages?.online || "", testing: stages?.testing || "" }
  }
}, { immediate: true })

async function save(code: string) {
  const draft = drafts[code]
  if (!draft) return
  saving[code] = true
  try {
    await defectApi.saveVersions(code, { online: draft.online.trim(), testing: draft.testing.trim() })
    dirty[code] = false
    ElMessage.success(code + " 版本已保存")
    emit("changed")
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || "版本号保存失败")
  } finally {
    saving[code] = false
  }
}
</script>

<template>
  <article class="version-config">
    <header class="version-header">
      <strong>版本号管理</strong>
      <p>按项目分别维护当前线上版本与提测版本，保存后将自动同步至缺陷版本选择下拉项中。</p>
    </header>
    <div class="version-table">
      <div class="version-head">
        <span>项目</span>
        <span>当前线上版本</span>
        <span>提测版本</span>
        <span />
      </div>
      <div v-for="project in meta?.projects || []" :key="project.id" class="version-row">
        <div class="project-info">
          <strong>{{ project.project_code }}</strong>
          <span>{{ project.project_name }}</span>
          <small v-if="!meta?.project_version_stages?.[project.project_code]?.online && !meta?.project_version_stages?.[project.project_code]?.testing && meta?.project_versions?.[project.project_code]?.length">
            原版本：{{ meta.project_versions[project.project_code]?.join("、") }}（待指定用途）
          </small>
          <small v-if="meta?.project_version_stages?.[project.project_code]?.rule">
            {{ meta.project_version_stages[project.project_code]?.rule }}
          </small>
        </div>
        <template v-if="drafts[project.project_code]">
          <el-input
            v-model="drafts[project.project_code]!.online"
            class="online-input"
            :disabled="saving[project.project_code]"
            maxlength="128"
            placeholder="当前线上版本"
            :aria-label="project.project_code + ' 当前线上版本'"
            @input="dirty[project.project_code] = true"
          />
          <el-input
            v-model="drafts[project.project_code]!.testing"
            class="testing-input"
            :disabled="saving[project.project_code]"
            maxlength="128"
            placeholder="提测版本"
            :aria-label="project.project_code + ' 提测版本'"
            @input="dirty[project.project_code] = true"
          />
          <el-button
            type="primary"
            plain
            :disabled="!dirty[project.project_code]"
            :loading="saving[project.project_code]"
            @click="save(project.project_code)"
          >
            保存
          </el-button>
        </template>
      </div>
      <el-empty v-if="!meta?.projects?.length" description="暂无项目" :image-size="64" />
    </div>
    <p class="version-tip">说明：版本号可留空；两项相同时自动合并为一个选项。更新版本不会改写历史缺陷记录。</p>
  </article>
</template>

<style scoped>
.version-config {
  padding: 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.version-header strong {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.version-header p {
  font-size: 13px;
  color: #64748b;
  line-height: 1.6;
  margin: 4px 0 0;
}

.version-table {
  margin-top: 18px;
}

.version-head,
.version-row {
  display: grid;
  grid-template-columns: minmax(220px, 1.5fr) minmax(160px, 1fr) minmax(160px, 1fr) 76px;
  gap: 16px;
  align-items: center;
}

.version-head {
  padding: 10px 14px;
  background: #f8fafc;
  border-radius: 8px;
  color: #64748b;
  font-size: 12px;
  font-weight: 600;
}

.version-row {
  padding: 14px 12px;
  border-bottom: 1px solid #f1f5f9;
}

.version-row:last-child {
  border-bottom: 0;
}

.project-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.project-info strong {
  font-size: 13px;
  font-weight: 700;
  color: #2563eb;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.project-info span {
  font-size: 13px;
  color: #334155;
  overflow-wrap: anywhere;
}

.project-info small {
  font-size: 12px;
  color: #94a3b8;
}

.online-input :deep(.el-input__wrapper) {
  background: #f4fbf7;
}

.testing-input :deep(.el-input__wrapper) {
  background: #f5f8ff;
}

.version-tip {
  padding-top: 12px;
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
}

@media (max-width: 760px) {
  .version-head {
    display: none;
  }
  .version-row {
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .project-info {
    grid-column: 1 / -1;
  }
  .version-row > .el-button {
    justify-self: end;
    grid-column: 2;
  }
}
</style>
