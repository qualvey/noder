import { createRouter, createWebHistory } from 'vue-router'
import FilesView from './views/FilesView.vue'
import HelpView from './views/HelpView.vue'
import NodesView from './views/NodesView.vue'
import ToolsView from './views/ToolsView.vue'
import UsersView from './views/UsersView.vue'
import WatermarkTool from './tools/WatermarkTool.vue'
import NodeTransferTool from './tools/NodeTransferTool.vue'
import QRCodeTool from './tools/QRCodeTool.vue'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: '/nodes',
    },
    {
      path: '/nodes',
      name: 'nodes',
      component: NodesView,
      meta: { navKey: 'nodes' },
    },
    {
      path: '/users',
      name: 'users',
      component: UsersView,
      meta: { navKey: 'users' },
    },
    {
      path: '/files',
      name: 'files',
      component: FilesView,
      meta: { navKey: 'files' },
    },
    {
      path: '/help',
      name: 'help',
      component: HelpView,
      meta: { navKey: 'help' },
    },
    {
      path: '/tools',
      component: ToolsView,
      meta: { navKey: 'tools' },
      children: [
        {
          path: '',
          redirect: { name: 'tool-watermark' },
        },
        {
          path: 'watermark',
          name: 'tool-watermark',
          component: WatermarkTool,
        },
        {
          path: 'transfer',
          name: 'tool-transfer',
          component: NodeTransferTool,
        },
        {
          path: 'qrcode',
          name: 'tool-qrcode',
          component: QRCodeTool,
        },
      ],
    },
  ],
})
