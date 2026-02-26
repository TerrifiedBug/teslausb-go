package gadget

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const gadgetName = "teslausb"

var configfsRoot string

func findConfigFS() string {
	if configfsRoot != "" {
		return configfsRoot
	}
	out, err := exec.Command("findmnt", "-o", "TARGET", "-n", "configfs").Output()
	if err != nil {
		configfsRoot = "/sys/kernel/config"
	} else {
		configfsRoot = strings.TrimSpace(string(out))
	}
	return configfsRoot
}

func gadgetRoot() string {
	return filepath.Join(findConfigFS(), "usb_gadget", gadgetName)
}

func writeFile(path, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}

func serialNumber() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "TeslaUSB-unknown"
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("TeslaUSB-%x", h[:8])
}

func detectMaxPower() string {
	data, _ := os.ReadFile("/proc/device-tree/model")
	model := string(data)
	switch {
	case strings.Contains(model, "Pi 5"):
		return "600"
	case strings.Contains(model, "Pi 4"):
		return "500"
	case strings.Contains(model, "Pi Zero 2"):
		return "200"
	default:
		return "100"
	}
}

func Enable(backingFile string) error {
	// Unload g_ether placeholder
	exec.Command("modprobe", "-r", "g_ether").Run()

	// Load libcomposite
	if err := exec.Command("modprobe", "libcomposite").Run(); err != nil {
		return fmt.Errorf("modprobe libcomposite: %w", err)
	}

	root := gadgetRoot()

	// Create gadget structure
	for _, dir := range []string{
		root,
		filepath.Join(root, "strings", "0x409"),
		filepath.Join(root, "configs", "c.1"),
		filepath.Join(root, "configs", "c.1", "strings", "0x409"),
		filepath.Join(root, "functions", "mass_storage.0"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// USB descriptors
	descriptors := map[string]string{
		"idVendor":  "0x1d6b",
		"idProduct": "0x0104",
		"bcdDevice": "0x0100",
		"bcdUSB":    "0x0200",
	}
	for k, v := range descriptors {
		writeFile(filepath.Join(root, k), v)
	}

	// Strings
	writeFile(filepath.Join(root, "strings", "0x409", "serialnumber"), serialNumber())
	writeFile(filepath.Join(root, "strings", "0x409", "manufacturer"), "teslausb-go")
	writeFile(filepath.Join(root, "strings", "0x409", "product"), "TeslaUSB Composite Gadget")
	writeFile(filepath.Join(root, "configs", "c.1", "strings", "0x409", "configuration"), "TeslaUSB Config")
	writeFile(filepath.Join(root, "configs", "c.1", "MaxPower"), detectMaxPower())

	// Mass storage LUN
	writeFile(filepath.Join(root, "functions", "mass_storage.0", "lun.0", "file"), backingFile)

	// Symlink function to config
	linkPath := filepath.Join(root, "configs", "c.1", "mass_storage.0")
	os.Remove(linkPath)
	os.Symlink(filepath.Join(root, "functions", "mass_storage.0"), linkPath)

	// Bind to UDC
	entries, err := os.ReadDir("/sys/class/udc")
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no UDC found")
	}
	writeFile(filepath.Join(root, "UDC"), entries[0].Name())

	log.Printf("USB gadget enabled with %s", backingFile)
	return nil
}

func Disable() error {
	root := gadgetRoot()

	// Unbind from UDC
	writeFile(filepath.Join(root, "UDC"), "")

	// Remove symlink
	os.Remove(filepath.Join(root, "configs", "c.1", "mass_storage.0"))

	// Remove dirs in reverse order
	for _, dir := range []string{
		filepath.Join(root, "configs", "c.1", "strings", "0x409"),
		filepath.Join(root, "configs", "c.1"),
		filepath.Join(root, "functions", "mass_storage.0"),
		filepath.Join(root, "strings", "0x409"),
		root,
	} {
		os.Remove(dir)
	}

	// Unload modules
	for _, mod := range []string{"usb_f_mass_storage", "libcomposite"} {
		exec.Command("modprobe", "-r", mod).Run()
	}

	log.Println("USB gadget disabled")
	return nil
}
