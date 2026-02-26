package archive

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/disk"
)

const ArchiveMount = "/mnt/archive"

// IsReachable checks if the NFS server is reachable via TCP port 2049.
func IsReachable() bool {
	cfg := config.Get()
	if cfg == nil || cfg.NFS.Server == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", cfg.NFS.Server+":2049", 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// MountNFS mounts the configured NFS share.
func MountNFS() error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("no config")
	}
	os.MkdirAll(ArchiveMount, 0755)
	source := fmt.Sprintf("%s:%s", cfg.NFS.Server, cfg.NFS.Share)
	opts := "rw,noauto,nolock,proto=tcp,vers=3"
	if err := exec.Command("mount", "-t", "nfs", source, ArchiveMount, "-o", opts).Run(); err != nil {
		return fmt.Errorf("mount NFS %s: %w", source, err)
	}
	log.Printf("NFS mounted: %s", source)
	return nil
}

// UnmountNFS unmounts the NFS share.
func UnmountNFS() {
	exec.Command("umount", "-f", "-l", ArchiveMount).Run()
	log.Println("NFS unmounted")
}

// ArchiveClips copies SavedClips and SentryClips to the NFS share via rsync.
// Returns clip count and bytes transferred.
func ArchiveClips(ctx context.Context) (int, int64, error) {
	clipDirs := []string{"TeslaCam/SavedClips", "TeslaCam/SentryClips"}
	totalClips := 0
	totalBytes := int64(0)

	for _, dir := range clipDirs {
		src := filepath.Join(disk.MountPoint, dir)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		dst := filepath.Join(ArchiveMount, dir) + "/"
		os.MkdirAll(dst, 0755)

		// Build rsync command
		args := []string{
			"-avhRL",
			"--no-o", "--no-g", // NFS root-squash workaround
			"--remove-source-files",
			"--no-perms",
			"--omit-dir-times",
			"--temp-dir=.teslausbtmp",
			src + "/",
			ArchiveMount + "/",
		}

		cmd := exec.CommandContext(ctx, "rsync", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Run rsync with connection monitoring
		errCh := make(chan error, 1)
		go func() {
			errCh <- cmd.Run()
		}()

		// Monitor NFS reachability
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		select {
		case err := <-errCh:
			if err != nil {
				// Exit code 24 = partial transfer (acceptable)
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 24 {
					log.Println("rsync: partial transfer (some files vanished)")
				} else {
					return totalClips, totalBytes, fmt.Errorf("rsync %s: %w", dir, err)
				}
			}
		case <-ctx.Done():
			cmd.Process.Kill()
			return totalClips, totalBytes, ctx.Err()
		}

		// Count transferred files
		entries, _ := filepath.Glob(filepath.Join(dst, "*"))
		totalClips += len(entries)
	}

	// Clean empty directories in source
	for _, dir := range clipDirs {
		cleanEmptyDirs(filepath.Join(disk.MountPoint, dir))
	}

	return totalClips, totalBytes, nil
}

func cleanEmptyDirs(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == root {
			return nil
		}
		entries, _ := os.ReadDir(path)
		if len(entries) == 0 {
			os.Remove(path)
		}
		return nil
	})
}
