package agent

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// executableInstalled only accepts the exact path used by ExecStart. A binary
// with the same name elsewhere in PATH cannot satisfy a managed installation.
func executableInstalled(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0 &&
		(runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0)
}

// mirrorURL prefixes a full download URL with the configured mirror. Mirrors are
// plain prefix proxies (for example https://ghfast.top/https://github.com/...),
// so the scheme of the original URL has to be kept intact.
func mirrorURL(mirror, rawURL string) string {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return rawURL
	}
	return strings.TrimRight(mirror, "/") + "/" + rawURL
}

// installExecutable copies src to dst with mode 0755. The copy goes through a
// temporary file created next to dst so that installing from /tmp onto another
// filesystem (EXDEV) cannot fail, and so that a running binary is replaced
// atomically instead of being truncated in place.
func installExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".merit-install-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

// extractArchive unpacks a zip archive or a tar archive (.tar, .tar.gz, .tgz,
// .tar.xz, .txz) into destDir. Tar archives are delegated to the system tar,
// which handles both gzip and xz without extra Go dependencies.
func extractArchive(archivePath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar"),
		strings.HasSuffix(lower, ".tar.gz"),
		strings.HasSuffix(lower, ".tgz"),
		strings.HasSuffix(lower, ".tar.xz"),
		strings.HasSuffix(lower, ".txz"):
		cmd := exec.Command("tar", "-xf", archivePath, "-C", destDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			hint := ""
			if strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz") {
				hint = "（tar 解压 xz 依赖 xz-utils，请先安装：apt install xz-utils 或 yum install xz）"
			}
			return fmt.Errorf("解压 %s 失败: %v %s%s", filepath.Base(archivePath), err, strings.TrimSpace(string(out)), hint)
		}
		return nil
	default:
		return fmt.Errorf("不支持的压缩包格式: %s", filepath.Base(archivePath))
	}
}

// extractZip unpacks a zip archive, rejecting entries that would escape destDir.
func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	root := filepath.Clean(destDir) + string(os.PathSeparator)
	for _, entry := range reader.File {
		target := filepath.Join(destDir, entry.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), root) {
			return fmt.Errorf("压缩包包含越界路径: %s", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := entry.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		if err := writeZipEntry(entry, target, mode); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(entry *zip.File, target string, mode os.FileMode) error {
	src, err := entry.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

// findBinary returns the first regular file called name below root. Release
// archives either keep binaries at the top level or inside a versioned folder,
// so the whole tree has to be searched.
func findBinary(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == name {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("归档中未找到 %s", name)
	}
	return found, nil
}

// systemdEscapeArg quotes one program argument for an ExecStart line. Unit files
// give a special meaning to spaces, quotes, backslashes, "$" (variable
// expansion) and "%" (specifier expansion), all of which may legitimately appear
// in generated proxy passwords.
func systemdEscapeArg(arg string) string {
	needsQuote := arg == "" || strings.ContainsAny(arg, " \t\n\r\"'\\$%`*?[]#;{}()<>|&")
	if !needsQuote {
		return arg
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `$$`,
		`%`, `%%`,
	)
	return `"` + replacer.Replace(arg) + `"`
}

// systemdExecStart renders a full ExecStart value, escaping every argument.
func systemdExecStart(program string, args ...string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, systemdEscapeArg(program))
	for _, arg := range args {
		parts = append(parts, systemdEscapeArg(arg))
	}
	return strings.Join(parts, " ")
}
