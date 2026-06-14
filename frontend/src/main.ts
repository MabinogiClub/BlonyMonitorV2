/**
 * Vue 应用入口文件
 * 初始化 Vue 3 应用和 Pinia 状态管理
 */

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

// 导入 Vant 暗色主题
import 'vant/es/toast/style'
import 'vant/es/dialog/style'
import 'vant/es/notify/style'

// 导入全局样式（包含 Vant 暗色主题变量覆盖）
import './styles/base.scss'

// 创建 Vue 应用实例
const app = createApp(App)

// 使用 Pinia 状态管理
const pinia = createPinia()
app.use(pinia)

// 挂载应用
app.mount('#app')
