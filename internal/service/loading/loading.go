package loading

import (
	"fmt"
	"html"

	"noder/internal/model"
)

func RenderLoadingPage(dist *model.DistFile, user *model.User) string {
	rawFilename := "config.zip"
	if dist.DownloadName != nil && *dist.DownloadName != "" {
		rawFilename = *dist.DownloadName
	} else if dist.OriginalName != "" {
		rawFilename = dist.OriginalName
	}
	safeFilename := html.EscapeString(rawFilename)
	safeUserName := html.EscapeString(user.Name)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>正在生成配置包 - %s</title>
  <style>
    :root {
      --bg-dark: #090d16;
      --bg-card: rgba(17, 24, 39, 0.78);
      --border-glass: rgba(255, 255, 255, 0.08);
      --border-glow: rgba(59, 130, 246, 0.35);
      --primary: #3b82f6;
      --primary-hover: #2563eb;
      --accent-cyan: #06b6d4;
      --accent-emerald: #10b981;
      --accent-rose: #f43f5e;
      --text-main: #f8fafc;
      --text-muted: #94a3b8;
      --text-dim: #64748b;
      --radius: 20px;
    }
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }
    body {
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      background-color: var(--bg-dark);
      background-image: 
        radial-gradient(ellipse 80%% 80%% at 50%% -20%%, rgba(59, 130, 246, 0.18), transparent),
        radial-gradient(ellipse 60%% 60%% at 50%% 120%%, rgba(6, 182, 212, 0.12), transparent);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      color: var(--text-main);
      padding: 20px;
      overflow-x: hidden;
    }
    .card {
      background: var(--bg-card);
      border: 1px solid var(--border-glass);
      backdrop-filter: blur(16px);
      -webkit-backdrop-filter: blur(16px);
      border-radius: var(--radius);
      padding: 36px 32px;
      width: 100%%;
      max-width: 440px;
      text-align: center;
      box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 255, 255, 0.05);
      position: relative;
    }
    .loader-container {
      position: relative;
      width: 80px;
      height: 80px;
      margin: 0 auto 24px;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .spinner-ring {
      position: absolute;
      inset: 0;
      border-radius: 50%%;
      border: 3px solid transparent;
      border-top-color: var(--primary);
      border-right-color: var(--accent-cyan);
      animation: spin 1.2s cubic-bezier(0.55, 0.25, 0.25, 0.95) infinite;
    }
    .spinner-ring-inner {
      position: absolute;
      inset: 8px;
      border-radius: 50%%;
      border: 2px solid transparent;
      border-bottom-color: rgba(59, 130, 246, 0.5);
      animation: spin-reverse 1.8s linear infinite;
    }
    .loader-icon {
      font-size: 2rem;
      animation: pulse-icon 2s ease-in-out infinite;
      user-select: none;
    }
    @keyframes spin {
      0%% { transform: rotate(0deg); }
      100%% { transform: rotate(360deg); }
    }
    @keyframes spin-reverse {
      0%% { transform: rotate(0deg); }
      100%% { transform: rotate(-360deg); }
    }
    @keyframes pulse-icon {
      0%%, 100%% { transform: scale(1); opacity: 0.9; }
      50%% { transform: scale(1.08); opacity: 1; }
    }
    .title {
      font-size: 1.35rem;
      font-weight: 700;
      color: var(--text-main);
      margin-bottom: 10px;
    }
    .subtitle {
      font-size: 0.88rem;
      color: var(--text-muted);
      line-height: 1.5;
      margin-bottom: 24px;
    }
    .info-box {
      background: rgba(15, 23, 42, 0.6);
      border: 1px solid rgba(255, 255, 255, 0.05);
      border-radius: 12px;
      padding: 14px 16px;
      margin-bottom: 24px;
      display: flex;
      flex-direction: column;
      gap: 8px;
      text-align: left;
      font-size: 0.82rem;
    }
    .info-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .info-label { color: var(--text-dim); }
    .info-value { color: var(--text-main); font-family: monospace; }
    .progress-wrap { margin: 0 0 24px; text-align: left; }
    .progress-meta { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 8px; color: var(--text-muted); font-size: 0.8rem; }
    .progress-track { height: 8px; overflow: hidden; border-radius: 999px; background: rgba(148, 163, 184, 0.16); }
    .progress-fill { width: 0%%; height: 100%%; border-radius: inherit; background: linear-gradient(90deg, var(--primary), var(--accent-cyan)); transition: width 180ms ease; }
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 10px 20px;
      background: var(--primary);
      color: #fff;
      border-radius: 10px;
      text-decoration: none;
      font-weight: 600;
      font-size: 0.88rem;
    }
  </style>
</head>
<body>
  <div class="card" id="main-card">
    <div class="loader-container">
      <div class="spinner-ring" id="spinner-outer"></div>
      <div class="spinner-ring-inner" id="spinner-inner"></div>
      <div class="loader-icon" id="loader-icon">📦</div>
    </div>
    <div class="title" id="status-title">正在生成专属配置包...</div>
    <div class="subtitle" id="status-desc">系统正在将最新的节点集群与您的安全凭证注入客户端模板，请稍候</div>
    <div class="info-box">
      <div class="info-row">
        <span class="info-label">目标文件</span>
        <span class="info-value">%s</span>
      </div>
      <div class="info-row">
        <span class="info-label">订阅用户</span>
        <span class="info-value">%s</span>
      </div>
    </div>
    <div class="progress-wrap" aria-live="polite">
      <div class="progress-meta">
        <span id="progress-stage">准备生成配置包</span>
        <span id="progress-percent">0%%</span>
      </div>
      <div class="progress-track" role="progressbar" aria-label="配置包生成进度" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0" id="progress-track">
        <div class="progress-fill" id="progress-fill"></div>
      </div>
    </div>
    <div id="actions" style="display: none;">
      <a href="#" class="btn" id="download-btn">点击下载</a>
    </div>
  </div>
  <script>
    (function() {
      let displayedPercent = 0;
      const updateProgress = (percent, stage) => {
        if (percent < displayedPercent) return;
        displayedPercent = percent;
        document.getElementById('progress-fill').style.width = percent + '%%';
        document.getElementById('progress-percent').textContent = percent + '%%';
        document.getElementById('progress-stage').textContent = stage;
        document.getElementById('progress-track').setAttribute('aria-valuenow', percent);
      };
      const makeProgressId = () => {
        if (window.crypto && crypto.randomUUID) return crypto.randomUUID();
        if (window.crypto && crypto.getRandomValues) {
          return Array.from(crypto.getRandomValues(new Uint8Array(16)), b => b.toString(16).padStart(2, '0')).join('');
        }
        return Date.now().toString(16) + Math.random().toString(16).slice(2);
      };
      const progressId = makeProgressId();
      const progressUrl = new URL(window.location.href);
      progressUrl.searchParams.set('progress_stream', '1');
      progressUrl.searchParams.set('progress_id', progressId);
      const progressEvents = new EventSource(progressUrl.toString());
      progressEvents.onmessage = event => {
        const progress = JSON.parse(event.data);
        updateProgress(progress.percent, progress.stage);
        if (progress.done) progressEvents.close();
      };

      const readBlobWithProgress = async response => {
        if (!response.body || !response.body.getReader) return response.blob();
        const total = Number(response.headers.get('Content-Length')) || 0;
        const reader = response.body.getReader();
        const chunks = [];
        let received = 0;
        while (true) {
          const {done, value} = await reader.read();
          if (done) break;
          chunks.push(value);
          received += value.byteLength;
          const percent = total > 0
            ? Math.min(99, 90 + Math.floor(Math.min(1, received / total) * 9))
            : displayedPercent;
          const stage = total > 0
            ? '正在接收配置包（' + (received / 1048576).toFixed(1) + ' / ' + (total / 1048576).toFixed(1) + ' MB）'
            : '正在接收配置包（' + (received / 1048576).toFixed(1) + ' MB）';
          updateProgress(percent, stage);
        }
        return new Blob(chunks, {type: response.headers.get('Content-Type') || 'application/zip'});
      };

      const url = new URL(window.location.href);
      url.searchParams.set('download', '1');
      url.searchParams.set('progress_id', progressId);
      fetch(url.toString())
        .then(res => {
          if (!res.ok) throw new Error('HTTP ' + res.status);
          return readBlobWithProgress(res);
        })
        .then(blob => {
          const blobUrl = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = blobUrl;
          a.download = '%s';
          document.body.appendChild(a);
          a.click();
          a.remove();
          document.getElementById('status-title').textContent = '配置包生成完成！';
          document.getElementById('status-desc').textContent = '若浏览器未自动触发下载，请点击下方按钮：';
          document.getElementById('loader-icon').textContent = '✅';
          updateProgress(100, '配置包已接收，正在开始下载');
          progressEvents.close();
          document.getElementById('download-btn').href = blobUrl;
          document.getElementById('download-btn').download = '%s';
          document.getElementById('actions').style.display = 'block';
        })
        .catch(err => {
          progressEvents.close();
          document.getElementById('status-title').textContent = '生成失败';
          document.getElementById('status-desc').textContent = err.message || '网络连接超时，请重试';
          document.getElementById('loader-icon').textContent = '⚠️';
        });
    })();
  </script>
</body>
</html>`, safeFilename, safeFilename, safeUserName, safeFilename, safeFilename)
}
