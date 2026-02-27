package monitor

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type NetworkInfo struct {
	SSID      string `json:"wifi_ssid"`
	SignalDBM int    `json:"wifi_signal_dbm"`
	IP        string `json:"wifi_ip"`
}

func GetNetworkInfo() NetworkInfo {
	info := NetworkInfo{}

	if out, err := exec.Command("iwgetid", "-r").Output(); err == nil {
		info.SSID = strings.TrimSpace(string(out))
	}

	// Signal from /proc/net/wireless (skip 2 header lines)
	if f, err := os.Open("/proc/net/wireless"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for lineNum := 0; scanner.Scan(); lineNum++ {
			if lineNum < 2 {
				continue
			}
			if fields := strings.Fields(scanner.Text()); len(fields) >= 4 {
				if dbm, err := strconv.Atoi(strings.TrimRight(fields[3], ".")); err == nil {
					info.SignalDBM = dbm
				}
			}
			break
		}
	}

	// IP from wlan0: "3: wlan0    inet 192.168.1.5/24 ..."
	if out, err := exec.Command("ip", "-4", "-o", "addr", "show", "wlan0").Output(); err == nil {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "inet" && i+1 < len(fields) {
				info.IP, _, _ = strings.Cut(fields[i+1], "/")
				break
			}
		}
	}

	return info
}
