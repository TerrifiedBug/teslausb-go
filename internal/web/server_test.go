package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslausb-go/teslausb/internal/state"
	"github.com/teslausb-go/teslausb/internal/webhook"
)

func webTestDeps() state.Deps {
	return state.Deps{
		Disk:      &tdisk{},
		Gadget:    &tgadget{},
		Archive:   &tarchive{},
		System:    &tsystem{},
		KeepAwake: &tkeep{},
		Notify:    &tnotify{},
	}
}

type tdisk struct{}

func (d *tdisk) Exists() bool           { return true }
func (d *tdisk) Create() error           { return nil }
func (d *tdisk) Mount() error            { return nil }
func (d *tdisk) Unmount() error          { return nil }
func (d *tdisk) CleanArtifacts()         {}
func (d *tdisk) BackingFilePath() string { return "/tmp/test.bin" }

type tgadget struct{}

func (g *tgadget) Enable(string) error { return nil }
func (g *tgadget) Disable() error      { return nil }
func (g *tgadget) WaitForIdle() error  { return nil }

type tarchive struct{}

func (a *tarchive) IsReachable() bool                                { return false }
func (a *tarchive) MountArchive() error                              { return nil }
func (a *tarchive) UnmountArchive()                                  {}
func (a *tarchive) ArchiveClips(context.Context) (int, int64, error) { return 0, 0, nil }
func (a *tarchive) ManageFreeSpace()                                 {}

type tsystem struct{}

func (s *tsystem) SetLED(string) {}
func (s *tsystem) SyncTime()     {}

type tkeep struct{}

func (k *tkeep) Send(context.Context, string) {}

type tnotify struct{}

func (n *tnotify) Send(context.Context, webhook.Event) {}

func TestStatusEndpoint(t *testing.T) {
	m := state.New(webTestDeps())
	s := NewServer(m, "test-version", "/tmp/test-config.yaml")

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["version"] != "test-version" {
		t.Errorf("expected test-version, got %v", result["version"])
	}
}

func TestGetConfigEndpoint(t *testing.T) {
	m := state.New(webTestDeps())
	s := NewServer(m, "test", "/tmp/test.yaml")

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	s.handleGetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBLEStatusEndpoint(t *testing.T) {
	m := state.New(webTestDeps())
	s := NewServer(m, "test", "/tmp/test.yaml")

	req := httptest.NewRequest("GET", "/api/ble/status", nil)
	w := httptest.NewRecorder()
	s.handleBLEStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["keys_exist"] != false {
		t.Errorf("expected keys_exist=false")
	}
}
