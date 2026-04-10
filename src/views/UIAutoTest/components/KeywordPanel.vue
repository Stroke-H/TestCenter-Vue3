<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { usePlaywrightStore } from '@/stores/modules/playwright'
import { Search, Plus, CollectionTag } from '@element-plus/icons-vue'

const props = defineProps({
  associatedSuites: {
    type: Array as () => string[],
    default: () => []
  }
})

const pwStore = usePlaywrightStore()
const searchQuery = ref('')

onMounted(async () => {
  await pwStore.fetchUserKeywords()
})

const groups = computed(() => {
  // 1. Builtin Keywords
  const builtin = [...pwStore.builtinKeywords]
  const grouped: Record<string, any[]> = {}
  
  builtin.forEach(kw => {
    if (searchQuery.value && !kw.name.includes(searchQuery.value) && !kw.keyword.includes(searchQuery.value)) {
      return
    }
    const groupItems = grouped[kw.group] || (grouped[kw.group] = [])
    groupItems.push(kw)
  })

  const finalGroups = Object.entries(grouped).map(([name, items]) => ({ 
    name, 
    items, 
    type: 'builtin' 
  }))

  // 2. Associated User Keywords (Skills)
  if (props.associatedSuites.length > 0) {
    const userKws = (pwStore.userKeywords || []).filter(kw => 
      props.associatedSuites.includes(kw.suite_name)
    )

    const userGrouped: Record<string, any[]> = {}
    userKws.forEach(kw => {
      const groupName = `套件库-${kw.suite_name}`
      if (searchQuery.value && !kw.name.includes(searchQuery.value) && !kw.description.includes(searchQuery.value)) {
        return
      }
      const groupItems = userGrouped[groupName] || (userGrouped[groupName] = [])
      // Map UserKeyword to the format expected by the editor
      groupItems.push({
        name: kw.name,
        keyword: kw.name, // Use name as the keyword identifier for execution
        group: groupName,
        description: kw.description,
        args: (kw.args || []).map((a: any) => a.name || a),
        type: 'skill'
      })
    })

    const skillGroups = Object.entries(userGrouped).map(([name, items]) => ({
      name,
      items,
      type: 'skill'
    }))

    // Find index of '断言' to insert after
    const assertionIndex = finalGroups.findIndex(g => g.name === '断言')
    if (assertionIndex !== -1) {
      finalGroups.splice(assertionIndex + 1, 0, ...skillGroups)
    } else {
      finalGroups.push(...skillGroups)
    }
  }
  
  return finalGroups
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
          <div class="group-title">
            <el-icon v-if="group.type === 'skill'"><CollectionTag /></el-icon>
            {{ group.name }}
          </div>
          <div 
            v-for="kw in group.items" 
            :key="kw.keyword" 
            class="keyword-item"
            :class="{ 'skill-item': kw.type === 'skill' }"
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
  display: flex;
  align-items: center;
  gap: 6px;
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

.skill-item {
  background: #f0f7ff33;
}

.skill-item:hover {
  background: #f0f7ff;
  border-color: #a0cfff;
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
