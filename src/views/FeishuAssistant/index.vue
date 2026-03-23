<template>
  <div class="feishu-assistant">
    <!-- 头部信息 -->
    <div class="header-section">
      <div class="title-area">
        <h1 class="page-title">Feishu Assistant</h1>
        <el-breadcrumb separator="/">
          <el-breadcrumb-item :to="{ path: '/' }">Home</el-breadcrumb-item>
          <el-breadcrumb-item>飞书助手</el-breadcrumb-item>
        </el-breadcrumb>
      </div>
      <div class="action-area">
        <el-button type="primary" :icon="ChatRound" round @click="handleConfig">
          Bot Config
        </el-button>
      </div>
    </div>

    <!-- 统计指标 (Mock 数据) -->
    <el-row :gutter="24" class="stat-cards">
      <el-col :span="8">
        <el-card shadow="hover" class="stat-card blue-card">
          <div class="stat-content">
            <div class="stat-info">
              <span class="stat-label">Total Messages</span>
              <span class="stat-value"><strong>{{ mockStats.totalMessages }}</strong></span>
            </div>
            <div class="stat-icon-wrapper">
              <el-icon :size="24"><ChatLineSquare /></el-icon>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="8">
        <el-card shadow="hover" class="stat-card green-card">
          <div class="stat-content">
            <div class="stat-info">
              <span class="stat-label">Active Users</span>
              <span class="stat-value"><strong>{{ mockStats.activeUsers }}</strong></span>
            </div>
            <div class="stat-icon-wrapper">
              <el-icon :size="24"><User /></el-icon>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="8">
        <el-card shadow="hover" class="stat-card purple-card">
          <div class="stat-content">
            <div class="stat-info">
              <span class="stat-label">Bot Status</span>
              <span class="stat-value"><strong>{{ mockStats.status }}</strong></span>
            </div>
            <div class="stat-icon-wrapper">
              <el-icon :size="24"><Connection /></el-icon>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 业务数据展示区 -->
    <el-row :gutter="24" class="data-view-section">
      <!-- 左侧：最新消息列表 (2/3 宽度) -->
      <el-col :span="16">
        <el-card shadow="never" class="table-card">
          <template #header>
            <div class="card-header-flex">
              <span class="card-title">Recent Messages</span>
              <div class="header-filters">
                <el-input 
                  v-model="searchQuery" 
                  placeholder="Search contents..." 
                  prefix-icon="Search"
                  style="width: 200px" 
                />
              </div>
            </div>
          </template>
          
          <el-table :data="mockMessages" stripe style="width: 100%">
            <el-table-column label="Sender" min-width="120">
              <template #default="scope">
                <div class="sender-info">
                  <el-avatar :size="30" :style="getAvatarStyle(scope.row.senderName)">
                    {{ scope.row.senderName.charAt(0) }}
                  </el-avatar>
                  <span class="sender-name">{{ scope.row.senderName }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="msgType" label="Type" width="100">
              <template #default="scope">
                <el-tag :type="getTypeStyle(scope.row.msgType)" size="small" round>
                  {{ scope.row.msgType }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column prop="content" label="Content" min-width="250" show-overflow-tooltip />
            
            <el-table-column prop="time" label="Time" width="150" />
            
            <el-table-column label="Action" width="100" fixed="right">
              <template #default="scope">
                <el-button link type="primary" @click="handleReply(scope.row)">Reply</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <!-- 右侧：快捷操作或活跃用户 (1/3 宽度) -->
      <el-col :span="8">
        <el-card shadow="never" class="side-card active-users-card">
          <template #header>
            <div class="card-header-flex">
              <span class="card-title">Top Active Users</span>
            </div>
          </template>
          
          <div class="user-list">
            <div v-for="(user, index) in mockActiveUsers" :key="index" class="user-item">
              <div class="user-meta">
                <el-avatar :size="32" :style="getAvatarStyle(user.name)">
                  {{ user.name.charAt(0) }}
                </el-avatar>
                <div class="user-details">
                  <div class="user-name">{{ user.name }}</div>
                  <div class="user-dept">{{ user.dept }}</div>
                </div>
              </div>
              <div class="user-count">
                <el-badge :value="user.msgCount" class="count-badge" type="primary" />
              </div>
            </div>
          </div>
        </el-card>

        <!-- 模拟发消息表单 (未来替换为真实 API) -->
        <el-card shadow="never" class="side-card quick-reply-card" style="margin-top: 24px;">
          <template #header>
            <div class="card-header-flex">
              <span class="card-title">Quick Reply Test</span>
            </div>
          </template>
          <div class="quick-reply-form">
            <el-input 
              v-model="replyContent" 
              type="textarea" 
              :rows="3" 
              placeholder="Enter message to broadcast or test..." 
            />
            <el-button type="primary" style="margin-top: 12px; width: 100%" @click="sendMockReply">
              Send Message
            </el-button>
          </div>
        </el-card>

      </el-col>
    </el-row>

  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ChatRound, ChatLineSquare, User, Connection, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

// State
const searchQuery = ref('')
const replyContent = ref('')

// Mock Data
const mockStats = ref({
  totalMessages: 1284,
  activeUsers: 45,
  status: 'Online'
})

const mockMessages = ref([
  { id: 1, senderName: 'Alice Wong', msgType: 'text', content: 'Can you help me check the server status?', time: '10:23 AM' },
  { id: 2, senderName: 'Bob Chen', msgType: 'post', content: '[Rich Text] Bug report for the new landing page.', time: '09:45 AM' },
  { id: 3, senderName: 'Charlie Liu', msgType: 'image', content: '[Image: dashboard_error.png]', time: '09:12 AM' },
  { id: 4, senderName: 'Diana Ma', msgType: 'text', content: 'LGTM!', time: 'Yesterday' },
  { id: 5, senderName: 'Alice Wong', msgType: 'text', content: 'Thanks, the fix works perfectly.', time: 'Yesterday' }
])

const mockActiveUsers = ref([
  { name: 'Alice Wong', dept: 'QA Engineering', msgCount: 42 },
  { name: 'Bob Chen', dept: 'Frontend Team', msgCount: 38 },
  { name: 'Diana Ma', dept: 'Product Manager', msgCount: 21 },
  { name: 'Charlie Liu', dept: 'DevOps', msgCount: 15 },
  { name: 'Eve Zhang', dept: 'Backend Team', msgCount: 9 }
])

// Handlers
const handleConfig = () => {
  ElMessage.info('Bot configuration dialog will open here.')
}

const handleReply = (row: any) => {
  ElMessage.success(`Replying to ${row.senderName}... (Not implemented yet)`)
}

const sendMockReply = () => {
  if (!replyContent.value) {
    ElMessage.warning('Please enter a message first.')
    return
  }
  ElMessage.success('Message sent! (Mock implementation)')
  replyContent.value = ''
}

// Helpers
const getAvatarStyle = (name: string) => {
  const colors = ['#409EFF', '#67C23A', '#E6A23C', '#F56C6C', '#8E44AD', '#3498DB']
  const charCode = name.charCodeAt(0)
  return {
    backgroundColor: colors[charCode % colors.length],
    color: '#fff'
  }
}

const getTypeStyle = (type: string) => {
  switch (type) {
    case 'text': return 'primary'
    case 'post': return 'success'
    case 'image': return 'warning'
    case 'file': return 'info'
    default: return 'info'
  }
}
</script>

<style scoped>
.feishu-assistant {
  padding: 24px;
  background-color: #f7f9fa;
  min-height: calc(100vh - 60px);
}

/* Header Section */
.header-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.title-area .page-title {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  color: #1f2329;
}

/* Stat Cards */
.stat-cards {
  margin-bottom: 24px;
}

.stat-card {
  border-radius: 12px;
  border: none;
  transition: transform 0.2s, box-shadow 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08) !important;
}

.stat-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-label {
  font-size: 14px;
  color: #8f959e;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 28px;
  color: #1f2329;
  line-height: 1;
}

.stat-icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Color Themes for Cards */
.blue-card .stat-icon-wrapper { background-color: rgba(64, 158, 255, 0.1); color: #409EFF; }
.green-card .stat-icon-wrapper { background-color: rgba(103, 194, 58, 0.1); color: #67C23A; }
.purple-card .stat-icon-wrapper { background-color: rgba(142, 68, 173, 0.1); color: #8E44AD; }

/* Data View Section */
.data-view-section {
  margin-bottom: 24px;
}

.table-card, .side-card {
  border-radius: 12px;
  border: 1px solid #ebeef5;
}

.card-header-flex {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #1f2329;
}

.sender-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sender-name {
  font-weight: 500;
  color: #333;
}

/* Side Card: Expected User List */
.user-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.user-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.user-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-details {
  display: flex;
  flex-direction: column;
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: #1f2329;
}

.user-dept {
  font-size: 12px;
  color: #8f959e;
  margin-top: 2px;
}

:deep(.el-badge__content.is-fixed) {
  position: static;
  transform: none;
}
</style>
