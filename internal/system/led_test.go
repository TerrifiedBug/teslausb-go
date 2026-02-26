package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetLEDNoLED(t *testing.T) {
	ledPath = ""
	// Should not panic when no LED is available
	SetLED("slowblink")
	SetLED("fastblink")
	SetLED("heartbeat")
	SetLED("off")
}

func TestSetLEDFakeLED(t *testing.T) {
	dir := t.TempDir()
	ledPath = dir
	// Create the trigger file
	os.WriteFile(filepath.Join(dir, "trigger"), []byte("none"), 0644)
	os.WriteFile(filepath.Join(dir, "delay_off"), []byte("0"), 0644)
	os.WriteFile(filepath.Join(dir, "delay_on"), []byte("0"), 0644)
	os.WriteFile(filepath.Join(dir, "brightness"), []byte("0"), 0644)

	SetLED("slowblink")
	data, _ := os.ReadFile(filepath.Join(dir, "trigger"))
	if string(data) != "timer" {
		t.Errorf("expected timer, got %s", string(data))
	}
}
