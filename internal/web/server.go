package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/teslausb-go/teslausb/internal/ble"
	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/disk"
	"github.com/teslausb-go/teslausb/internal/monitor"
	"github.com/teslausb-go/teslausb/internal/state"
	"github.com/teslausb-go/teslausb/internal/update"
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
	mux.HandleFunc("POST /api/cifs/test", s.handleTestCIFS)
	mux.HandleFunc("POST /api/archive/trigger", s.handleTriggerArchive)
	mux.HandleFunc("POST /api/ble/pair", s.handleBLEPair)
	mux.HandleFunc("GET /api/ble/status", s.handleBLEStatus)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("GET /api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /api/update/apply", s.handleUpdateApply)
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

	// Network info
	net := monitor.GetNetworkInfo()
	info["wifi_ssid"] = net.SSID
	info["wifi_signal_dbm"] = net.SignalDBM
	info["wifi_ip"] = net.IP

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
	fullPath, err := safePath(disk.MountPoint, reqPath)
	if err != nil {
		http.Error(w, "invalid path", 400)
		return
	}
	reqPath = filepath.Clean(reqPath) // normalize for JSON response paths
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
	fullPath, err := safePath(disk.MountPoint, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "invalid path", 400)
		return
	}
	http.ServeFile(w, r, fullPath)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	fullPath, err := safePath(disk.MountPoint, req.Path)
	if err != nil {
		http.Error(w, "invalid path", 400)
		return
	}
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

	source := fmt.Sprintf("%s:%s", req.Server, req.Share)
	s.runMountTest(w, mountTest{
		host:    req.Server,
		port:    "2049",
		source:  source,
		testDir: "/tmp/nfs-test",
		mount: func(dir string) ([]byte, error) {
			cmd := exec.Command("mount", "-t", "nfs", source, dir,
				"-o", "ro,nolock,proto=tcp,vers=3,timeo=10,retrans=1")
			return cmd.CombinedOutput()
		},
	})
}

func (s *Server) handleTestCIFS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Server   string `json:"server"`
		Share    string `json:"share"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Server == "" || req.Share == "" {
		http.Error(w, "server and share required", 400)
		return
	}

	source := fmt.Sprintf("//%s/%s", req.Server, req.Share)
	credFile := "/tmp/.cifs-test-credentials"

	s.runMountTest(w, mountTest{
		host:    req.Server,
		port:    "445",
		source:  source,
		testDir: "/tmp/cifs-test",
		mount: func(dir string) ([]byte, error) {
			os.WriteFile(credFile, []byte(fmt.Sprintf("username=%s\npassword=%s\n", req.Username, req.Password)), 0600)
			opts := fmt.Sprintf("credentials=%s,vers=3.0", credFile)
			cmd := exec.Command("mount", "-t", "cifs", source, dir, "-o", opts)
			return cmd.CombinedOutput()
		},
		cleanup: func() { os.Remove(credFile) },
	})
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

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	release, err := update.CheckForUpdate(s.version)
	if err != nil {
		jsonResponse(w, map[string]any{"available": false, "error": err.Error()})
		return
	}
	if release == nil {
		jsonResponse(w, map[string]any{"available": false})
		return
	}
	jsonResponse(w, map[string]any{
		"available": true,
		"version":   release.TagName,
		"notes":     release.Body,
	})
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "updating"})

	// Apply in background — service will restart
	go func() {
		if err := update.Apply(s.version); err != nil {
			log.Printf("update error: %v", err)
		}
	}()
}
