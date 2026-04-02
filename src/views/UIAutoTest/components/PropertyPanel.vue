<script setup lang="ts">
import { ref, watch } from 'vue'
import { type TestStep } from '@/stores/modules/playwright'
import { InfoFilled } from '@element-plus/icons-vue'

const props = defineProps<{
  step: TestStep
  variables: Record<string, string>
}>()

const localArgs = ref<Record<string, string>>({})

watch(() => props.step, (newStep) => {
  if (newStep) {
    localArgs.value = { ...newStep.args }
  }
}, { immediate: true, deep: true })

const updateArg = (key: string, val: string) => {
  props.step.args[key] = val
}

const updateReturnVar = (val: string) => {
  props.step.return_var = val
}

const updateDescription = (val: string) => {
  props.step.description = val
}

const updateDisabled = (_val: boolean) => {
  props.step.disabled = _val
}

</script>

<template>
  <div class="property-panel">
    <div class="panel-header">
      <span>步骤详情: {{ step.keyword }}</span>
    </div>
    
    <el-scrollbar>
      <div class="property-form">
        <el-form label-position="top">
          <el-form-item label="步骤描述">
            <el-input 
              v-model="step.description" 
              placeholder="例如: 点击登录按钮" 
              @input="updateDescription"
            />
          </el-form-item>
          
          <el-divider>关键字参数</el-divider>
          
          <div v-for="(_, key) in step.args" :key="key" class="arg-item">
            <el-form-item :label="key">
              <el-input 
                v-model="step.args[key]" 
                placeholder="输入参数值，支持 ${变量}" 
                @input="(v: string) => updateArg(key, v)"
              />
            </el-form-item>
          </div>
          
          <el-divider>高级配置</el-divider>
          
          <el-form-item label="返回值存入变量">
            <template #label>
              <div class="label-with-info">
                <span>返回值存入变量</span>
                <el-tooltip content="执行结果将存入该变量名，供后续步骤使用">
                  <el-icon><InfoFilled /></el-icon>
                </el-tooltip>
              </div>
            </template>
            <el-input 
              v-model="step.return_var" 
              placeholder="例如: token_result" 
              @input="updateReturnVar"
            />
          </el-form-item>
          
          <el-form-item label="状态控制">
            <el-checkbox 
              v-model="step.disabled" 
              label="禁用该步骤" 
              @change="updateDisabled"
            />
          </el-form-item>
        </el-form>
      </div>
    </el-scrollbar>
  </div>
</template>

<style scoped>
.property-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.panel-header {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid #f0f0f0;
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}

.property-form {
  padding: 20px;
}

.label-with-info {
  display: flex;
  align-items: center;
  gap: 6px;
}

:deep(.el-divider__text) {
  font-size: 11px;
  color: #909399;
  background: #fff;
}

.arg-item {
  margin-bottom: 12px;
}
</style>
