package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/teslausb-go/teslausb/internal/archive"
	"github.com/teslausb-go/teslausb/internal/ble"
	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/disk"
	"github.com/teslausb-go/teslausb/internal/gadget"
	"github.com/teslausb-go/teslausb/internal/notify"
	"github.com/teslausb-go/teslausb/internal/system"
	"github.com/teslausb-go/teslausb/internal/webhook"
)

type State string

const (
	StateBooting   State = "booting"
	StateAway      State = "away"
	StateArriving  State = "arriving"
	StateArchiving State = "archiving"
	StateIdle      State = "idle"
	StateError     State = "error"
)

type CumulativeStats struct {
	TotalClips   int       `json:"total_clips"`
	TotalBytes   int64     `json:"total_bytes"`
	ArchiveCount int       `json:"archive_count"`
	LastArchive  time.Time `json:"last_archive"`
}

type Machine struct {
	mu            sync.RWMutex
	state         State
	lastArchive   time.Time
	lastError     string
	archiveClips  int
	archiveBytes  int64
	cumulative    CumulativeStats
	gadgetEnabled bool
	listeners     []func(State)
}

const lastArchiveFile = "/mutable/teslausb/last_archive"
const statsFile = "/mutable/teslausb/stats.json"

func New() *Machine {
	m := &Machine{state: StateBooting}
	// Restore last archive timestamp
	if data, err := os.ReadFile(lastArchiveFile); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			m.lastArchive = t
		}
	}
	// Restore cumulative stats
	if data, err := os.ReadFile(statsFile); err == nil {
		json.Unmarshal(data, &m.cumulative)
	}
	return m
}

func (m *Machine) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Machine) Info() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]any{
		"state":               string(m.state),
		"last_archive":        m.lastArchive,
		"last_error":          m.lastError,
		"archive_clips":       m.archiveClips,
		"archive_bytes":       m.archiveBytes,
		"total_archive_clips": m.cumulative.TotalClips,
		"total_archive_bytes": m.cumulative.TotalBytes,
		"archive_count":       m.cumulative.ArchiveCount,
	}
}

// TriggerArchive forces a transition to arriving state if currently idle.
func (m *Machine) TriggerArchive() bool {
	if m.State() == StateIdle {
		m.setState(StateArriving)
		return true
	}
	return false
}

func (m *Machine) OnStateChange(fn func(State)) {
	m.mu.Lock()
	m.listeners = append(m.listeners, fn)
	m.mu.Unlock()
}

func (m *Machine) setState(s State) {
	m.mu.Lock()
	old := m.state
	m.state = s
	listeners := m.listeners
	m.mu.Unlock()

	if old != s {
		log.Printf("state: %s -> %s", old, s)
		for _, fn := range listeners {
			fn(s)
		}
	}
}

// Run starts the main state machine loop.
func (m *Machine) Run(ctx context.Context) error {
	// First-run: create disk image if needed
	if !disk.Exists() {
		log.Println("first run: creating cam disk image...")
		if err := disk.Create(); err != nil {
			return fmt.Errorf("create disk: %w", err)
		}
	}

	// Enable USB gadget (non-fatal — web UI should work even without UDC)
	if err := gadget.Enable(disk.BackingFile); err != nil {
		log.Printf("warning: %v (web UI still available, gadget will retry)", err)
		m.mu.Lock()
		m.lastError = err.Error()
		m.mu.Unlock()
	} else {
		m.gadgetEnabled = true
	}

	m.setState(StateAway)
	system.SetLED("slowblink")

	for {
		select {
		case <-ctx.Done():
			gadget.Disable()
			return nil
		default:
		}

		switch m.State() {
		case StateAway:
			m.runAway(ctx)
		case StateArriving:
			m.runArriving(ctx)
		case StateArchiving:
			m.runArchiving(ctx)
		case StateIdle:
			m.runIdle(ctx)
		}
	}
}

func (m *Machine) runAway(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Retry gadget enable if it failed (e.g. UDC wasn't available at boot)
			if !m.gadgetEnabled {
				if err := gadget.Enable(disk.BackingFile); err == nil {
					m.gadgetEnabled = true
					log.Println("USB gadget enabled (delayed)")
				}
			}
			if archive.IsReachable() {
				m.setState(StateArriving)
				return
			}
		}
	}
}

func (m *Machine) runArriving(ctx context.Context) {
	system.SetLED("fastblink")

	log.Println("archive server reachable, waiting 20s for network to stabilize...")
	time.Sleep(20 * time.Second)

	system.SyncTime()

	if err := gadget.WaitForIdle(); err != nil {
		log.Printf("wait for idle: %v", err)
	}

	if err := gadget.Disable(); err != nil {
		log.Printf("disable gadget: %v", err)
		m.gadgetEnabled = false
		m.setState(StateAway)
		return
	}
	m.gadgetEnabled = false

	notify.Send(ctx, webhook.Event{Event: "usb_disconnected", Message: "USB gadget disabled for archiving"})

	if err := disk.Mount(); err != nil {
		log.Printf("mount cam: %v", err)
		gadget.Enable(disk.BackingFile)
		m.setState(StateAway)
		return
	}

	disk.CleanArtifacts()

	if err := archive.MountArchive(); err != nil {
		log.Printf("mount archive: %v", err)
		disk.Unmount()
		gadget.Enable(disk.BackingFile)
		m.setState(StateAway)
		return
	}

	m.setState(StateArchiving)
}

func (m *Machine) runArchiving(ctx context.Context) {
	cfg := config.Get()

	m.sendKeepAwake(ctx, cfg, "start")

	keepAliveCtx, keepAliveCancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case <-ticker.C:
				m.sendKeepAwake(keepAliveCtx, cfg, "nudge")
			}
		}
	}()

	notify.Send(ctx, webhook.Event{Event: "archive_started", Message: "Archiving dashcam clips"})
	start := time.Now()
	clips, bytes, err := archive.ArchiveClips(ctx)
	duration := time.Since(start)

	keepAliveCancel()

	if err != nil {
		m.mu.Lock()
		m.lastError = err.Error()
		m.mu.Unlock()
		log.Printf("archive error: %v", err)
		notify.Send(ctx, webhook.Event{
			Event:   "archive_error",
			Message: err.Error(),
		})
	} else {
		now := time.Now()
		m.mu.Lock()
		m.lastArchive = now
		m.archiveClips = clips
		m.archiveBytes = bytes
		m.cumulative.TotalClips += clips
		m.cumulative.TotalBytes += bytes
		m.cumulative.ArchiveCount++
		m.cumulative.LastArchive = now
		cumSnapshot := m.cumulative
		m.mu.Unlock()
		os.WriteFile(lastArchiveFile, []byte(now.Format(time.RFC3339)), 0644)
		if statsData, err := json.Marshal(cumSnapshot); err == nil {
			if err := os.WriteFile(statsFile, statsData, 0644); err != nil {
				log.Printf("save stats: %v", err)
			}
		}
		notify.Send(ctx, webhook.Event{
			Event:   "archive_complete",
			Message: fmt.Sprintf("Archived %d clips in %s", clips, duration.Round(time.Second)),
			Data: map[string]any{
				"clips":            clips,
				"bytes":            bytes,
				"duration_seconds": int(duration.Seconds()),
			},
		})
	}

	archive.ManageFreeSpace()
	m.setState(StateIdle)
}

func (m *Machine) runIdle(ctx context.Context) {
	system.SetLED("heartbeat")

	cfg := config.Get()
	m.sendKeepAwake(ctx, cfg, "stop")

	archive.UnmountArchive()
	disk.Unmount()

	if err := gadget.Enable(disk.BackingFile); err != nil {
		log.Printf("warning: gadget re-enable failed: %v", err)
		m.gadgetEnabled = false
	} else {
		m.gadgetEnabled = true
		notify.Send(ctx, webhook.Event{Event: "usb_connected", Message: "USB gadget re-enabled"})
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Retry gadget if it failed
			if !m.gadgetEnabled {
				if err := gadget.Enable(disk.BackingFile); err == nil {
					m.gadgetEnabled = true
					log.Println("USB gadget enabled (delayed)")
					notify.Send(ctx, webhook.Event{Event: "usb_connected", Message: "USB gadget re-enabled"})
				}
			}
			if !archive.IsReachable() {
				log.Println("archive server unreachable — user left home")
				m.setState(StateAway)
				system.SetLED("slowblink")
				return
			}
		}
	}
}

func (m *Machine) sendKeepAwake(ctx context.Context, cfg *config.Config, command string) {
	if cfg == nil {
		return
	}
	switch cfg.KeepAwake.Method {
	case "ble":
		if cfg.KeepAwake.VIN != "" {
			if command == "stop" {
				ble.SentryOff(cfg.KeepAwake.VIN)
			} else {
				ble.KeepAwake(cfg.KeepAwake.VIN)
			}
		}
	case "webhook":
		if cfg.KeepAwake.WebhookURL != "" {
			// Send flat {"awake_command":"..."} matching original teslausb format
			webhook.SendRaw(ctx, cfg.KeepAwake.WebhookURL, map[string]string{"awake_command": command})
		}
	}
}
