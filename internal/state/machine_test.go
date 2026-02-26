package state

import (
	"testing"
)

func TestNewMachine(t *testing.T) {
	m := New()
	if m.State() != StateBooting {
		t.Errorf("expected booting, got %s", m.State())
	}
}

func TestStateTransition(t *testing.T) {
	m := New()
	var received State
	m.OnStateChange(func(s State) { received = s })
	m.setState(StateAway)
	if received != StateAway {
		t.Errorf("expected away, got %s", received)
	}
}

func TestInfo(t *testing.T) {
	m := New()
	info := m.Info()
	if info["state"] != "booting" {
		t.Errorf("expected booting, got %s", info["state"])
	}
}
