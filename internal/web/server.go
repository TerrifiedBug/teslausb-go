package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
	mux.HandleFunc("POST /api/nfs/test", s.handleTestNFS)
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
		jsonResponse(w, []any{})
		return
	}

	type fileInfo struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
		Path  string `json:"path"`
	}
	files := make([]fileInfo, 0)
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

func (s *Server) handleTestNFS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Server string `json:"server"`
		Share  string `json:"share"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Server == "" || req.Share == "" {
		http.Error(w, "server and share required", 400)
		return
	}

	// Test TCP connectivity to NFS port
	conn, err := net.DialTimeout("tcp", req.Server+":2049", 5*time.Second)
	if err != nil {
		jsonResponse(w, map[string]any{"ok": false, "error": fmt.Sprintf("Cannot reach %s:2049 — %v", req.Server, err)})
		return
	}
	conn.Close()

	// Try a temporary mount
	testDir := "/tmp/nfs-test"
	os.MkdirAll(testDir, 0755)
	defer func() {
		exec.Command("umount", "-f", "-l", testDir).Run()
		os.Remove(testDir)
	}()

	source := fmt.Sprintf("%s:%s", req.Server, req.Share)
	out, err := exec.Command("mount", "-t", "nfs", source, testDir, "-o", "ro,nolock,proto=tcp,vers=3,timeo=10,retrans=1").CombinedOutput()
	if err != nil {
		jsonResponse(w, map[string]any{"ok": false, "error": fmt.Sprintf("Mount failed: %s", strings.TrimSpace(string(out)))})
		return
	}

	jsonResponse(w, map[string]any{"ok": true, "message": fmt.Sprintf("Successfully mounted %s", source)})
}

func (s *Server) handleTriggerArchive(w http.ResponseWriter, r *http.Request) {
	if s.machine.TriggerArchive() {
		jsonResponse(w, map[string]string{"status": "triggered"})
	} else {
		jsonResponse(w, map[string]string{"status": "not_idle", "error": "can only trigger archive from idle state"})
	}
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
	jsonResponse(w, map[string]any{
		"keys_exist": ble.KeysExist(),
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("journalctl", "-u", "teslausb", "-n", "100", "--no-pager", "-o", "short-iso").Output()
	if err != nil {
		jsonResponse(w, []string{})
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	jsonResponse(w, lines)
}
