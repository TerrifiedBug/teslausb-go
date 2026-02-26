package archive

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/teslausb-go/teslausb/internal/disk"
)

const minReserveBytes = 2 * 1024 * 1024 * 1024 // 2GB minimum reserve

func reserveBytes() int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(disk.MountPoint, &stat); err != nil {
		return minReserveBytes
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	tenPercent := total / 10
	if tenPercent < minReserveBytes {
		return minReserveBytes
	}
	return tenPercent
}

type fileEntry struct {
	path    string
	modTime time.Time
	size    int64
}

func ManageFreeSpace() {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(disk.MountPoint, &stat); err != nil {
		return
	}
	free := int64(stat.Bavail) * int64(stat.Bsize)
	reserve := reserveBytes()

	if free >= reserve {
		return
	}

	needed := reserve - free
	log.Printf("free space: %d MB, need %d MB more", free/(1024*1024), needed/(1024*1024))

	// Collect all clips sorted by modification time (oldest first)
	var files []fileEntry
	for _, dir := range []string{"TeslaCam/RecentClips", "TeslaCam/SavedClips", "TeslaCam/SentryClips"} {
		filepath.Walk(filepath.Join(disk.MountPoint, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			files = append(files, fileEntry{path: path, modTime: info.ModTime(), size: info.Size()})
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	freed := int64(0)
	for _, f := range files {
		if freed >= needed {
			break
		}
		os.Remove(f.path)
		freed += f.size
		log.Printf("freed: %s (%d KB)", filepath.Base(f.path), f.size/1024)
	}
	log.Printf("freed %d MB total", freed/(1024*1024))
}
