package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"merit/internal/model"
)

func (s *Store) PendingTasks() map[string]map[string]*model.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]map[string]*model.Task)
	raw, _ := json.Marshal(s.data.Tasks)
	_ = json.Unmarshal(raw, &out)
	return out
}

func (s *Store) SetTasks(id string, tasks map[string]*model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Tasks == nil {
		s.data.Tasks = make(map[string]map[string]*model.Task)
	}
	copyTasks := make(map[string]*model.Task)
	raw, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &copyTasks); err != nil {
		return err
	}
	previous, existed := s.data.Tasks[id]
	s.data.Tasks[id] = copyTasks
	if err := s.saveLocked(); err != nil {
		if existed {
			s.data.Tasks[id] = previous
		} else {
			delete(s.data.Tasks, id)
		}
		return err
	}
	return nil
}

// Export omits transient tasks (which may contain stale proxy credentials).
func (s *Store) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyData := s.data
	copyData.Tasks = nil
	if copyData.Nodes == nil {
		copyData.Nodes = []*model.Node{}
	}
	return json.MarshalIndent(copyData, "", "  ")
}

// Restore preserves the current administrator and panel path so restoring a
// backup cannot lock the current operator out. It creates a private recovery
// file before replacing nodes and subscription settings.
func (s *Store) Restore(raw []byte) error {
	var data persisted
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("备份 JSON 无效: %w", err)
	}
	if data.Nodes == nil {
		return fmt.Errorf("备份缺少 nodes 数组")
	}
	index := make(map[string]*model.Node)
	keys := make(map[string]bool)
	for _, n := range data.Nodes {
		if n == nil || n.ID == "" || n.APIKey == "" || index[n.ID] != nil || keys[n.APIKey] {
			return fmt.Errorf("备份节点标识或密钥无效/重复")
		}
		keys[n.APIKey] = true
		ports := make(map[int]bool)
		portIDs := make(map[string]bool)
		for _, p := range n.Ports {
			if p == nil || p.ID == "" || p.Port < 1025 || p.Port > 65535 || ports[p.Port] || portIDs[p.ID] {
				return fmt.Errorf("备份端口无效或重复")
			}
			ports[p.Port] = true
			portIDs[p.ID] = true
			if p.Protocol != model.ProtocolTCP && p.Protocol != model.ProtocolUDP {
				return fmt.Errorf("备份端口协议必须为 TCP 或 UDP")
			}
			p.InstanceRunning = false
			p.InstanceError = ""
			p.LastSyncAttempt, p.InstanceSyncedAt = time.Time{}, time.Time{}
		}
		n.Registered, n.MitaRunning = false, false
		n.LastSeen = time.Time{}
		n.LastError, n.AgentID = "", ""
		index[n.ID] = n
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.data
	old, err := json.MarshalIndent(previous, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(fmt.Sprintf("%s.before-restore-%d.json", s.path, time.Now().UnixNano()), old, 0o600); err != nil {
		return err
	}
	data.Admin = previous.Admin
	if data.Settings == nil {
		data.Settings = &Settings{}
	}
	if previous.Settings != nil {
		data.Settings.PanelPath = previous.Settings.PanelPath
	}
	data.Tasks = nil
	s.data = data
	if err := s.saveLocked(); err != nil {
		s.data = previous
		return err
	}
	s.index = index
	return nil
}
