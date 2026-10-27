package manager

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"merit/internal/store"
)

const maxCloudBackup = 16 << 20

var cloudBackupName = regexp.MustCompile(`^merit-backup-[0-9]{8}T[0-9]{6}\.[0-9]{9}Z-[a-f0-9]{8}\.json$`)

type cloudDAV struct {
	config store.CloudBackupConfig
	base   *url.URL
	client *http.Client
}

func newCloudDAV(c store.CloudBackupConfig) (*cloudDAV, error) {
	u, err := url.Parse(c.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && c.AllowHTTP)) {
		return nil, errors.New("请先保存有效的 WebDAV 云备份地址")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	u.RawPath = ""
	return &cloudDAV{config: c, base: u, client: &http.Client{
		Timeout: 45 * time.Second,
		// Never forward credentials or a sensitive backup through a redirect.
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (d *cloudDAV) request(ctx context.Context, method, name string, body []byte, headers map[string]string) (*http.Response, error) {
	u := *d.base
	if name != "" {
		u.Path += name
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("无法创建 WebDAV 请求")
	}
	req.SetBasicAuth(d.config.Username, d.config.Password)
	req.Header.Set("User-Agent", "MieruPilot-CloudBackup")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := d.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("WebDAV 请求已取消")
		}
		return nil, errors.New("WebDAV 连接失败，请检查地址、网络、证书与服务端状态")
	}
	return res, nil
}

func cloudStatus(res *http.Response, allowed ...int) error {
	for _, status := range allowed {
		if res.StatusCode == status {
			return nil
		}
	}
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		return errors.New("WebDAV 返回重定向，请直接填写最终目录地址")
	}
	return fmt.Errorf("WebDAV 返回 HTTP %d，请检查账号权限、目录或配额", res.StatusCode)
}

func (d *cloudDAV) ensureDirectory(ctx context.Context) error {
	res, err := d.request(ctx, "PROPFIND", "", []byte(`<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`), map[string]string{"Depth": "0", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return err
	}
	status := res.StatusCode
	res.Body.Close()
	if status == http.StatusMultiStatus || status == http.StatusOK {
		return nil
	}
	if status != http.StatusNotFound {
		return cloudStatus(res, 207)
	}
	res, err = d.request(ctx, "MKCOL", "", nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 405 {
		// Another client may have created this directory between requests.
		check, err := d.request(ctx, "PROPFIND", "", nil, map[string]string{"Depth": "0"})
		if err != nil {
			return err
		}
		defer check.Body.Close()
		return cloudStatus(check, 207)
	}
	return cloudStatus(res, 201)
}

func cloudUniqueName(prefix, suffix string) (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(b) + suffix, nil
}

type cloudBackupFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

type cloudMultistatus struct {
	XMLName   xml.Name `xml:"DAV: multistatus"`
	Responses []struct {
		Href     string `xml:"DAV: href"`
		Propstat []struct {
			Status string `xml:"DAV: status"`
			Prop   struct {
				Length   string `xml:"DAV: getcontentlength"`
				Modified string `xml:"DAV: getlastmodified"`
				Type     struct {
					Collection *struct{} `xml:"DAV: collection"`
				} `xml:"DAV: resourcetype"`
			} `xml:"DAV: prop"`
		} `xml:"DAV: propstat"`
	} `xml:"DAV: response"`
}

func (d *cloudDAV) list(ctx context.Context) ([]cloudBackupFile, error) {
	body := []byte(`<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:getcontentlength/><d:getlastmodified/><d:resourcetype/></d:prop></d:propfind>`)
	res, err := d.request(ctx, "PROPFIND", "", body, map[string]string{"Depth": "1", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := cloudStatus(res, 207); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, errors.New("WebDAV 目录响应超过 2 MiB 或读取失败，请使用专用备份目录")
	}
	var multi cloudMultistatus
	if err := xml.Unmarshal(raw, &multi); err != nil {
		return nil, errors.New("WebDAV 目录响应不是有效的 DAV XML")
	}
	files := []cloudBackupFile{}
	seen := make(map[string]bool)
	for _, entry := range multi.Responses {
		u, err := url.Parse(strings.TrimSpace(entry.Href))
		if err != nil {
			continue
		}
		u = d.base.ResolveReference(u)
		if u.Scheme != d.base.Scheme || u.Host != d.base.Host || u.RawQuery != "" || u.Fragment != "" {
			continue
		}
		name := strings.TrimPrefix(u.Path, d.base.Path)
		if !strings.HasPrefix(u.Path, d.base.Path) || !cloudBackupName.MatchString(name) || strings.Contains(name, "/") || seen[name] {
			continue
		}
		file := cloudBackupFile{Name: name}
		valid, collection := false, false
		for _, ps := range entry.Propstat {
			parts := strings.Fields(ps.Status)
			if len(parts) < 2 || parts[1] != "200" {
				continue
			}
			valid = true
			if ps.Prop.Type.Collection != nil {
				collection = true
			}
			if n, err := strconv.ParseInt(ps.Prop.Length, 10, 64); err == nil && n >= 0 {
				file.Size = n
			}
			if stamp, err := http.ParseTime(ps.Prop.Modified); err == nil {
				file.Modified = stamp.UTC().Format(time.RFC3339)
			}
		}
		if valid && !collection {
			seen[name] = true
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name > files[j].Name })
	return files, nil
}

func (s *Server) handleCloudBackupSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c := s.store.CloudBackup()
	writeJSON(w, 200, map[string]any{"url": c.URL, "username": c.Username, "hasPassword": c.Password != "", "allowHTTP": c.AllowHTTP})
}

func (s *Server) handleSaveCloudBackupSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL           string `json:"url"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		AllowHTTP     bool   `json:"allowHTTP"`
		ClearPassword bool   `json:"clearPassword"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "云备份配置格式无效"})
		return
	}
	s.cloudMu.Lock()
	defer s.cloudMu.Unlock()
	if err := s.store.SaveCloudBackup(store.CloudBackupConfig{URL: req.URL, Username: req.Username, Password: req.Password, AllowHTTP: req.AllowHTTP}, req.ClearPassword); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) cloudOperation(w http.ResponseWriter) (*cloudDAV, func(), bool) {
	if !s.cloudMu.TryLock() {
		writeJSON(w, 409, map[string]any{"error": "另一项云备份操作正在执行，请稍后重试"})
		return nil, nil, false
	}
	d, err := newCloudDAV(s.store.CloudBackup())
	if err != nil {
		s.cloudMu.Unlock()
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return nil, nil, false
	}
	return d, s.cloudMu.Unlock, true
}

func cloudError(w http.ResponseWriter, err error) {
	writeJSON(w, 502, map[string]any{"error": err.Error()})
}

// Test with a unique probe only; never overwrite or delete a real backup.
func (s *Server) handleTestCloudBackup(w http.ResponseWriter, r *http.Request) {
	d, unlock, ok := s.cloudOperation(w)
	if !ok {
		return
	}
	defer unlock()
	if err := d.ensureDirectory(r.Context()); err != nil {
		cloudError(w, err)
		return
	}
	name, err := cloudUniqueName(".merit-probe-", ".txt")
	if err != nil {
		cloudError(w, errors.New("无法生成测试文件名"))
		return
	}
	data := []byte("MieruPilot WebDAV connection test")
	res, err := d.request(r.Context(), "PUT", name, data, map[string]string{"Content-Type": "text/plain", "If-None-Match": "*"})
	if err != nil {
		cloudError(w, err)
		return
	}
	res.Body.Close()
	if err := cloudStatus(res, 200, 201, 204); err != nil {
		cloudError(w, err)
		return
	}
	// Try cleanup even after read failure or client cancellation.
	cleanup := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := d.request(ctx, "DELETE", name, nil, nil)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		return cloudStatus(res, 200, 204, 404)
	}
	res, err = d.request(r.Context(), "GET", name, nil, nil)
	if err == nil {
		err = cloudStatus(res, 200)
		if err == nil {
			raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1024))
			if readErr != nil || !bytes.Equal(raw, data) {
				err = errors.New("WebDAV 测试文件读取校验失败")
			}
		}
		res.Body.Close()
	}
	cleanupErr := cleanup()
	if err != nil {
		if cleanupErr != nil {
			err = fmt.Errorf("%v；测试文件清理失败：%s", err, name)
		}
		cloudError(w, err)
		return
	}
	if cleanupErr != nil {
		cloudError(w, fmt.Errorf("读写成功，但测试文件清理失败，请检查删除权限：%s", name))
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "WebDAV 连接、上传、下载和删除测试均成功"})
}

func (s *Server) handleListCloudBackups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	d, unlock, ok := s.cloudOperation(w)
	if !ok {
		return
	}
	defer unlock()
	files, err := d.list(r.Context())
	if err != nil {
		cloudError(w, err)
		return
	}
	writeJSON(w, 200, files)
}

func (s *Server) handleUploadCloudBackup(w http.ResponseWriter, r *http.Request) {
	d, unlock, ok := s.cloudOperation(w)
	if !ok {
		return
	}
	defer unlock()
	raw, err := s.store.Export()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "导出备份失败"})
		return
	}
	if len(raw) > maxCloudBackup {
		writeJSON(w, 400, map[string]any{"error": "备份超过可恢复的 16 MiB 上限"})
		return
	}
	if err := d.ensureDirectory(r.Context()); err != nil {
		cloudError(w, err)
		return
	}
	name, err := cloudUniqueName("merit-backup-", ".json")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "生成备份文件名失败"})
		return
	}
	res, err := d.request(r.Context(), "PUT", name, raw, map[string]string{"Content-Type": "application/json", "If-None-Match": "*"})
	if err != nil {
		cloudError(w, err)
		return
	}
	res.Body.Close()
	if err := cloudStatus(res, 200, 201, 204); err != nil {
		cloudError(w, err)
		return
	}
	// Verify the exact bytes stored remotely before reporting a good backup.
	res, err = d.request(r.Context(), "GET", name, nil, nil)
	if err != nil {
		cloudError(w, fmt.Errorf("备份已上传，但下载验证失败：%s", name))
		return
	}
	defer res.Body.Close()
	if err := cloudStatus(res, 200); err != nil {
		cloudError(w, fmt.Errorf("备份已上传，但验证失败：%s", name))
		return
	}
	got, err := io.ReadAll(io.LimitReader(res.Body, maxCloudBackup+1))
	if err != nil || !bytes.Equal(got, raw) {
		cloudError(w, fmt.Errorf("远端备份校验失败，请勿用于恢复：%s", name))
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "name": name, "message": "云备份上传并校验成功"})
}

func (s *Server) handleRestoreCloudBackup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil || !cloudBackupName.MatchString(req.Name) || path.Base(req.Name) != req.Name {
		writeJSON(w, 400, map[string]any{"error": "请选择列表中的有效云备份文件"})
		return
	}
	d, unlock, ok := s.cloudOperation(w)
	if !ok {
		return
	}
	defer unlock()
	res, err := d.request(r.Context(), "GET", req.Name, nil, nil)
	if err != nil {
		cloudError(w, err)
		return
	}
	defer res.Body.Close()
	if err := cloudStatus(res, 200); err != nil {
		cloudError(w, err)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxCloudBackup+1))
	if err != nil || len(raw) > maxCloudBackup {
		writeJSON(w, 400, map[string]any{"error": "云备份读取失败或超过 16 MiB"})
		return
	}
	s.restoreBackup(w, raw)
}
