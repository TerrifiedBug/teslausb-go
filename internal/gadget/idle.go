package gadget

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func findMassStoragePID() (int, error) {
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if strings.TrimSpace(string(comm)) == "file-storage" {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("file-storage process not found")
}

func readWriteBytes(pid int) (int64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "write_bytes:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "write_bytes:"))
			return strconv.ParseInt(val, 10, 64)
		}
	}
	return 0, fmt.Errorf("write_bytes not found")
}

// WaitForIdle waits until USB mass storage writes have been idle for 5 seconds.
// Returns nil if idle detected, error if timeout (90 seconds).
func WaitForIdle() error {
	pid, err := findMassStoragePID()
	if err != nil {
		log.Printf("mass storage not active, OK to proceed")
		return nil
	}

	prevBytes := int64(-1)
	idleCount := 0
	threshold := int64(500_000)

	log.Println("waiting for USB write idle...")
	for i := 0; i < 90; i++ {
		time.Sleep(1 * time.Second)
		written, err := readWriteBytes(pid)
		if err != nil {
			return nil // process gone, that's fine
		}
		if prevBytes == -1 {
			prevBytes = written
			continue
		}
		delta := written - prevBytes
		prevBytes = written

		if delta < threshold {
			idleCount++
			if idleCount >= 5 {
				log.Println("USB write idle detected")
				return nil
			}
		} else {
			idleCount = 0
		}
	}
	return fmt.Errorf("timeout waiting for USB idle after 90 seconds")
}
