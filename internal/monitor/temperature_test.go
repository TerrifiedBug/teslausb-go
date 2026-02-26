package monitor

import (
	"testing"
)

func TestGetTemp(t *testing.T) {
	temp := GetTemp()
	// On dev machine, will return 0 if no thermal zone
	if temp < 0 {
		t.Errorf("unexpected negative temperature: %f", temp)
	}
}
