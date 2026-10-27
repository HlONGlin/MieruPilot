// 增强功能：证书管理、节点测试、配置查看

// 上传证书（增强版）
async function uploadCertEnhanced(nodeId, portId) {
  const p = (state.node?.ports || []).find((x) => x.id === portId);
  if (!p) return;

  // 获取当前证书信息
  let certInfo = null;
  try {
    const info = await api("GET", `/api/nodes/${nodeId}/ports/${portId}/cert`);
    certInfo = info.certInfo;
  } catch (e) {
    console.log("无法获取证书信息:", e);
  }

  const certInfoHTML = certInfo
    ? `<div class="cert-info">
        <h4>当前证书信息</h4>
        <p><strong>主题:</strong> ${esc(certInfo.subject)}</p>
        <p><strong>颁发者:</strong> ${esc(certInfo.issuer)}</p>
        <p><strong>有效期:</strong> ${new Date(certInfo.notBefore).toLocaleDateString()} - ${new Date(certInfo.notAfter).toLocaleDateString()}</p>
        <p><strong>剩余天数:</strong> <span class="${certInfo.daysToExpire < 7 ? "error" : ""}">${certInfo.daysToExpire} 天</span></p>
        ${certInfo.dnsNames?.length ? `<p><strong>域名:</strong> ${certInfo.dnsNames.join(", ")}</p>` : ""}
      </div>`
    : "";

  modal(
    "TLS 证书配置",
    `<div class="field">
      <label>域名</label>
      <input name="domain" value="${esc(p.domain || "")}" placeholder="example.com" required>
      <small class="muted">NaïveProxy 需要使用域名访问</small>
    </div>
    <div class="field">
      <label>
        <input type="checkbox" name="autoACME" ${p.autoACME ? "checked" : ""}>
        自动申请和续期证书（Let's Encrypt ACME）
      </label>
      <small class="muted">需要域名已解析到节点IP，且80/443端口可访问</small>
    </div>
    <div id="manualCertFields" style="display:${p.autoACME ? "none" : "block"}">
      ${certInfoHTML}
      <div class="field">
        <label>TLS 证书（PEM格式）</label>
        <textarea name="cert" rows="8" placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----">${esc(p.tlsCert || "")}</textarea>
        <small class="muted">支持证书链，可包含中间证书</small>
      </div>
      <div class="field">
        <label>TLS 私钥（PEM格式）</label>
        <textarea name="key" rows="8" placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----">${esc(p.tlsKey || "")}</textarea>
        <small class="muted">支持 RSA 和 ECDSA 私钥</small>
      </div>
    </div>
    <script>
      document.querySelector('[name="autoACME"]').onchange = function() {
        document.getElementById('manualCertFields').style.display = this.checked ? 'none' : 'block';
      };
    </script>`,
    async (formData) => {
      const autoACME = formData.get("autoACME") === "on";
      const payload = {
        autoACME: autoACME,
        cert: autoACME ? "" : formData.get("cert").trim(),
        key: autoACME ? "" : formData.get("key").trim(),
      };

      try {
        const result = await api("POST", `/api/nodes/${nodeId}/ports/${portId}/cert`, payload);

        if (result.certInfo) {
          toast(
            `证书配置成功！有效期至 ${new Date(result.certInfo.notAfter).toLocaleDateString()}`,
            "success",
          );
        } else {
          toast(autoACME ? "已启用自动证书申请" : "证书配置已更新", "success");
        }

        // 同时更新域名
        await api("PATCH", `/api/nodes/${nodeId}/ports/${portId}`, {
          domain: formData.get("domain"),
        });

        renderDetail();
      } catch (e) {
        toast("证书配置失败: " + e.message, "error");
      }
    },
  );
}

// 测试节点连通性
async function testNode(nodeId) {
  const btn = event.target;
  btn.disabled = true;
  btn.textContent = "测试中...";

  try {
    const result = await api("POST", `/api/nodes/${nodeId}/test`);

    const errors = result.errors || [];
    const success = result.success;

    let message = `<h3>${result.status === 'unknown' ? '⚠️ 连通性待验证' : success ? '✅ 端口连通检查通过' : '❌ 连通性检查失败'}</h3>`;

    if (result.pingLatency > 0) {
      message += `<p>⚡ Ping延迟: <strong>${result.pingLatency}ms</strong></p>`;
    }

    const portResults = result.portResults || [];
    if (portResults.some((port) => port.tcpChecked)) {
      message += `<p>🔌 TCP连通: ${result.tcpConnectable ? "✅ 至少一个端口可达" : "❌ 不可达"}</p>`;
    }
    for (const port of portResults) {
      if (port.udpStatus === 'unknown') message += `<p>端口 ${esc(port.port)}：UDP 待协议客户端验证</p>`;
    }
    for (const warning of result.warnings || []) message += `<p class="muted">${esc(warning)}</p>`;

    if (errors.length > 0) {
      message += `<div class="error"><h4>错误信息:</h4><ul>`;
      errors.forEach((err) => {
        message += `<li>${esc(err)}</li>`;
      });
      message += `</ul></div>`;
    }

    if (result.details) {
      message += `<details><summary>详细信息</summary><pre>${JSON.stringify(result.details, null, 2)}</pre></details>`;
    }

    modal("节点测试结果", message);
  } catch (e) {
    toast("测试失败: " + e.message, "error");
  } finally {
    btn.disabled = false;
    btn.textContent = "测试连通性";
  }
}

// 测试端口
async function testPort(nodeId, portId) {
  const btn = event.target;
  btn.disabled = true;
  btn.textContent = "测试中...";

  try {
    const result = await api("POST", `/api/nodes/${nodeId}/ports/${portId}/test`);

    const errors = result.errors || [];
    const hasErrors = errors.length > 0;

    let message = `<h3>${hasErrors || result.status === 'unknown' ? "⚠️" : "✅"} 端口 ${result.port} 连通性检查</h3>`;
    message += `<p><strong>代理类型:</strong> ${esc(result.proxyType)}</p>`;
    if (result.tcpChecked) message += `<p>🔌 TCP连通: ${result.tcpConnectable ? "✅" : "❌"}</p>`;

    if (result.udpStatus) {
      message += `<p>📡 UDP可达性：待协议客户端验证</p>`;
    }
    for (const warning of result.warnings || []) message += `<p class="muted">${esc(warning)}</p>`;
    if (result.serviceStatusKnown) message += `<p>Agent 上报服务状态：${result.serviceRunning ? '运行中' : '未运行'}</p>`;

    if (result.tlsValid) {
      const tls = result.tlsValid;
      message += `<h4>🔒 TLS证书</h4>`;
      message += `<p>状态: ${tls.valid ? "✅ 有效" : "❌ 无效"}</p>`;
      if (tls.subject) {
        message += `<p>主题: ${esc(tls.subject)}</p>`;
        message += `<p>颁发者: ${esc(tls.issuer)}</p>`;
        message += `<p>有效期: ${new Date(tls.notBefore).toLocaleDateString()} - ${new Date(tls.notAfter).toLocaleDateString()}</p>`;
        message += `<p>剩余: <span class="${tls.daysToExpire < 7 ? "error" : ""}">${tls.daysToExpire} 天</span></p>`;
      }
      if (tls.error) {
        message += `<p class="error">${esc(tls.error)}</p>`;
      }
    }

    if (errors.length > 0) {
      message += `<div class="error"><h4>错误:</h4><ul>`;
      errors.forEach((err) => {
        message += `<li>${esc(err)}</li>`;
      });
      message += `</ul></div>`;
    }

    modal("端口测试结果", message);
  } catch (e) {
    toast("测试失败: " + e.message, "error");
  } finally {
    btn.disabled = false;
    btn.textContent = "测试端口";
  }
}

// 查看节点详细配置
async function viewNodeConfig(nodeId) {
  try {
    const config = await api("GET", `/api/nodes/${nodeId}/config`);

    let html = `<h3>📋 节点详细配置</h3>`;

    // 节点信息
    const node = config.nodeInfo;
    html += `<h4>节点信息</h4>`;
    html += `<table class="config-table">`;
    html += `<tr><td>ID</td><td><code>${esc(node.id)}</code></td></tr>`;
    html += `<tr><td>名称</td><td>${esc(node.name)}</td></tr>`;
    html += `<tr><td>地址</td><td><code>${esc(node.address)}</code></td></tr>`;
    if (node.domain) html += `<tr><td>域名</td><td><code>${esc(node.domain)}</code></td></tr>`;
    html += `<tr><td>系统</td><td>${esc(node.os)} / ${esc(node.arch)}</td></tr>`;
    html += `<tr><td>在线</td><td>${node.online ? "✅ 在线" : "❌ 离线"}</td></tr>`;
    html += `<tr><td>最后心跳</td><td>${new Date(node.lastSeen).toLocaleString()}</td></tr>`;
    html += `</table>`;

    // 端口配置
    html += `<h4>端口配置 (${config.ports.length})</h4>`;
    config.ports.forEach((p) => {
      const statusIcon = p.enabled ? "🟢" : "🔴";
      html += `<details open><summary>${statusIcon} 端口 ${p.port} - ${esc(p.proxyType)}</summary>`;
      html += `<table class="config-table">`;
      html += `<tr><td>协议</td><td>${esc(p.protocol)}</td></tr>`;
      html += `<tr><td>类型</td><td>${esc(p.proxyType)}</td></tr>`;
      html += `<tr><td>状态</td><td>${p.enabled ? "✅ 启用" : "🔴 停用"}</td></tr>`;
      html += `<tr><td>用户名</td><td><code>${esc(p.username)}</code></td></tr>`;
      if (p.label) html += `<tr><td>标签</td><td>${esc(p.label)}</td></tr>`;

      // 流量信息
      const usedGB = (p.traffic.used / 1073741824).toFixed(2);
      const limitGB = p.traffic.monthlyLimit > 0 ? (p.traffic.monthlyLimit / 1073741824).toFixed(2) : "∞";
      html += `<tr><td>流量</td><td>${usedGB} GB / ${limitGB} GB</td></tr>`;
      if (p.autoDisabled) html += `<tr><td>⚠️</td><td>已因超流量停用</td></tr>`;

      // 代理特定配置
      if (p.naive) {
        html += `<tr><td colspan="2"><strong>NaïveProxy 配置</strong></td></tr>`;
        html += `<tr><td>域名</td><td><code>${esc(p.naive.domain || "未设置")}</code></td></tr>`;
        html += `<tr><td>证书</td><td>${p.naive.autoACME ? "🔄 自动(ACME)" : p.naive.hasCert ? "✅ 已上传" : "❌ 未配置"}</td></tr>`;
      }
      if (p.shadowtls) {
        html += `<tr><td colspan="2"><strong>ShadowTLS 配置</strong></td></tr>`;
        html += `<tr><td>握手服务器</td><td><code>${esc(p.shadowtls.handshakeAddr)}</code></td></tr>`;
        html += `<tr><td>后端数据服务</td><td><code>${esc(p.shadowtls.backendAddr || "未配置")}</code> ${p.shadowtls.backendManaged ? "🔄 自动部署" : "✋ 手动配置"}</td></tr>`;
      }
      if (p.anytls) {
        html += `<tr><td colspan="2"><strong>AnyTLS 配置</strong></td></tr>`;
        html += `<tr><td>密码</td><td><code>${esc(p.anytls.password.substring(0, 8))}...</code></td></tr>`;
      }

      if (p.error) {
        html += `<tr><td colspan="2" class="error">错误: ${esc(p.error)}</td></tr>`;
      }

      html += `</table></details>`;
    });

    // 系统信息
    const sys = config.system;
    html += `<h4>系统信息</h4>`;
    html += `<table class="config-table">`;
    html += `<tr><td>Agent 版本</td><td>${esc(sys.agentVersion || "未知")}</td></tr>`;
    html += `<tr><td>Mita 版本</td><td>${esc(sys.mitaVersion || "未安装")}</td></tr>`;
    html += `<tr><td>Mita 运行</td><td>${sys.mitaRunning ? "✅ 运行中" : "❌ 未运行"}</td></tr>`;
    html += `</table>`;

    // 出站配置
    if (config.egress) {
      html += `<h4>出站配置</h4>`;
      html += `<table class="config-table">`;
      html += `<tr><td>类型</td><td>${esc(config.egress.type)}</td></tr>`;
      html += `<tr><td>服务器</td><td><code>${esc(config.egress.server)}:${config.egress.port}</code></td></tr>`;
      html += `<tr><td>用户名</td><td><code>${esc(config.egress.username)}</code></td></tr>`;
      html += `</table>`;
    }

    modal("节点配置", html, null, { wide: true });
  } catch (e) {
    toast("获取配置失败: " + e.message, "error");
  }
}

// 辅助函数：HTML转义
function esc(str) {
  if (str === null || str === undefined) return "";
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
