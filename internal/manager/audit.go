package manager

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"merit/internal/model"
)

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) auditMutation(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next(w, r)
			return
		}
		writer := &auditResponseWriter{ResponseWriter: w}
		action, target := s.describeMutation(r)
		next(writer, r)
		if writer.status >= 300 {
			return
		}
		admin := s.store.Admin()
		actor := "管理员"
		if admin != nil && admin.Username != "" {
			actor = admin.Username
		}
		if err := s.store.AddAudit(model.AuditEntry{
			Time: time.Now().UTC(), Actor: actor, Action: action, Target: target,
		}); err != nil {
			log.Printf("记录管理操作失败: %v", err)
		}
	}
}

func (s *Server) describeMutation(r *http.Request) (string, string) {
	path := r.URL.Path
	action := "配置变更"
	switch {
	case strings.HasSuffix(path, "/api/backup/webdav"):
		action = "更新 WebDAV 云备份设置"
	case strings.HasSuffix(path, "/api/backup/webdav/upload"):
		action = "上传 WebDAV 云备份"
	case strings.HasSuffix(path, "/api/backup/webdav/restore"):
		action = "恢复 WebDAV 云备份"
	case strings.HasSuffix(path, "/api/backup/restore"):
		action = "恢复备份"
	case strings.HasSuffix(path, "/api/settings/sub-token/rotate"):
		action = "重置订阅令牌"
	case strings.HasSuffix(path, "/api/update/apply"):
		action = "在线更新 Manager"
	case strings.HasSuffix(path, "/retry") && strings.Contains(path, "/ports/"):
		action = "重试端口同步"
	case strings.Contains(path, "/ports/") && r.Method == http.MethodPatch:
		action = "更新端口设置"
	case strings.Contains(path, "/ports/") && r.Method == http.MethodDelete:
		action = "删除端口"
	case strings.HasSuffix(path, "/ports") && r.Method == http.MethodPost:
		action = "新增端口"
	case strings.HasSuffix(path, "/egress") && r.Method == http.MethodPut:
		action = "更新出站配置"
	case strings.HasSuffix(path, "/api/nodes") && r.Method == http.MethodPost:
		action = "新增节点"
	case strings.Contains(path, "/api/nodes/") && r.Method == http.MethodPatch:
		action = "更新节点信息"
	case strings.Contains(path, "/api/nodes/") && r.Method == http.MethodDelete:
		action = "删除节点"
	}
	target := ""
	if id := r.PathValue("id"); id != "" {
		target = id
		if node := s.store.Node(id); node != nil && node.Name != "" {
			target = node.Name
		}
	} else if strings.HasSuffix(path, "/api/nodes") && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &request); err == nil {
			target = strings.TrimSpace(request.Name)
		}
	}
	return action, target
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	writeJSON(w, http.StatusOK, s.store.Audit(limit))
}
