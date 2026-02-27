package update

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const repo = "TerrifiedBug/teslausb-go"

type Release struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
}

// CheckForUpdate checks GitHub for a newer release.
func CheckForUpdate(currentVersion string) (*Release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github returned %d", resp.StatusCode)
	}
	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	if release.TagName == currentVersion || release.TagName == "v"+currentVersion {
		return nil, nil // up to date
	}
	return &release, nil
}

// Apply downloads and installs the latest release.
func Apply(currentVersion string) error {
	release, err := CheckForUpdate(currentVersion)
	if err != nil {
		return err
	}
	if release == nil {
		log.Println("already up to date")
		return nil
	}

	arch := runtime.GOARCH
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/teslausb-linux-%s", repo, release.TagName, arch)

	log.Printf("downloading %s...", release.TagName)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Write to temp file
	tmpFile := "/usr/local/bin/teslausb.new"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("write: %w", err)
	}
	f.Close()

	// Backup current binary
	currentBin := "/usr/local/bin/teslausb"
	os.Rename(currentBin, currentBin+".prev")

	// Replace
	if err := os.Rename(tmpFile, currentBin); err != nil {
		// Rollback
		os.Rename(currentBin+".prev", currentBin)
		return fmt.Errorf("replace: %w", err)
	}

	log.Printf("updated to %s, restarting...", release.TagName)
	exec.Command("systemctl", "restart", "teslausb").Run()
	return nil
}

// Version strips the "v" prefix for comparison.
func Version(v string) string {
	return strings.TrimPrefix(v, "v")
}
