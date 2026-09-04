# -*- coding: utf-8 -*-
"""下载加载页与错误页渲染服务（深色高质感毛玻璃动效界面）。"""
import html
from app.models import DistFile, User


def render_loading_page(dist: DistFile, user: User) -> str:
    """渲染个性化 ZIP 配置包在浏览器打开时的 Loading 界面。"""
    raw_filename = dist.download_name or dist.original_name or "config.zip"
    safe_filename = html.escape(raw_filename)
    safe_user_name = html.escape(user.name)

    return f"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>正在生成配置包 - {safe_filename}</title>
  <style>
    :root {{
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
    }}
    * {{
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }}
    body {{
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      background-color: var(--bg-dark);
      background-image: 
        radial-gradient(ellipse 80% 80% at 50% -20%, rgba(59, 130, 246, 0.18), transparent),
        radial-gradient(ellipse 60% 60% at 50% 120%, rgba(6, 182, 212, 0.12), transparent);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      color: var(--text-main);
      padding: 20px;
      overflow-x: hidden;
    }}
    .ambient-glow {{
      position: fixed;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      width: 520px;
      height: 520px;
      background: radial-gradient(circle, rgba(59, 130, 246, 0.14) 0%, rgba(6, 182, 212, 0.06) 50%, transparent 70%);
      filter: blur(60px);
      pointer-events: none;
      z-index: 0;
    }}
    .card {{
      position: relative;
      z-index: 1;
      background: var(--bg-card);
      backdrop-filter: blur(20px);
      -webkit-backdrop-filter: blur(20px);
      border: 1px solid var(--border-glass);
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 40px rgba(59, 130, 246, 0.08);
      border-radius: var(--radius);
      width: 100%;
      max-width: 480px;
      padding: 36px 32px;
      text-align: center;
      transition: border-color 0.3s ease, box-shadow 0.3s ease;
    }}
    .card.success {{
      border-color: rgba(16, 185, 129, 0.35);
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 40px rgba(16, 185, 129, 0.15);
    }}
    .card.error {{
      border-color: rgba(244, 63, 94, 0.35);
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 40px rgba(244, 63, 94, 0.15);
    }}
    .brand-tag {{
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 4px 12px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 9999px;
      font-size: 0.75rem;
      color: var(--text-muted);
      margin-bottom: 24px;
      letter-spacing: 0.03em;
    }}
    .loader-container {{
      position: relative;
      width: 88px;
      height: 88px;
      margin: 0 auto 24px;
      display: flex;
      align-items: center;
      justify-content: center;
    }}
    .spinner-ring {{
      position: absolute;
      inset: 0;
      border-radius: 50%;
      border: 3px solid transparent;
      border-top-color: var(--primary);
      border-right-color: var(--accent-cyan);
      animation: spin 1.2s cubic-bezier(0.5, 0, 0.5, 1) infinite;
    }}
    .spinner-ring-inner {{
      position: absolute;
      inset: 8px;
      border-radius: 50%;
      border: 2px solid transparent;
      border-bottom-color: rgba(59, 130, 246, 0.5);
      animation: spin-reverse 1.8s linear infinite;
    }}
    .loader-icon {{
      font-size: 2rem;
      animation: pulse-icon 2s ease-in-out infinite;
      user-select: none;
    }}
    @keyframes spin {{
      0% {{ transform: rotate(0deg); }}
      100% {{ transform: rotate(360deg); }}
    }}
    @keyframes spin-reverse {{
      0% {{ transform: rotate(0deg); }}
      100% {{ transform: rotate(-360deg); }}
    }}
    @keyframes pulse-icon {{
      0%, 100% {{ transform: scale(1); opacity: 0.9; }}
      50% {{ transform: scale(1.08); opacity: 1; }}
    }}
    .title {{
      font-size: 1.35rem;
      font-weight: 700;
      color: var(--text-main);
      margin-bottom: 10px;
      letter-spacing: -0.01em;
    }}
    .subtitle {{
      font-size: 0.88rem;
      color: var(--text-muted);
      line-height: 1.5;
      margin-bottom: 24px;
    }}
    .info-box {{
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
    }}
    .info-row {{
      display: flex;
      justify-content: space-between;
      align-items: center;
    }}
    .info-label {{
      color: var(--text-dim);
    }}
    .info-value {{
      color: var(--text-main);
      font-family: 'Fira Code', Consolas, monospace;
      font-weight: 500;
      max-width: 240px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }}
    .progress-track {{
      width: 100%;
      height: 4px;
      background: rgba(255, 255, 255, 0.06);
      border-radius: 9999px;
      overflow: hidden;
      margin-bottom: 20px;
      position: relative;
    }}
    .progress-bar {{
      height: 100%;
      width: 40%;
      background: linear-gradient(90deg, var(--primary), var(--accent-cyan));
      border-radius: 9999px;
      position: absolute;
      animation: indeterminate 1.8s cubic-bezier(0.65, 0.815, 0.735, 0.395) infinite;
    }}
    @keyframes indeterminate {{
      0% {{ left: -40%; width: 40%; }}
      50% {{ left: 30%; width: 60%; }}
      100% {{ left: 100%; width: 40%; }}
    }}
    .actions {{
      display: none;
      flex-direction: column;
      gap: 10px;
      margin-top: 12px;
    }}
    .btn {{
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      padding: 12px 24px;
      border-radius: 10px;
      font-size: 0.9rem;
      font-weight: 600;
      text-decoration: none;
      transition: all 0.2s ease;
      cursor: pointer;
      border: none;
    }}
    .btn-primary {{
      background: linear-gradient(135deg, #3b82f6, #06b6d4);
      color: #fff;
      box-shadow: 0 4px 14px rgba(59, 130, 246, 0.35);
    }}
    .btn-primary:hover {{
      opacity: 0.95;
      transform: translateY(-1px);
      box-shadow: 0 6px 20px rgba(59, 130, 246, 0.45);
    }}
    .error-box {{
      display: none;
      background: rgba(244, 63, 94, 0.1);
      border: 1px solid rgba(244, 63, 94, 0.25);
      color: #fda4af;
      padding: 12px;
      border-radius: 10px;
      font-size: 0.82rem;
      margin-bottom: 16px;
      word-break: break-word;
    }}
  </style>
</head>
<body>
  <div class="ambient-glow"></div>
  <div class="card" id="card">
    <div class="brand-tag">⚡ Sing-Box Sub Middleman · 动态配置分发</div>
    
    <div class="loader-container">
      <div class="spinner-ring" id="spinner-outer"></div>
      <div class="spinner-ring-inner" id="spinner-inner"></div>
      <div class="loader-icon" id="loader-icon">📦</div>
    </div>
    
    <div class="progress-track" id="progress-track">
      <div class="progress-bar" id="progress-bar"></div>
    </div>
    
    <h1 class="title" id="status-title">正在生成专属配置包...</h1>
    <p class="subtitle" id="status-desc">正在注入用户凭证并个性化渲染配置模板，请稍候</p>
    
    <div class="info-box">
      <div class="info-row">
        <span class="info-label">目标文件</span>
        <span class="info-value" title="{safe_filename}">{safe_filename}</span>
      </div>
      <div class="info-row">
        <span class="info-label">订阅用户</span>
        <span class="info-value">{safe_user_name}</span>
      </div>
      <div class="info-row">
        <span class="info-label">分发格式</span>
        <span class="info-value">ZIP 个性化模板渲染</span>
      </div>
    </div>
    
    <div class="error-box" id="error-box"></div>
    
    <div class="actions" id="actions">
      <a href="#" id="download-btn" class="btn btn-primary">📥 点击重新下载</a>
    </div>
  </div>

  <script>
    (function() {{
      const params = new URLSearchParams(window.location.search);
      params.set('download', '1');
      const downloadUrl = window.location.pathname + '?' + params.toString();

      const card = document.getElementById('card');
      const spinnerOuter = document.getElementById('spinner-outer');
      const spinnerInner = document.getElementById('spinner-inner');
      const loaderIcon = document.getElementById('loader-icon');
      const progressTrack = document.getElementById('progress-track');
      const title = document.getElementById('status-title');
      const desc = document.getElementById('status-desc');
      const errorBox = document.getElementById('error-box');
      const actions = document.getElementById('actions');
      const downloadBtn = document.getElementById('download-btn');

      const startTime = Date.now();
      const minDisplayTime = 800;

      fetch(downloadUrl)
        .then(async function(res) {{
          if (!res.ok) {{
            let detail = '下载失败 (' + res.status + ')';
            try {{
              const data = await res.json();
              if (data.detail) detail = data.detail;
            }} catch (_) {{}}
            throw new Error(detail);
          }}

          let filename = '{safe_filename}';
          const cd = res.headers.get('Content-Disposition');
          if (cd) {{
            const star = cd.match(/filename\\*=UTF-8''([^;]+)/i);
            if (star) {{
              filename = decodeURIComponent(star[1]);
            }} else {{
              const match = cd.match(/filename="?([^";]+)"?/i);
              if (match) filename = match[1];
            }}
          }}

          const blob = await res.blob();
          const elapsed = Date.now() - startTime;
          const delay = Math.max(0, minDisplayTime - elapsed);

          setTimeout(function() {{
            const blobUrl = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = blobUrl;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            a.remove();

            spinnerOuter.style.display = 'none';
            spinnerInner.style.display = 'none';
            loaderIcon.textContent = '✅';
            loaderIcon.style.animation = 'none';
            progressTrack.style.display = 'none';
            card.classList.add('success');

            title.textContent = '配置包生成完成！';
            desc.textContent = '浏览器已自动开始下载。若未自动触发，请点击下方按钮：';

            downloadBtn.href = blobUrl;
            downloadBtn.download = filename;
            actions.style.display = 'flex';
          }}, delay);
        }})
        .catch(function(err) {{
          const elapsed = Date.now() - startTime;
          const delay = Math.max(0, minDisplayTime - elapsed);

          setTimeout(function() {{
            spinnerOuter.style.display = 'none';
            spinnerInner.style.display = 'none';
            loaderIcon.textContent = '⚠️';
            loaderIcon.style.animation = 'none';
            progressTrack.style.display = 'none';
            card.classList.add('error');

            title.textContent = '配置包生成失败';
            desc.textContent = '处理个性化配置时发生错误，请检查网络或联系管理员。';
            errorBox.textContent = err.message || '未知异常';
            errorBox.style.display = 'block';

            downloadBtn.textContent = '🔄 刷新重试';
            downloadBtn.href = window.location.href;
            downloadBtn.removeAttribute('download');
            actions.style.display = 'flex';
          }}, delay);
        }});
    }})();
  </script>
</body>
</html>"""
