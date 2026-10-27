# 证书上传进度可视化实施方案

## 当前状态

**现有流程**:
1. 用户点击"上传证书"
2. 弹出模态框输入域名/证书/私钥
3. 点击确认后调用 API: `PATCH /api/nodes/:id/ports/:portId`
4. 显示 toast: "证书配置已更新"
5. 1.2 秒后刷新节点详情

**问题**:
- 无视觉反馈表示上传进度
- 不知道证书是否已同步到 Agent
- 不知道服务是否已重启
- 1.2 秒固定延迟可能不够(大证书/慢网络)

---

## 改进方案

### 阶段 1: 基础进度反馈 (v1.6.1)

**目标**: 让用户知道上传流程的每个步骤

**UI 设计**:

```
┌─────────────────────────────────────┐
│ 📤 上传证书                          │
│                                     │
│ 域名: example.com                   │
│                                     │
│ ✅ 1. 上传到 Manager                │
│ ⏳ 2. 验证证书格式...               │
│ ⏸  3. 等待同步到 Agent              │
│ ⏸  4. 重启 NaïveProxy               │
│                                     │
│ [取消]                              │
└─────────────────────────────────────┘
```

**实施步骤**:

#### 1. 前端:更新模态框添加进度区域

```javascript
// app.js: uploadCert 函数
async function uploadCert(id) {
  const p = (state.node?.ports || []).find((x) => x.id === id);
  if (!p) return;
  
  modal(
    "上传 TLS 证书",
    `<div class="field">
       <label>域名</label>
       <input name="domain" value="${esc(p.domain || '')}" placeholder="example.com" required>
     </div>
     <div class="field">
       <label><input type="checkbox" name="autoACME" ${p.autoACME ? 'checked' : ''}> 自动申请证书（ACME）</label>
     </div>
     <div id="manualCertFields" style="display:${p.autoACME ? 'none' : 'block'}">
       <div class="field">
         <label>TLS 证书</label>
         <textarea name="tlsCert" rows="6" placeholder="-----BEGIN CERTIFICATE-----">${esc(p.tlsCert || '')}</textarea>
       </div>
       <div class="field">
         <label>TLS 私钥</label>
         <textarea name="tlsKey" rows="6" placeholder="-----BEGIN PRIVATE KEY-----">${esc(p.tlsKey || '')}</textarea>
       </div>
     </div>
     
     <!-- 进度显示区域 (初始隐藏) -->
     <div id="uploadProgress" style="display:none; margin-top:16px; padding:12px; background:#f5f5f5; border-radius:4px;">
       <div class="progress-title">📤 上传进度</div>
       <div class="progress-steps">
         <div class="progress-step" data-step="upload">
           <span class="progress-icon">⏸</span>
           <span class="progress-text">上传到 Manager</span>
         </div>
         <div class="progress-step" data-step="validate">
           <span class="progress-icon">⏸</span>
           <span class="progress-text">验证证书格式</span>
         </div>
         <div class="progress-step" data-step="sync">
           <span class="progress-icon">⏸</span>
           <span class="progress-text">同步到 Agent</span>
         </div>
         <div class="progress-step" data-step="restart">
           <span class="progress-icon">⏸</span>
           <span class="progress-text">重启服务</span>
         </div>
       </div>
     </div>
    `,
    async (f) => {
      const autoACME = f.get("autoACME") === "on";
      const payload = {
        domain: f.get("domain"),
        autoACME: autoACME,
      };
      if (!autoACME) {
        payload.tlsCert = f.get("tlsCert");
        payload.tlsKey = f.get("tlsKey");
      }
      
      // 显示进度区域
      const progressEl = document.getElementById('uploadProgress');
      const submitBtn = modalEl.querySelector('button[type="submit"]');
      if (progressEl) {
        progressEl.style.display = 'block';
        submitBtn.disabled = true;
        submitBtn.textContent = '上传中...';
      }
      
      try {
        // 步骤 1: 上传到 Manager
        updateProgressStep('upload', 'loading', '上传到 Manager...');
        await api("PATCH", "/api/nodes/" + state.node.id + "/ports/" + id, payload);
        updateProgressStep('upload', 'success', '上传到 Manager');
        
        // 步骤 2: 验证证书 (API 内部完成)
        updateProgressStep('validate', 'success', '验证证书格式');
        
        // 步骤 3: 等待同步
        updateProgressStep('sync', 'loading', '同步到 Agent...');
        await pollSyncStatus(state.node.id, id, 10000); // 最多等待 10 秒
        updateProgressStep('sync', 'success', '同步到 Agent');
        
        // 步骤 4: 等待服务重启
        updateProgressStep('restart', 'loading', '重启服务...');
        await new Promise(resolve => setTimeout(resolve, 2000)); // 等待 2 秒
        updateProgressStep('restart', 'success', '重启服务');
        
        // 完成
        setTimeout(() => {
          toast("证书部署完成 ✅");
          closeModal();
          renderDetail();
        }, 500);
        
      } catch (e) {
        // 失败处理
        const currentStep = getCurrentLoadingStep();
        if (currentStep) {
          updateProgressStep(currentStep, 'error', e.message);
        }
        toast('证书上传失败: ' + e.message);
        submitBtn.disabled = false;
        submitBtn.textContent = '重试';
      }
    },
    bindCertificateForm,
  );
}

// 更新进度步骤状态
function updateProgressStep(step, status, text) {
  const stepEl = document.querySelector(`[data-step="${step}"]`);
  if (!stepEl) return;
  
  const iconEl = stepEl.querySelector('.progress-icon');
  const textEl = stepEl.querySelector('.progress-text');
  
  const icons = {
    pending: '⏸',
    loading: '⏳',
    success: '✅',
    error: '❌'
  };
  
  iconEl.textContent = icons[status] || icons.pending;
  if (text) textEl.textContent = text;
  
  stepEl.className = `progress-step progress-step-${status}`;
}

// 获取当前正在加载的步骤
function getCurrentLoadingStep() {
  const loadingStep = document.querySelector('.progress-step-loading');
  return loadingStep ? loadingStep.dataset.step : null;
}

// 轮询同步状态 (临时方案,后续改为 SSE)
async function pollSyncStatus(nodeId, portId, timeout) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    try {
      const response = await api('GET', `/api/nodes/${nodeId}/ports/${portId}`);
      // 检查是否同步完成 (需要后端添加 lastSyncTime 字段)
      if (response.lastSyncTime && new Date(response.lastSyncTime) > new Date(start)) {
        return true;
      }
    } catch (e) {
      // 继续轮询
    }
    await new Promise(resolve => setTimeout(resolve, 1000)); // 每秒检查一次
  }
  throw new Error('同步超时');
}
```

#### 2. 样式增强 (style.css)

```css
/* 进度显示 */
.progress-title {
  font-weight: 600;
  margin-bottom: 12px;
  color: #333;
}

.progress-steps {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.progress-step {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-radius: 4px;
  transition: all 0.2s;
}

.progress-step-pending {
  opacity: 0.5;
}

.progress-step-loading {
  background: #e3f2fd;
  animation: pulse 1.5s ease-in-out infinite;
}

.progress-step-success {
  background: #e8f5e9;
}

.progress-step-error {
  background: #ffebee;
}

.progress-icon {
  font-size: 18px;
  min-width: 24px;
}

.progress-text {
  font-size: 14px;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.7;
  }
}

/* 暗色主题适配 */
.theme-dark .progress-step-loading {
  background: rgba(33, 150, 243, 0.2);
}

.theme-dark .progress-step-success {
  background: rgba(76, 175, 80, 0.2);
}

.theme-dark .progress-step-error {
  background: rgba(244, 67, 54, 0.2);
}
```

---

### 阶段 2: Server-Sent Events 实时推送 (v1.6.2)

**目标**: 后端主动推送部署进度,不需要前端轮询

#### 1. 后端:添加 SSE 端点

```go
// internal/manager/handlers.go

// DeploymentProgress 部署进度事件
type DeploymentProgress struct {
    NodeID    string    `json:"nodeId"`
    PortID    string    `json:"portId"`
    Stage     string    `json:"stage"`     // upload, validate, sync, restart, complete, error
    Progress  int       `json:"progress"`  // 0-100
    Message   string    `json:"message"`
    Error     string    `json:"error,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

// SSE 部署进度推送
func (s *Server) handleDeployProgress(c *gin.Context) {
    nodeID := c.Param("id")
    portID := c.Param("portId")
    
    // 设置 SSE 响应头
    c.Writer.Header().Set("Content-Type", "text/event-stream")
    c.Writer.Header().Set("Cache-Control", "no-cache")
    c.Writer.Header().Set("Connection", "keep-alive")
    
    // 创建事件通道
    eventChan := make(chan DeploymentProgress, 10)
    
    // 注册事件监听器
    s.registerProgressListener(nodeID, portID, eventChan)
    defer s.unregisterProgressListener(nodeID, portID, eventChan)
    
    // 持续推送事件
    for {
        select {
        case event := <-eventChan:
            data, _ := json.Marshal(event)
            fmt.Fprintf(c.Writer, "data: %s\n\n", data)
            c.Writer.Flush()
            
            // 如果是完成或错误,关闭连接
            if event.Stage == "complete" || event.Stage == "error" {
                return
            }
            
        case <-c.Request.Context().Done():
            return
        }
    }
}

// 更新证书时发送进度事件
func (s *Server) updatePortCertificate(c *gin.Context) {
    nodeID := c.Param("id")
    portID := c.Param("portId")
    
    // ... 解析请求
    
    // 发送进度:上传完成
    s.emitProgress(nodeID, portID, DeploymentProgress{
        Stage:    "upload",
        Progress: 25,
        Message:  "上传到 Manager",
    })
    
    // 验证证书
    if err := validateCertificate(req.TLSCert, req.TLSKey); err != nil {
        s.emitProgress(nodeID, portID, DeploymentProgress{
            Stage:   "error",
            Message: "证书验证失败",
            Error:   err.Error(),
        })
        return
    }
    
    s.emitProgress(nodeID, portID, DeploymentProgress{
        Stage:    "validate",
        Progress: 50,
        Message:  "证书验证通过",
    })
    
    // 同步到 Agent
    s.emitProgress(nodeID, portID, DeploymentProgress{
        Stage:    "sync",
        Progress: 75,
        Message:  "同步到 Agent...",
    })
    
    if err := s.syncToAgent(nodeID, portID); err != nil {
        s.emitProgress(nodeID, portID, DeploymentProgress{
            Stage:   "error",
            Message: "同步失败",
            Error:   err.Error(),
        })
        return
    }
    
    s.emitProgress(nodeID, portID, DeploymentProgress{
        Stage:    "restart",
        Progress: 90,
        Message:  "重启服务...",
    })
    
    // 等待服务重启
    time.Sleep(2 * time.Second)
    
    s.emitProgress(nodeID, portID, DeploymentProgress{
        Stage:    "complete",
        Progress: 100,
        Message:  "部署完成",
    })
}
```

#### 2. 前端:使用 EventSource 接收事件

```javascript
async function uploadCert(id) {
  // ... 前面的代码

  // 建立 SSE 连接
  const eventSource = new EventSource(
    url(`/api/nodes/${state.node.id}/ports/${id}/deploy/progress`)
  );
  
  eventSource.onmessage = (e) => {
    const progress = JSON.parse(e.data);
    
    // 更新 UI
    updateProgressStep(progress.stage, 
      progress.stage === 'error' ? 'error' : 'loading',
      progress.message
    );
    
    if (progress.stage === 'complete') {
      toast("证书部署完成 ✅");
      eventSource.close();
      closeModal();
      renderDetail();
    } else if (progress.stage === 'error') {
      toast(`部署失败: ${progress.error}`);
      eventSource.close();
    }
  };
  
  eventSource.onerror = () => {
    toast("连接中断,请刷新重试");
    eventSource.close();
  };
  
  // 提交表单
  try {
    await api("PATCH", `/api/nodes/${state.node.id}/ports/${id}`, payload);
  } catch (e) {
    eventSource.close();
    toast('证书上传失败: ' + e.message);
  }
}
```

---

### 阶段 3: 完整的部署验收检查 (v1.6.3)

**目标**: 自动验证部署是否真正成功

**检查项**:
1. ✅ 证书文件存在于 Manager
2. ✅ 证书文件已同步到 Agent
3. ✅ NaïveProxy 服务正在运行
4. ✅ 端口正在监听
5. ✅ TLS 握手成功
6. ✅ 证书域名匹配

```go
// internal/manager/handlers.go

type DeploymentStatus struct {
    CertFileExists    bool      `json:"certFileExists"`
    AgentSynced       bool      `json:"agentSynced"`
    ServiceRunning    bool      `json:"serviceRunning"`
    PortListening     bool      `json:"portListening"`
    TLSHandshake      bool      `json:"tlsHandshake"`
    DomainMatch       bool      `json:"domainMatch"`
    LastCheck         time.Time `json:"lastCheck"`
}

func (s *Server) checkDeploymentComplete(nodeID, portID string) DeploymentStatus {
    status := DeploymentStatus{LastCheck: time.Now()}
    
    // 1. 检查证书文件
    port := s.getPort(nodeID, portID)
    if port.TLSCert != "" && port.TLSKey != "" {
        status.CertFileExists = true
    }
    
    // 2. 检查 Agent 同步状态
    if port.LastSyncTime.After(port.UpdatedAt) {
        status.AgentSynced = true
    }
    
    // 3. 检查服务状态 (从 Agent 上报)
    node := s.getNode(nodeID)
    if serviceStatus, ok := node.ServiceStatus["caddy"]; ok && serviceStatus == "running" {
        status.ServiceRunning = true
    }
    
    // 4. 检查端口监听
    if s.checkPortOpen(node.Address, port.Port) {
        status.PortListening = true
    }
    
    // 5. 检查 TLS 握手
    if s.checkTLSHandshake(node.Address, port.Port, port.Domain) {
        status.TLSHandshake = true
    }
    
    // 6. 检查证书域名
    if cert := s.getCertFromServer(node.Address, port.Port); cert != nil {
        if s.verifyCertDomain(cert, port.Domain) {
            status.DomainMatch = true
        }
    }
    
    return status
}
```

---

## 实施优先级

| 功能 | 优先级 | 版本 | 工作量 |
|------|--------|------|--------|
| 基础进度反馈 (前端) | P0 | v1.6.1 | 0.5 天 |
| 轮询同步状态 | P1 | v1.6.1 | 0.5 天 |
| SSE 实时推送 | P1 | v1.6.2 | 1 天 |
| 完整部署验收 | P2 | v1.6.3 | 1.5 天 |

**总计**: ~3.5 天开发时间

---

## 测试清单

- [ ] 上传有效证书 - 所有步骤显示成功
- [ ] 上传无效证书 - 验证步骤显示错误
- [ ] 网络中断 - 显示同步失败并允许重试
- [ ] Agent 离线 - 显示同步失败
- [ ] 证书格式错误 - 在验证步骤捕获
- [ ] 暗色主题下进度显示正常

---

## 相关文档

- [生产加固路线图](./PRODUCTION_HARDENING_ROADMAP.md)
- [前端增强功能](./ENHANCED_FEATURES.md)

---

**创建日期**: 2026-10-27  
**维护者**: holi
