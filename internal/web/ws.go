package web

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) Run() {
	// Hub is passive — broadcasts are triggered externally
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	websocket.Handler(func(ws *websocket.Conn) {
		h.mu.Lock()
		h.clients[ws] = true
		h.mu.Unlock()

		defer func() {
			h.mu.Lock()
			delete(h.clients, ws)
			h.mu.Unlock()
			ws.Close()
		}()

		buf := make([]byte, 1024)
		for {
			if _, err := ws.Read(buf); err != nil {
				return
			}
		}
	}).ServeHTTP(w, r)
}

func (h *Hub) Broadcast(data any) {
	msg, err := json.Marshal(data)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ws := range h.clients {
		if _, err := ws.Write(msg); err != nil {
			log.Printf("ws write error: %v", err)
		}
	}
}
