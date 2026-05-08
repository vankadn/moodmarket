// infrastructure/notifications/factory.go
package notifications

import (
	"log"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

func NewNotificationProvider() ports.NotificationProvider {
	provider := os.Getenv("NOTIFICATION_PROVIDER")
	if provider != "" && provider != "log" {
		log.Printf("[notify] unknown NOTIFICATION_PROVIDER=%q, defaulting to log", provider)
	}
	return &logNotificationProvider{}
}
