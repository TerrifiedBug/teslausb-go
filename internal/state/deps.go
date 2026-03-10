package state

import (
	"context"

	"github.com/teslausb-go/teslausb/internal/webhook"
)

// DiskOps abstracts disk image operations.
type DiskOps interface {
	Exists() bool
	Create() error
	Mount() error
	Unmount() error
	CleanArtifacts()
	BackingFilePath() string
}

// GadgetOps abstracts USB gadget lifecycle.
type GadgetOps interface {
	Enable(backingFile string) error
	Disable() error
	WaitForIdle() error
}

// ArchiveOps abstracts clip archiving operations.
type ArchiveOps interface {
	IsReachable() bool
	MountArchive() error
	UnmountArchive()
	ArchiveClips(ctx context.Context) (clips int, bytes int64, err error)
	ManageFreeSpace()
}

// SystemOps abstracts system-level operations (LED, time sync).
type SystemOps interface {
	SetLED(mode string)
	SyncTime()
}

// KeepAwaker abstracts vehicle keep-awake signaling.
type KeepAwaker interface {
	Send(ctx context.Context, command string)
}

// Notifier abstracts event notifications.
type Notifier interface {
	Send(ctx context.Context, event webhook.Event)
}

// Deps bundles all external dependencies for the state machine.
type Deps struct {
	Disk      DiskOps
	Gadget    GadgetOps
	Archive   ArchiveOps
	System    SystemOps
	KeepAwake KeepAwaker
	Notify    Notifier
}
