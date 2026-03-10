package state

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teslausb-go/teslausb/internal/webhook"
)

type noopDisk struct{}

func (d *noopDisk) Exists() bool           { return true }
func (d *noopDisk) Create() error           { return nil }
func (d *noopDisk) Mount() error            { return nil }
func (d *noopDisk) Unmount() error          { return nil }
func (d *noopDisk) CleanArtifacts()         {}
func (d *noopDisk) BackingFilePath() string { return "/tmp/test.bin" }

type noopGadget struct{}

func (g *noopGadget) Enable(string) error { return nil }
func (g *noopGadget) Disable() error      { return nil }
func (g *noopGadget) WaitForIdle() error  { return nil }

type noopArchive struct {
	reachable atomic.Bool
}

func (a *noopArchive) IsReachable() bool                                { return a.reachable.Load() }
func (a *noopArchive) MountArchive() error                              { return nil }
func (a *noopArchive) UnmountArchive()                                  {}
func (a *noopArchive) ArchiveClips(context.Context) (int, int64, error) { return 0, 0, nil }
func (a *noopArchive) ManageFreeSpace()                                 {}

type noopSystem struct{ lastLED string }

func (s *noopSystem) SetLED(mode string) { s.lastLED = mode }
func (s *noopSystem) SyncTime()          {}

type noopKeepAwake struct{ lastCmd string }

func (k *noopKeepAwake) Send(_ context.Context, cmd string) { k.lastCmd = cmd }

type noopNotify struct{ events []webhook.Event }

func (n *noopNotify) Send(_ context.Context, e webhook.Event) { n.events = append(n.events, e) }

func testDeps() Deps {
	return Deps{
		Disk:      &noopDisk{},
		Gadget:    &noopGadget{},
		Archive:   &noopArchive{},
		System:    &noopSystem{},
		KeepAwake: &noopKeepAwake{},
		Notify:    &noopNotify{},
	}
}

func TestNewMachine(t *testing.T) {
	m := New(testDeps())
	if m.State() != StateBooting {
		t.Errorf("expected booting, got %s", m.State())
	}
}

func TestStateTransition(t *testing.T) {
	m := New(testDeps())
	var received State
	m.OnStateChange(func(s State) { received = s })
	m.setState(StateAway)
	if received != StateAway {
		t.Errorf("expected away, got %s", received)
	}
}

func TestInfo(t *testing.T) {
	m := New(testDeps())
	info := m.Info()
	if info["state"] != "booting" {
		t.Errorf("expected booting, got %s", info["state"])
	}
}

func TestTriggerArchive(t *testing.T) {
	m := New(testDeps())
	// Not idle — should return false
	if m.TriggerArchive() {
		t.Error("should not trigger from booting state")
	}
	// Set to idle
	m.setState(StateIdle)
	if !m.TriggerArchive() {
		t.Error("should trigger from idle state")
	}
	if m.State() != StateArriving {
		t.Errorf("expected arriving, got %s", m.State())
	}
}

func TestTryEnableGadget(t *testing.T) {
	g := &noopGadget{}
	deps := testDeps()
	deps.Gadget = g
	m := New(deps)

	// Already enabled — should return false
	m.gadgetEnabled = true
	if m.tryEnableGadget() {
		t.Error("should return false when already enabled")
	}

	// Not enabled — should return true
	m.gadgetEnabled = false
	if !m.tryEnableGadget() {
		t.Error("should return true when enable succeeds")
	}
	if !m.gadgetEnabled {
		t.Error("gadgetEnabled should be true after successful enable")
	}
}

func TestRunAwayTransitionsOnReachable(t *testing.T) {
	archive := &noopArchive{}
	deps := testDeps()
	deps.Archive = archive
	m := New(deps)
	m.setState(StateAway)

	ctx, cancel := context.WithCancel(context.Background())

	// Start runAway in goroutine
	done := make(chan struct{})
	go func() {
		m.runAway(ctx)
		close(done)
	}()

	// Simulate archive becoming reachable (thread-safe via atomic.Bool)
	time.Sleep(100 * time.Millisecond)
	archive.reachable.Store(true)

	// Wait for state change (with timeout)
	select {
	case <-done:
	case <-time.After(pollInterval + 5*time.Second):
		cancel()
		t.Fatal("runAway did not return after archive became reachable")
	}
	cancel()

	if m.State() != StateArriving {
		t.Errorf("expected arriving, got %s", m.State())
	}
}

func TestRunIdleNotifiesOnGadgetEnable(t *testing.T) {
	n := &noopNotify{}
	archive := &noopArchive{}
	archive.reachable.Store(true)
	deps := testDeps()
	deps.Notify = n
	deps.Archive = archive
	m := New(deps)
	m.setState(StateIdle)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		m.runIdle(ctx)
	}()

	// Wait for initial gadget enable + notification
	time.Sleep(100 * time.Millisecond)
	cancel()

	found := false
	for _, e := range n.events {
		if e.Event == "usb_connected" {
			found = true
		}
	}
	if !found {
		t.Error("expected usb_connected notification")
	}
}
