// Package store provides file backed persistence for the manager.
package store

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"merit/internal/model"
)

const (
	saltLen    = 16
	keyLen     = 32
	iterations = 120000
)

// Admin is the single panel administrator.
type Admin struct {
	Username     string    `json:"username"`
	PasswordSalt string    `json:"passwordSalt"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Settings holds panel wide configuration.
type Settings struct {
	SubToken  string `json:"subToken"`
	PanelPath string `json:"panelPath"`
}

type persisted struct {
	Admin    *Admin        `json:"admin"`
	Settings *Settings     `json:"settings"`
	Nodes    []*model.Node `json:"nodes"`
}

// Store is a concurrency safe JSON backed data store.
type Store struct {
	mu    sync.RWMutex
	path  string
	data  persisted
	index map[string]*model.Node
}

// Open loads (or creates) the store at path.
func Open(path string) (*Store, error) {
	s := &Store{path: path, index: map[string]*model.Node{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, err
		}
	}
	for _, n := range s.data.Nodes {
		if n.Ports == nil {
			n.Ports = []*model.Port{}
		}
		s.index[n.ID] = n
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(&s.data, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Save flushes the store to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// Admin returns the current administrator, or nil when not configured.
func (s *Store) Admin() *Admin {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Admin
}

// SetAdmin configures the administrator account.
func (s *Store) SetAdmin(username, password string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Admin = &Admin{
		Username:     username,
		PasswordSalt: hex.EncodeToString(salt),
		PasswordHash: hex.EncodeToString(hash),
		CreatedAt:    time.Now(),
	}
	return s.saveLocked()
}

// VerifyAdmin checks the given credentials against the stored administrator.
func (s *Store) VerifyAdmin(username, password string) bool {
	s.mu.RLock()
	admin := s.data.Admin
	s.mu.RUnlock()
	if admin == nil || admin.Username != username {
		return false
	}
	salt, err := hex.DecodeString(admin.PasswordSalt)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(admin.PasswordHash)
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// SubToken returns the subscription token, creating one if needed.
func (s *Store) SubToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Settings == nil || s.data.Settings.SubToken == "" {
		token := make([]byte, 24)
		if _, err := rand.Read(token); err != nil {
			return ""
		}
		s.data.Settings = &Settings{SubToken: hex.EncodeToString(token)}
		_ = s.saveLocked()
	}
	return s.data.Settings.SubToken
}

// EnsurePanelPath returns the persistent random path used to access the panel.
// preferred is used on first startup when supplied by the installer.
func (s *Store) EnsurePanelPath(preferred string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Settings != nil && s.data.Settings.PanelPath != "" {
		return s.data.Settings.PanelPath, nil
	}
	path := preferred
	if path == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		path = "/panel/" + hex.EncodeToString(b)
	}
	if !strings.HasPrefix(path, "/panel/") || strings.ContainsAny(path, "?#") || len(path) > 128 {
		return "", errors.New("invalid panel path")
	}
	if s.data.Settings == nil {
		s.data.Settings = &Settings{}
	}
	s.data.Settings.PanelPath = path
	return path, s.saveLocked()
}

// RotateSubToken replaces the subscription token with a new random value.
func (s *Store) RotateSubToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	s.data.Settings = &Settings{SubToken: hex.EncodeToString(token)}
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return s.data.Settings.SubToken, nil
}

// Nodes returns a snapshot of all nodes.
func (s *Store) Nodes() []*model.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Node, len(s.data.Nodes))
	copy(out, s.data.Nodes)
	return out
}

// Node returns a node by id.
func (s *Store) Node(id string) *model.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index[id]
}

// NodeByAPIKey returns a node matching the given API key.
func (s *Store) NodeByAPIKey(key string) *model.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, n := range s.data.Nodes {
		if n.APIKey == key {
			return n
		}
	}
	return nil
}

// AddNode inserts a node.
func (s *Store) AddNode(n *model.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.Ports == nil {
		n.Ports = []*model.Port{}
	}
	s.data.Nodes = append(s.data.Nodes, n)
	s.index[n.ID] = n
	return s.saveLocked()
}

// Update mutates a node under lock and persists the result.
func (s *Store) Update(id string, fn func(*model.Node) error) (*model.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.index[id]
	if n == nil {
		return nil, errors.New("node not found")
	}
	if err := fn(n); err != nil {
		return nil, err
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return n, nil
}

// Delete removes a node.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.index[id]; !ok {
		return errors.New("node not found")
	}
	delete(s.index, id)
	out := s.data.Nodes[:0]
	for _, n := range s.data.Nodes {
		if n.ID != id {
			out = append(out, n)
		}
	}
	s.data.Nodes = out
	return s.saveLocked()
}
