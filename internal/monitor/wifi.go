package monitor

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"strings"
	"time"
)

// RunWiFiMonitor watches dmesg for brcmfmac failures and reloads the module.
func RunWiFiMonitor(ctx context.Context) {
	// Wait for boot to settle before monitoring
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return
	}

	cmd := exec.CommandContext(ctx, "dmesg", "-w")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("wifi monitor: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("wifi monitor start: %v", err)
		return
	}

	// Skip initial buffered output (dmesg -w replays existing boot messages)
	ready := make(chan struct{})
	go func() {
		time.Sleep(5 * time.Second)
		close(ready)
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ready:
		default:
			continue
		}
		line := scanner.Text()
		if strings.Contains(line, "failed to enable fw supplicant") ||
			strings.Contains(line, "brcmf_fw_alloc_request") {
			log.Println("WiFi driver crash detected, reloading brcmfmac...")
			exec.Command("modprobe", "-r", "brcmfmac").Run()
			exec.Command("modprobe", "brcmfmac").Run()
			log.Println("brcmfmac reloaded")
		}
	}
}
