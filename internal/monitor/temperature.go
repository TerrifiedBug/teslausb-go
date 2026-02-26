package monitor

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/notify"
	"github.com/teslausb-go/teslausb/internal/webhook"
)

func readTemp() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}
	millideg, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	return float64(millideg) / 1000.0, nil
}

// GetTemp returns current CPU temperature in Celsius.
func GetTemp() float64 {
	t, _ := readTemp()
	return t
}

// RunTemperatureMonitor runs a background temperature monitor.
func RunTemperatureMonitor(ctx context.Context) {
	warningFired := false
	cautionFired := false
	hysteresis := 5.0

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			temp, err := readTemp()
			if err != nil {
				continue
			}

			cfg := config.Get()
			if cfg == nil {
				continue
			}

			if temp >= cfg.Temperature.WarningCelsius && !warningFired {
				warningFired = true
				notify.Send(ctx, webhook.Event{
					Event:   "temperature_warning",
					Message: strconv.FormatFloat(temp, 'f', 1, 64) + "C",
					Data:    map[string]any{"celsius": temp},
				})
			} else if temp < cfg.Temperature.WarningCelsius-hysteresis {
				warningFired = false
			}

			if temp >= cfg.Temperature.CautionCelsius && !cautionFired {
				cautionFired = true
				notify.Send(ctx, webhook.Event{
					Event:   "temperature_caution",
					Message: strconv.FormatFloat(temp, 'f', 1, 64) + "C",
					Data:    map[string]any{"celsius": temp},
				})
			} else if temp < cfg.Temperature.CautionCelsius-hysteresis {
				cautionFired = false
			}
		}
	}
}
