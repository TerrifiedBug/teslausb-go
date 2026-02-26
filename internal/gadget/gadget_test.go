package gadget

import (
	"testing"
)

func TestSerialNumber(t *testing.T) {
	sn := serialNumber()
	if sn == "" {
		t.Error("serial number should not be empty")
	}
}

func TestDetectMaxPower(t *testing.T) {
	power := detectMaxPower()
	if power == "" {
		t.Error("max power should not be empty")
	}
}

func TestFindMassStoragePIDNotRunning(t *testing.T) {
	_, err := findMassStoragePID()
	// Should return error on dev machine (no file-storage process)
	if err == nil {
		t.Skip("file-storage process found, skipping")
	}
}

func TestWaitForIdleNoProcess(t *testing.T) {
	// Should return nil immediately when no mass storage process
	err := WaitForIdle()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
