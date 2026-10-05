// Package model contains the shared data types used by both the merit
// manager and the merit agent.
package model

import "time"

// Proxy transport protocols supported by mita.
const (
	ProtocolTCP = "TCP"
	ProtocolUDP = "UDP"
)

// Defaults applied when generating a mieru client profile.
const (
	DefaultProfile        = "default"
	DefaultMTU            = 1400
	DefaultMultiplexing   = "MULTIPLEXING_OFF"
	DefaultHandshake      = "HANDSHAKE_NO_WAIT"
	DefaultTrafficPattern = "CIXu+asFEAAiCAgBEAEYBCAIKgUIABCAATICCAA="
)

// Port is a single auto generated mieru node bound to a node host. Each port
// owns its own randomly generated username / password so that it can be shared
// independently as one mieru node.
type Port struct {
	ID        string    `json:"id"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Label     string    `json:"label"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

// Node represents one managed host running the merit agent and mita.
type Node struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"apiKey"`
	Address   string    `json:"address"`
	Domain    string    `json:"domain"`
	Remark    string    `json:"remark"`
	Ports     []*Port   `json:"ports"`
	CreatedAt time.Time `json:"createdAt"`

	// Last known information reported by the agent.
	AgentVer    string    `json:"agentVer,omitempty"`
	MitaVer     string    `json:"mitaVer,omitempty"`
	OS          string    `json:"os,omitempty"`
	Arch        string    `json:"arch,omitempty"`
	MitaRunning bool      `json:"mitaRunning"`
	Registered  bool      `json:"registered"`
	LastSeen    time.Time `json:"lastSeen"`
	LastError   string    `json:"lastError,omitempty"`

	// Runtime only fields (not persisted).
	SeenIP string `json:"-"`
	Online bool   `json:"-"`
}

// FindPort returns the port with the given id, or nil.
func (n *Node) FindPort(id string) *Port {
	for _, p := range n.Ports {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// PortBinding is a mita portBindings entry.
type PortBinding struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// UserCred is a mita user entry.
type UserCred struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// DesiredConfig is the full desired mita server configuration for a node.
type DesiredConfig struct {
	Enable       bool          `json:"enable"`
	PortBindings []PortBinding `json:"portBindings"`
	Users        []UserCred    `json:"users"`
	LoggingLevel string        `json:"loggingLevel"`
}

// BuildDesired converts the enabled ports of a node into a mita config.
func (n *Node) BuildDesired() *DesiredConfig {
	cfg := &DesiredConfig{LoggingLevel: "INFO"}
	for _, p := range n.Ports {
		if !p.Enabled {
			continue
		}
		cfg.PortBindings = append(cfg.PortBindings, PortBinding{Port: p.Port, Protocol: p.Protocol})
		cfg.Users = append(cfg.Users, UserCred{Name: p.Username, Password: p.Password})
	}
	cfg.Enable = len(cfg.PortBindings) > 0
	return cfg
}

// Task is a command delivered from the manager to an agent.
type Task struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Config    *DesiredConfig `json:"config,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// Task kinds.
const (
	TaskSync = "sync"
)

// AgentStatus is a status snapshot reported by an agent.
type AgentStatus struct {
	MitaInstalled bool   `json:"mitaInstalled"`
	MitaRunning   bool   `json:"mitaRunning"`
	MitaVersion   string `json:"mitaVersion"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	PublicIP      string `json:"publicIP"`
	AgentVersion  string `json:"agentVersion"`
	Error         string `json:"error,omitempty"`
}

// TaskResult is the result of executing a Task.
type TaskResult struct {
	TaskID  string       `json:"taskId"`
	OK      bool         `json:"ok"`
	Message string       `json:"message"`
	Status  *AgentStatus `json:"status,omitempty"`
}
