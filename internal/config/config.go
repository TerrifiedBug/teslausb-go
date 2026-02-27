package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NFS           NFS           `yaml:"nfs" json:"nfs"`
	CIFS          CIFS          `yaml:"cifs" json:"cifs"`
	Archive       Archive       `yaml:"archive" json:"archive"`
	KeepAwake     KeepAwake     `yaml:"keep_awake" json:"keep_awake"`
	Notifications Notifications `yaml:"notifications" json:"notifications"`
	Temperature   Temperature   `yaml:"temperature" json:"temperature"`
}

type Archive struct {
	RecentClips    bool   `yaml:"recent_clips" json:"recent_clips"`
	ReservePercent int    `yaml:"reserve_percent" json:"reserve_percent"`
	Method         string `yaml:"method" json:"method"` // "nfs" or "cifs"
}

type CIFS struct {
	Server   string `yaml:"server" json:"server"`
	Share    string `yaml:"share" json:"share"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type NFS struct {
	Server string `yaml:"server" json:"server"`
	Share  string `yaml:"share" json:"share"`
}

type KeepAwake struct {
	Method     string `yaml:"method" json:"method"` // "ble" or "webhook"
	VIN        string `yaml:"vin" json:"vin"`
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`
}

type Notifications struct {
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`
}

type Temperature struct {
	WarningCelsius float64 `yaml:"warning_celsius" json:"warning_celsius"`
	CautionCelsius float64 `yaml:"caution_celsius" json:"caution_celsius"`
}

var (
	current *Config
	mu      sync.RWMutex
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	cfg.Temperature.WarningCelsius = 70
	cfg.Temperature.CautionCelsius = 60
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	mu.Lock()
	current = &cfg
	mu.Unlock()
	return &cfg, nil
}

func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func Save(path string, cfg *Config) error {
	mu.Lock()
	current = cfg
	mu.Unlock()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
