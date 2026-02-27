package ble

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	KeyDir     = "/mutable/ble"
	PrivateKey = "/mutable/ble/key_private.pem"
	PublicKey  = "/mutable/ble/key_public.pem"
)

func teslaControl() string {
	if p, err := exec.LookPath("tesla-control"); err == nil {
		return p
	}
	return "/usr/local/bin/tesla-control"
}

func teslaKeygen() string {
	if p, err := exec.LookPath("tesla-keygen"); err == nil {
		return p
	}
	return "/usr/local/bin/tesla-keygen"
}

// acquireHCI stops bluetoothd so tesla-control can get exclusive HCI access.
// Returns a cleanup function that restarts bluetoothd.
func acquireHCI() func() {
	exec.Command("systemctl", "stop", "bluetooth").Run()
	exec.Command("rfkill", "unblock", "bluetooth").Run()
	exec.Command("hciconfig", "hci0", "up").Run()
	return func() {
		exec.Command("systemctl", "start", "bluetooth").Run()
	}
}

func runBLE(vin string, args ...string) error {
	baseArgs := []string{"-ble", "-key-file", PrivateKey, "-vin", strings.ToUpper(vin)}
	baseArgs = append(baseArgs, args...)

	release := acquireHCI()
	defer release()

	for attempt := 1; attempt <= 3; attempt++ {
		cmd := exec.Command(teslaControl(), baseArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		outStr := strings.TrimSpace(string(out))
		// Car rejects BLE when USB cable is connected — no need to retry
		if strings.Contains(outStr, "cable connected") {
			log.Printf("BLE skipped: car has USB cable connected (keep-awake not needed)")
			return nil
		}
		log.Printf("BLE attempt %d/%d failed: %s: %v", attempt, 3, outStr, err)
		if attempt < 3 {
			time.Sleep(30 * time.Second)
		}
	}
	return fmt.Errorf("BLE command failed after 3 attempts: %v", args)
}

func KeysExist() bool {
	_, err1 := os.Stat(PrivateKey)
	_, err2 := os.Stat(PublicKey)
	return err1 == nil && err2 == nil
}

func GenerateKeys() error {
	os.MkdirAll(KeyDir, 0700)
	cmd := exec.Command(teslaKeygen(), "-key-file", PrivateKey, "-output", PublicKey, "create")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keygen: %s: %w", out, err)
	}
	os.Chmod(PrivateKey, 0600)
	os.Chmod(PublicKey, 0644)
	log.Println("BLE keys generated")
	return nil
}

func Pair(vin string) error {
	if !KeysExist() {
		if err := GenerateKeys(); err != nil {
			return err
		}
	}

	release := acquireHCI()
	defer release()

	cmd := exec.Command(teslaControl(), "-ble", "-vin", strings.ToUpper(vin),
		"add-key-request", PublicKey, "owner", "cloud_key")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pair: %s: %w", out, err)
	}
	log.Println("BLE pairing request sent — tap NFC card on center console")
	return nil
}

func IsPaired(vin string) bool {
	release := acquireHCI()
	defer release()

	cmd := exec.Command(teslaControl(), "-ble", "-key-file", PrivateKey,
		"-vin", strings.ToUpper(vin), "body-controller-state")
	return cmd.Run() == nil
}

func KeepAwake(vin string) error {
	return runBLE(vin, "charge-port-close")
}

func SentryOn(vin string) error {
	return runBLE(vin, "sentry-mode", "on")
}

func SentryOff(vin string) error {
	return runBLE(vin, "sentry-mode", "off")
}
