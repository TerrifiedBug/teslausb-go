package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestHubNewHub(t *testing.T) {
	h := NewHub()
	if h.clients == nil {
		t.Error("clients map should be initialized")
	}
}

func TestHubBroadcastEmptyHub(t *testing.T) {
	h := NewHub()
	// Should not panic on empty hub
	h.Broadcast(map[string]string{"test": "data"})

	h.mu.Lock()
	count := len(h.clients)
	h.mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestHubBroadcastRemovesDeadClients(t *testing.T) {
	h := NewHub()

	// Create a real WebSocket server + client using httptest
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		h.mu.Lock()
		h.clients[ws] = true
		h.mu.Unlock()

		// Read until error (keeps handler alive)
		buf := make([]byte, 1024)
		for {
			if _, err := ws.Read(buf); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	// Connect a client, then close it to simulate a dead connection
	wsURL := "ws" + srv.URL[4:] // http -> ws
	ws, err := websocket.Dial(wsURL, "", srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Wait for the handler to register the client
	for i := 0; i < 100; i++ {
		h.mu.Lock()
		n := len(h.clients)
		h.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Close the client connection — next write will fail
	ws.Close()

	// Allow TCP close to propagate to the server side
	time.Sleep(50 * time.Millisecond)

	// Broadcast should detect the dead connection and remove it
	h.Broadcast(map[string]string{"test": "data"})

	h.mu.Lock()
	remaining := len(h.clients)
	h.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected dead client to be removed, got %d remaining", remaining)
	}
}

// Ensure json.Marshal works for Broadcast payloads
func TestBroadcastPayloadMarshal(t *testing.T) {
	data := map[string]any{"type": "state", "state": "idle"}
	_, err := json.Marshal(data)
	if err != nil {
		t.Errorf("expected payload to marshal: %v", err)
	}
}
