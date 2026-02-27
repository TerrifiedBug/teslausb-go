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

// IsReachable checks if the archive server is reachable.
func IsReachable() bool {
	cfg := config.Get()
	if cfg == nil {
		return false
	}
	if cfg.Archive.Method == "cifs" {
		return isCIFSReachable(cfg)
	}
	return isNFSReachable(cfg)
}

func isNFSReachable(cfg *config.Config) bool {
	if cfg.NFS.Server == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", cfg.NFS.Server+":2049", 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func isCIFSReachable(cfg *config.Config) bool {
	if cfg.CIFS.Server == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", cfg.CIFS.Server+":445", 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// MountArchive mounts the archive share based on configured method.
func MountArchive() error {
	cfg := config.Get()
	if cfg != nil && cfg.Archive.Method == "cifs" {
		return MountCIFS()
	}
	return MountNFS()
}

// UnmountArchive unmounts whichever archive share is mounted.
func UnmountArchive() {
	UnmountNFS()
}

// MountCIFS mounts the configured CIFS/SMB share with auto-negotiation.
func MountCIFS() error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("no config")
	}
	os.MkdirAll(ArchiveMount, 0755)
	source := fmt.Sprintf("//%s/%s", cfg.CIFS.Server, cfg.CIFS.Share)

	// Build options
	opts := fmt.Sprintf("username=%s,password=%s,iocharset=utf8,file_mode=0777,dir_mode=0777",
		cfg.CIFS.Username, cfg.CIFS.Password)

	// Try SMB versions in order: 3.0, 2.1, 2.0
	for _, ver := range []string{"3.0", "2.1", "2.0"} {
		verOpts := opts + ",vers=" + ver
		if err := exec.Command("mount", "-t", "cifs", source, ArchiveMount, "-o", verOpts).Run(); err == nil {
			log.Printf("CIFS mounted: %s (SMB %s)", source, ver)
			return nil
		}
	}
	return fmt.Errorf("mount CIFS %s: all SMB versions failed", source)
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

// ArchiveClips copies SavedClips and SentryClips (and optionally RecentClips) to the NFS share via rsync.
// Returns clip count and bytes transferred.
func ArchiveClips(ctx context.Context) (int, int64, error) {
	clipDirs := []string{"TeslaCam/SavedClips", "TeslaCam/SentryClips"}
	if cfg := config.Get(); cfg != nil && cfg.Archive.RecentClips {
		clipDirs = append(clipDirs, "TeslaCam/RecentClips")
	}
	totalClips := 0
	totalBytes := int64(0)

	for _, dir := range clipDirs {
		src := filepath.Join(disk.MountPoint, dir)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		// Check if there are any files to archive
		entries, _ := os.ReadDir(src)
		if len(entries) == 0 {
			continue
		}

		dst := filepath.Join(ArchiveMount, dir) + "/"
		os.MkdirAll(dst, 0755)

		log.Printf("archiving %s (%d items)", dir, len(entries))

		// Build rsync command — use direct src/dst (no -R with absolute paths)
		args := []string{
			"-avhL",
			"--no-o", "--no-g", // NFS root-squash workaround
			"--remove-source-files",
			"--no-perms",
			"--omit-dir-times",
			src + "/",
			dst,
		}

		cmd := exec.CommandContext(ctx, "rsync", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		errCh := make(chan error, 1)
		go func() {
			errCh <- cmd.Run()
		}()

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

		// Count archived items and bytes on destination
		dstEntries, _ := os.ReadDir(dst)
		for _, e := range dstEntries {
			if e.IsDir() {
				continue
			}
			totalClips++
			if info, err := e.Info(); err == nil {
				totalBytes += info.Size()
			}
		}
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
