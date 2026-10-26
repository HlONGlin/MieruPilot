// ============================================
// MieruPilot 增强功能前端集成示例
// ============================================

// ============================================
// 1. 增强的证书上传功能
// ============================================

/**
 * 上传证书并显示验证结果
 */
async function uploadCertificate(nodeID, portNumber, certFile, keyFile) {
  const formData = new FormData();

  // 读取文件内容
  const certContent = await certFile.text();
  const keyContent = await keyFile.text();

  try {
    showLoading('正在上传和验证证书...');

    const response = await fetch(`/api/nodes/${nodeID}/ports/${portNumber}/cert`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        certificate: certContent,
        privateKey: keyContent
      })
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || '上传失败');
    }

    const result = await response.json();

    // 显示成功消息和证书信息
    showSuccess('证书上传成功！');
    displayCertificateInfo(result.certInfo);

    return result;
  } catch (error) {
    showError(`证书上传失败: ${error.message}`);
    throw error;
  } finally {
    hideLoading();
  }
}

/**
 * 显示证书信息
 */
function displayCertificateInfo(certInfo) {
  const html = `
    <div class="cert-info-card">
      <div class="cert-info-header">
        <i class="icon-certificate"></i>
        <h3>证书信息</h3>
      </div>

      <div class="cert-info-body">
        <!-- 域名列表 -->
        <div class="info-row">
          <label>域名:</label>
          <div class="domains">
            ${certInfo.domains.map(d => `<span class="domain-chip">${d}</span>`).join('')}
          </div>
        </div>

        <!-- 有效期 -->
        <div class="info-row">
          <label>有效期:</label>
          <div class="validity">
            <span class="date">${formatDate(certInfo.validFrom)}</span>
            <i class="icon-arrow-right"></i>
            <span class="date">${formatDate(certInfo.validUntil)}</span>
          </div>
        </div>

        <!-- 剩余天数 -->
        <div class="info-row">
          <label>剩余天数:</label>
          <div class="days-remaining ${getDaysRemainingClass(certInfo.daysRemaining)}">
            <span class="number">${certInfo.daysRemaining}</span>
            <span class="unit">天</span>
            ${certInfo.daysRemaining < 30 ? '<i class="icon-warning"></i>' : ''}
          </div>
        </div>

        <!-- 颁发者 -->
        <div class="info-row">
          <label>颁发者:</label>
          <span>${certInfo.issuer}</span>
        </div>

        <!-- 密钥信息 -->
        <div class="info-row">
          <label>密钥类型:</label>
          <span>${certInfo.keyType} ${certInfo.keySize} 位</span>
        </div>
      </div>
    </div>
  `;

  document.getElementById('cert-info-container').innerHTML = html;
}

/**
 * 查看现有证书信息
 */
async function viewCertificateInfo(nodeID, portNumber) {
  try {
    const response = await fetch(`/api/nodes/${nodeID}/ports/${portNumber}/cert/info`);

    if (!response.ok) {
      if (response.status === 404) {
        showInfo('该端口尚未配置证书');
        return null;
      }
      throw new Error('获取证书信息失败');
    }

    const certInfo = await response.json();
    displayCertificateInfo(certInfo);
    return certInfo;
  } catch (error) {
    showError(error.message);
    return null;
  }
}

/**
 * 证书上传对话框
 */
function showCertUploadDialog(nodeID, portNumber) {
  const dialog = `
    <div class="modal" id="cert-upload-modal">
      <div class="modal-content">
        <div class="modal-header">
          <h2>上传 TLS 证书</h2>
          <button class="close-btn" onclick="closeModal('cert-upload-modal')">&times;</button>
        </div>

        <div class="modal-body">
          <!-- 证书文件选择 -->
          <div class="form-group">
            <label for="cert-file">证书文件 (PEM 格式)</label>
            <input type="file" id="cert-file" accept=".pem,.crt,.cer" required>
            <small class="form-text">
              支持 .pem, .crt, .cer 格式，包含 -----BEGIN CERTIFICATE-----
            </small>
          </div>

          <!-- 私钥文件选择 -->
          <div class="form-group">
            <label for="key-file">私钥文件 (PEM 格式)</label>
            <input type="file" id="key-file" accept=".pem,.key" required>
            <small class="form-text">
              支持 .pem, .key 格式，包含 -----BEGIN PRIVATE KEY-----
            </small>
          </div>

          <!-- 或者粘贴内容 -->
          <div class="form-divider">或者</div>

          <div class="form-group">
            <label for="cert-text">粘贴证书内容</label>
            <textarea id="cert-text" rows="8" placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"></textarea>
          </div>

          <div class="form-group">
            <label for="key-text">粘贴私钥内容</label>
            <textarea id="key-text" rows="8" placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----"></textarea>
          </div>

          <!-- 验证结果显示区域 -->
          <div id="cert-validation-result" class="validation-result" style="display:none;"></div>

          <!-- 证书信息显示区域 -->
          <div id="cert-info-container"></div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick="closeModal('cert-upload-modal')">取消</button>
          <button class="btn btn-primary" onclick="handleCertUpload(${nodeID}, ${portNumber})">
            上传证书
          </button>
        </div>
      </div>
    </div>
  `;

  document.body.insertAdjacentHTML('beforeend', dialog);
  showModal('cert-upload-modal');
}

/**
 * 处理证书上传
 */
async function handleCertUpload(nodeID, portNumber) {
  const certFile = document.getElementById('cert-file').files[0];
  const keyFile = document.getElementById('key-file').files[0];
  const certText = document.getElementById('cert-text').value.trim();
  const keyText = document.getElementById('key-text').value.trim();

  let certContent, keyContent;

  // 优先使用文件，其次使用文本输入
  if (certFile && keyFile) {
    certContent = await certFile.text();
    keyContent = await keyFile.text();
  } else if (certText && keyText) {
    certContent = certText;
    keyContent = keyText;
  } else {
    showError('请选择证书和私钥文件，或粘贴证书和私钥内容');
    return;
  }

  try {
    const result = await uploadCertificate(nodeID, portNumber, {
      text: async () => certContent
    }, {
      text: async () => keyContent
    });

    // 上传成功后关闭对话框并刷新
    closeModal('cert-upload-modal');
    refreshNodeDetails(nodeID);
  } catch (error) {
    // 错误已在 uploadCertificate 中处理
  }
}

// ============================================
// 2. 节点测试功能
// ============================================

/**
 * 测试节点
 */
async function testNode(nodeID, testType = 'full') {
  try {
    showLoading(`正在测试节点... (${testType === 'quick' ? '快速模式' : '完整模式'})`);

    const response = await fetch(`/api/nodes/${nodeID}/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ testType })
    });

    if (!response.ok) {
      throw new Error('测试请求失败');
    }

    const result = await response.json();
    displayTestResults(result);

    return result;
  } catch (error) {
    showError(`节点测试失败: ${error.message}`);
    throw error;
  } finally {
    hideLoading();
  }
}

/**
 * 显示测试结果
 */
function displayTestResults(result) {
  const statusClass = {
    'success': 'test-success',
    'warning': 'test-warning',
    'error': 'test-error'
  }[result.overallStatus];

  const statusIcon = {
    'success': '✓',
    'warning': '⚠',
    'error': '✗'
  }[result.overallStatus];

  const html = `
    <div class="test-results-card ${statusClass}">
      <div class="test-header">
        <div class="test-status">
          <span class="status-icon">${statusIcon}</span>
          <h3>测试结果</h3>
        </div>
        <span class="test-time">${formatDateTime(result.testTime)}</span>
      </div>

      <!-- 测试摘要 -->
      <div class="test-summary">
        <p>${result.summary}</p>
      </div>

      <!-- 详细测试项 -->
      <div class="test-items">
        ${result.tests.map(test => `
          <div class="test-item ${test.status}">
            <div class="test-item-header">
              <span class="test-name">
                <i class="icon-${getTestIcon(test.status)}"></i>
                ${test.name}
              </span>
              ${test.latency > 0 ? `<span class="test-latency">${test.latency}ms</span>` : ''}
            </div>
            <div class="test-message">${test.message}</div>
          </div>
        `).join('')}
      </div>

      <!-- 警告和错误 -->
      ${result.warnings && result.warnings.length > 0 ? `
        <div class="test-warnings">
          <h4><i class="icon-warning"></i> 警告</h4>
          <ul>
            ${result.warnings.map(w => `<li>${w}</li>`).join('')}
          </ul>
        </div>
      ` : ''}

      ${result.errors && result.errors.length > 0 ? `
        <div class="test-errors">
          <h4><i class="icon-error"></i> 错误</h4>
          <ul>
            ${result.errors.map(e => `<li>${e}</li>`).join('')}
          </ul>
        </div>
      ` : ''}

      <!-- 操作按钮 -->
      <div class="test-actions">
        <button class="btn btn-secondary" onclick="testNode('${result.nodeID}', 'quick')">
          快速重测
        </button>
        <button class="btn btn-primary" onclick="testNode('${result.nodeID}', 'full')">
          完整重测
        </button>
        <button class="btn btn-secondary" onclick="viewNodeConfig('${result.nodeID}')">
          查看配置
        </button>
      </div>
    </div>
  `;

  // 显示在模态框或侧边栏
  showTestResultsModal(html);
}

/**
 * 显示测试结果模态框
 */
function showTestResultsModal(html) {
  const modal = `
    <div class="modal large" id="test-results-modal">
      <div class="modal-content">
        <div class="modal-header">
          <h2>节点测试报告</h2>
          <button class="close-btn" onclick="closeModal('test-results-modal')">&times;</button>
        </div>
        <div class="modal-body">
          ${html}
        </div>
      </div>
    </div>
  `;

  // 移除旧的模态框
  const oldModal = document.getElementById('test-results-modal');
  if (oldModal) oldModal.remove();

  document.body.insertAdjacentHTML('beforeend', modal);
  showModal('test-results-modal');
}

// ============================================
// 3. 详细配置信息查看
// ============================================

/**
 * 查看节点配置
 */
async function viewNodeConfig(nodeID, includeSensitive = false) {
  try {
    showLoading('正在加载配置...');

    const url = `/api/nodes/${nodeID}/config${includeSensitive ? '?includeSensitive=true' : ''}`;
    const response = await fetch(url);

    if (!response.ok) {
      throw new Error('获取配置失败');
    }

    const config = await response.json();
    displayNodeConfig(config, includeSensitive);

    return config;
  } catch (error) {
    showError(`加载配置失败: ${error.message}`);
    throw error;
  } finally {
    hideLoading();
  }
}

/**
 * 显示节点配置
 */
function displayNodeConfig(config, includeSensitive) {
  const html = `
    <div class="config-view">
      <!-- 基本信息 -->
      <section class="config-section">
        <h3><i class="icon-info"></i> 基本信息</h3>
        <div class="config-grid">
          <div class="config-item">
            <label>节点 ID:</label>
            <span class="config-value mono">${config.nodeID}</span>
          </div>
          <div class="config-item">
            <label>IP 地址:</label>
            <span class="config-value mono">${config.nodeIP}</span>
          </div>
          <div class="config-item">
            <label>平台:</label>
            <span class="config-value">${config.platform}</span>
          </div>
          <div class="config-item">
            <label>版本:</label>
            <span class="config-value">${config.version}</span>
          </div>
          <div class="config-item">
            <label>状态:</label>
            <span class="config-value status-${config.status}">${config.status}</span>
          </div>
        </div>
      </section>

      <!-- 端口配置 -->
      <section class="config-section">
        <h3><i class="icon-port"></i> 端口配置</h3>
        ${config.ports.map(port => `
          <div class="port-config-card">
            <div class="port-header">
              <h4>端口 ${port.portNumber}</h4>
              <span class="proxy-type-badge ${port.proxyType}">${port.proxyType}</span>
            </div>

            <div class="port-details">
              <div class="config-item">
                <label>协议:</label>
                <span>${port.protocol}</span>
              </div>

              ${port.tlsConfig ? `
                <div class="tls-config">
                  <h5><i class="icon-lock"></i> TLS 配置</h5>
                  <div class="config-item">
                    <label>服务器名称:</label>
                    <span class="mono">${port.tlsConfig.serverName}</span>
                  </div>
                  <div class="config-item">
                    <label>证书路径:</label>
                    <span class="mono">${port.tlsConfig.certPath}</span>
                  </div>
                  ${port.tlsConfig.certInfo ? `
                    <div class="cert-info-compact">
                      <span class="domains">${port.tlsConfig.certInfo.domains.join(', ')}</span>
                      <span class="expiry ${port.tlsConfig.certInfo.daysRemaining < 30 ? 'warning' : ''}">
                        ${port.tlsConfig.certInfo.daysRemaining} 天后到期
                      </span>
                    </div>
                  ` : ''}
                </div>
              ` : ''}

              ${port.users && port.users.length > 0 ? `
                <div class="users-config">
                  <h5><i class="icon-users"></i> 用户 (${port.users.length})</h5>
                  ${port.users.map((user, i) => `
                    <div class="user-item">
                      <span class="username">${user.username}</span>
                      <span class="password mono">${user.password}</span>
                    </div>
                  `).join('')}
                </div>
              ` : ''}
            </div>
          </div>
        `).join('')}
      </section>

      <!-- 出站配置 -->
      ${config.egress && (config.egress.proxies.length > 0 || config.egress.rules.length > 0) ? `
        <section class="config-section">
          <h3><i class="icon-network"></i> 出站配置</h3>

          ${config.egress.proxies.length > 0 ? `
            <div class="egress-proxies">
              <h4>出站代理</h4>
              ${config.egress.proxies.map(proxy => `
                <div class="egress-proxy-card">
                  <div class="proxy-header">
                    <span class="proxy-name">${proxy.name}</span>
                    <span class="proxy-status ${proxy.enabled ? 'enabled' : 'disabled'}">
                      ${proxy.enabled ? '已启用' : '已禁用'}
                    </span>
                  </div>
                  <div class="proxy-details">
                    <span class="mono">${proxy.protocol}://${proxy.host}:${proxy.port}</span>
                  </div>
                </div>
              `).join('')}
            </div>
          ` : ''}

          ${config.egress.rules.length > 0 ? `
            <div class="egress-rules">
              <h4>路由规则</h4>
              ${config.egress.rules.map(rule => `
                <div class="egress-rule-card">
                  <div class="rule-header">
                    <span class="rule-name">${rule.name}</span>
                    <span class="rule-action ${rule.action.toLowerCase()}">${rule.action}</span>
                  </div>
                </div>
              `).join('')}
            </div>
          ` : ''}
        </section>
      ` : ''}

      <!-- 操作按钮 -->
      <div class="config-actions">
        ${!includeSensitive ? `
          <button class="btn btn-warning" onclick="viewNodeConfig('${config.nodeID}', true)">
            <i class="icon-eye"></i> 显示敏感信息
          </button>
        ` : ''}
        <button class="btn btn-secondary" onclick="exportConfig('${config.nodeID}')">
          <i class="icon-download"></i> 导出配置
        </button>
        <button class="btn btn-secondary" onclick="copyConfig('${config.nodeID}')">
          <i class="icon-copy"></i> 复制配置
        </button>
      </div>
    </div>
  `;

  showConfigModal(html);
}

/**
 * 显示配置模态框
 */
function showConfigModal(html) {
  const modal = `
    <div class="modal large" id="config-view-modal">
      <div class="modal-content">
        <div class="modal-header">
          <h2>节点配置详情</h2>
          <button class="close-btn" onclick="closeModal('config-view-modal')">&times;</button>
        </div>
        <div class="modal-body">
          ${html}
        </div>
      </div>
    </div>
  `;

  const oldModal = document.getElementById('config-view-modal');
  if (oldModal) oldModal.remove();

  document.body.insertAdjacentHTML('beforeend', modal);
  showModal('config-view-modal');
}

/**
 * 导出配置为 JSON 文件
 */
async function exportConfig(nodeID) {
  try {
    const response = await fetch(`/api/nodes/${nodeID}/config`);
    const config = await response.json();

    const blob = new Blob([JSON.stringify(config, null, 2)], {
      type: 'application/json'
    });

    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `node-${nodeID}-config-${Date.now()}.json`;
    a.click();

    URL.revokeObjectURL(url);
    showSuccess('配置已导出');
  } catch (error) {
    showError('导出失败: ' + error.message);
  }
}

/**
 * 复制配置到剪贴板
 */
async function copyConfig(nodeID) {
  try {
    const response = await fetch(`/api/nodes/${nodeID}/config`);
    const config = await response.json();

    await navigator.clipboard.writeText(JSON.stringify(config, null, 2));
    showSuccess('配置已复制到剪贴板');
  } catch (error) {
    showError('复制失败: ' + error.message);
  }
}

// ============================================
// 工具函数
// ============================================

function formatDate(dateString) {
  const date = new Date(dateString);
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  });
}

function formatDateTime(dateString) {
  const date = new Date(dateString);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

function getDaysRemainingClass(days) {
  if (days < 7) return 'critical';
  if (days < 30) return 'warning';
  return 'normal';
}

function getTestIcon(status) {
  return {
    'success': 'check-circle',
    'warning': 'alert-triangle',
    'error': 'x-circle'
  }[status] || 'circle';
}

function showLoading(message) {
  // 实现加载提示
  console.log('Loading:', message);
}

function hideLoading() {
  // 隐藏加载提示
}

function showSuccess(message) {
  // 显示成功提示
  console.log('Success:', message);
}

function showError(message) {
  // 显示错误提示
  console.error('Error:', message);
}

function showInfo(message) {
  // 显示信息提示
  console.info('Info:', message);
}

function showModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.style.display = 'flex';
    document.body.style.overflow = 'hidden';
  }
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.style.display = 'none';
    document.body.style.overflow = '';
    modal.remove();
  }
}

function refreshNodeDetails(nodeID) {
  // 刷新节点详情页面
  console.log('Refreshing node:', nodeID);
}
