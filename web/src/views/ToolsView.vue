<script setup lang="ts">
import { ref } from "vue";
import WatermarkTool from "../tools/WatermarkTool.vue";
import NodeTransferTool from "../tools/NodeTransferTool.vue";
import QRCodeTool from "../tools/QRCodeTool.vue";

type ToolKey = "watermark" | "transfer" | "qrcode";
const activeTool = ref<ToolKey>("watermark");

const tools = [
  { key: "watermark" as const, icon: "✦", label: "图片水印", hint: "编辑图片" },
  { key: "transfer" as const, icon: "⇄", label: "节点转换", hint: "转换配置" },
  { key: "qrcode" as const, icon: "⌁", label: "二维码", hint: "生成与解析" },
];
</script>

<template>
  <section class="tools-page">
    <nav class="tools-nav" aria-label="工具选择">
      <button
        v-for="tool in tools"
        :key="tool.key"
        type="button"
        :class="{ active: activeTool === tool.key }"
        @click="activeTool = tool.key"
      >
        <span class="nav-icon">{{ tool.icon }}</span>
        <span
          ><strong>{{ tool.label }}</strong
          ></span
        >
      </button>
    </nav>

    <main class="tools-content">
      <WatermarkTool v-if="activeTool === 'watermark'" />
      <NodeTransferTool v-else-if="activeTool === 'transfer'" />
      <QRCodeTool v-else />
    </main>
  </section>
</template>

<style scoped>
.tools-page {
  min-width: 0;
  color: var(--text-main);
}
.tools-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}
.tools-eyebrow {
  color: var(--primary);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.16em;
}
.tools-header h1 {
  margin: 5px 0 4px;
  font-size: clamp(1.35rem, 2.5vw, 1.8rem);
  letter-spacing: -0.03em;
}
.tools-header p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.76rem;
}
.tools-status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 10px;
  border: 1px solid var(--border-glass);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 0.66rem;
  white-space: nowrap;
}
.tools-status i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-emerald);
  box-shadow: 0 0 0 3px
    color-mix(in srgb, var(--accent-emerald) 14%, transparent);
}
.tools-nav {
  display: flex;
  justify-content: center;
  gap: 6px;
  margin-bottom: 16px;
  padding: 5px;
  border: 1px solid var(--border-glass);
  border-radius: 12px;
  background: var(--bg-soft);
}
.tools-nav button {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 150px;
  padding: 8px 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  text-align: left;
  transition: 0.2s;
}
.tools-nav button:hover {
  color: var(--text-main);
  background: color-mix(in srgb, var(--primary) 8%, transparent);
}
.tools-nav button.active {
  border-color: color-mix(in srgb, var(--primary) 25%, var(--border-glass));
  background: var(--bg-elevated);
  color: var(--text-main);
  box-shadow: 0 3px 10px rgba(2, 8, 23, 0.12);
}
.nav-icon {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  background: color-mix(in srgb, var(--primary) 12%, transparent);
  color: var(--primary);
  font-size: 1rem;
  font-weight: 800;
}
.active .nav-icon {
  background: var(--primary);
  color: white;
}
.tools-nav button span:last-child {
  display: grid;
  gap: 1px;
}
.tools-nav strong {
  font-size: 0.72rem;
}
.tools-nav small {
  color: var(--text-dim);
  font-size: 0.6rem;
}
.tools-content {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--border-glass);
  border-radius: 14px;
  background: var(--bg-card);
  box-shadow: var(--shadow-lg);
}
@media (max-width: 600px) {
  .tools-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .tools-nav {
    gap: 3px;
  }
  .tools-nav button {
    min-width: 0;
    flex: 1;
    justify-content: center;
    padding: 7px 4px;
  }
  .tools-nav button span:last-child {
    text-align: center;
  }
  .tools-nav small {
    display: none;
  }
  .tools-content {
    padding: 12px;
  }
}
@media (max-width: 380px) {
  .nav-icon {
    display: none;
  }
  .tools-nav strong {
    font-size: 0.66rem;
  }
}
</style>
