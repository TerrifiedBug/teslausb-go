package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSend(t *testing.T) {
	var received Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := Send(context.Background(), srv.URL, Event{
		Event:   "test_event",
		Message: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.Event != "test_event" {
		t.Errorf("expected test_event, got %s", received.Event)
	}
}

func TestSendEmptyURL(t *testing.T) {
	err := Send(context.Background(), "", Event{Event: "test"})
	if err != nil {
		t.Error("empty URL should be a no-op")
	}
}

func TestSendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	err := Send(context.Background(), srv.URL, Event{Event: "test"})
	if err == nil {
		t.Error("expected error for 500 status")
	}
}
