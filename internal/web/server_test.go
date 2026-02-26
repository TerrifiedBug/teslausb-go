package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslausb-go/teslausb/internal/state"
)

func TestStatusEndpoint(t *testing.T) {
	m := state.New()
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
	m := state.New()
	s := NewServer(m, "test", "/tmp/test.yaml")

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	s.handleGetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBLEStatusEndpoint(t *testing.T) {
	m := state.New()
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
