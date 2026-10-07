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
	ID               string    `json:"id"`
	Port             int       `json:"port"`
	Protocol         string    `json:"protocol"`
	Username         string    `json:"username"`
	Password         string    `json:"password"`
	Label            string    `json:"label"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"createdAt"`
	InstanceRunning  bool      `json:"instanceRunning"`
	InstanceSyncedAt time.Time `json:"instanceSyncedAt,omitempty"`
	LastSyncAttempt  time.Time `json:"lastSyncAttempt,omitempty"`
	InstanceError    string    `json:"instanceError,omitempty"`
}

// Node represents one managed host running the merit agent and mita.
type Node struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	APIKey    string        `json:"apiKey"`
	Address   string        `json:"address"`
	Domain    string        `json:"domain"`
	Remark    string        `json:"remark"`
	Ports     []*Port       `json:"ports"`
	Egress    *EgressConfig `json:"egress,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`

	// Last known information reported by the agent.
	AgentVer    string    `json:"agentVer,omitempty"`
	AgentID     string    `json:"agentId,omitempty"`
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

const (
	EgressProtocolSocks5 = "SOCKS5_PROXY_PROTOCOL"
	EgressDirect         = "DIRECT"
	EgressProxyAction    = "PROXY"
	EgressReject         = "REJECT"
)

type EgressProxy struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark,omitempty"`
}

type EgressRule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	IPRanges   []string `json:"ipRanges,omitempty"`
	Domains    []string `json:"domainNames,omitempty"`
	Action     string   `json:"action"`
	ProxyNames []string `json:"proxyNames,omitempty"`
	Ports      []int    `json:"ports,omitempty"`
	Enabled    bool     `json:"enabled"`
	Order      int      `json:"order"`
}

type EgressConfig struct {
	Proxies []EgressProxy `json:"proxies,omitempty"`
	Rules   []EgressRule  `json:"rules,omitempty"`
}

// DesiredConfig is the full desired mita server configuration for a node.
type DesiredConfig struct {
	Enable        bool                 `json:"enable"`
	Partial       bool                 `json:"partial,omitempty"`
	PortBindings  []PortBinding        `json:"portBindings"`
	Users         []UserCred           `json:"users"`
	PortIDs       []string             `json:"portIds,omitempty"`
	DisabledPorts []PortInstanceTarget `json:"disabledPorts,omitempty"`
	LoggingLevel  string               `json:"loggingLevel"`
	Egress        *EgressConfig        `json:"egress,omitempty"`
}

type PortInstanceTarget struct {
	ID   string `json:"id"`
	Port int    `json:"port"`
}

// BuildDesired converts the enabled ports of a node into a mita config.
func (n *Node) BuildDesired() *DesiredConfig {
	cfg := &DesiredConfig{LoggingLevel: "INFO", Egress: n.Egress}
	for _, p := range n.Ports {
		if !p.Enabled {
			cfg.DisabledPorts = append(cfg.DisabledPorts, PortInstanceTarget{ID: p.ID, Port: p.Port})
			continue
		}
		cfg.PortBindings = append(cfg.PortBindings, PortBinding{Port: p.Port, Protocol: p.Protocol})
		cfg.Users = append(cfg.Users, UserCred{Name: p.Username, Password: p.Password})
		cfg.PortIDs = append(cfg.PortIDs, p.ID)
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
	TestProxy *EgressProxy   `json:"testProxy,omitempty"`
}

// Task kinds.
const (
	TaskSync       = "sync"
	TaskTestEgress = "test-egress"
)

// AgentStatus is a status snapshot reported by an agent.
type AgentStatus struct {
	MitaInstalled bool                 `json:"mitaInstalled"`
	MitaRunning   bool                 `json:"mitaRunning"`
	MitaVersion   string               `json:"mitaVersion"`
	OS            string               `json:"os"`
	Arch          string               `json:"arch"`
	PublicIP      string               `json:"publicIP"`
	AgentVersion  string               `json:"agentVersion"`
	AgentID       string               `json:"agentId,omitempty"`
	Error         string               `json:"error,omitempty"`
	PortInstances []PortInstanceStatus `json:"portInstances,omitempty"`
}

type PortInstanceStatus struct {
	Port    int    `json:"port"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// TaskResult is the result of executing a Task.
type TaskResult struct {
	TaskID      string               `json:"taskId"`
	OK          bool                 `json:"ok"`
	Message     string               `json:"message"`
	Status      *AgentStatus         `json:"status,omitempty"`
	PortResults []PortInstanceResult `json:"portResults,omitempty"`
	EgressTest  *EgressTestResult    `json:"egressTest,omitempty"`
}

// EgressTestResult contains observations only; no proxy credentials are echoed.
type EgressTestResult struct {
	TaskID    string    `json:"taskId"`
	ProxyID   string    `json:"proxyId"`
	State     string    `json:"state"`
	OK        bool      `json:"ok"`
	ExitIP    string    `json:"exitIP,omitempty"`
	LatencyMs int64     `json:"latencyMs"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	CheckedAt time.Time `json:"checkedAt"`
}

type PortInstanceResult struct {
	PortID    string    `json:"portId"`
	Port      int       `json:"port"`
	Running   bool      `json:"running"`
	OK        bool      `json:"ok"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
}
