/* ------------------------------------------------------------------
 * Sing-Box Subscription Middleman - Admin Dashboard SPA Logic
 * 方案 3：独立凭证 + 多节点选择 (支持 TUIC / VLESS REALITY / AnyTLS 动态拼接)
 * ------------------------------------------------------------------ */

document.addEventListener('DOMContentLoaded', () => {
  // --- 全局状态 ---
  let adminToken = localStorage.getItem('admin_token') || 'admin-secret';
  let nodesCache = [];
  let usersCache = [];
  let filesCache = [];

  // --- DOM 元素定义 ---
  const adminTokenInput = document.getElementById('adminTokenInput');
  const saveTokenBtn = document.getElementById('saveTokenBtn');
  
  const metricTotalNodes = document.getElementById('metricTotalNodes');
  const metricTotalUsers = document.getElementById('metricTotalUsers');
  
  const nodesContainer = document.getElementById('nodesContainer');
  const usersTableBody = document.getElementById('usersTableBody');
  const filesTableBody = document.getElementById('filesTableBody');
  const userNodesCheckboxes = document.getElementById('userNodesCheckboxes');
  
  const toastContainer = document.getElementById('toastContainer');
  
  // 初始化 Token 输入框
  adminTokenInput.value = adminToken;

  // --- Toast 消息提示 ---
  function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.style.borderColor = type === 'error' ? 'var(--accent-rose)' : 'var(--primary)';
    toast.innerHTML = `<span>${type === 'error' ? '⚠️' : '✨'}</span><span>${message}</span>`;
    toastContainer.appendChild(toast);
    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateX(100%)';
      setTimeout(() => toast.remove(), 300);
    }, 3000);
  }

  // --- API 通用请求拦截包装 ---
  async function apiFetch(url, options = {}) {
    options.headers = options.headers || {};
    options.headers['X-Admin-Token'] = adminToken;
    if (options.body && typeof options.body === 'object' && !(options.body instanceof FormData)) {
      options.headers['Content-Type'] = 'application/json';
      options.body = JSON.stringify(options.body);
    }
    try {
      const response = await fetch(url, options);
      if (response.status === 401) {
        showToast('鉴权失败：Admin Token 无效', 'error');
        throw new Error('Unauthorized');
      }
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        showToast(`请求失败: ${errorData.detail || response.statusText}`, 'error');
        throw new Error(errorData.detail || 'API Error');
      }
      return await response.json();
    } catch (err) {
      console.error(err);
      throw err;
    }
  }

  // --- 保存 Admin Token ---
  saveTokenBtn.addEventListener('click', () => {
    adminToken = adminTokenInput.value.trim();
    localStorage.setItem('admin_token', adminToken);
    showToast('Admin Token 已成功保存并更新');
    loadDashboardData();
  });

  // --- 标签页切换逻辑 ---
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');
      btn.classList.add('active');
      const tabId = `tab-${btn.dataset.tab}`;
      document.getElementById(tabId).style.display = 'block';
    });
  });

  // --- 协议与 Security 智能交互控制 ---
  const nodeProtocolSelect = document.getElementById('nodeProtocol');
  const nodeSecuritySelect = document.getElementById('nodeSecurity');
  const groupReality = document.getElementById('groupReality');
  const groupTransport = document.getElementById('groupTransport');

  function updateProtocolFormFields() {
    const proto = nodeProtocolSelect.value;
    
    // 协议与 Security 智能绑定锁定
    if (proto === 'tuic') {
      nodeSecuritySelect.value = 'tls';
      Array.from(nodeSecuritySelect.options).forEach(opt => {
        opt.disabled = opt.value !== 'tls';
      });
    } else if (proto === 'vless') {
      nodeSecuritySelect.value = 'reality';
      Array.from(nodeSecuritySelect.options).forEach(opt => {
        opt.disabled = opt.value !== 'reality';
      });
    } else {
      Array.from(nodeSecuritySelect.options).forEach(opt => {
        opt.disabled = false;
      });
    }

    const sec = nodeSecuritySelect.value;

    // REALITY 专属配置区展开控制
    groupReality.style.display = (sec === 'reality') ? 'block' : 'none';
    groupTransport.style.display = ['vless', 'anytls'].includes(proto) ? 'flex' : 'none';
  }

  nodeProtocolSelect.addEventListener('change', updateProtocolFormFields);
  nodeSecuritySelect.addEventListener('change', updateProtocolFormFields);

  // --- Modal 控制逻辑 ---
  function openModal(modalId) {
    document.getElementById(modalId).classList.add('active');
  }
  function closeModal(modalId) {
    document.getElementById(modalId).classList.remove('active');
  }
  document.querySelectorAll('.closeModalBtn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      const modal = e.target.closest('.modal-overlay');
      if (modal) modal.classList.remove('active');
    });
  });

  // --- 鼠标位置 Popover 删除确认框控制器 ---
  let selectedNodeIds = new Set();
  let selectedUserIds = new Set();
  let activePopoverCallback = null;

  const deleteConfirmPopover = document.getElementById('deleteConfirmPopover');
  const popoverTitle = document.getElementById('popoverTitle');
  const popoverCancelBtn = document.getElementById('popoverCancelBtn');
  const popoverConfirmBtn = document.getElementById('popoverConfirmBtn');

  const selectAllNodesCb = document.getElementById('selectAllNodesCb');
  const bulkDeleteNodesBtn = document.getElementById('bulkDeleteNodesBtn');
  const selectedNodeCountEl = document.getElementById('selectedNodeCount');

  const selectAllUsersCb = document.getElementById('selectAllUsersCb');
  const bulkDeleteUsersBtn = document.getElementById('bulkDeleteUsersBtn');
  const selectedUserCountEl = document.getElementById('selectedUserCount');

  function hideDeletePopover() {
    if (deleteConfirmPopover) {
      deleteConfirmPopover.classList.remove('active');
      activePopoverCallback = null;
    }
  }

  if (popoverCancelBtn) {
    popoverCancelBtn.addEventListener('click', hideDeletePopover);
  }

  if (popoverConfirmBtn) {
    popoverConfirmBtn.addEventListener('click', async () => {
      if (typeof activePopoverCallback === 'function') {
        const cb = activePopoverCallback;
        hideDeletePopover();
        await cb();
      }
    });
  }

  // 点击空白处隐去 Popover
  document.addEventListener('click', (e) => {
    if (deleteConfirmPopover && deleteConfirmPopover.classList.contains('active')) {
      if (!deleteConfirmPopover.contains(e.target) && !e.target.closest('.delete-node-btn') && !e.target.closest('.delete-user-btn') && !e.target.closest('.delete-file-btn') && !e.target.closest('#bulkDeleteNodesBtn') && !e.target.closest('#bulkDeleteUsersBtn')) {
        hideDeletePopover();
      }
    }
  });

  function showDeletePopover(targetEl, titleText, onConfirm) {
    if (!deleteConfirmPopover || !targetEl) return;
    popoverTitle.textContent = titleText;
    activePopoverCallback = onConfirm;

    deleteConfirmPopover.classList.add('active');

    const rect = targetEl.getBoundingClientRect();
    const popoverWidth = deleteConfirmPopover.offsetWidth || 230;
    const popoverHeight = deleteConfirmPopover.offsetHeight || 80;

    let left = rect.left + rect.width / 2 - popoverWidth / 2;
    let top = rect.bottom + 8;

    if (left < 10) left = 10;
    if (left + popoverWidth > window.innerWidth - 10) {
      left = window.innerWidth - popoverWidth - 10;
    }

    if (top + popoverHeight > window.innerHeight - 10) {
      top = rect.top - popoverHeight - 8;
    }

    deleteConfirmPopover.style.left = `${left}px`;
    deleteConfirmPopover.style.top = `${top}px`;
  }

  // --- 加载节点数据与渲染 ---
  async function fetchNodes() {
    try {
      nodesCache = await apiFetch('/api/nodes');
      metricTotalNodes.textContent = nodesCache.length;
      renderNodes(nodesCache);
      renderUserNodeCheckboxes(nodesCache, []);
    } catch (err) {}
  }

  function updateNodeSelectionUI() {
    if (selectedNodeCountEl) selectedNodeCountEl.textContent = selectedNodeIds.size;
    if (bulkDeleteNodesBtn) bulkDeleteNodesBtn.style.display = selectedNodeIds.size > 0 ? 'inline-flex' : 'none';
    if (selectAllNodesCb) selectAllNodesCb.checked = nodesCache.length > 0 && selectedNodeIds.size === nodesCache.length;
  }

  if (selectAllNodesCb) {
    selectAllNodesCb.addEventListener('change', () => {
      if (selectAllNodesCb.checked) {
        nodesCache.forEach(n => selectedNodeIds.add(n.id));
      } else {
        selectedNodeIds.clear();
      }
      renderNodes(nodesCache);
    });
  }

  if (bulkDeleteNodesBtn) {
    bulkDeleteNodesBtn.addEventListener('click', (e) => {
      if (selectedNodeIds.size === 0) return;
      const count = selectedNodeIds.size;
      showDeletePopover(e.currentTarget, `⚠️ 确定批量删除已选中的 ${count} 个节点？`, async () => {
        const ids = Array.from(selectedNodeIds);
        for (const id of ids) {
          try {
            await apiFetch(`/api/nodes/${id}`, { method: 'DELETE' });
          } catch (err) {}
        }
        selectedNodeIds.clear();
        showToast(`已成功批量删除 ${count} 个代理节点`);
        fetchNodes();
      });
    });
  }

  function renderNodes(nodes) {
    updateNodeSelectionUI();
    if (!nodes || nodes.length === 0) {
      nodesContainer.innerHTML = `<div style="grid-column: 1/-1; text-align: center; padding: 40px; color: var(--text-muted);">暂无代理节点，请点击右上角新增节点</div>`;
      return;
    }
    nodesContainer.innerHTML = nodes.map(node => {
      const isSelected = selectedNodeIds.has(node.id);
      return `
        <div class="node-card ${isSelected ? 'selected' : ''}" data-id="${node.id}">
          <div class="node-card-header">
            <div style="display:flex; align-items:center; gap:8px;">
              <input type="checkbox" class="node-select-cb" data-id="${node.id}" ${isSelected ? 'checked' : ''} style="cursor:pointer;">
              <div class="node-title">${node.node_name}</div>
            </div>
            <span class="badge badge-${node.protocol}">${node.protocol.toUpperCase()}</span>
          </div>
          <div class="node-details">
            <div class="detail-row">
              <span>服务器地址:</span>
              <span class="value">${node.server_address}:${node.server_port}</span>
            </div>
            <div class="detail-row">
              <span>安全模式:</span>
              <span class="value">${node.security || 'tls'}</span>
            </div>
            ${node.remark ? `<div class="detail-row" style="color:var(--accent-amber); font-style:italic;"><span>管理员备注:</span><span class="value">${node.remark}</span></div>` : ''}
            ${node.sni ? `<div class="detail-row"><span>SNI / 域名:</span><span class="value">${node.sni}</span></div>` : ''}
            ${node.security === 'reality' ? `
              <div class="detail-row"><span>Public Key:</span><span class="value">${node.public_key || '-'}</span></div>
              <div class="detail-row"><span>Short ID:</span><span class="value">${node.short_id || '-'}</span></div>
            ` : ''}
            ${['vless', 'anytls'].includes(node.protocol) ? `<div class="detail-row"><span>传输协议:</span><span class="value">${node.transport_type || 'direct'}</span></div>` : ''}
          </div>
          <div class="card-actions">
            <button class="btn btn-secondary btn-sm edit-node-btn" data-id="${node.id}">编辑</button>
            <button class="btn btn-danger btn-sm delete-node-btn" data-id="${node.id}">删除</button>
          </div>
        </div>
      `;
    }).join('');

    // 绑定多选勾选
    document.querySelectorAll('.node-select-cb').forEach(cb => {
      cb.addEventListener('change', (e) => {
        const id = parseInt(cb.dataset.id);
        if (cb.checked) {
          selectedNodeIds.add(id);
        } else {
          selectedNodeIds.delete(id);
        }
        const card = cb.closest('.node-card');
        if (card) card.classList.toggle('selected', cb.checked);
        updateNodeSelectionUI();
      });
    });

    // 绑定编辑与 Popover 删除
    document.querySelectorAll('.edit-node-btn').forEach(btn => {
      btn.addEventListener('click', () => editNode(parseInt(btn.dataset.id)));
    });
    document.querySelectorAll('.delete-node-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const id = parseInt(btn.dataset.id);
        const node = nodesCache.find(n => n.id === id);
        const name = node ? node.node_name : '';
        showDeletePopover(e.currentTarget, `⚠️ 确定删除节点 "${name}"？`, async () => {
          try {
            await apiFetch(`/api/nodes/${id}`, { method: 'DELETE' });
            selectedNodeIds.delete(id);
            showToast('节点已删除');
            fetchNodes();
          } catch (err) {}
        });
      });
    });
  }

  // --- 从 JSON 一键解析并填充 ---
  const toggleJsonImportBtn = document.getElementById('toggleJsonImportBtn');
  const jsonImportContainer = document.getElementById('jsonImportContainer');
  const parseJsonBtn = document.getElementById('parseJsonBtn');
  const nodeJsonInput = document.getElementById('nodeJsonInput');

  if (toggleJsonImportBtn) {
    toggleJsonImportBtn.addEventListener('click', () => {
      const isHidden = jsonImportContainer.style.display === 'none';
      jsonImportContainer.style.display = isHidden ? 'block' : 'none';
    });
  }

  function safeParseJson(rawStr) {
    let str = rawStr.trim();
    // 1. 去除单行注释 // ... 和多行注释 /* ... */
    str = str.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*/g, '');
    str = str.trim();

    // 2. 去除整体结尾处多余的逗号 (例如从数组复制带出的 `},`)
    if (str.endsWith(',')) {
      str = str.slice(0, -1).trim();
    }

    // 3. 去除内部属性或数组末尾的多余逗号 Trailing Comma (例如 `"a": 1, }`)
    str = str.replace(/,(\s*[\}\]])/g, '$1');

    return JSON.parse(str);
  }

  function doParseNodeJson(jsonStr) {
    if (!jsonStr || !jsonStr.trim()) {
      showToast('请先粘贴 Outbound JSON 文本！', 'error');
      return;
    }
    try {
      let obj = safeParseJson(jsonStr);
      if (Array.isArray(obj.outbounds) && obj.outbounds.length > 0) {
        obj = obj.outbounds[0];
      }
      if (!obj || !obj.type) {
        showToast('解析失败：无效的 Sing-Box Outbound 对象', 'error');
        return;
      }

      if (obj.tag) document.getElementById('nodeName').value = obj.tag;
      if (obj.type && ['tuic', 'vless', 'anytls'].includes(obj.type.toLowerCase())) {
        document.getElementById('nodeProtocol').value = obj.type.toLowerCase();
      }
      if (obj.server) document.getElementById('nodeServer').value = obj.server;
      if (obj.server_port) document.getElementById('nodePort').value = obj.server_port;

      const tls = obj.tls || {};
      if (tls.enabled) {
        if (tls.reality && tls.reality.enabled) {
          document.getElementById('nodeSecurity').value = 'reality';
          if (tls.reality.public_key) document.getElementById('nodePublicKey').value = tls.reality.public_key;
          if (tls.reality.short_id) document.getElementById('nodeShortId').value = tls.reality.short_id;
        } else {
          document.getElementById('nodeSecurity').value = 'tls';
        }
        if (tls.server_name) document.getElementById('nodeSni').value = tls.server_name;
        if (tls.utls && tls.utls.fingerprint) document.getElementById('nodeFingerprint').value = tls.utls.fingerprint;
      } else {
        document.getElementById('nodeSecurity').value = 'none';
      }

      if (obj.flow) document.getElementById('nodeFlow').value = obj.flow;

      const transport = obj.transport || {};
      if (transport.type) document.getElementById('nodeTransport').value = transport.type;
      if (transport.path) document.getElementById('nodePath').value = transport.path;

      updateProtocolFormFields();
      showToast('✨ Sing-Box JSON 解析成功并完成表单填充！');
    } catch (err) {
      showToast(`JSON 解析错误: ${err.message}`, 'error');
    }
  }

  if (parseJsonBtn) {
    parseJsonBtn.addEventListener('click', () => {
      doParseNodeJson(nodeJsonInput.value);
    });
  }

  // 节点 Modal 绑定的 Ctrl+V 全局快捷粘贴监听
  const nodeModalEl = document.getElementById('nodeModal');
  if (nodeModalEl) {
    nodeModalEl.addEventListener('paste', (e) => {
      const active = document.activeElement;
      // 如果焦点不在任何具有值的输入框上，或者正好在 nodeJsonInput 框
      const isOtherInput = active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA' || active.tagName === 'SELECT') && active.id !== 'nodeJsonInput';
      if (!isOtherInput) {
        const text = (e.clipboardData || window.clipboardData).getData('text');
        if (text && text.trim()) {
          if (jsonImportContainer) jsonImportContainer.style.display = 'block';
          nodeJsonInput.value = text;
          doParseNodeJson(text);
        }
      }
    });
  }

  function renderUserNodeCheckboxes(nodes, selectedNodeIds = []) {
    if (!nodes || nodes.length === 0) {
      userNodesCheckboxes.innerHTML = `<div style="color:var(--text-muted); font-size:0.85rem;">暂无可绑定的节点，请先在“节点管理”中创建节点</div>`;
      return;
    }
    userNodesCheckboxes.innerHTML = nodes.map(node => {
      const isChecked = selectedNodeIds.includes(node.id) ? 'checked' : '';
      return `
        <label class="checkbox-label" style="display:flex; align-items:center; gap:8px; background:rgba(15,23,42,0.4); padding:8px 12px; border-radius:6px; border:1px solid var(--border-glass); cursor:pointer;">
          <input type="checkbox" name="userNodes" value="${node.id}" ${isChecked}>
          <span>${node.node_name}</span>
          <span class="badge badge-${node.protocol}" style="font-size:0.65rem;">${node.protocol.toUpperCase()}</span>
        </label>
      `;
    }).join('');
  }

  // --- 新增/编辑节点 Modal ---
  document.getElementById('openAddNodeModalBtn').addEventListener('click', () => {
    document.getElementById('nodeModalTitle').textContent = '新增代理节点';
    document.getElementById('nodeForm').reset();
    document.getElementById('nodeId').value = '';
    document.getElementById('nodeRemark').value = '';
    document.getElementById('nodePublicKey').value = '';
    document.getElementById('nodeShortId').value = '';
    document.getElementById('nodeFingerprint').value = 'chrome';
    document.getElementById('nodeFlow').value = 'xtls-rprx-vision';
    if (jsonImportContainer) jsonImportContainer.style.display = 'none';
    updateProtocolFormFields();
    openModal('nodeModal');
  });

  function editNode(id) {
    const node = nodesCache.find(n => n.id === id);
    if (!node) return;
    document.getElementById('nodeModalTitle').textContent = '编辑代理节点';
    document.getElementById('nodeId').value = node.id;
    document.getElementById('nodeName').value = node.node_name;
    document.getElementById('nodeRemark').value = node.remark || '';
    document.getElementById('nodeProtocol').value = node.protocol;
    document.getElementById('nodeServer').value = node.server_address;
    document.getElementById('nodePort').value = node.server_port;
    document.getElementById('nodeSecurity').value = node.security || 'tls';
    document.getElementById('nodeSni').value = node.sni || '';
    document.getElementById('nodePublicKey').value = node.public_key || '';
    document.getElementById('nodeShortId').value = node.short_id || '';
    document.getElementById('nodeFingerprint').value = node.fingerprint || 'chrome';
    document.getElementById('nodeFlow').value = node.flow || 'xtls-rprx-vision';
    document.getElementById('nodeTransport').value = node.transport_type || 'direct';
    document.getElementById('nodePath').value = node.path || '';

    if (jsonImportContainer) jsonImportContainer.style.display = 'none';
    updateProtocolFormFields();
    openModal('nodeModal');
  }

  document.getElementById('nodeForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const proto = document.getElementById('nodeProtocol').value;
    const sec = document.getElementById('nodeSecurity').value;
    const pbk = document.getElementById('nodePublicKey').value.trim();
    const sid = document.getElementById('nodeShortId').value.trim();

    if (proto === 'tuic' && sec !== 'tls') {
      showToast('保存失败：TUIC 协议必须使用 TLS 传输安全！', 'error');
      return;
    }

    if (proto === 'vless' && sec !== 'reality') {
      showToast('保存失败：VLESS 协议必须使用 REALITY 传输安全！', 'error');
      return;
    }

    if (sec === 'reality' && (!pbk || !sid)) {
      showToast('保存失败：REALITY 安全模式下必须输入 Public Key 和 Short ID！', 'error');
      return;
    }

    const id = document.getElementById('nodeId').value;
    const payload = {
      node_name: document.getElementById('nodeName').value,
      remark: document.getElementById('nodeRemark').value || null,
      protocol: proto,
      server_address: document.getElementById('nodeServer').value,
      server_port: parseInt(document.getElementById('nodePort').value),
      security: sec || 'tls',
      sni: document.getElementById('nodeSni').value || null,
      public_key: pbk || null,
      short_id: sid || null,
      fingerprint: document.getElementById('nodeFingerprint').value || 'chrome',
      flow: document.getElementById('nodeFlow').value || 'xtls-rprx-vision',
      transport_type: document.getElementById('nodeTransport').value || 'direct',
      path: document.getElementById('nodePath').value || null,
      is_active: true
    };

    try {
      if (id) {
        await apiFetch(`/api/nodes/${id}`, { method: 'PUT', body: payload });
        showToast('节点信息更新成功');
      } else {
        await apiFetch('/api/nodes', { method: 'POST', body: payload });
        showToast('新节点添加成功');
      }
      closeModal('nodeModal');
      fetchNodes();
    } catch (err) {}
  });

  async function deleteNode(id) {
    if (!confirm('确定要删除该代理节点吗？')) return;
    try {
      await apiFetch(`/api/nodes/${id}`, { method: 'DELETE' });
      showToast('节点已删除');
      fetchNodes();
    } catch (err) {}
  }

  // --- 加载用户数据与渲染 ---
  async function fetchUsers() {
    try {
      usersCache = await apiFetch('/api/users');
      metricTotalUsers.textContent = usersCache.length;
      renderUsers(usersCache);
    } catch (err) {}
  }

  function updateUserSelectionUI() {
    if (selectedUserCountEl) selectedUserCountEl.textContent = selectedUserIds.size;
    if (bulkDeleteUsersBtn) bulkDeleteUsersBtn.style.display = selectedUserIds.size > 0 ? 'inline-flex' : 'none';
    if (selectAllUsersCb) selectAllUsersCb.checked = usersCache.length > 0 && selectedUserIds.size === usersCache.length;
  }

  if (selectAllUsersCb) {
    selectAllUsersCb.addEventListener('change', () => {
      if (selectAllUsersCb.checked) {
        usersCache.forEach(u => selectedUserIds.add(u.id));
      } else {
        selectedUserIds.clear();
      }
      renderUsers(usersCache);
    });
  }

  if (bulkDeleteUsersBtn) {
    bulkDeleteUsersBtn.addEventListener('click', (e) => {
      if (selectedUserIds.size === 0) return;
      const count = selectedUserIds.size;
      showDeletePopover(e.currentTarget, `⚠️ 确定批量删除已选中的 ${count} 个用户？`, async () => {
        const ids = Array.from(selectedUserIds);
        for (const id of ids) {
          try {
            await apiFetch(`/api/users/${id}`, { method: 'DELETE' });
          } catch (err) {}
        }
        selectedUserIds.clear();
        showToast(`已成功批量删除 ${count} 个用户`);
        fetchUsers();
      });
    });
  }

  function renderUsers(users) {
    updateUserSelectionUI();
    if (!users || users.length === 0) {
      usersTableBody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-muted);">暂无订阅用户</td></tr>`;
      return;
    }
    usersTableBody.innerHTML = users.map(user => {
      const boundNodes = (user.node_ids || []).map(id => nodesCache.find(n => n.id === id)).filter(Boolean);
      const subUrl = `${window.location.origin}/sub?token=${user.token}`;
      const isSelected = selectedUserIds.has(user.id);
      
      const nodesBadgesHtml = boundNodes.length > 0
        ? boundNodes.map(n => `<span class="badge badge-${n.protocol}" style="margin-right:4px;">${n.node_name}</span>`).join('')
        : '<span style="color:var(--text-dim)">未绑定节点</span>';

      return `
        <tr class="${isSelected ? 'selected' : ''}">
          <td style="text-align: center;">
            <input type="checkbox" class="user-select-cb" data-id="${user.id}" ${isSelected ? 'checked' : ''} style="cursor:pointer;">
          </td>
          <td>${user.id}</td>
          <td>
            <div><strong>${user.name}</strong></div>
            ${user.remark ? `<div style="font-size: 0.75rem; color: var(--accent-amber); font-style: italic;">📝 ${user.remark}</div>` : ''}
          </td>
          <td style="font-family: var(--font-mono);">${user.token}</td>
          <td>
            <div style="font-size: 0.82rem; font-family: var(--font-mono);">
              <div><span style="color:var(--text-muted)">UUID:</span> ${user.uuid || '未生成'}</div>
              <div><span style="color:var(--text-muted)">密码:</span> ${user.password || '未生成'}</div>
            </div>
          </td>
          <td><span class="badge ${user.is_active ? 'badge-active' : 'badge-inactive'}">${user.is_active ? '已启用' : '已禁用'}</span></td>
          <td><div style="display:flex; flex-wrap:wrap; gap:4px;">${nodesBadgesHtml}</div></td>
          <td>
            <div style="display: flex; gap: 6px;">
              <button class="btn btn-secondary btn-sm copy-sub-btn" data-url="${subUrl}">复制订阅</button>
              <button class="btn btn-secondary btn-sm preview-sub-btn" data-token="${user.token}">预览JSON</button>
              <button class="btn btn-secondary btn-sm edit-user-btn" data-id="${user.id}">编辑</button>
              <button class="btn btn-danger btn-sm delete-user-btn" data-id="${user.id}">删除</button>
            </div>
          </td>
        </tr>
      `;
    }).join('');

    document.querySelectorAll('.user-select-cb').forEach(cb => {
      cb.addEventListener('change', () => {
        const id = parseInt(cb.dataset.id);
        if (cb.checked) {
          selectedUserIds.add(id);
        } else {
          selectedUserIds.delete(id);
        }
        updateUserSelectionUI();
      });
    });

    document.querySelectorAll('.copy-sub-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        navigator.clipboard.writeText(btn.dataset.url);
        showToast('订阅链接已复制到剪贴板！');
      });
    });

    document.querySelectorAll('.preview-sub-btn').forEach(btn => {
      btn.addEventListener('click', () => previewSubscriptionJson(btn.dataset.token));
    });

    document.querySelectorAll('.edit-user-btn').forEach(btn => {
      btn.addEventListener('click', () => editUser(parseInt(btn.dataset.id)));
    });

    document.querySelectorAll('.delete-user-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const id = parseInt(btn.dataset.id);
        const user = usersCache.find(u => u.id === id);
        const name = user ? user.name : '';
        showDeletePopover(e.currentTarget, `⚠️ 确定删除用户 "${name}"？`, async () => {
          try {
            await apiFetch(`/api/users/${id}`, { method: 'DELETE' });
            selectedUserIds.delete(id);
            showToast('用户已删除');
            fetchUsers();
          } catch (err) {}
        });
      });
    });
  }

  // --- 新增/编辑用户 Modal ---
  const toggleUserJsonBtn = document.getElementById('toggleUserJsonBtn');
  const userJsonContainer = document.getElementById('userJsonContainer');
  const parseUserJsonBtn = document.getElementById('parseUserJsonBtn');
  const userJsonInput = document.getElementById('userJsonInput');

  if (toggleUserJsonBtn) {
    toggleUserJsonBtn.addEventListener('click', () => {
      const isHidden = userJsonContainer.style.display === 'none';
      userJsonContainer.style.display = isHidden ? 'block' : 'none';
    });
  }

  function doParseUserCredentials(rawText) {
    if (!rawText || !rawText.trim()) {
      showToast('请先粘贴包含凭证的文本或 JSON 片段！', 'error');
      return;
    }
    let foundCount = 0;
    
    // 优先匹配 UUID
    const uuidMatch = rawText.match(/"uuid"\s*:\s*"([^"]+)"/i) || rawText.match(/uuid\s*[:=]\s*"?([a-f0-9\-]{36})"?/i);
    if (uuidMatch && uuidMatch[1]) {
      document.getElementById('userUuid').value = uuidMatch[1].trim();
      foundCount++;
    }

    // 匹配 Password
    const pwdMatch = rawText.match(/"password"\s*:\s*"([^"]+)"/i) || rawText.match(/password\s*[:=]\s*"?([^"\s,]+)"?/i);
    if (pwdMatch && pwdMatch[1]) {
      document.getElementById('userPassword').value = pwdMatch[1].trim();
      foundCount++;
    }

    // 匹配 Token
    const tokenMatch = rawText.match(/"token"\s*:\s*"([^"]+)"/i);
    if (tokenMatch && tokenMatch[1]) {
      document.getElementById('userToken').value = tokenMatch[1].trim();
      foundCount++;
    }

    // 匹配 Name / 用户名
    const nameMatch = rawText.match(/"name"\s*:\s*"([^"]+)"/i);
    if (nameMatch && nameMatch[1]) {
      document.getElementById('userName').value = nameMatch[1].trim();
      foundCount++;
    }

    if (foundCount > 0) {
      showToast(`✨ 成功提取并填入 ${foundCount} 项凭证字段！`);
    } else {
      showToast('未从粘贴文本中识别到有效的 UUID / Password 字段', 'error');
    }
  }

  if (parseUserJsonBtn) {
    parseUserJsonBtn.addEventListener('click', () => {
      doParseUserCredentials(userJsonInput.value);
    });
  }

  // 用户 Modal 绑定的 Ctrl+V 全局快捷粘贴监听
  const userModalEl = document.getElementById('userModal');
  if (userModalEl) {
    userModalEl.addEventListener('paste', (e) => {
      const active = document.activeElement;
      const isOtherInput = active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA' || active.tagName === 'SELECT') && active.id !== 'userJsonInput';
      if (!isOtherInput) {
        const text = (e.clipboardData || window.clipboardData).getData('text');
        if (text && text.trim()) {
          if (userJsonContainer) userJsonContainer.style.display = 'block';
          userJsonInput.value = text;
          doParseUserCredentials(text);
        }
      }
    });
  }

  document.getElementById('openAddUserModalBtn').addEventListener('click', () => {
    document.getElementById('userModalTitle').textContent = '新增订阅用户';
    document.getElementById('userForm').reset();
    document.getElementById('userId').value = '';
    document.getElementById('userRemark').value = '';
    document.getElementById('userConfigOverride').value = '';
    if (userJsonContainer) userJsonContainer.style.display = 'none';
    document.getElementById('userToken').value = crypto.randomUUID();
    document.getElementById('userUuid').value = crypto.randomUUID();
    document.getElementById('userPassword').value = Math.random().toString(36).substring(2, 10) + Math.random().toString(36).substring(2, 10);
    renderUserNodeCheckboxes(nodesCache, []);
    openModal('userModal');
  });

  function editUser(id) {
    const user = usersCache.find(u => u.id === id);
    if (!user) return;
    document.getElementById('userModalTitle').textContent = '编辑订阅用户';
    document.getElementById('userId').value = user.id;
    document.getElementById('userName').value = user.name;
    document.getElementById('userRemark').value = user.remark || '';
    document.getElementById('userConfigOverride').value = user.config_override || '';
    document.getElementById('userToken').value = user.token;
    document.getElementById('userUuid').value = user.uuid || '';
    document.getElementById('userPassword').value = user.password || '';
    if (userJsonContainer) userJsonContainer.style.display = 'none';
    renderUserNodeCheckboxes(nodesCache, user.node_ids || []);
    openModal('userModal');
  }

  document.getElementById('genTokenBtn').addEventListener('click', () => {
    document.getElementById('userToken').value = crypto.randomUUID();
  });
  document.getElementById('genUuidBtn').addEventListener('click', () => {
    document.getElementById('userUuid').value = crypto.randomUUID();
  });
  document.getElementById('genPwdBtn').addEventListener('click', () => {
    document.getElementById('userPassword').value = Math.random().toString(36).substring(2, 10) + Math.random().toString(36).substring(2, 10);
  });

  document.getElementById('userForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('userId').value;
    
    // 收集所有勾选的 node_ids
    const checkedNodes = Array.from(document.querySelectorAll('input[name="userNodes"]:checked')).map(el => parseInt(el.value));

    const payload = {
      name: document.getElementById('userName').value,
      remark: document.getElementById('userRemark').value || null,
      config_override: (document.getElementById('userConfigOverride').value || '').trim() || null,
      token: document.getElementById('userToken').value || undefined,
      uuid: document.getElementById('userUuid').value || undefined,
      password: document.getElementById('userPassword').value || undefined,
      node_ids: checkedNodes,
      is_active: true
    };

    try {
      if (id) {
        await apiFetch(`/api/users/${id}`, { method: 'PUT', body: payload });
        showToast('用户信息更新成功');
      } else {
        await apiFetch('/api/users', { method: 'POST', body: payload });
        showToast('新用户添加成功');
      }
      closeModal('userModal');
      fetchUsers();
    } catch (err) {}
  });

  async function deleteUser(id) {
    if (!confirm('确定要删除该用户吗？')) return;
    try {
      await apiFetch(`/api/users/${id}`, { method: 'DELETE' });
      showToast('用户已删除');
      fetchUsers();
    } catch (err) {}
  }

  // --- 文件分发：加载与渲染 ---
  function escapeHtml(str) {
    return String(str ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function formatFileSize(bytes) {
    if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB';
    if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return bytes + ' B';
  }

  async function fetchFiles() {
    try {
      filesCache = await apiFetch('/api/files');
      renderFiles(filesCache);
    } catch (err) {}
  }

  function renderFiles(files) {
    if (!files || files.length === 0) {
      filesTableBody.innerHTML = `<tr><td colspan="9" style="text-align: center; color: var(--text-muted); padding: 30px;">暂无分发文件，点击右上角上传</td></tr>`;
      return;
    }
    const userOptions = usersCache.map(u => `<option value="${u.token}">${escapeHtml(u.name)} (${u.token.slice(0, 8)}…)</option>`).join('');
    filesTableBody.innerHTML = files.map(f => `
      <tr>
        <td>${f.id}</td>
        <td>${escapeHtml(f.name)}${f.remark ? `<div style="font-size:0.72rem;color:var(--text-muted);">${escapeHtml(f.remark)}</div>` : ''}</td>
        <td><span class="badge badge-${f.file_type === 'zip' ? 'vless' : 'tuic'}">${f.file_type.toUpperCase()}</span></td>
        <td>${formatFileSize(f.size)}</td>
        <td>${f.source_url ? `<span title="${escapeHtml(f.source_url)}" style="cursor:help;">🔗 远程</span>` : '<span style="color:var(--text-muted);">📁 本地</span>'}</td>
        <td>${f.file_type === 'zip' ? (f.template_name || '-') : '-'}</td>
        <td>${f.is_active ? '<span style="color:var(--accent-emerald);">🟢 启用</span>' : '<span style="color:var(--accent-rose);">🔴 停用</span>'}</td>
        <td>
          <div style="display:flex; gap:6px; align-items:center;">
            <select class="file-user-select" data-id="${f.id}" style="max-width:150px; padding:4px 6px; border-radius:6px; background:var(--bg-card); color:var(--text); border:1px solid var(--border-glass); font-size:0.75rem;">
              ${userOptions}
            </select>
            <button class="btn btn-secondary btn-sm copy-file-link-btn" data-id="${f.id}">复制链接</button>
          </div>
        </td>
        <td>
          <div style="display:flex; gap:6px;">
            ${f.source_url ? `<button class="btn btn-secondary btn-sm refresh-file-btn" data-id="${f.id}">🔄 刷新</button>` : ''}
            <button class="btn btn-secondary btn-sm toggle-file-btn" data-id="${f.id}">${f.is_active ? '停用' : '启用'}</button>
            <button class="btn btn-danger btn-sm delete-file-btn" data-id="${f.id}">删除</button>
          </div>
        </td>
      </tr>
    `).join('');
  }

  // 文件列表行内操作 (事件委托)
  if (filesTableBody) {
    filesTableBody.addEventListener('click', (e) => {
      const copyBtn = e.target.closest('.copy-file-link-btn');
      const toggleBtn = e.target.closest('.toggle-file-btn');
      const deleteBtn = e.target.closest('.delete-file-btn');
      const refreshBtn = e.target.closest('.refresh-file-btn');

      if (copyBtn) {
        const id = copyBtn.dataset.id;
        const sel = filesTableBody.querySelector(`.file-user-select[data-id="${id}"]`);
        const token = sel ? sel.value : '';
        const url = `${location.origin}/dl/${id}?token=${token}`;
        navigator.clipboard.writeText(url);
        showToast('下载链接已复制到剪贴板');
      } else if (refreshBtn) {
        const id = refreshBtn.dataset.id;
        refreshBtn.disabled = true;
        refreshBtn.textContent = '刷新中...';
        apiFetch(`/api/files/${id}/refresh`, { method: 'POST' })
          .then(() => { showToast('远程文件已刷新'); fetchFiles(); })
          .catch(() => { refreshBtn.disabled = false; refreshBtn.textContent = '🔄 刷新'; });
      } else if (toggleBtn) {
        const file = filesCache.find(f => f.id == toggleBtn.dataset.id);
        if (!file) return;
        apiFetch(`/api/files/${file.id}`, { method: 'PUT', body: { is_active: !file.is_active } })
          .then(() => { showToast(file.is_active ? '文件已停用' : '文件已启用'); fetchFiles(); })
          .catch(() => {});
      } else if (deleteBtn) {
        showDeletePopover(deleteBtn, '⚠️ 确定删除该分发文件？(磁盘文件一并删除)', async () => {
          try {
            await apiFetch(`/api/files/${deleteBtn.dataset.id}`, { method: 'DELETE' });
            showToast('分发文件已删除');
            fetchFiles();
          } catch (err) {}
        });
      }
    });
  }

  // 上传分发文件
  document.getElementById('openAddFileModalBtn').addEventListener('click', () => {
    document.getElementById('fileForm').reset();
    openModal('fileModal');
  });

  document.getElementById('fileForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const fileInput = document.getElementById('fileInput');
    const sourceUrl = document.getElementById('fileSourceUrl').value.trim();

    const hasFile = fileInput.files.length > 0;
    if (!hasFile && !sourceUrl) {
      showToast('请选择文件或填写远程链接', 'error');
      return;
    }
    if (hasFile && sourceUrl) {
      showToast('本地文件与远程链接只能二选一', 'error');
      return;
    }

    const fd = new FormData();
    if (hasFile) {
      fd.append('file', fileInput.files[0]);
    } else {
      fd.append('source_url', sourceUrl);
    }
    fd.append('file_type', document.getElementById('fileType').value);
    fd.append('template_name', document.getElementById('fileTemplateName').value.trim());
    fd.append('name', document.getElementById('fileName').value.trim());
    fd.append('remark', document.getElementById('fileRemark').value.trim());
    try {
      await apiFetch('/api/files', { method: 'POST', body: fd });
      showToast(sourceUrl ? '远程文件已添加 (首次拉取完成)' : '分发文件上传成功');
      closeModal('fileModal');
      fetchFiles();
    } catch (err) {}
  });

  // --- 预览 Sing-Box 订阅配置 ---
  async function previewSubscriptionJson(token) {
    const jsonViewer = document.getElementById('configJsonViewer');
    jsonViewer.textContent = '正在从节点表与用户表动态拼接生成多节点配置文件...';
    openModal('configModal');

    try {
      const res = await fetch(`/sub?token=${token}`);
      if (!res.ok) throw new Error('Failed to fetch config');
      const data = await res.json();
      jsonViewer.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      jsonViewer.textContent = '获取配置文件失败，请检查用户是否已绑定有效的节点。';
    }
  }

  document.getElementById('copyConfigJsonBtn').addEventListener('click', () => {
    const text = document.getElementById('configJsonViewer').textContent;
    navigator.clipboard.writeText(text);
    showToast('Sing-Box 配置 JSON 已复制到剪贴板！');
  });

  // --- 初始化加载 ---
  async function loadDashboardData() {
    await fetchNodes();
    await fetchUsers();
    await fetchFiles();
  }

  loadDashboardData();
});
