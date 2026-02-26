package system

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

var ledPath string

func initLED() {
	candidates := []string{"led0", "ACT", "status"}
	entries, err := os.ReadDir("/sys/class/leds")
	if err != nil {
		log.Printf("no LEDs found: %v", err)
		return
	}
	for _, c := range candidates {
		for _, e := range entries {
			if strings.Contains(e.Name(), c) {
				ledPath = filepath.Join("/sys/class/leds", e.Name())
				log.Printf("using LED: %s", ledPath)
				return
			}
		}
	}
	if len(entries) > 0 {
		ledPath = filepath.Join("/sys/class/leds", entries[0].Name())
		log.Printf("using fallback LED: %s", ledPath)
	}
}

func sysWrite(path, value string) {
	os.WriteFile(path, []byte(value), 0644)
}

func SetLED(mode string) {
	if ledPath == "" {
		initLED()
	}
	if ledPath == "" {
		return
	}
	trigger := filepath.Join(ledPath, "trigger")
	switch mode {
	case "slowblink":
		sysWrite(trigger, "timer")
		sysWrite(filepath.Join(ledPath, "delay_off"), "900")
		sysWrite(filepath.Join(ledPath, "delay_on"), "100")
	case "fastblink":
		sysWrite(trigger, "timer")
		sysWrite(filepath.Join(ledPath, "delay_off"), "150")
		sysWrite(filepath.Join(ledPath, "delay_on"), "50")
	case "heartbeat":
		sysWrite(trigger, "heartbeat")
	case "off":
		sysWrite(trigger, "none")
		sysWrite(filepath.Join(ledPath, "brightness"), "0")
	}
}
