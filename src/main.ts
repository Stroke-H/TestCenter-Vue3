// 应用入口 — 注册 Vue Router、Pinia、Element Plus
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'

import App from './App.vue'
import router from './router'
import './assets/styles/index.css'

// 创建 Vue 应用实例
const app = createApp(App)

// 注册插件
app.use(createPinia())  // 状态管理
app.use(router)          // 路由
app.use(ElementPlus)     // UI 组件库

// 挂载到 DOM
app.mount('#app')
