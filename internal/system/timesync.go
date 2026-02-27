package system

import (
	"log"
	"os/exec"
	"strings"
)

func SyncTime() error {
	for i := 0; i < 5; i++ {
		for _, cmd := range []string{"sntp", "ntpdig", "ntpdate"} {
			if path, err := exec.LookPath(cmd); err == nil {
				args := []string{"time.google.com"}
				if cmd != "ntpdate" {
					args = []string{"-S", "time.google.com"}
				}
				out, err := exec.Command(path, args...).CombinedOutput()
				if err == nil {
					log.Printf("time synced via %s: %s", cmd, strings.TrimSpace(string(out)))
					return nil
				}
			}
		}
	}
	log.Printf("warning: time sync failed after 5 attempts")
	return nil // non-fatal
}
