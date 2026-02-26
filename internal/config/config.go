package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NFS           NFS           `yaml:"nfs"`
	KeepAwake     KeepAwake     `yaml:"keep_awake"`
	Notifications Notifications `yaml:"notifications"`
	Temperature   Temperature   `yaml:"temperature"`
}

type NFS struct {
	Server string `yaml:"server"`
	Share  string `yaml:"share"`
}

type KeepAwake struct {
	Method     string `yaml:"method"` // "ble" or "webhook"
	VIN        string `yaml:"vin"`
	WebhookURL string `yaml:"webhook_url"`
}

type Notifications struct {
	WebhookURL string `yaml:"webhook_url"`
}

type Temperature struct {
	WarningCelsius float64 `yaml:"warning_celsius"`
	CautionCelsius float64 `yaml:"caution_celsius"`
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
