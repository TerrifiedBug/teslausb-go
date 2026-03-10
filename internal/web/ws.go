package web

import (
	"encoding/json"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
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

	h.mu.Lock()
	defer h.mu.Unlock()

	var dead []*websocket.Conn
	for ws := range h.clients {
		if _, err := ws.Write(msg); err != nil {
			dead = append(dead, ws)
		}
	}
	for _, ws := range dead {
		delete(h.clients, ws)
		ws.Close()
	}
}
