package ble

import (
	"strings"
	"testing"
)

func TestKeysExist(t *testing.T) {
	// On dev machine, keys won't exist
	if KeysExist() {
		t.Skip("keys exist, skipping")
	}
}

func TestVINUppercase(t *testing.T) {
	vin := "5yj3e1ea1nf000000"
	upper := strings.ToUpper(vin)
	if upper != "5YJ3E1EA1NF000000" {
		t.Errorf("expected uppercase, got %s", upper)
	}
}
