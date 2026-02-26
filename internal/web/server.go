package web

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/teslausb-go/teslausb/internal/ble"
	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/disk"
	"github.com/teslausb-go/teslausb/internal/monitor"
	"github.com/teslausb-go/teslausb/internal/state"
)

type Server struct {
	machine  *state.Machine
	version  string
	hub      *Hub
	cfgPath  string
	staticFS fs.FS
}

func NewServer(machine *state.Machine, version, cfgPath string) *Server {
	return &Server{
		machine: machine,
		version: version,
		hub:     NewHub(),
		cfgPath: cfgPath,
	}
}

func (s *Server) SetStaticFS(staticFS fs.FS) {
	s.staticFS = staticFS
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/files", s.handleListFiles)
	mux.HandleFunc("GET /api/files/download", s.handleDownloadFile)
	mux.HandleFunc("POST /api/files/delete", s.handleDeleteFile)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/archive/trigger", s.handleTriggerArchive)
	mux.HandleFunc("POST /api/ble/pair", s.handleBLEPair)
	mux.HandleFunc("GET /api/ble/status", s.handleBLEStatus)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("/api/ws", s.hub.HandleWS)

	// Static files (React build)
	if s.staticFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.staticFS)))
	} else {
		mux.Handle("/", http.FileServer(http.Dir("web/dist")))
	}

	// Broadcast state changes to WebSocket clients
	s.machine.OnStateChange(func(st state.State) {
		s.hub.Broadcast(map[string]any{"type": "state", "state": string(st)})
	})

	go s.hub.Run()

	log.Printf("web server starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	info := s.machine.Info()
	info["version"] = s.version
	info["temperature"] = monitor.GetTemp()

	// Disk usage
	var stat syscall.Statfs_t
	if err := syscall.Statfs(disk.MountPoint, &stat); err == nil {
		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bavail) * int64(stat.Bsize)
		info["disk_total"] = total
		info["disk_free"] = free
		info["disk_used"] = total - free
	}

	jsonResponse(w, info)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "TeslaCam"
	}
	reqPath = filepath.Clean(reqPath)
	if strings.Contains(reqPath, "..") {
		http.Error(w, "invalid path", 400)
		return
	}
	fullPath := filepath.Join(disk.MountPoint, reqPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	type fileInfo struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
		Path  string `json:"path"`
	}
	var files []fileInfo
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		files = append(files, fileInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
			Path:  filepath.Join(reqPath, e.Name()),
		})
	}
	jsonResponse(w, files)
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	reqPath := filepath.Clean(r.URL.Query().Get("path"))
	if strings.Contains(reqPath, "..") {
		http.Error(w, "invalid path", 400)
		return
	}
	fullPath := filepath.Join(disk.MountPoint, reqPath)
	http.ServeFile(w, r, fullPath)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	req.Path = filepath.Clean(req.Path)
	if strings.Contains(req.Path, "..") {
		http.Error(w, "invalid path", 400)
		return
	}
	fullPath := filepath.Join(disk.MountPoint, req.Path)
	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg == nil {
		cfg = &config.Config{}
	}
	jsonResponse(w, cfg)
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := config.Save(s.cfgPath, &cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) handleTriggerArchive(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "triggered"})
}

func (s *Server) handleBLEPair(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VIN string `json:"vin"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.VIN == "" {
		http.Error(w, "VIN required", 400)
		return
	}
	if err := ble.Pair(req.VIN); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "pairing_requested"})
}

func (s *Server) handleBLEStatus(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	result := map[string]any{
		"keys_exist": ble.KeysExist(),
		"paired":     false,
	}
	if cfg != nil && cfg.KeepAwake.VIN != "" {
		result["paired"] = ble.IsPaired(cfg.KeepAwake.VIN)
	}
	jsonResponse(w, result)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("/mutable/logs/teslausb.log")
	if err != nil {
		jsonResponse(w, []string{})
		return
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}
	jsonResponse(w, lines)
}
