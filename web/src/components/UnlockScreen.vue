<script setup lang="ts">
defineProps<{
  token: string
  showPassword: boolean
  unlockError: boolean
}>()

const emit = defineEmits<{
  (e: 'update:token', val: string): void
  (e: 'toggle-visibility'): void
  (e: 'clear-error'): void
  (e: 'submit'): void
}>()
</script>

<template>
  <section class="decoy-page">
    <div class="decoy-window">
      <header class="decoy-header">
        <div class="decoy-brand">
          <span class="decoy-brand-mark">☁</span>
          <div>
            <strong>今日天气</strong>
            <span>本地生活助手</span>
          </div>
        </div>
        <span class="decoy-date">每日小站</span>
      </header>

      <main class="decoy-content">
        <div class="decoy-location">当前位置 · 晴朗</div>
        <div class="decoy-weather">
          <span class="decoy-sun">☀</span>
          <div>
            <strong>24°</strong>
            <span>舒适 · 微风</span>
          </div>
        </div>
        <div class="decoy-details">
          <div><span>空气质量</span><strong>良</strong></div>
          <div><span>湿度</span><strong>48%</strong></div>
          <div><span>风力</span><strong>2 级</strong></div>
        </div>

        <form class="unlock-form" @submit.prevent="emit('submit')">
          <label for="unlock-password">查看完整内容</label>
          <div class="unlock-input-wrap">
            <input
              id="unlock-password"
              :value="token"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="current-password"
              placeholder="请输入访问口令"
              autofocus
              @input="emit('update:token', ($event.target as HTMLInputElement).value); emit('clear-error')"
            />
            <button
              type="button"
              class="unlock-visibility"
              :aria-label="showPassword ? '隐藏口令' : '显示口令'"
              @click="emit('toggle-visibility')"
            >
              {{ showPassword ? '隐藏' : '显示' }}
            </button>
          </div>
          <p v-if="unlockError" class="unlock-error">口令不正确，请重试</p>
          <button type="submit" class="unlock-button">进入</button>
        </form>
      </main>

      <footer class="decoy-footer">生活有序，心情自在</footer>
    </div>
  </section>
</template>

<style scoped>
.decoy-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: #eef3f5;
  color: #24333b;
  font-family: 'Segoe UI', 'Microsoft YaHei', sans-serif;
}
.decoy-window {
  width: min(100%, 460px);
  overflow: hidden;
  border: 1px solid #dbe4e7;
  border-radius: 18px;
  background: #fbfcfc;
  box-shadow: 0 18px 50px rgba(49, 72, 82, 0.12);
}
.decoy-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22px 26px;
  border-bottom: 1px solid #e9eff0;
  background: #f7faf9;
}
.decoy-brand,
.decoy-weather,
.decoy-details,
.unlock-input-wrap {
  display: flex;
  align-items: center;
}
.decoy-brand {
  gap: 11px;
}
.decoy-brand-mark {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 11px;
  background: #dcefeb;
  color: #4b988c;
  font-size: 1.35rem;
}
.decoy-brand strong,
.decoy-brand span,
.decoy-weather strong,
.decoy-weather span {
  display: block;
}
.decoy-brand strong {
  font-size: 0.98rem;
  font-weight: 650;
}
.decoy-brand div span,
.decoy-date,
.decoy-location,
.decoy-weather span,
.decoy-details span,
.decoy-footer {
  color: #829198;
  font-size: 0.78rem;
}
.decoy-content {
  padding: 30px 30px 28px;
}
.decoy-location {
  margin-bottom: 18px;
}
.decoy-weather {
  gap: 18px;
  padding-bottom: 25px;
}
.decoy-sun {
  color: #e9b54f;
  font-size: 3.9rem;
  line-height: 1;
}
.decoy-weather strong {
  color: #30434c;
  font-size: 2.6rem;
  font-weight: 500;
  line-height: 1;
}
.decoy-weather span {
  margin-top: 8px;
}
.decoy-details {
  gap: 10px;
  padding: 15px 0 25px;
  border-top: 1px solid #edf1f1;
  border-bottom: 1px solid #edf1f1;
}
.decoy-details div {
  flex: 1;
  display: grid;
  gap: 4px;
}
.decoy-details strong {
  color: #48616a;
  font-size: 0.9rem;
  font-weight: 600;
}
.unlock-form {
  margin-top: 25px;
}
.unlock-form > label {
  display: block;
  margin-bottom: 9px;
  color: #52666e;
  font-size: 0.82rem;
}
.unlock-input-wrap {
  gap: 8px;
  padding: 4px 5px 4px 13px;
  border: 1px solid #d4e0e1;
  border-radius: 10px;
  background: #fff;
}
.unlock-input-wrap:focus-within {
  border-color: #70aaa0;
  box-shadow: 0 0 0 3px rgba(112, 170, 160, 0.13);
}
.unlock-input-wrap input {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: #30434c;
  font: inherit;
  font-size: 0.9rem;
}
.unlock-visibility {
  border: 0;
  background: transparent;
  color: #6c9992;
  cursor: pointer;
  font-size: 0.76rem;
}
.unlock-button {
  width: 100%;
  margin-top: 12px;
  padding: 11px 16px;
  border: 0;
  border-radius: 10px;
  background: #5d9d92;
  color: #fff;
  cursor: pointer;
  font: inherit;
  font-size: 0.9rem;
  font-weight: 600;
  transition: background 0.2s ease, transform 0.2s ease;
}
.unlock-button:hover {
  background: #4d8d82;
  transform: translateY(-1px);
}
.unlock-error {
  margin: 8px 0 -2px;
  color: #c06464;
  font-size: 0.76rem;
}
.decoy-footer {
  padding: 17px 26px;
  border-top: 1px solid #e9eff0;
  text-align: center;
}
@media (max-width: 520px) {
  .decoy-page {
    padding: 14px;
  }
  .decoy-content {
    padding: 25px 22px 24px;
  }
  .decoy-header {
    padding: 18px 20px;
  }
}
</style>
