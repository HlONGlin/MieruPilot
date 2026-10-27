// Package model contains the shared data types used by both the merit
// manager and the merit agent.
package model

import (
	"strings"
	"time"
)

// Proxy transport protocols supported by mita.
const (
	ProtocolTCP = "TCP"
	ProtocolUDP = "UDP"
)

// Proxy types supported by the system.
const (
	ProxyTypeMieru     = "mieru"
	ProxyTypeNaive     = "naive"
	ProxyTypeAnyTLS    = "anytls"
	ProxyTypeShadowTLS = "shadowtls"
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
	ProxyType        string    `json:"proxyType,omitempty"` // mieru, naive, anytls, shadowtls; defaults to mieru
	Username         string    `json:"username"`
	Password         string    `json:"password"`
	Label            string    `json:"label"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"createdAt"`
	InstanceRunning  bool      `json:"instanceRunning"`
	InstanceSyncedAt time.Time `json:"instanceSyncedAt,omitempty"`
	LastSyncAttempt  time.Time `json:"lastSyncAttempt,omitempty"`
	InstanceError    string    `json:"instanceError,omitempty"`

	// Traffic control and statistics
	TrafficLimit   int64     `json:"trafficLimit,omitempty"`   // Monthly limit in bytes, 0 = unlimited
	TrafficUsed    int64     `json:"trafficUsed,omitempty"`    // Used traffic in bytes
	TrafficResetAt time.Time `json:"trafficResetAt,omitempty"` // When the traffic counter was last reset
	BandwidthLimit int64     `json:"bandwidthLimit,omitempty"` // Speed limit in bytes/sec, 0 = unlimited
	TrafficRaw     int64     `json:"trafficRaw,omitempty"`     // Last raw mita metric counter
	AutoDisabled   bool      `json:"autoDisabled,omitempty"`   // Disabled automatically after exceeding the limit

	// Proxy-specific configuration
	TLSCert        string `json:"tlsCert,omitempty"`        // TLS certificate (for naive proxy)
	TLSKey         string `json:"tlsKey,omitempty"`         // TLS private key (for naive proxy)
	Domain         string `json:"domain,omitempty"`         // Domain name for ACME or SNI
	TLSFingerprint string `json:"tlsFingerprint,omitempty"` // AnyTLS client TLS fingerprint; empty preserves chrome.
	NaiveTransport string `json:"naiveTransport,omitempty"` // https (default) or quic; server supports both.
	HandshakeAddr  string `json:"handshakeAddr,omitempty"`  // Handshake server for ShadowTLS
	AutoACME       bool   `json:"autoACME,omitempty"`       // Enable automatic ACME certificate
	ACMEStaging    bool   `json:"acmeStaging,omitempty"`    // Use Let's Encrypt staging CA (for testing)
	// BackendAddr is the data server that ShadowTLS relays decrypted traffic to
	// (for example a shadowsocks instance reachable at 127.0.0.1:8388).
	BackendAddr string `json:"backendAddr,omitempty"`
	// BackendPassword and BackendCipher are the credentials of that data server.
	// They are only used to generate client configurations: ShadowTLS carries no
	// authentication of its own.
	BackendPassword string `json:"backendPassword,omitempty"`
	BackendCipher   string `json:"backendCipher,omitempty"`
	// BackendManaged marks a backend that the Manager allocated itself. The agent
	// then deploys a loopback-only shadowsocks server for it. A manually entered
	// backend is never touched by the agent.
	BackendManaged bool `json:"backendManaged,omitempty"`

	// Runtime state (not persisted, populated from agent reports)
	RuntimeACMEStatus *ACMEStatus `json:"-"` // Excludes from JSON marshaling
}

// NormalizeProxyType returns the effective proxy type of a port, defaulting to
// mieru for older records that have no proxy type stored.
func NormalizeProxyType(proxyType string) string {
	if strings.TrimSpace(proxyType) == "" {
		return ProxyTypeMieru
	}
	return proxyType
}

// MinListenPort reflects the dedicated, unprivileged user used by mita.
// The other servers run as root and can bind HTTPS and other reserved ports.
func MinListenPort(proxyType string) int {
	if NormalizeProxyType(proxyType) == ProxyTypeMieru {
		return 1025
	}
	return 1
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

	// Deleting marks a node whose remote cleanup has been requested. The node
	// record is kept until the agent acknowledges the purge task so it can
	// still authenticate and report progress.
	Deleting bool `json:"deleting,omitempty"`

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

// AuditEntry records a successful administrative change without request data.
type AuditEntry struct {
	Time   time.Time `json:"time"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Target string    `json:"target,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// DesiredConfig is the full desired mita server configuration for a node.
type DesiredConfig struct {
	Enable        bool                 `json:"enable"`
	Partial       bool                 `json:"partial,omitempty"`
	PortBindings  []PortBinding        `json:"portBindings"`
	Users         []UserCred           `json:"users"`
	PortIDs       []string             `json:"portIds,omitempty"`
	Ports         []*Port              `json:"ports,omitempty"` // 完整Port信息用于多代理支持
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
		cfg.Ports = append(cfg.Ports, p) // 添加完整Port信息
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
	// TaskPurge tears down everything the agent installed on the host: port
	// instances, configs/accounts, mita and the agent service itself.
	TaskPurge = "purge"
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
	Port         int         `json:"port"`
	Running      bool        `json:"running"`
	Error        string      `json:"error,omitempty"`
	TrafficUsed  int64       `json:"trafficUsed,omitempty"`  // Traffic used since last report
	TotalTraffic int64       `json:"totalTraffic,omitempty"` // Total accumulated traffic
	ACMEStatus   *ACMEStatus `json:"acmeStatus,omitempty"`   // NaïveProxy ACME status (optional)
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

// ACMEStatus holds the last known ACME renewal status from systemd journal logs.
type ACMEStatus struct {
	Status       string    `json:"status"`                 // "success", "failed", "unknown"
	LastEvent    time.Time `json:"lastEvent,omitempty"`    // Time of last ACME event
	LastError    string    `json:"lastError,omitempty"`    // Last error message if failed
	RenewalCount int       `json:"renewalCount,omitempty"` // Number of successful renewals observed
}
