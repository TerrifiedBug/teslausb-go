package system

import (
	"log"
	"os"
)

func ApplyTuning() {
	tunings := map[string]string{
		"/proc/sys/vm/dirty_background_bytes": "65536",
		"/proc/sys/vm/dirty_ratio":            "80",
	}
	for path, val := range tunings {
		if err := os.WriteFile(path, []byte(val), 0644); err != nil {
			log.Printf("tuning %s: %v", path, err)
		}
	}
	// Set CPU governor to conservative
	govPath := "/sys/devices/system/cpu/cpufreq/policy0/scaling_governor"
	if err := os.WriteFile(govPath, []byte("conservative"), 0644); err != nil {
		log.Printf("cpu governor: %v", err)
	}
	log.Println("system tuning applied")
}
