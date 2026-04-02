<script setup lang="ts">
import { ref, computed } from 'vue'
import { usePlaywrightStore } from '@/stores/modules/playwright'
import { Search, Plus } from '@element-plus/icons-vue'

const pwStore = usePlaywrightStore()
const searchQuery = ref('')

const groups = computed(() => {
  const all = [...pwStore.builtinKeywords]
  const grouped: Record<string, any[]> = {}
  
  all.forEach(kw => {
    if (searchQuery.value && !kw.name.includes(searchQuery.value) && !kw.keyword.includes(searchQuery.value)) {
      return
    }
    const groupItems = grouped[kw.group] || (grouped[kw.group] = [])
    groupItems.push(kw)
  })
  
  return Object.entries(grouped).map(([name, items]) => ({ name, items }))
})

const emit = defineEmits(['add-step'])

const addStep = (keyword: any) => {
  emit('add-step', keyword)
}

</script>

<template>
  <div class="keyword-panel">
    <div class="search-box">
      <el-input 
        v-model="searchQuery" 
        placeholder="搜索关键字..." 
        :prefix-icon="Search" 
        clearable 
      />
    </div>
    
    <div class="keyword-list">
      <el-scrollbar>
        <div v-for="group in groups" :key="group.name" class="keyword-group">
          <div class="group-title">{{ group.name }}</div>
          <div 
            v-for="kw in group.items" 
            :key="kw.keyword" 
            class="keyword-item"
            @click="addStep(kw)"
          >
            <div class="kw-info">
              <span class="kw-name">{{ kw.name }}</span>
              <span class="kw-code">{{ kw.keyword }}</span>
            </div>
            <el-icon class="add-icon"><Plus /></el-icon>
          </div>
        </div>
      </el-scrollbar>
    </div>
  </div>
</template>

<style scoped>
.keyword-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.search-box {
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;
}

.keyword-list {
  flex: 1;
  overflow: hidden;
}

.keyword-group {
  padding: 12px 16px;
}

.group-title {
  font-size: 12px;
  text-transform: uppercase;
  color: #909399;
  font-weight: 600;
  margin-bottom: 10px;
  letter-spacing: 0.5px;
}

.keyword-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-radius: 6px;
  cursor: pointer;
  margin-bottom: 4px;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.keyword-item:hover {
  background: #fdfdfd;
  border-color: #dcdfe6;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}

.kw-info {
  display: flex;
  flex-direction: column;
}

.kw-name {
  font-size: 13.5px;
  color: #303133;
  margin-bottom: 2px;
}

.kw-code {
  font-size: 10px;
  color: #c0c4cc;
  font-family: monospace;
}

.add-icon {
  color: #409eff;
  font-size: 14px;
  opacity: 0;
  transition: opacity 0.2s;
}

.keyword-item:hover .add-icon {
  opacity: 1;
}
</style>
