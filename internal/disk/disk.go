package disk

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	BackingDir  = "/backingfiles"
	BackingFile = "/backingfiles/cam_disk.bin"
	MountPoint  = "/mnt/cam"
)

func Exists() bool {
	_, err := os.Stat(BackingFile)
	return err == nil
}

// Create creates the cam disk image with auto-sized exFAT.
func Create() error {
	if Exists() {
		log.Println("cam_disk.bin already exists")
		return nil
	}

	// Calculate available space
	var stat syscall.Statfs_t
	if err := syscall.Statfs(BackingDir, &stat); err != nil {
		return fmt.Errorf("statfs: %w", err)
	}
	available := int64(stat.Bavail) * int64(stat.Bsize)
	reserve := int64(500 * 1024 * 1024) // 500MB headroom
	size := available - reserve
	if size < 1024*1024*1024 { // minimum 1GB
		return fmt.Errorf("not enough space: %d bytes available", available)
	}

	log.Printf("creating cam_disk.bin: %d GB", size/(1024*1024*1024))

	// Create sparse file
	f, err := os.Create(BackingFile)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if err := f.Truncate(size); err != nil {
		f.Close()
		os.Remove(BackingFile)
		return fmt.Errorf("truncate: %w", err)
	}
	f.Close()

	// Create partition table
	cmd := exec.Command("sfdisk", BackingFile)
	cmd.Stdin = strings.NewReader("type=7\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(BackingFile)
		return fmt.Errorf("sfdisk: %s: %w", out, err)
	}

	// Setup loop device with partition scan
	out, err := exec.Command("losetup", "--find", "--show", "--partscan", BackingFile).Output()
	if err != nil {
		os.Remove(BackingFile)
		return fmt.Errorf("losetup: %w", err)
	}
	loopDev := strings.TrimSpace(string(out))
	partDev := loopDev + "p1"
	defer exec.Command("losetup", "-d", loopDev).Run()

	// Format as exFAT
	if out, err := exec.Command("mkfs.exfat", "-L", "CAM", partDev).CombinedOutput(); err != nil {
		os.Remove(BackingFile)
		return fmt.Errorf("mkfs.exfat: %s: %w", out, err)
	}

	// Mount and create TeslaCam directory
	os.MkdirAll(MountPoint, 0755)
	if err := exec.Command("mount", partDev, MountPoint).Run(); err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	os.MkdirAll(filepath.Join(MountPoint, "TeslaCam"), 0755)
	exec.Command("umount", MountPoint).Run()

	log.Printf("cam_disk.bin created and formatted (%d GB exFAT)", size/(1024*1024*1024))
	return nil
}

// Mount mounts the cam disk image locally after running fsck.
func Mount() error {
	os.MkdirAll(MountPoint, 0755)

	// Setup loop device
	out, err := exec.Command("losetup", "--find", "--show", "--partscan", BackingFile).Output()
	if err != nil {
		return fmt.Errorf("losetup: %w", err)
	}
	loopDev := strings.TrimSpace(string(out))
	partDev := loopDev + "p1"

	// fsck (repair mode)
	log.Println("running fsck.exfat on cam image...")
	exec.Command("fsck.exfat", "-p", partDev).Run() // ignore errors, best-effort

	// Mount
	if err := exec.Command("mount", "-o", "umask=000", partDev, MountPoint).Run(); err != nil {
		exec.Command("losetup", "-d", loopDev).Run()
		return fmt.Errorf("mount: %w", err)
	}

	log.Println("cam image mounted at", MountPoint)
	return nil
}

// Unmount unmounts the cam disk image and detaches the loop device.
func Unmount() error {
	exec.Command("umount", MountPoint).Run()

	// Find and detach loop device for our backing file
	out, err := exec.Command("losetup", "-j", BackingFile).Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if parts := strings.SplitN(line, ":", 2); len(parts) > 0 && parts[0] != "" {
				exec.Command("losetup", "-d", parts[0]).Run()
			}
		}
	}

	log.Println("cam image unmounted")
	return nil
}

// CleanArtifacts removes FSCK recovery files and truncated clips.
func CleanArtifacts() {
	patterns := []string{"FSCK*.REC", "*~[0-9].MP4", "*~[0-9].mp4"}
	for _, p := range patterns {
		matches, _ := filepath.Glob(filepath.Join(MountPoint, p))
		for _, m := range matches {
			os.Remove(m)
			log.Printf("cleaned: %s", filepath.Base(m))
		}
	}

	// Remove truncated clips (<100KB)
	filepath.Walk(MountPoint, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".mp4") && info.Size() < 100_000 {
			os.Remove(path)
			log.Printf("cleaned truncated: %s (%d bytes)", filepath.Base(path), info.Size())
		}
		return nil
	})
}
