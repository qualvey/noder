前端，在删除节点后后的自动刷新，页面会跳一下，请让它优雅一些，比如自动卡片平滑移位

乐观删除（Optimistic Removal）：

点击确认删除时，前端立即在本地响应式数组中剔除该节点，零等待立即启动动画；
后台静默发送 DELETE /api/nodes/{id} 请求，请求成功后静默校验，若网络异常则优雅回滚恢复状态。
Vue TransitionGroup + FLIP 移位动画：

使用 <TransitionGroup name="node-list"> 接管卡片网格；
增加 onBeforeLeave 几何尺寸锚定钩子：卡片在离开瞬间自动锁定当前坐标（width/height/left/top），无缝脱离文档流并淡出缩小；
其余兄弟卡片通过 CSS 贝塞尔曲线（cubic-bezier(0.25, 1, 0.5, 1)）平滑滑向新位置，彻底告别突兀闪烁。
静默刷新机制（Silent Fetch）：

fetchNodes(silent = true)：只有在初次加载且无数据时才展示骨架/转圈，后续的任何删除、新增、修改均采用静默无感更新。
