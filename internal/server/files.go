package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"web-sealdice/internal/filemgr"
	"web-sealdice/internal/services"
)

// handleServiceFiles 是文件管理子路由入口，按 action 分发到具体处理函数。
func (s *Server) handleServiceFiles(w http.ResponseWriter, r *http.Request, serviceID string, extra []string) {
	if !s.cfg.FileManager.Enabled {
		writeJSON(w, http.StatusForbidden, jsonMessage{Message: "file manager is disabled by panel settings"})
		return
	}

	item, rootDir, ok := s.getServiceFileRoot(w, serviceID)
	if !ok {
		return
	}

	if len(extra) == 0 {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
			return
		}
		current, parent, entries, err := filemgr.List(rootDir, r.URL.Query().Get("path"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"service_id":   item.ID,
			"root":         rootDir,
			"current_path": current,
			"parent_path":  parent,
			"entries":      entries,
		})
		log.Printf("filemgr: list service=%s path=%s", serviceID, r.URL.Query().Get("path"))
		return
	}

	switch extra[0] {
	case "download":
		s.handleFileDownload(w, r, rootDir)
	case "upload":
		s.handleFileUpload(w, r, rootDir)
	case "mkdir":
		s.handleFileMkdir(w, r, rootDir)
	case "mkfile":
		s.handleFileMkfile(w, r, rootDir)
	case "delete":
		s.handleFileDelete(w, r, rootDir)
	case "rename":
		s.handleFileRename(w, r, rootDir)
	case "text":
		s.handleFileText(w, r, rootDir)
	case "copy":
		s.handleFileCopy(w, r, rootDir)
	case "compress":
		s.handleFileCompress(w, r, rootDir)
	case "extract":
		s.handleFileExtract(w, r, rootDir)
	default:
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "unknown file action"})
	}
}

// getServiceFileRoot 解析并校验服务文件根目录（InstallDir 优先，WorkDir 兜底）。
func (s *Server) getServiceFileRoot(w http.ResponseWriter, serviceID string) (services.Service, string, bool) {
	item, err := s.serviceStore.Get(serviceID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return services.Service{}, "", false
	}

	root := strings.TrimSpace(item.InstallDir)
	if root == "" {
		root = strings.TrimSpace(item.WorkDir)
	}
	if root == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "service root directory is not configured"})
		return services.Service{}, "", false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid service root directory"})
		return services.Service{}, "", false
	}
	if _, err := os.Stat(absRoot); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "service root directory does not exist"})
		return services.Service{}, "", false
	}
	return item, absRoot, true
}

// handleFileDownload 处理单文件下载请求。
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	rel := r.URL.Query().Get("path")
	full, cleanRel, err := filemgr.ResolvePath(root, rel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	if info.IsDir() {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "cannot download a directory"})
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(cleanRel)))
	http.ServeFile(w, r, full)
}

// handleFileUpload 处理多文件上传并写入目标目录。
func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	maxBytes := int64(s.cfg.FileManager.UploadMaxMB) << 20
	if maxBytes <= 0 {
		maxBytes = 512 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1<<20)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid multipart form"})
		return
	}

	dirRel := r.URL.Query().Get("path")
	dirAbs, _, err := filemgr.ResolvePath(root, dirRel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	info, err := os.Stat(dirAbs)
	if err != nil || !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "target path must be a directory"})
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "no files uploaded"})
		return
	}

	uploaded := make([]string, 0, len(files))
	for _, fh := range files {
		if fh.Size > maxBytes {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "file too large for current upload limit"})
			return
		}
		in, err := fh.Open()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		targetName := path.Base(fh.Filename)
		if targetName == "." || targetName == "/" || targetName == "" {
			_ = in.Close()
			continue
		}
		targetPath := filepath.Join(dirAbs, targetName)
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = in.Close()
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		_ = in.Close()
		_ = out.Close()
		uploaded = append(uploaded, targetName)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uploaded": uploaded,
		"message":  "upload completed",
	})
	log.Printf("filemgr: upload root=%s count=%d", root, len(uploaded))
}

// handleFileText 读取或保存文本文件内容，用于在线编辑器。
func (s *Server) handleFileText(w http.ResponseWriter, r *http.Request, root string) {
	switch r.Method {
	case http.MethodGet:
		rel := r.URL.Query().Get("path")
		content, err := filemgr.ReadText(root, rel, 2<<20)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"path":    filemgr.CleanRelPath(rel),
			"content": content,
		})
	case http.MethodPost:
		// reqBody 定义当前接口的请求体结构。
		type reqBody struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
			return
		}
		if err := filemgr.WriteText(root, body.Path, body.Content); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, jsonMessage{Message: "file saved"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
	}
}

// handleFileMkdir 在指定父目录创建子目录。
func (s *Server) handleFileMkdir(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if err := filemgr.CreateDir(root, body.Parent, body.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: mkdir root=%s parent=%s name=%s", root, body.Parent, body.Name)
	writeJSON(w, http.StatusOK, jsonMessage{Message: "directory created"})
}

// handleFileMkfile 在指定父目录创建空文件。
func (s *Server) handleFileMkfile(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if err := filemgr.CreateFile(root, body.Parent, body.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: mkfile root=%s parent=%s name=%s", root, body.Parent, body.Name)
	writeJSON(w, http.StatusOK, jsonMessage{Message: "file created"})
}

// handleFileDelete 删除一个或多个文件/目录。
func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Paths []string `json:"paths"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if err := filemgr.DeletePaths(root, body.Paths); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: delete root=%s count=%d", root, len(body.Paths))
	writeJSON(w, http.StatusOK, jsonMessage{Message: "delete completed"})
}

// handleFileRename 重命名文件或目录。
func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Path    string `json:"path"`
		NewName string `json:"new_name"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	newPath, err := filemgr.RenamePath(root, body.Path, body.NewName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: rename root=%s path=%s new_name=%s", root, body.Path, body.NewName)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":  "rename completed",
		"new_path": newPath,
	})
}

// handleFileCopy 执行复制/粘贴操作。
func (s *Server) handleFileCopy(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Sources     []string `json:"sources"`
		Destination string   `json:"destination"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if len(body.Sources) == 0 {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "no sources selected"})
		return
	}
	if err := filemgr.CopyPaths(root, body.Sources, body.Destination); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: copy root=%s count=%d destination=%s", root, len(body.Sources), body.Destination)
	writeJSON(w, http.StatusOK, jsonMessage{Message: "paste completed"})
}

// handleFileCompress 将多个输入项压缩为 zip。
func (s *Server) handleFileCompress(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Sources     []string `json:"sources"`
		Destination string   `json:"destination"`
		OutputName  string   `json:"output_name"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	archiveRel, err := filemgr.CompressToZip(root, body.Sources, body.Destination, body.OutputName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: compress root=%s count=%d destination=%s output=%s", root, len(body.Sources), body.Destination, body.OutputName)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":      "compress completed",
		"archive_path": archiveRel,
	})
}

// handleFileExtract 解压 zip/tar.gz/tgz 到目标目录。
func (s *Server) handleFileExtract(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 定义当前接口的请求体结构。
	type reqBody struct {
		Path        string `json:"path"`
		Destination string `json:"destination"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if strings.TrimSpace(body.Path) == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "path is required"})
		return
	}
	dst := body.Destination
	if strings.TrimSpace(dst) == "" {
		dst = path.Dir(filemgr.CleanRelPath(body.Path))
	}
	err := filemgr.ExtractArchive(root, body.Path, dst)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("filemgr: extract root=%s archive=%s destination=%s", root, body.Path, dst)
	writeJSON(w, http.StatusOK, jsonMessage{Message: "extract completed"})
}
