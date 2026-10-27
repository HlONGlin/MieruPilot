package store

import (
	"errors"
	"net/url"
	"strings"
)

// CloudBackupConfig contains outbound WebDAV credentials, not DAV accounts.
// Secrets are saved only in the private local store, never exported in backups.
type CloudBackupConfig struct {
	URL       string `json:"url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	AllowHTTP bool   `json:"allowHTTP"`
}

func (s *Store) CloudBackup() CloudBackupConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.CloudBackup == nil {
		return CloudBackupConfig{}
	}
	return *s.data.CloudBackup
}

func (s *Store) SaveCloudBackup(c CloudBackupConfig, clearPassword bool) error {
	c.URL = strings.TrimSpace(c.URL)
	u, err := url.Parse(c.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("请输入不含账号、查询参数或片段的 WebDAV 目录 HTTP(S) 地址")
	}
	if u.Scheme == "http" && !c.AllowHTTP {
		return errors.New("HTTP 不加密凭据；仅在可信内网明确允许 HTTP 后使用")
	}
	if len(c.URL) > 2048 || len(c.Username) > 256 || len(c.Password) > 4096 || strings.ContainsAny(c.Username, ":\r\n") {
		return errors.New("WebDAV 地址或账号格式无效")
	}
	if strings.ContainsAny(u.Path, "\\\x00") {
		return errors.New("WebDAV 目录无效")
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == ".." || part == "." {
			return errors.New("WebDAV 目录不能包含相对路径")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	u.RawPath = ""
	c.URL = u.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.data.CloudBackup
	if c.Password == "" && !clearPassword && previous != nil {
		if c.URL != previous.URL || c.Username != previous.Username {
			return errors.New("变更地址或用户名时请重新填写密码，或明确清除已保存密码")
		}
		c.Password = previous.Password
	}
	if clearPassword {
		c.Password = ""
	}
	s.data.CloudBackup = &c
	if err := s.saveLocked(); err != nil {
		s.data.CloudBackup = previous
		return err
	}
	return nil
}
