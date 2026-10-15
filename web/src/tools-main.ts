import { createApp } from 'vue'
import ToolsApp from './ToolsApp.vue'
import './styles.css'

const savedTheme = localStorage.getItem('noder_tools_theme')
const systemLight = window.matchMedia?.('(prefers-color-scheme: light)').matches
const theme = savedTheme === 'light' || savedTheme === 'dark'
  ? savedTheme
  : systemLight ? 'light' : 'dark'

document.documentElement.dataset.theme = theme
createApp(ToolsApp).mount('#app')
