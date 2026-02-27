package monitor

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

type NetworkInfo struct {
	SSID      string `json:"wifi_ssid"`
	SignalDBM int    `json:"wifi_signal_dbm"`
	IP        string `json:"wifi_ip"`
}

func GetNetworkInfo() NetworkInfo {
	info := NetworkInfo{}

	// SSID
	if out, err := exec.Command("iwgetid", "-r").Output(); err == nil {
		info.SSID = strings.TrimSpace(string(out))
	}

	// Signal from /proc/net/wireless
	if f, err := os.Open("/proc/net/wireless"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if lineNum <= 2 {
				continue // skip header lines
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 {
				// field 3 is signal level (dBm), may have trailing "."
				val := strings.TrimRight(fields[3], ".")
				var dbm int
				for i, c := range val {
					if c == '-' && i == 0 {
						continue
					}
					if c < '0' || c > '9' {
						break
					}
				}
				// Simple atoi
				neg := false
				for _, c := range val {
					if c == '-' {
						neg = true
						continue
					}
					if c >= '0' && c <= '9' {
						dbm = dbm*10 + int(c-'0')
					}
				}
				if neg {
					dbm = -dbm
				}
				info.SignalDBM = dbm
			}
			break // only first interface
		}
	}

	// IP address — find wlan0 IP
	if out, err := exec.Command("ip", "-4", "-o", "addr", "show", "wlan0").Output(); err == nil {
		// Format: "3: wlan0    inet 192.168.1.5/24 ..."
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "inet" && i+1 < len(fields) {
				ip := fields[i+1]
				if idx := strings.Index(ip, "/"); idx > 0 {
					ip = ip[:idx]
				}
				info.IP = ip
				break
			}
		}
	}

	return info
}
