package wire

import (
	"context"

	"github.com/teslausb-go/teslausb/internal/archive"
	"github.com/teslausb-go/teslausb/internal/ble"
	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/disk"
	"github.com/teslausb-go/teslausb/internal/gadget"
	"github.com/teslausb-go/teslausb/internal/notify"
	"github.com/teslausb-go/teslausb/internal/state"
	"github.com/teslausb-go/teslausb/internal/system"
	"github.com/teslausb-go/teslausb/internal/webhook"
)

// DiskAdapter wraps the disk package functions.
type DiskAdapter struct{}

func (d *DiskAdapter) Exists() bool           { return disk.Exists() }
func (d *DiskAdapter) Create() error           { return disk.Create() }
func (d *DiskAdapter) Mount() error            { return disk.Mount() }
func (d *DiskAdapter) Unmount() error          { return disk.Unmount() }
func (d *DiskAdapter) CleanArtifacts()         { disk.CleanArtifacts() }
func (d *DiskAdapter) BackingFilePath() string { return disk.BackingFile }

// GadgetAdapter wraps the gadget package functions.
type GadgetAdapter struct{}

func (g *GadgetAdapter) Enable(backingFile string) error { return gadget.Enable(backingFile) }
func (g *GadgetAdapter) Disable() error                  { return gadget.Disable() }
func (g *GadgetAdapter) WaitForIdle() error              { return gadget.WaitForIdle() }

// ArchiveAdapter wraps the archive package functions.
type ArchiveAdapter struct{}

func (a *ArchiveAdapter) IsReachable() bool                                    { return archive.IsReachable() }
func (a *ArchiveAdapter) MountArchive() error                                  { return archive.MountArchive() }
func (a *ArchiveAdapter) UnmountArchive()                                      { archive.UnmountArchive() }
func (a *ArchiveAdapter) ArchiveClips(ctx context.Context) (int, int64, error) { return archive.ArchiveClips(ctx) }
func (a *ArchiveAdapter) ManageFreeSpace()                                     { archive.ManageFreeSpace() }

// SystemAdapter wraps the system package functions.
type SystemAdapter struct{}

func (s *SystemAdapter) SetLED(mode string) { system.SetLED(mode) }
func (s *SystemAdapter) SyncTime()          { system.SyncTime() }

// KeepAwakeAdapter dispatches keep-awake commands based on config.
type KeepAwakeAdapter struct{}

func (k *KeepAwakeAdapter) Send(ctx context.Context, command string) {
	cfg := config.Get()
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
			webhook.SendRaw(ctx, cfg.KeepAwake.WebhookURL, map[string]string{"awake_command": command})
		}
	}
}

// NotifyAdapter wraps the notify package.
type NotifyAdapter struct{}

func (n *NotifyAdapter) Send(ctx context.Context, event webhook.Event) {
	notify.Send(ctx, event)
}

// NewDeps creates a Deps struct wired to the real implementations.
func NewDeps() state.Deps {
	return state.Deps{
		Disk:      &DiskAdapter{},
		Gadget:    &GadgetAdapter{},
		Archive:   &ArchiveAdapter{},
		System:    &SystemAdapter{},
		KeepAwake: &KeepAwakeAdapter{},
		Notify:    &NotifyAdapter{},
	}
}
