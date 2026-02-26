package notify

import (
	"context"
	"log"

	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/webhook"
)

func Send(ctx context.Context, event webhook.Event) {
	cfg := config.Get()
	if cfg == nil || cfg.Notifications.WebhookURL == "" {
		return
	}
	if err := webhook.Send(ctx, cfg.Notifications.WebhookURL, event); err != nil {
		log.Printf("notification failed: %v", err)
	}
}
