package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type uploadResponse struct {
	URL string `json:"url"`
}

// HandleUpload обслуживает POST /api/upload
// Принимает multipart/form-data с полем file, сохраняет файл в UploadDir
// и возвращает { "url": "/uploads/имяфайла" }.
func (e *Env) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if e.UploadDir == "" {
		e.UploadDir = "../frontend/uploads"
	}

	if err := os.MkdirAll(e.UploadDir, 0755); err != nil {
		http.Error(w, "cannot create upload dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 МБ
		http.Error(w, "bad multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".bin"
	}
	nameOnly := strings.TrimSuffix(filepath.Base(header.Filename), ext)
	nameOnly = sanitizeFilename(nameOnly)

	filename := time.Now().Format("20060102_150405") + "_" + nameOnly + ext
	dstPath := filepath.Join(e.UploadDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "cannot create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "cannot save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	publicURL := "/uploads/" + filename

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(uploadResponse{URL: publicURL})
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "file"
	}
	// простая зачистка
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}
