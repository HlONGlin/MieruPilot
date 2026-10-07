
      const app = document.getElementById("app"),
        modalEl = document.getElementById("modal"),
        toastEl = document.getElementById("toast");
      const initialTheme = localStorage.getItem("merit-theme") || "light";
      applyTheme(initialTheme);
      const base = location.pathname.replace(/\/+$/, "");
      const state = {
        me: { authenticated: false },
        view: "nodes",
        nodes: [],
        node: null,
        links: [],
        install: null,
        settings: null,
        loading: false,
      };
      const icon = {
        grid: '<svg viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>',
        settings:
          '<svg viewBox="0 0 24 24"><path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-1.7 1.7-.06-.06a1.7 1.7 0 0 0-1.88-.34 1.7 1.7 0 0 0-1.03 1.56V20h-2.4v-.2a1.7 1.7 0 0 0-1.03-1.56 1.7 1.7 0 0 0-1.88.34l-.06.06-1.7-1.7.06-.06A1.7 1.7 0 0 0 8.46 15a1.7 1.7 0 0 0-1.56-1.03h-.2v-2.4h.2A1.7 1.7 0 0 0 8.46 10a1.7 1.7 0 0 0-.34-1.88l-.06-.06 1.7-1.7.06.06a1.7 1.7 0 0 0 1.88.34 1.7 1.7 0 0 0 1.03-1.56V5h2.4v.2a1.7 1.7 0 0 0 1.03 1.56 1.7 1.7 0 0 0 1.88-.34l.06-.06 1.7 1.7-.06.06A1.7 1.7 0 0 0 19.54 10a1.7 1.7 0 0 0 1.56 1.03h.2v2.4h-.2A1.7 1.7 0 0 0 19.4 15Z"/></svg>',
        logout: "↪",
      };
      function esc(v) {
        return String(v ?? "").replace(
          /[&<>"']/g,
          (c) =>
            ({
              "&": "&amp;",
              "<": "&lt;",
              ">": "&gt;",
              '"': "&quot;",
              "'": "&#39;",
            })[c],
        );
      }
      function arrayOf(v) {
        return Array.isArray(v) ? v : [];
      }
      function applyTheme(theme) {
        const dark = theme === "dark";
        document.documentElement.classList.toggle("theme-dark", dark);
        document.documentElement.classList.toggle("theme-light", !dark);
      }
      function logo() {
        return '<span class="logo-mark">M</span>';
      }
      function url(path) {
        return base + path;
      }
      function toast(msg) {
        toastEl.textContent = msg;
        toastEl.classList.add("show");
        clearTimeout(toastEl.t);
        toastEl.t = setTimeout(() => toastEl.classList.remove("show"), 2200);
      }
      function toggleTheme(e) {
        e?.preventDefault();
        e?.stopPropagation();
        const next = document.documentElement.classList.contains("theme-dark")
          ? "light"
          : "dark";
        applyTheme(next);
        localStorage.setItem("merit-theme", next);
      }
      async function api(method, path, body) {
        const opt = { method, headers: {} };
        if (body !== undefined) {
          opt.headers["Content-Type"] = "application/json";
          opt.body = JSON.stringify(body);
        }
        const r = await fetch(url(path), opt);
        const ct = r.headers.get("content-type") || "";
        const data = ct.includes("json") ? await r.json() : await r.text();
        if (r.status === 401) {
          state.me.authenticated = false;
          render();
          throw Error("登录已失效");
        }
        if (!r.ok) {
          const e = Error(data?.error || data || `请求失败 (${r.status})`);
          e.status = r.status;
          throw e;
        }
        return data;
      }
      function copy(s) {
        if (navigator.clipboard && window.isSecureContext) {
          navigator.clipboard.writeText(s).then(
            () => toast("已复制到剪贴板"),
            () => fallbackCopy(s),
          );
        } else {
          fallbackCopy(s);
        }
      }
      function fallbackCopy(s) {
        const input = document.createElement("textarea");
        input.value = s;
        input.style.position = "fixed";
        input.style.opacity = "0";
        document.body.appendChild(input);
        input.focus();
        input.select();
        try {
          document.execCommand("copy");
          toast("已复制到剪贴板");
        } catch (e) {
          toast("复制失败，请手动复制");
        }
        input.remove();
      }
      function ago(ts) {
        if (!ts || ts.startsWith("0001")) return "从未";
        const d = Math.max(0, Math.floor((Date.now() - new Date(ts)) / 1000));
        return d < 60
          ? `${d} 秒前`
          : d < 3600
            ? `${Math.floor(d / 60)} 分钟前`
            : d < 86400
              ? `${Math.floor(d / 3600)} 小时前`
              : `${Math.floor(d / 86400)} 天前`;
      }
      function status(n) {
        return !n.registered
          ? ["未部署", ""]
          : n.online
            ? [n.mitaRunning ? "运行中" : "在线·未启动", "on"]
            : ["离线", ""];
      }
      function renderLogin() {
        app.innerHTML = `<main class="auth"><form class="auth-card" id="login"><div class="brand">${logo()}<span>merit<i>.</i></span></div><h1>登录控制台</h1><p>管理你的 mieru 节点和端口配置</p><div class="field"><label>管理员账号</label><input name="username" autocomplete="username" required autofocus></div><div class="field"><label>管理员密码</label><input name="password" type="password" autocomplete="current-password" required></div><div class="error" id="loginError"></div><button class="btn primary full" type="submit">登录控制台</button></form></main>`;
        document.getElementById("login").onsubmit = async (e) => {
          e.preventDefault();
          const f = new FormData(e.target);
          try {
            await api("POST", "/api/login", {
              username: f.get("username").trim(),
              password: f.get("password"),
            });
            state.me.authenticated = true;
            render();
          } catch (err) {
            document.getElementById("loginError").textContent = err.message;
          }
        };
      }
      function shell(title, content) {
        app.innerHTML = `<div class="layout"><aside class="sidebar"><div class="brand">${logo()}<span>merit<i>.</i></span></div><nav class="nav"><button type="button" class="${state.view === "nodes" ? "active" : ""}" data-nav="nodes">${icon.grid}<span>节点总览</span></button><button type="button" class="${state.view === "settings" ? "active" : ""}" data-nav="settings">${icon.settings}<span>系统设置</span></button></nav><div class="sidebar-bottom"><button type="button" data-logout>${icon.logout} <span>退出登录</span></button></div></aside><section class="main"><header class="topbar"><h1>${title}</h1><div class="top-actions"><span class="status-dot"></span><span class="muted">系统在线</span><button type="button" class="btn small" id="themeToggle">☼ / ☾</button><button type="button" class="btn small" data-refresh>刷新</button></div></header><main class="content">${content}</main></section></div>`;
      }
      function render() {
        if (!state.me.authenticated) return renderLogin();
        if (state.view === "detail") return renderDetail();
        if (state.view === "settings") return renderSettings();
        return renderNodes();
      }
      async function loadNodes() {
        state.nodes = await api("GET", "/api/nodes");
      }
      async function renderNodes() {
        shell(
          "节点总览",
          `<div class="page-head"><div><h2>节点总览</h2><p>集中管理所有主机与 mieru 端口</p></div><button class="btn primary" data-add-node>＋ 添加节点</button></div><div id="nodeBody"><div class="empty">加载中…</div></div>`,
        );
        try {
          await loadNodes();
          document.getElementById("nodeBody").innerHTML = nodesView();
          initNodeFilters();
        } catch (e) {
          document.getElementById("nodeBody").innerHTML =
            `<div class="empty">${esc(e.message)}</div>`;
        }
        bindGlobal();
      }
      function nodesView() {
        state.nodes = arrayOf(state.nodes);
        const total = state.nodes.length,
          online = state.nodes.filter((n) => n.online).length,
          ports = state.nodes.reduce(
            (a, n) => a + arrayOf(n.ports).filter((p) => p.enabled).length,
            0,
          ),
          running = state.nodes.filter((n) => n.mitaRunning).length;
        return `<div class="stats"><div class="stat"><div class="label">全部节点</div><strong>${total}</strong><em>已添加主机</em></div><div class="stat"><div class="label">在线节点</div><strong>${online}</strong><em>实时状态</em></div><div class="stat"><div class="label">运行中</div><strong>${running}</strong><em>mita 服务</em></div><div class="stat"><div class="label">启用端口</div><strong>${ports}</strong><em>全部节点合计</em></div></div>${total ? `<div class="node-grid">${arrayOf(state.nodes).map(nodeCard).join("")}</div>` : `<div class="empty"><div style="font-size:38px">⌁</div><h3>还没有节点</h3><p>添加一台主机后，就可以在这里集中管理。</p><button class="btn primary" data-add-node>添加第一个节点</button></div>`}`;
      }
      const nodeFilter = {query: '', status: 'all'};
      function filteredNodes() {
        const query = nodeFilter.query.trim().toLocaleLowerCase();
        return arrayOf(state.nodes).filter(node => {
          const matches = [node.name, node.address, node.domain, node.remark, node.os].join(' ').toLocaleLowerCase().includes(query);
          return matches && (nodeFilter.status === 'all' ||
            (nodeFilter.status === 'online' && node.online) ||
            (nodeFilter.status === 'offline' && node.registered && !node.online) ||
            (nodeFilter.status === 'new' && !node.registered));
        });
      }
      function bindNodeCards(root) {
        root.querySelectorAll('[data-node]').forEach(card => {
          const open = () => { state.node={id:card.dataset.node}; state.view='detail'; history.replaceState(null,'',base+'#/node/'+card.dataset.node); renderDetail(); };
          card.tabIndex = 0;
          card.setAttribute('role', 'button');
          card.setAttribute('aria-label', '管理节点 '+card.querySelector('strong').textContent);
          card.onclick = open;
          card.onkeydown = event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); open(); } };
        });
      }
      function initNodeFilters() {
        const grid = document.querySelector('.node-grid');
        if (!grid) return;
        const controls = document.createElement('div');
        controls.className = 'node-filters';
        controls.innerHTML = `<input type="search" id="nodeSearch" aria-label="搜索节点" placeholder="搜索节点名称、IP、域名或备注" value="${esc(nodeFilter.query)}"><select id="nodeStatus" aria-label="节点状态筛选"><option value="all">全部状态</option><option value="online">在线</option><option value="offline">离线</option><option value="new">未部署</option></select><span id="nodeCount" class="muted" aria-live="polite"></span>`;
        grid.before(controls);
        const search = controls.querySelector('#nodeSearch'), statusSelect = controls.querySelector('#nodeStatus');
        statusSelect.value = nodeFilter.status;
        const update = () => {
          nodeFilter.query = search.value;
          nodeFilter.status = statusSelect.value;
          const nodes = filteredNodes();
          grid.innerHTML = nodes.length ? nodes.map(nodeCard).join('') : '<div class="empty filter-empty">没有匹配的节点，试试其他关键词或状态。</div>';
          controls.querySelector('#nodeCount').textContent = `${nodes.length} / ${state.nodes.length} 个节点`;
          bindNodeCards(grid);
        };
        search.oninput = update;
        statusSelect.onchange = update;
        update();
      }
      function nodeCard(n) {
        const [st, cl] = status(n),
          enabled = (n.ports || []).filter((p) => p.enabled).length;
        return `<article class="node-card" data-node="${esc(n.id)}"><div class="node-top"><div class="node-icon">${esc((n.name || "N")[0].toUpperCase())}</div><div class="node-title"><strong>${esc(n.name)}</strong><small>${esc(n.address || n.seenIP || "等待 Agent 上线")}</small></div><span class="state ${cl}">${st}</span></div><div class="node-meta"><div>架构：${esc(n.arch || "—")}　系统：${esc(n.os || "—")}</div><div>端口：${enabled} 个启用　·　最后在线：${ago(n.lastSeen)}</div>${n.lastError ? `<div style="color:var(--red)">${esc(n.lastError)}</div>` : ""}</div></article>`;
      }
      function modal(title, body, onSubmit) {
        modalEl.innerHTML = `<div class="modal-back" data-modal-backdrop><form class="modal" id="modalForm"><h3>${title}</h3>${body}<div class="modal-foot"><button type="button" class="btn" data-modal-cancel>取消</button><button type="submit" class="btn primary">确认</button></div></form></div>`;
        const backdrop = modalEl.querySelector("[data-modal-backdrop]");
        const form = document.getElementById('modalForm');
        const previousFocus = document.activeElement;
        let submitting = false;
        form.setAttribute('role', 'dialog');
        form.setAttribute('aria-modal', 'true');
        form.setAttribute('aria-label', title);
        document.body.style.overflow = 'hidden';
        const errorEl = document.createElement('div');
        errorEl.className = 'modal-error';
        errorEl.setAttribute('role', 'alert');
        form.querySelector('.modal-foot').before(errorEl);
        const onKeydown = event => {
          if (event.key === 'Escape') { event.preventDefault(); close(); }
          if (event.key === 'Tab') {
            const focusable = Array.from(form.querySelectorAll('input,select,button,textarea')).filter(el => !el.disabled);
            const first = focusable[0], last = focusable[focusable.length - 1];
            if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
            else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
          }
        };
        const close = () => {
          if (submitting) return;
          document.removeEventListener('keydown', onKeydown);
          document.body.style.overflow = '';
          modalEl.innerHTML = "";
          if (previousFocus?.isConnected) previousFocus.focus();
        };
        document.addEventListener('keydown', onKeydown);
        form.querySelector('input,select,textarea')?.focus();
        backdrop.addEventListener("click", (e) => {
          if (e.target === backdrop) close();
        });
        modalEl
          .querySelector("[data-modal-cancel]")
          .addEventListener("click", close);
        document.getElementById("modalForm").onsubmit = async (e) => {
          e.preventDefault();
          if (submitting) return;
          submitting = true;
          errorEl.textContent = '';
          const submit = form.querySelector('[type="submit"]');
          const cancel = form.querySelector('[data-modal-cancel]');
          submit.disabled = cancel.disabled = true;
          submit.textContent = '保存中…';
          try {
            await onSubmit(new FormData(e.target));
            submitting = false;
            close();
          } catch (err) {
            errorEl.textContent = err.message;
          } finally {
            submitting = false;
            submit.disabled = cancel.disabled = false;
            submit.textContent = '确认';
          }
        };
      }
      function addNode() {
        modal(
          "添加节点",
          `<div class="field"><label>节点名称</label><input name="name" placeholder="例如 hk-1" required autofocus></div><div class="field"><label>备注（可选）</label><input name="remark" placeholder="线路或用途说明"></div>`,
          async (f) => {
            const n = await api("POST", "/api/nodes", {
              name: f.get("name"),
              remark: f.get("remark"),
            });
            state.view = "detail";
            state.node = n;
            renderDetail();
          },
        );
      }
      async function renderDetail() {
        shell("节点详情", '<div class="empty">加载中…</div>');
        try {
          const id = state.node?.id || location.hash.slice(7);
          const [rawNode, rawInstall, rawLinks] = await Promise.all([
            api("GET", "/api/nodes/" + id),
            api("GET", "/api/nodes/" + id + "/install"),
            api("GET", "/api/nodes/" + id + "/links"),
          ]);
          const n = rawNode && typeof rawNode === "object" ? rawNode : {};
          n.ports = arrayOf(n.ports);
          const install =
            rawInstall && typeof rawInstall === "object"
              ? rawInstall
              : { command: "" };
          const links = arrayOf(rawLinks);
          state.node = n;
          state.install = install;
          state.links = links;
          document.querySelector(".content").innerHTML = detailView(
            n,
            install,
            links,
          );
          renderEgressPanel(n);
          bindGlobal();
        } catch (e) {
          document.querySelector(".content").innerHTML =
            `<div class="empty">${esc(e.message || "加载节点详情失败")}</div>`;
          bindGlobal();
        }
      }
      function portStatus(p) {
        const status = !p.enabled
            ? "已停用"
            : p.instanceError
              ? "同步失败"
              : p.instanceRunning
                ? "运行中"
                : p.lastSyncAttempt
                  ? "未运行"
                  : "等待 Agent 上报",
          statusClass = !p.enabled
            ? "off"
            : p.instanceError
              ? "off"
              : p.instanceRunning
                ? "on"
                : "off",
          synced = p.instanceSyncedAt
            ? new Date(p.instanceSyncedAt).toLocaleString()
            : "暂无成功记录",
          attempt = p.lastSyncAttempt
            ? new Date(p.lastSyncAttempt).toLocaleString()
            : "尚未同步";
        return `<span class="pill ${statusClass}">${esc(status)}</span><div class="muted" title="${esc(p.instanceError || "上次尝试：" + attempt + "；最近成功：" + synced)}">${esc(p.instanceError || "最近成功：" + synced)}</div>`;
      }
      function addPort() {
        modal(
          "生成 mieru 端口",
          `<div class="field"><label>端口（1025-65535）</label><input name="port" type="number" min="1025" max="65535" placeholder="23456" required autofocus></div><div class="field"><label>协议</label><select name="protocol"><option>TCP</option><option>UDP</option></select></div><div class="field"><label>标签（可选）</label><input name="label" placeholder="例如 香港-1"></div>`,
          async (f) => {
            await api("POST", "/api/nodes/" + state.node.id + "/ports", {
              port: Number(f.get("port")),
              protocol: f.get("protocol"),
              label: f.get("label"),
            });
            toast("端口已生成并下发");
            renderDetail();
          },
        );
      }
      async function renderSettings() {
        shell("系统设置", '<div class="empty">加载中…</div>');
        try {
          const s = await api("GET", "/api/settings");
          state.settings = s;
          document.querySelector(".content").innerHTML =
            `<div class="page-head"><div><h2>系统设置</h2><p>管理订阅地址与系统信息</p></div></div><div class="panel"><div class="panel-head"><h3>公开订阅地址</h3><span class="pill">Token 保护</span></div><p class="muted">将此地址填入 Clash Verge、mihomo 等客户端，即可订阅全部已启用节点。</p><div class="command"><code>${esc(s.subscriptionURL)}</code><button class="btn" data-copy="${esc(s.subscriptionURL)}">复制地址</button><button class="btn danger" data-rotate>重置令牌</button></div></div><div class="panel"><div class="panel-head"><h3>使用流程</h3></div><ol class="muted"><li>添加节点，复制部署命令到目标服务器。</li><li>节点上线后生成端口，等待 Agent 同步配置。</li><li>复制 mierus 链接，或使用上方订阅地址导入客户端。</li></ol></div><div class="panel"><div class="panel-head"><h3>系统信息</h3></div><div class="muted">Manager 版本：${esc(s.version || "—")}<br>数据文件：服务端本地 JSON 文件</div></div>`;
          document.querySelector('.content').insertAdjacentHTML('beforeend', `<section class="panel"><h3>备份与恢复</h3><p class="muted">备份包含节点账号、出站凭据与订阅令牌，请妥善保存。恢复会替换节点配置，保留当前管理员与管理地址，并自动同步节点。</p><div class="toolbar"><a class="btn" href="${url('/api/backup')}">导出备份</a><label class="btn">选择备份文件<input id="backupFile" type="file" accept=".json,application/json" hidden></label></div><p id="backupResult" class="muted"></p></section>`);
          document.getElementById('backupFile').onchange = async event => {
            const file = event.target.files[0];
            if (!file) return;
            const feedback = document.getElementById('backupResult');
            try {
              if (file.size > 16 * 1024 * 1024) throw new Error('备份文件不能超过 16MB');
              const data = JSON.parse(await file.text());
              if (!Array.isArray(data.nodes)) throw new Error('备份缺少 nodes 数组');
              if (!confirm(`恢复 ${data.nodes.length} 个节点？当前节点配置将被替换，服务端会先创建恢复前备份。`)) return;
              feedback.textContent = '正在恢复…';
              const result = await api('POST', '/api/backup/restore', data);
              feedback.textContent = result.message;
            } catch (error) { feedback.textContent = error.message; }
            finally { event.target.value = ''; }
          };
          bindGlobal();
        } catch (e) {
          document.querySelector(".content").innerHTML =
            `<div class="empty">${esc(e.message)}</div>`;
          bindGlobal();
        }
      }
      function bindGlobal() {
        document.querySelectorAll("[data-nav]").forEach(
          (b) =>
            (b.onclick = () => {
              state.view = b.dataset.nav;
              history.replaceState(null, "", base);
              render();
            }),
        );
        document
          .querySelector("[data-logout]")
          ?.addEventListener("click", async () => {
            await api("POST", "/api/logout");
            state.me.authenticated = false;
            render();
          });
        document
          .getElementById("themeToggle")
          ?.addEventListener("click", toggleTheme);
        document
          .querySelector("[data-refresh]")
          ?.addEventListener("click", () =>
            state.view === "detail"
              ? renderDetail()
              : state.view === "settings"
                ? renderSettings()
                : renderNodes(),
          );
        document
          .querySelector("[data-add-node]")
          ?.addEventListener("click", addNode);
        document.querySelector("[data-back]")?.addEventListener("click", () => {
          state.view = "nodes";
          state.node = null;
          render();
        });
        document
          .querySelector("[data-add-port]")
          ?.addEventListener("click", addPort);
        document
          .querySelector("[data-edit-node]")
          ?.addEventListener("click", editNode);
        document
          .querySelector("[data-delete-node]")
          ?.addEventListener("click", deleteNode);
        document
          .querySelector("[data-save-node]")
          ?.addEventListener("click", saveNode);
        document
          .querySelector("[data-download-clash]")
          ?.addEventListener("click", downloadClash);
        document
          .querySelector("[data-rotate]")
          ?.addEventListener("click", rotateToken);
        document.querySelectorAll("[data-node]").forEach(
          (x) =>
            (x.onclick = () => {
              state.node = { id: x.dataset.node };
              state.view = "detail";
              history.replaceState(null, "", base + "#/node/" + x.dataset.node);
              renderDetail();
            }),
        );
        document
          .querySelectorAll("[data-copy]")
          .forEach((x) => (x.onclick = () => copy(x.dataset.copy)));
        document
          .querySelectorAll("[data-toggle-port]")
          .forEach((x) => (x.onclick = () => togglePort(x.dataset.togglePort)));
        document
          .querySelectorAll("[data-regenerate]")
          .forEach((x) => (x.onclick = () => regenerate(x.dataset.regenerate)));
        document
          .querySelectorAll("[data-delete-port]")
          .forEach((x) => (x.onclick = () => deletePort(x.dataset.deletePort)));
        document
          .querySelectorAll("[data-retry-port]")
          .forEach((x) => (x.onclick = () => retryPort(x.dataset.retryPort)));
      }
      function editNode() {
        const n = state.node;
        modal(
          "编辑节点信息",
          `<div class="field"><label>节点名称</label><input name="name" value="${esc(n.name)}" required></div><div class="field"><label>备注</label><input name="remark" value="${esc(n.remark)}"></div>`,
          async (f) => {
            await api("PATCH", "/api/nodes/" + n.id, {
              name: f.get("name"),
              remark: f.get("remark"),
            });
            toast("已保存");
            renderDetail();
          },
        );
      }
      async function saveNode() {
        await api("PATCH", "/api/nodes/" + state.node.id, {
          address: document.getElementById("address").value.trim(),
          domain: document.getElementById("domain").value.trim(),
        });
        toast("节点信息已保存");
        renderDetail();
      }
      async function deleteNode() {
        if (!confirm("确定删除此节点及其所有端口？")) return;
        await api("DELETE", "/api/nodes/" + state.node.id);
        state.view = "nodes";
        state.node = null;
        render();
      }
      async function togglePort(id) {
        const p = state.node.ports.find((x) => x.id === id);
        await api("PATCH", "/api/nodes/" + state.node.id + "/ports/" + id, {
          enabled: !p.enabled,
        });
        renderDetail();
      }
      async function regenerate(id) {
        if (!confirm("重置后旧分享链接立即失效，确定继续？")) return;
        await api("PATCH", "/api/nodes/" + state.node.id + "/ports/" + id, {
          regenerate: true,
        });
        toast("账号已重置");
        renderDetail();
      }
      async function deletePort(id) {
        if (!confirm("确定删除这个端口？")) return;
        await api("DELETE", "/api/nodes/" + state.node.id + "/ports/" + id);
        renderDetail();
      }
      async function retryPort(id) {
        try {
          await api(
            "POST",
            "/api/nodes/" + state.node.id + "/ports/" + id + "/retry",
          );
          toast("已加入该端口同步队列");
          setTimeout(renderDetail, 1200);
        } catch (e) {
          toast(e.message);
        }
      }
      async function downloadClash() {
        const r = await fetch(
          url("/api/nodes/" + state.node.id + "/clash.yaml"),
        );
        const b = await r.blob();
        const a = document.createElement("a");
        a.href = URL.createObjectURL(b);
        a.download = (state.node.name || "node") + ".yaml";
        a.click();
        URL.revokeObjectURL(a.href);
      }
      async function rotateToken() {
        if (!confirm("重置后旧订阅地址将失效，确定？")) return;
        await api("POST", "/api/settings/sub-token/rotate");
        toast("订阅令牌已重置");
        renderSettings();
      }
      function portRow(p, link, nodeID) {
        const clashURL =
            location.origin + url("/api/nodes/" + nodeID + "/clash.yaml"),
          detailID = "port-detail-" + p.id;
        return `<tr><td><code>${p.port}</code></td><td><span class="pill">${esc(p.protocol)}</span></td><td>${esc(p.label || "—")}</td><td><code>${esc(p.username)}</code></td><td>${portStatus(p)}</td><td><div class="toolbar">${link ? `<button type="button" class="btn small" data-copy="${esc(link)}">复制 Mieru</button>` : ""}<button type="button" class="btn small" data-retry-port="${esc(p.id)}" ${p.enabled ? "" : "disabled"}>重试同步</button><button type="button" class="btn small" data-expand-port="${p.id}" data-target="${detailID}">展开</button><button type="button" class="btn small" data-toggle-port="${p.id}">${p.enabled ? "停用" : "启用"}</button><button type="button" class="btn small" data-regenerate="${p.id}">重置账号</button><button type="button" class="btn small danger" data-delete-port="${p.id}">删除</button></div></td></tr><tr id="${detailID}" class="port-detail-row"><td colspan="6" class="port-detail"><div class="port-detail-title">原生 Mieru 链接</div><div class="command"><code>${esc(link || "节点地址未设置，暂时无法生成链接")}</code>${link ? `<button type="button" class="btn small" data-copy="${esc(link)}">复制 Mieru</button>` : ""}</div><div class="port-detail-title" style="margin-top:12px">Clash / mihomo 单节点 YAML</div><div class="command"><code>${esc(clashURL)}</code><button type="button" class="btn small" data-copy="${esc(clashURL)}">复制地址</button><button type="button" class="btn small" data-download-clash>下载 YAML</button></div></td></tr>`;
      }
      function detailView(n, install, links) {
        n = n && typeof n === "object" ? n : {};
        install = install && typeof install === "object" ? install : {};
        const safeLinks = arrayOf(links),
          ports = arrayOf(n.ports),
          [st, cl] = status(n),
          map = Object.fromEntries(
            arrayOf(safeLinks).map((x) => [x?.id, x?.link]),
          );
        return `<button type="button" class="back" data-back>← 返回节点总览</button><div class="page-head"><div><h2>${esc(n.name)} <span class="state ${cl}">${st}</span></h2><p>${esc(n.address || n.seenIP || "等待 Agent 上线")} · 最后在线 ${ago(n.lastSeen)}</p></div><div class="toolbar"><button type="button" class="btn" data-edit-node>编辑信息</button><button type="button" class="btn danger" data-delete-node>删除节点</button></div></div><div class="panel"><div class="panel-head"><h3>① 部署 Agent</h3><span class="muted">在目标服务器以 root 执行</span></div><div class="command"><code>${esc(install.command || "部署命令暂不可用")}</code><button type="button" class="btn" data-copy="${esc(install.command || "")}">复制命令</button></div><div class="form-row" style="margin-top:16px"><div class="field"><label>公网 IP / 地址</label><input id="address" value="${esc(n.address)}" placeholder="${esc(n.seenIP || "自动探测")}"></div><div class="field"><label>域名（可选）</label><input id="domain" value="${esc(n.domain)}" placeholder="proxy.example.com"></div><button type="button" class="btn primary" data-save-node>保存节点信息</button></div><p class="muted" style="font-size:12px;margin:12px 0 0">系统：${esc(n.os || "—")} · 架构：${esc(n.arch || "—")} · mita：${esc(n.mitaVer || "—")} · Agent：${esc(n.agentVer || "—")}</p></div><div class="panel"><div class="panel-head"><h3>② 端口与账号</h3><span class="muted">点击“展开”查看该端口的 Mieru 和 Clash 操作</span><button type="button" class="btn primary" data-add-port>＋ 生成端口</button></div>${
          ports.length
            ? `<div class="table-wrap"><table><thead><tr><th>端口</th><th>协议</th><th>标签</th><th>账号</th><th>状态</th><th>操作</th></tr></thead><tbody>${arrayOf(
                ports,
              )
                .map((p) => portRow(p, map[p?.id], n.id))
                .join("")}</tbody></table></div>`
            : '<div class="empty">还没有端口，点击右上角生成第一个端口。</div>'
        }</div>`;
      }
      function normalizeEgress(egress) {
        return {
          proxies: Array.isArray(egress?.proxies) ? egress.proxies : [],
          rules: Array.isArray(egress?.rules) ? egress.rules : [],
        };
      }
      function renderEgressPanel(n) {
        const e = normalizeEgress(n && n.egress),
          proxies = arrayOf(e.proxies),
          rules = arrayOf(e.rules);
        const proxyOptions = arrayOf(proxies)
          .map((p) => `<option value="${esc(p.name)}">${esc(p.name)}</option>`)
          .join("");
        const portOptions = arrayOf(n.ports)
          .map(
            (p) =>
              `<option value="${p.port}">${p.port} · ${esc(p.label || p.protocol)}</option>`,
          )
          .join("");
        document
          .querySelector(".content")
          .insertAdjacentHTML(
            "beforeend",
            `<div class="panel" id="egressPanel"><div class="panel-head"><div><h3>③ 出站管理</h3><span class="muted">落地机 SOCKS5 出站与连通性测试</span></div><button type="button" class="btn primary" data-add-egress>添加落地机</button></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>服务器</th><th>认证</th><th>状态</th><th>操作</th></tr></thead><tbody>${proxies.length ? proxies.map((p) => `<tr><td>${esc(p.name)}</td><td>${esc(p.host)}:${esc(p.port)}</td><td>${p.username ? "已配置" : "无认证"}</td><td>${p.enabled ? "启用" : "停用"}</td><td><div class="toolbar"><button type="button" class="btn small" data-edit-egress="${esc(p.id)}">编辑</button><button type="button" class="btn small" data-test-egress="${esc(p.id)}">测速</button><button type="button" class="btn small danger" data-remove-egress="${esc(p.id)}">删除</button></div></td></tr>`).join("") : '<tr><td colspan="5" class="muted">暂无落地机</td></tr>'}</tbody></table></div><p class="muted" style="font-size:12px">测速只验证 Manager 到落地机 SOCKS5 地址的 TCP 连通性。</p></div><div class="panel" id="routesPanel"><div class="panel-head"><div><h3>④ 路由规则</h3><span class="muted">按顺序匹配，第一条命中规则生效；不填域名和 IP 表示该端口全部目标</span></div><button type="button" class="btn primary" data-add-route>添加路由规则</button></div><div class="table-wrap"><table><thead><tr><th>顺序</th><th>规则名称</th><th>作用端口</th><th>匹配条件</th><th>动作</th><th>操作</th></tr></thead><tbody>${rules.length ? rules.map((r, i) => `<tr draggable="true" data-route-row="${esc(r.id)}"><td><span class="drag-handle">☷</span> ${i + 1}</td><td>${esc(r.name)}</td><td>${arrayOf(r.ports).length ? esc(r.ports.join(", ")) : "全部端口"}</td><td>${esc([...arrayOf(r.domainNames || r.Domains || r.domains), ...arrayOf(r.ipRanges || r.IPRanges)].join(", ") || "全部目标")}</td><td>${esc(r.action)}${arrayOf(r.proxyNames).length ? " → " + esc(r.proxyNames.join(", ")) : ""}</td><td><div class="toolbar"><button type="button" class="btn small" data-edit-route="${esc(r.id)}">编辑</button><button type="button" class="btn small danger" data-remove-route="${esc(r.id)}">删除</button></div></td></tr>`).join("") : '<tr><td colspan="6" class="muted">暂无路由规则，默认直连</td></tr>'}</tbody></table></div></div>`,
          );
        bindEgressEvents(n);
        bindRouteDrag(n);
      }
      function addEgress(n) {
        modal(
          "添加 SOCKS5 出站",
          `<div class="field"><label>名称</label><input name="name" required placeholder="proxy-jp"></div><div class="field"><label>地址</label><input name="host" required placeholder="127.0.0.1"></div><div class="field"><label>端口</label><input name="port" type="number" min="1" max="65535" required></div><div class="field"><label>用户名（可选）</label><input name="username"></div><div class="field"><label>密码（可选）</label><input name="password" type="password"></div>`,
          async (f) => {
            const e = normalizeEgress(n.egress);
            e.proxies.push({
              name: f.get("name"),
              protocol: "SOCKS5_PROXY_PROTOCOL",
              host: f.get("host"),
              port: Number(f.get("port")),
              username: f.get("username"),
              password: f.get("password"),
              enabled: true,
            });
            await saveEgress(n, e);
          },
        );
      }
      function editEgress(n, id) {
        const e = normalizeEgress(n.egress),
          p = e.proxies.find((x) => x.id === id);
        if (!p) return;
        modal(
          "编辑落地机 SOCKS5 出站",
          `<div class="field"><label>名称</label><input name="name" value="${esc(p.name)}" required></div><div class="field"><label>地址</label><input name="host" value="${esc(p.host)}" required></div><div class="field"><label>端口</label><input name="port" type="number" value="${p.port}" min="1" max="65535" required></div><div class="field"><label>用户名（可选）</label><input name="username" value="${esc(p.username || "")}"></div><div class="field"><label>密码（可选）</label><input name="password" type="password" placeholder="留空保留原密码"></div>`,
          async (f) => {
            p.name = f.get("name");
            p.host = f.get("host");
            p.port = Number(f.get("port"));
            p.username = f.get("username");
            if (f.get("password")) p.password = f.get("password");
            await saveEgress(n, e);
          },
        );
      }
      function addRoute(n, options, portOptions) {
        modal(
          "添加路由规则",
          `<div class="field"><label>规则名称</label><input name="name" required placeholder="日本服务"></div><div class="field"><label>作用端口</label><select name="ports" multiple size="3"><option value="">全部端口</option>${portOptions || ""}</select><small class="muted">不选择具体端口时作用于全部端口，可按住 Ctrl 多选。</small></div><div class="field"><label>域名（逗号分隔）</label><input name="domains" placeholder="example.com,*.example.com"></div><div class="field"><label>IP/CIDR（逗号分隔）</label><input name="ips" placeholder="8.8.8.8/32"></div><div class="field"><label>动作 / 出站</label><select name="action"><option value="DIRECT">DIRECT · 直连</option><option value="PROXY">PROXY · 选择出站</option><option value="REJECT">REJECT · 拒绝</option></select></div><div class="field"><label>指定出站（PROXY 必填）</label><select name="proxy"><option value="">请选择</option>${options}</select></div>`,
          async (f) => {
            const e = normalizeEgress(n.egress);
            const domains = String(f.get("domains"))
                .split(",")
                .map((x) => x.trim())
                .filter(Boolean),
              ips = String(f.get("ips"))
                .split(",")
                .map((x) => x.trim())
                .filter(Boolean),
              action = f.get("action"),
              ports = Array.from(f.getAll("ports")).map(Number).filter(Boolean);
            e.rules.push({
              name: f.get("name"),
              ports,
              domainNames: domains,
              ipRanges: ips,
              action,
              proxyNames: action === "PROXY" ? [f.get("proxy")] : [],
              enabled: true,
              order: e.rules.length,
            });
            await saveEgress(n, e);
          },
        );
      }
      function bindRouteDrag(n) {
        let dragging = null;
        document.querySelectorAll("[data-route-row]").forEach((row) => {
          row.ondragstart = () => {
            dragging = row;
            row.classList.add("route-dragging");
          };
          row.ondragend = () => {
            row.classList.remove("route-dragging");
            dragging = null;
          };
          row.ondragover = (e) => {
            e.preventDefault();
            if (dragging && dragging !== row) {
              const rect = row.getBoundingClientRect();
              row.parentNode.insertBefore(
                dragging,
                e.clientY < rect.top + rect.height / 2 ? row : row.nextSibling,
              );
            }
          };
          row.ondrop = async (e) => {
            e.preventDefault();
            const ids = Array.from(
                document.querySelectorAll("[data-route-row]"),
              ).map((x) => x.dataset.routeRow),
              cfg = normalizeEgress(n.egress);
            cfg.rules
              .sort((a, b) => ids.indexOf(a.id) - ids.indexOf(b.id))
              .forEach((r, i) => (r.order = i));
            await saveEgress(n, cfg);
          };
        });
      }
      async function testEgress(n, id) {
        const button = Array.from(document.querySelectorAll('[data-test-egress]')).find(b => b.dataset.testEgress === id);
        if (button?.disabled) return;
        let feedback = button?.closest('td')?.querySelector('[data-test-feedback]');
        if (!feedback && button) {
          feedback = document.createElement('div');
          feedback.dataset.testFeedback = '';
          feedback.className = 'muted';
          feedback.style.cssText = 'white-space:normal;font-size:12px;margin-top:8px';
          button.closest('td').appendChild(feedback);
        }
        const show = message => { if (feedback) feedback.textContent = message; };
        if (button) { button.disabled = true; button.textContent = '检测中…'; }
        try {
          let result = await api(
            "POST",
            "/api/nodes/" + n.id + "/egress/" + id + "/test",
          );
          const taskID = result.taskId;
          show('测试来源：'+n.name+' 节点 · 等待 Agent 执行');
          const deadline = Date.now() + 65000;
          while (result.state === 'pending' && Date.now() < deadline) {
            await new Promise(resolve => setTimeout(resolve, 1500));
            if (button && !button.isConnected) return;
            result = await api('GET', '/api/nodes/'+n.id+'/egress/'+id+'/test');
            if (result.taskId !== taskID) throw new Error('测试已被新任务替换，请重新检查');
          }
          const message = result.state === 'pending' ? '节点测试超时，请检查 Agent' : result.ok
            ? `节点检测成功 · ${result.latencyMs} ms · 出口 IP ${result.exitIP}`
            : `节点检测失败：${result.message}`;
          show(message);
          toast(message);
        } catch (e) {
          show(e.message);
          toast(e.message);
        } finally {
          if (button) { button.disabled = false; button.textContent = '节点检测'; }
        }
      }
      async function removeEgress(n, id) {
        const e = normalizeEgress(n.egress);
        e.proxies = e.proxies.filter((x) => x.id !== id);
        e.rules.forEach(
          (x) =>
            (x.proxyNames = (x.proxyNames || []).filter((name) =>
              e.proxies.some((p) => p.name === name),
            )),
        );
        await saveEgress(n, e);
      }
      async function removeRoute(n, id) {
        const e = normalizeEgress(n.egress);
        e.rules = e.rules.filter((x) => x.id !== id);
        await saveEgress(n, e);
      }
      function bindEgressEvents(n) {
        const addProxy = document.querySelector("[data-add-egress]");
        if (addProxy) addProxy.onclick = () => addEgress(n);
        const addRule = document.querySelector("[data-add-route]");
        if (addRule)
          addRule.onclick = () =>
            addRoute(
              n,
              arrayOf(n.egress?.proxies)
                .map(
                  (p) =>
                    `<option value="${esc(p.name)}">${esc(p.name)}</option>`,
                )
                .join(""),
              arrayOf(n.ports)
                .map(
                  (p) =>
                    `<option value="${p.port}">${p.port} · ${esc(p.label || p.protocol)}</option>`,
                )
                .join(""),
            );
        document
          .querySelectorAll("[data-edit-egress]")
          .forEach(
            (b) => (b.onclick = () => editEgress(n, b.dataset.editEgress)),
          );
        document
          .querySelectorAll("[data-remove-egress]")
          .forEach(
            (b) => (b.onclick = () => removeEgress(n, b.dataset.removeEgress)),
          );
        document.querySelectorAll("[data-remove-route]").forEach((b) => {
          if (!b.parentElement.querySelector("[data-edit-route]")) {
            const edit = document.createElement("button");
            edit.type = "button";
            edit.className = "btn small";
            edit.textContent = "编辑";
            edit.dataset.editRoute = b.dataset.removeRoute;
            edit.onclick = () => editRoute(n, b.dataset.removeRoute);
            b.parentElement.insertBefore(edit, b);
          }
          b.onclick = () => removeRoute(n, b.dataset.removeRoute);
        });
        document
          .querySelectorAll("[data-edit-route]")
          .forEach(
            (b) => (b.onclick = () => editRoute(n, b.dataset.editRoute)),
          );
        document
          .querySelectorAll("[data-test-egress]")
          .forEach(
            (b) => (b.onclick = () => testEgress(n, b.dataset.testEgress)),
          );
        bindRouteDrag(n);
      }
      const renderEgressPanelLegacy = renderEgressPanel;
      renderEgressPanel = function (n) {
        document.getElementById("egressRoot")?.remove();
        renderEgressPanelLegacy(n);
        const content = document.querySelector(".content"),
          egress = document.getElementById("egressPanel"),
          routes = document.getElementById("routesPanel");
        if (!content || !egress || !routes) return;
        const root = document.createElement("section");
        root.id = "egressRoot";
        egress.before(root);
        root.append(egress, routes);
      };
      async function saveEgress(n, cfg) {
        cfg = normalizeEgress(cfg);
        const updated = await api("PUT", "/api/nodes/" + n.id + "/egress", cfg);
        n.egress = normalizeEgress(
          updated && updated.egress ? updated.egress : cfg,
        );
        state.node = n;
        document.getElementById("egressRoot")?.remove();
        renderEgressPanel(n);
        bindEgressEvents(n);
        toast("配置已保存，等待 Agent 应用");
      }
      function editRoute(n, id) {
        const e = normalizeEgress(n.egress),
          r = e.rules.find((x) => x.id === id);
        if (!r) return;
        const proxyOptions = arrayOf(e.proxies)
          .map(
            (p) =>
              `<option value="${esc(p.name)}" ${arrayOf(r.proxyNames).includes(p.name) ? "selected" : ""}>${esc(p.name)}</option>`,
          )
          .join("");
        const portOptions = arrayOf(n.ports)
          .map(
            (p) =>
              `<option value="${p.port}" ${arrayOf(r.ports).includes(p.port) ? "selected" : ""}>${p.port} · ${esc(p.label || p.protocol)}</option>`,
          )
          .join("");
        modal(
          "编辑路由规则",
          `<div class="field"><label>规则名称</label><input name="name" value="${esc(r.name)}" required></div><div class="field"><label>作用端口</label><select name="ports" multiple size="3"><option value="">全部端口</option>${portOptions}</select></div><div class="field"><label>域名（逗号分隔）</label><input name="domains" value="${esc(arrayOf(r.domainNames).join(", "))}" placeholder="留空表示全部目标"></div><div class="field"><label>IP/CIDR（逗号分隔）</label><input name="ips" value="${esc(arrayOf(r.ipRanges).join(", "))}" placeholder="留空表示全部目标"></div><div class="field"><label>动作 / 出站</label><select name="action"><option value="DIRECT" ${r.action === "DIRECT" ? "selected" : ""}>DIRECT · 直连</option><option value="PROXY" ${r.action === "PROXY" ? "selected" : ""}>PROXY · 选择落地机</option><option value="REJECT" ${r.action === "REJECT" ? "selected" : ""}>REJECT · 拒绝</option></select></div><div class="field"><label>指定落地机（PROXY 必填）</label><select name="proxy"><option value="">请选择</option>${proxyOptions}</select></div>`,
          async (f) => {
            r.name = f.get("name");
            r.ports = Array.from(f.getAll("ports"))
              .map(Number)
              .filter(Number.isFinite)
              .filter(Boolean);
            r.domainNames = String(f.get("domains"))
              .split(",")
              .map((x) => x.trim())
              .filter(Boolean);
            r.ipRanges = String(f.get("ips"))
              .split(",")
              .map((x) => x.trim())
              .filter(Boolean);
            r.action = f.get("action");
            r.proxyNames = r.action === "PROXY" ? [f.get("proxy")] : [];
            await saveEgress(n, e);
          },
        );
      }
      const bindGlobalBase = bindGlobal;
      bindGlobal = function () {
        bindGlobalBase();
        document.querySelectorAll("[data-expand-port]").forEach(
          (b) =>
            (b.onclick = () => {
              const row = document.getElementById(b.dataset.target);
              if (!row) return;
              const open = row.classList.toggle("open");
              b.textContent = open ? "收起" : "展开";
              b.setAttribute('aria-expanded', String(open));
              b.setAttribute('aria-controls', row.id);
            }),
        );
        document
          .querySelectorAll("[data-download-clash]")
          .forEach((b) => (b.onclick = downloadClash));
        if (state.node) bindEgressEvents(state.node);
        document.querySelectorAll('[data-expand-port]').forEach(button => {
          button.setAttribute('aria-expanded', 'false');
          button.setAttribute('aria-controls', button.dataset.target);
        });
      };
      async function init() {
        try {
          state.me = await api("GET", "/api/me");
        } catch (e) {
          state.me = { authenticated: false };
        }
        render();
      }
      window.addEventListener("hashchange", () => {
        const h = location.hash;
        if (h.startsWith("#/node/")) {
          state.view = "detail";
          state.node = { id: h.slice(7) };
          renderDetail();
        } else {
          state.view = h.startsWith("#/settings") ? "settings" : "nodes";
          render();
        }
      });
      init();
    
