package notify

import (
	"context"
	"testing"

	"github.com/teslausb-go/teslausb/internal/webhook"
)

func TestSendNoConfig(t *testing.T) {
	// Should not panic with no config loaded
	Send(context.Background(), webhook.Event{Event: "test"})
}
