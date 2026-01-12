// Copyright (C) by Ubaldo Porcheddu <ubaldo@eja.it>

package maps

import (
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed assets
var assetsFS embed.FS

type Maps struct {
	webPath   string
	filePath  string
	webAuth   string
	isFile    bool
	fileName  string
	templates *template.Template
}

func New(webPath, filePath, webAuth string) http.Handler {
	cleanPath, err := filepath.Abs(filePath)
	if err != nil {
		cleanPath = filePath
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		log.Fatalf("Error accessing file path '%s': %v", cleanPath, err)
	}

	isFile := !info.IsDir()
	fileName := ""
	if isFile {
		fileName = filepath.Base(cleanPath)
		cleanPath = filepath.Dir(cleanPath)
	}

	tmpl, err := template.ParseFS(assetsFS, "assets/index.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	return &Maps{
		webPath:   webPath,
		filePath:  cleanPath,
		webAuth:   webAuth,
		isFile:    isFile,
		fileName:  fileName,
		templates: tmpl,
	}
}

func (m *Maps) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.webAuth != "" {
		if !m.checkAuth(w, r) {
			return
		}
	}

	if !strings.HasPrefix(r.URL.Path, m.webPath) {
		http.NotFound(w, r)
		return
	}

	reqPath := strings.TrimPrefix(r.URL.Path, m.webPath)
	if reqPath == "" || reqPath == "/" {
		if m.isFile {
			target := m.webPath + "map/" + m.fileName
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		http.Error(w, "Map server running. Access /"+m.webPath+"map/{filename}", http.StatusOK)
		return
	}

	if strings.HasPrefix(reqPath, "web/") {
		m.serveStatic(w, strings.TrimPrefix(reqPath, "web/"))
		return
	}

	if strings.HasPrefix(reqPath, "map/") {
		m.serveMap(w, r, strings.TrimPrefix(reqPath, "map/"))
		return
	}

	http.NotFound(w, r)
}

func (m *Maps) serveStatic(w http.ResponseWriter, path string) {
	fsPath := "assets/" + path //?

	f, err := assetsFS.Open(fsPath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	defer f.Close()

	ext := filepath.Ext(path)
	mime := "application/octet-stream"
	switch ext {
	case ".css":
		mime = "text/css"
	case ".js":
		mime = "application/javascript"
	case ".json":
		mime = "application/json"
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	io.Copy(w, f)
}

func (m *Maps) serveMap(w http.ResponseWriter, r *http.Request, relativePath string) {
	if strings.HasSuffix(relativePath, "/metadata.json") {
		fPath := strings.TrimSuffix(relativePath, "/metadata.json")
		m.handleMetadata(w, fPath)
		return
	}

	parts := strings.Split(relativePath, "/")
	n := len(parts)
	if n >= 4 {
		z, errZ := strconv.Atoi(parts[n-3])
		x, errX := strconv.Atoi(parts[n-2])
		y, errY := strconv.Atoi(parts[n-1])

		if errZ == nil && errX == nil && errY == nil {
			filename := strings.Join(parts[:n-3], "/")
			if strings.HasSuffix(filename, ".mbtiles") {
				m.handleMBTiles(w, filename, z, x, y)
				return
			}
		}
	}

	if strings.HasSuffix(relativePath, ".mbtiles") || strings.HasSuffix(relativePath, ".pmtiles") {

		if strings.HasSuffix(relativePath, ".pmtiles") && r.Header.Get("Range") != "" {
			m.serveRawFile(w, r, relativePath)
			return
		}

		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			data := map[string]interface{}{
				"RootPath": m.webPath + "web",
				"File":     m.webPath + "map/" + relativePath,
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			m.templates.ExecuteTemplate(w, "index.html", data)
			return
		}

		m.serveRawFile(w, r, relativePath)
		return
	}

	http.NotFound(w, r)
}

func (m *Maps) handleMetadata(w http.ResponseWriter, filename string) {
	absPath, err := m.resolveFilePath(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	db, err := sql.Open("sqlite", absPath)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT name, value FROM metadata")
	if err != nil {
		http.Error(w, "Metadata read error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	meta := make(map[string]interface{})
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		var j interface{}
		if json.Unmarshal([]byte(v), &j) == nil {
			meta[k] = j
		} else {
			meta[k] = v
		}
	}

	for _, k := range []string{"minzoom", "maxzoom"} {
		if s, ok := meta[k].(string); ok {
			if i, err := strconv.Atoi(s); err == nil {
				meta[k] = i
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func (m *Maps) handleMBTiles(w http.ResponseWriter, filename string, z, x, y int) {
	absPath, err := m.resolveFilePath(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	db, err := sql.Open("sqlite", absPath)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	tmsY := (1 << z) - 1 - y
	var data []byte

	err = db.QueryRow("SELECT tile_data FROM tiles WHERE zoom_level=? AND tile_column=? AND tile_row=?", z, x, tmsY).Scan(&data)
	if err == sql.ErrNoRows {
		http.Error(w, "Tile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/x-protobuf")
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.Write(data)
}

func (m *Maps) serveRawFile(w http.ResponseWriter, r *http.Request, filename string) {
	absPath, err := m.resolveFilePath(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, absPath)
}

func (m *Maps) resolveFilePath(reqFile string) (string, error) {
	if m.isFile {
		if reqFile != m.fileName {
			return "", fmt.Errorf("file mismatch")
		}
		return filepath.Join(m.filePath, m.fileName), nil
	}

	full := filepath.Join(m.filePath, reqFile)
	clean := filepath.Clean(full)

	if !strings.HasPrefix(clean, m.filePath) {
		return "", fmt.Errorf("access denied")
	}

	if _, err := os.Stat(clean); os.IsNotExist(err) {
		return "", fmt.Errorf("not found")
	}

	return clean, nil
}

func (m *Maps) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Basic ") {
		payload := auth[6:]
		if subtle.ConstantTimeCompare([]byte(payload), []byte(m.webAuth)) == 1 {
			return true
		}
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="Maps"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}
