import { createApp } from 'vue'
import App from './App.vue'
import './styles.css'

// 主题初始化（mount 前设置，避免闪烁）：localStorage 优先，其次系统偏好，默认深色
const savedTheme = localStorage.getItem('noder_theme')
const systemLight = window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches
const initialTheme = savedTheme === 'light' || savedTheme === 'dark'
  ? savedTheme
  : systemLight ? 'light' : 'dark'
document.documentElement.dataset.theme = initialTheme

createApp(App).mount('#app')
