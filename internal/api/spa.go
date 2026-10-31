package api

import (
	"bytes"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// NewSPA serves the embedded client with history-mode fallback.
func NewSPA(fsys fs.FS, logger *slog.Logger) http.Handler {
	return &spaHandler{fsys: fsys, log: logger}
}

// spaHandler serves built assets and falls back to index.html for app routes.
type spaHandler struct {
	fsys fs.FS
	log  *slog.Logger
}

// ServeHTTP serves a real file when it exists and index.html for client routes.
func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}

	if data, err := fs.ReadFile(h.fsys, name); err == nil {
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		if ctype := mime.TypeByExtension(filepath.Ext(name)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
		return
	}

	index, err := fs.ReadFile(h.fsys, "index.html")
	if err != nil {
		h.log.Error("embedded UI missing; run the web build", "error", err)
		http.Error(w, "falafel: UI bundle not built", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
}
