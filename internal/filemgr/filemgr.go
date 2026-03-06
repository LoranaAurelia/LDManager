package filemgr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry 表示文件管理列表中的单个条目。
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// editableExt 定义允许在线文本编辑的文件后缀集合。
var editableExt = map[string]bool{
	".txt": true, ".md": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".ini": true, ".conf": true, ".cfg": true, ".env": true,
	".xml": true, ".log": true, ".csv": true, ".js": true, ".ts": true,
	".go": true, ".py": true, ".sh": true, ".html": true, ".css": true,
	".properties": true,
}

// CleanRelPath 规范化用户相对路径输入，统一为安全的相对路径。
func CleanRelPath(rel string) string {
	v := strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	v = path.Clean("/" + v)
	v = strings.TrimPrefix(v, "/")
	if v == "." {
		return ""
	}
	return v
}

// ResolvePath 将相对路径解析为绝对路径，并校验不越出根目录。
func ResolvePath(root string, rel string) (string, string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	cleanRoot = filepath.Clean(cleanRoot)
	if realRoot, err := filepath.EvalSymlinks(cleanRoot); err == nil {
		cleanRoot = filepath.Clean(realRoot)
	}

	cleanRel := CleanRelPath(rel)
	full := filepath.Join(cleanRoot, filepath.FromSlash(cleanRel))
	full = filepath.Clean(full)
	if full != cleanRoot && !strings.HasPrefix(full, cleanRoot+string(os.PathSeparator)) {
		return "", "", errors.New("invalid path")
	}
	if err := ensureNoSymlinkEscape(cleanRoot, full); err != nil {
		return "", "", err
	}
	return full, cleanRel, nil
}

// ensureNoSymlinkEscape 防止通过符号链接跳出根目录边界。
func ensureNoSymlinkEscape(cleanRoot string, full string) error {
	existing := full
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return nil
		}
		existing = parent
	}

	realExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	realExisting = filepath.Clean(realExisting)
	if realExisting != cleanRoot && !strings.HasPrefix(realExisting, cleanRoot+string(os.PathSeparator)) {
		return errors.New("invalid path")
	}
	return nil
}

// List 列出目录内容并返回当前路径、父路径与条目列表。
func List(root string, rel string) (string, string, []Entry, error) {
	full, cleanRel, err := ResolvePath(root, rel)
	if err != nil {
		return "", "", nil, err
	}

	entries, err := os.ReadDir(full)
	if err != nil {
		return "", "", nil, err
	}

	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entryRel := path.Join(cleanRel, entry.Name())
		result = append(result, Entry{
			Name:    entry.Name(),
			Path:    entryRel,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	parent := ""
	if cleanRel != "" {
		parent = path.Dir(cleanRel)
		if parent == "." {
			parent = ""
		}
	}
	return cleanRel, parent, result, nil
}

// ReadText 读取文本文件内容，包含大小限制与文本类型校验。
func ReadText(root string, rel string, maxBytes int64) (string, error) {
	full, _, err := ResolvePath(root, rel)
	if err != nil {
		return "", err
	}
	stat, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if stat.IsDir() {
		return "", errors.New("path is a directory")
	}
	if stat.Size() > maxBytes {
		return "", fmt.Errorf("file too large to edit (>%d bytes)", maxBytes)
	}
	if !IsEditableFile(full) {
		return "", errors.New("file type is not editable")
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if !looksLikeText(raw) {
		return "", errors.New("file is not text")
	}
	return string(raw), nil
}

// WriteText 写回文本文件并保持原文件权限。
func WriteText(root string, rel string, content string) error {
	full, _, err := ResolvePath(root, rel)
	if err != nil {
		return err
	}
	stat, err := os.Stat(full)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return errors.New("path is a directory")
	}
	if !IsEditableFile(full) {
		return errors.New("file type is not editable")
	}
	return os.WriteFile(full, []byte(content), stat.Mode().Perm())
}

// IsEditableFile 判断文件是否允许在前端编辑器中修改。
func IsEditableFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return editableExt[ext]
}

// CopyPaths 将多个源路径复制到目标目录，自动处理重名。
func CopyPaths(root string, sources []string, destination string) error {
	dstAbs, _, err := ResolvePath(root, destination)
	if err != nil {
		return err
	}
	dstInfo, err := os.Stat(dstAbs)
	if err != nil {
		return err
	}
	if !dstInfo.IsDir() {
		return errors.New("destination must be a directory")
	}

	for _, src := range sources {
		srcAbs, _, err := ResolvePath(root, src)
		if err != nil {
			return err
		}
		info, err := os.Stat(srcAbs)
		if err != nil {
			return err
		}
		targetName := uniqueName(dstAbs, filepath.Base(srcAbs))
		dstPath := filepath.Join(dstAbs, targetName)
		if info.IsDir() {
			if err := copyDir(srcAbs, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcAbs, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CompressToZip 将选中项压缩为 zip，并返回生成文件相对路径。
func CompressToZip(root string, sources []string, destination string, outputName string) (string, error) {
	if len(sources) == 0 {
		return "", errors.New("no source selected")
	}
	if strings.TrimSpace(outputName) == "" {
		outputName = "archive-" + time.Now().Format("20060102-150405") + ".zip"
	}
	if !strings.HasSuffix(strings.ToLower(outputName), ".zip") {
		outputName += ".zip"
	}

	dstAbs, dstRel, err := ResolvePath(root, destination)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dstAbs, 0o755); err != nil {
		return "", err
	}

	outName := uniqueName(dstAbs, outputName)
	outAbs := filepath.Join(dstAbs, outName)
	outRel := path.Join(dstRel, outName)

	file, err := os.Create(outAbs)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	zw := zip.NewWriter(file)
	defer func() { _ = zw.Close() }()

	for _, src := range sources {
		srcAbs, srcRel, err := ResolvePath(root, src)
		if err != nil {
			return "", err
		}
		if err := addZipPath(zw, srcAbs, srcRel); err != nil {
			return "", err
		}
	}
	return outRel, nil
}

// ExtractArchive 解压 zip/tar.gz/tgz 到目标目录。
func ExtractArchive(root string, archiveRel string, destination string) error {
	archiveAbs, _, err := ResolvePath(root, archiveRel)
	if err != nil {
		return err
	}
	dstAbs, _, err := ResolvePath(root, destination)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstAbs, 0o755); err != nil {
		return err
	}

	lower := strings.ToLower(archiveAbs)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archiveAbs, dstAbs)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGzFile(archiveAbs, dstAbs)
	default:
		return errors.New("unsupported archive type")
	}
}

// CreateDir 在指定父目录下创建子目录。
func CreateDir(root string, parentRel string, name string) error {
	parentAbs, _, err := ResolvePath(root, parentRel)
	if err != nil {
		return err
	}
	info, err := os.Stat(parentAbs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("parent path is not directory")
	}
	cleanName, err := sanitizeName(name)
	if err != nil {
		return err
	}
	target := filepath.Join(parentAbs, cleanName)
	if _, err := os.Stat(target); err == nil {
		return errors.New("target already exists")
	}
	return os.MkdirAll(target, 0o755)
}

// CreateFile 在指定父目录下创建空文件。
func CreateFile(root string, parentRel string, name string) error {
	parentAbs, _, err := ResolvePath(root, parentRel)
	if err != nil {
		return err
	}
	info, err := os.Stat(parentAbs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("parent path is not directory")
	}
	cleanName, err := sanitizeName(name)
	if err != nil {
		return err
	}
	target := filepath.Join(parentAbs, cleanName)
	if _, err := os.Stat(target); err == nil {
		return errors.New("target already exists")
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

// DeletePaths 批量删除文件或目录。
func DeletePaths(root string, paths []string) error {
	if len(paths) == 0 {
		return errors.New("no path selected")
	}
	for _, rel := range paths {
		full, cleanRel, err := ResolvePath(root, rel)
		if err != nil {
			return err
		}
		if cleanRel == "" {
			return errors.New("cannot delete root directory")
		}
		if err := os.RemoveAll(full); err != nil {
			return err
		}
	}
	return nil
}

// RenamePath 重命名路径并返回新相对路径。
func RenamePath(root string, relPath string, newName string) (string, error) {
	full, cleanRel, err := ResolvePath(root, relPath)
	if err != nil {
		return "", err
	}
	if cleanRel == "" {
		return "", errors.New("cannot rename root directory")
	}
	cleanName, err := sanitizeName(newName)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(full)
	target := filepath.Join(parent, cleanName)
	if _, err := os.Stat(target); err == nil {
		return "", errors.New("target already exists")
	}
	if err := os.Rename(full, target); err != nil {
		return "", err
	}
	parentRel := path.Dir(cleanRel)
	if parentRel == "." {
		parentRel = ""
	}
	return path.Join(parentRel, cleanName), nil
}

// looksLikeText 使用轻量规则判断字节流是否近似文本。
func looksLikeText(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

// uniqueName 为重名目标生成不冲突文件名。
func uniqueName(dir string, name string) string {
	candidate := name
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	i := 1
	for {
		if _, err := os.Stat(filepath.Join(dir, candidate)); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = fmt.Sprintf("%s_copy%d%s", base, i, ext)
		i++
	}
}

// sanitizeName 校验并清洗文件名，拒绝路径分隔符与危险名称。
func sanitizeName(name string) (string, error) {
	v := strings.TrimSpace(name)
	if v == "" {
		return "", errors.New("name is required")
	}
	if strings.Contains(v, "/") || strings.Contains(v, "\\") {
		return "", errors.New("name cannot contain path separator")
	}
	if v == "." || v == ".." {
		return "", errors.New("invalid name")
	}
	return v, nil
}

// copyFile 复制单文件内容并保留权限位。
func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}

// copyDir 递归复制目录。
func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(pathNow string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, pathNow)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(pathNow, target)
	})
}

// addZipPath 将文件或目录递归写入 zip。
func addZipPath(zw *zip.Writer, srcAbs string, srcRel string) error {
	info, err := os.Stat(srcAbs)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.WalkDir(srcAbs, func(pathNow string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(filepath.Dir(srcAbs), pathNow)
			if err != nil {
				return err
			}
			return addZipFile(zw, pathNow, filepath.ToSlash(rel))
		})
	}

	return addZipFile(zw, srcAbs, filepath.ToSlash(path.Base(srcRel)))
}

// addZipFile 将单文件写入 zip。
func addZipFile(zw *zip.Writer, filePath string, archiveName string) error {
	in, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archiveName
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

// extractZip 安全解压 zip，防止路径穿越。
func extractZip(archivePath string, dst string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	cleanDst := filepath.Clean(dst)
	for _, f := range zr.File {
		name := strings.Trim(filepath.ToSlash(f.Name), "/")
		if name == "" {
			continue
		}
		target := filepath.Join(cleanDst, filepath.FromSlash(name))
		target = filepath.Clean(target)
		if target != cleanDst && !strings.HasPrefix(target, cleanDst+string(os.PathSeparator)) {
			return errors.New("invalid archive entry path")
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			return err
		}
		_ = in.Close()
		_ = out.Close()
	}
	return nil
}

// extractTarGzFile 安全解压 tar.gz/tgz，防止路径穿越。
func extractTarGzFile(archivePath string, dst string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	cleanDst := filepath.Clean(dst)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		name := strings.Trim(filepath.ToSlash(hdr.Name), "/")
		if name == "" {
			continue
		}
		target := filepath.Join(cleanDst, filepath.FromSlash(name))
		target = filepath.Clean(target)
		if target != cleanDst && !strings.HasPrefix(target, cleanDst+string(os.PathSeparator)) {
			return errors.New("invalid archive entry path")
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			_ = out.Close()
		default:
		}
	}
	return nil
}
