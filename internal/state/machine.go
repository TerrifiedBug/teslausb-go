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
	deps          Deps
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

const (
	pollInterval          = 30 * time.Second
	networkStabilizeDelay = 20 * time.Second
	keepAliveInterval     = 5 * time.Minute
)

func New(deps Deps) *Machine {
	m := &Machine{state: StateBooting, deps: deps}
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

// tryEnableGadget attempts to enable the USB gadget if not already enabled.
// Returns true if the gadget was just enabled (for callers that want to notify).
func (m *Machine) tryEnableGadget() bool {
	if m.gadgetEnabled {
		return false
	}
	if err := m.deps.Gadget.Enable(m.deps.Disk.BackingFilePath()); err != nil {
		return false
	}
	m.gadgetEnabled = true
	log.Println("USB gadget enabled (delayed)")
	return true
}

// Run starts the main state machine loop.
func (m *Machine) Run(ctx context.Context) error {
	if !m.deps.Disk.Exists() {
		log.Println("first run: creating cam disk image...")
		if err := m.deps.Disk.Create(); err != nil {
			return fmt.Errorf("create disk: %w", err)
		}
	}

	if err := m.deps.Gadget.Enable(m.deps.Disk.BackingFilePath()); err != nil {
		log.Printf("warning: %v (web UI still available, gadget will retry)", err)
		m.mu.Lock()
		m.lastError = err.Error()
		m.mu.Unlock()
	} else {
		m.gadgetEnabled = true
	}

	m.setState(StateAway)
	m.deps.System.SetLED("slowblink")

	for {
		select {
		case <-ctx.Done():
			m.deps.Gadget.Disable()
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
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tryEnableGadget()
			if m.deps.Archive.IsReachable() {
				m.setState(StateArriving)
				return
			}
		}
	}
}

func (m *Machine) runArriving(ctx context.Context) {
	m.deps.System.SetLED("fastblink")

	log.Println("archive server reachable, waiting for network to stabilize...")
	time.Sleep(networkStabilizeDelay)

	m.deps.System.SyncTime()

	if err := m.deps.Gadget.WaitForIdle(); err != nil {
		log.Printf("wait for idle: %v", err)
	}

	if err := m.deps.Gadget.Disable(); err != nil {
		log.Printf("disable gadget: %v", err)
		m.gadgetEnabled = false
		m.setState(StateAway)
		return
	}
	m.gadgetEnabled = false

	m.deps.Notify.Send(ctx, webhook.Event{Event: "usb_disconnected", Message: "USB gadget disabled for archiving"})

	if err := m.deps.Disk.Mount(); err != nil {
		log.Printf("mount cam: %v", err)
		m.deps.Gadget.Enable(m.deps.Disk.BackingFilePath())
		m.setState(StateAway)
		return
	}

	m.deps.Disk.CleanArtifacts()

	if err := m.deps.Archive.MountArchive(); err != nil {
		log.Printf("mount archive: %v", err)
		m.deps.Disk.Unmount()
		m.deps.Gadget.Enable(m.deps.Disk.BackingFilePath())
		m.setState(StateAway)
		return
	}

	m.setState(StateArchiving)
}

// startKeepAlive begins periodic keep-awake nudges and returns a cancel function.
func (m *Machine) startKeepAlive(ctx context.Context) context.CancelFunc {
	m.deps.KeepAwake.Send(ctx, "start")
	keepAliveCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(keepAliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case <-ticker.C:
				m.deps.KeepAwake.Send(keepAliveCtx, "nudge")
			}
		}
	}()
	return cancel
}

// updateAndPersistStats records archive results in memory and writes to disk.
func (m *Machine) updateAndPersistStats(clips int, bytes int64) {
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
	if data, err := json.Marshal(cumSnapshot); err == nil {
		if err := os.WriteFile(statsFile, data, 0644); err != nil {
			log.Printf("save stats: %v", err)
		}
	}
}

func (m *Machine) runArchiving(ctx context.Context) {
	stopKeepAlive := m.startKeepAlive(ctx)

	m.deps.Notify.Send(ctx, webhook.Event{Event: "archive_started", Message: "Archiving dashcam clips"})
	start := time.Now()
	clips, bytes, err := m.deps.Archive.ArchiveClips(ctx)
	duration := time.Since(start)

	stopKeepAlive()

	if err != nil {
		m.mu.Lock()
		m.lastError = err.Error()
		m.mu.Unlock()
		log.Printf("archive error: %v", err)
		m.deps.Notify.Send(ctx, webhook.Event{
			Event:   "archive_error",
			Message: err.Error(),
		})
	} else {
		m.updateAndPersistStats(clips, bytes)
		m.deps.Notify.Send(ctx, webhook.Event{
			Event:   "archive_complete",
			Message: fmt.Sprintf("Archived %d clips in %s", clips, duration.Round(time.Second)),
			Data: map[string]any{
				"clips":            clips,
				"bytes":            bytes,
				"duration_seconds": int(duration.Seconds()),
			},
		})
	}

	m.deps.Archive.ManageFreeSpace()
	m.setState(StateIdle)
}

func (m *Machine) runIdle(ctx context.Context) {
	m.deps.System.SetLED("heartbeat")
	m.deps.KeepAwake.Send(ctx, "stop")

	m.deps.Archive.UnmountArchive()
	m.deps.Disk.Unmount()

	if err := m.deps.Gadget.Enable(m.deps.Disk.BackingFilePath()); err != nil {
		log.Printf("warning: gadget re-enable failed: %v", err)
		m.gadgetEnabled = false
	} else {
		m.gadgetEnabled = true
		m.deps.Notify.Send(ctx, webhook.Event{Event: "usb_connected", Message: "USB gadget re-enabled"})
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.tryEnableGadget() {
				m.deps.Notify.Send(ctx, webhook.Event{Event: "usb_connected", Message: "USB gadget re-enabled"})
			}
			if !m.deps.Archive.IsReachable() {
				log.Println("archive server unreachable — user left home")
				m.setState(StateAway)
				m.deps.System.SetLED("slowblink")
				return
			}
		}
	}
}
