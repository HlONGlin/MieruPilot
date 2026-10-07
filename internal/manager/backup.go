package manager

import (
	"encoding/json"
	"io"
	"merit/internal/model"
	"net/http"
)

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	raw, err := s.store.Export()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "导出失败"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="merit-backup.json"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "备份读取失败或超过 16MB"})
		return
	}
	var preview struct {
		Nodes []*model.Node `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &preview); err != nil {
		writeJSON(w, 400, map[string]any{"error": "备份 JSON 格式无效"})
		return
	}
	for _, n := range preview.Nodes {
		if n != nil && n.Egress != nil {
			if err := validateEgress(n.Egress); err != nil {
				writeJSON(w, 400, map[string]any{"error": err.Error()})
				return
			}
		}
	}
	if err := s.store.Restore(raw); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	s.mu.Lock()
	for _, runtime := range s.rt {
		runtime.mu.Lock()
		clear(runtime.pending)
		runtime.initialized = false
		runtime.mu.Unlock()
	}
	s.mu.Unlock()
	for _, node := range s.store.Nodes() {
		s.enqueueSync(node)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "已恢复，保留当前管理员和面板地址，等待节点同步"})
}
