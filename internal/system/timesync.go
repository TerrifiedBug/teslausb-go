package system

import (
	"log"
	"os/exec"
	"strings"
)

func SyncTime() error {
	for i := 0; i < 5; i++ {
		for _, cmd := range []string{"sntp", "ntpdig"} {
			if path, err := exec.LookPath(cmd); err == nil {
				out, err := exec.Command(path, "-S", "time.google.com").CombinedOutput()
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
