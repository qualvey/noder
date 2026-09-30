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
    <div id="actions" style="display: none;">
      <a href="#" class="btn" id="download-btn">点击下载</a>
    </div>
  </div>
  <script>
    (function() {
      const url = new URL(window.location.href);
      url.searchParams.set('download', '1');
      fetch(url.toString())
        .then(res => res.blob())
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
          document.getElementById('download-btn').href = blobUrl;
          document.getElementById('download-btn').download = '%s';
          document.getElementById('actions').style.display = 'block';
        })
        .catch(err => {
          document.getElementById('status-title').textContent = '生成失败';
          document.getElementById('status-desc').textContent = err.message || '网络连接超时，请重试';
          document.getElementById('loader-icon').textContent = '⚠️';
        });
    })();
  </script>
</body>
</html>`, safeFilename, safeFilename, safeUserName, safeFilename, safeFilename)
}
