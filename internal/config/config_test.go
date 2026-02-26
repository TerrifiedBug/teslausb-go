package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
nfs:
  server: "192.168.1.100"
  share: "/volume1/TeslaCam"
keep_awake:
  method: "ble"
  vin: "5YJ3E1EA1NF000000"
temperature:
  warning_celsius: 75
  caution_celsius: 65
`), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NFS.Server != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %s", cfg.NFS.Server)
	}
	if cfg.KeepAwake.Method != "ble" {
		t.Errorf("expected ble, got %s", cfg.KeepAwake.Method)
	}
	if cfg.Temperature.WarningCelsius != 75 {
		t.Errorf("expected 75, got %f", cfg.Temperature.WarningCelsius)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("nfs:\n  server: test\n"), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature.WarningCelsius != 70 {
		t.Errorf("expected default 70, got %f", cfg.Temperature.WarningCelsius)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := &Config{NFS: NFS{Server: "10.0.0.1", Share: "/data"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.NFS.Server != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", loaded.NFS.Server)
	}
}
