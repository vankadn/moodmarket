// infrastructure/notifications/factory.go
package notifications

import (
	"log"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

func NewNotificationProvider() ports.NotificationProvider {
	switch os.Getenv("NOTIFICATION_PROVIDER") {
	case "resend":
		apiKey := os.Getenv("RESEND_API_KEY")
		from := os.Getenv("RESEND_FROM")
		if apiKey == "" || from == "" {
			log.Printf("[notify] NOTIFICATION_PROVIDER=resend but RESEND_API_KEY or RESEND_FROM is unset, defaulting to log")
			return &logNotificationProvider{}
		}
		log.Printf("[notify] using Resend email provider (from=%s)", from)
		return newResendProvider(apiKey, from)
	case "twilio":
		sid := os.Getenv("TWILIO_ACCOUNT_SID")
		token := os.Getenv("TWILIO_AUTH_TOKEN")
		from := os.Getenv("TWILIO_FROM")
		if sid == "" || token == "" || from == "" {
			log.Printf("[notify] NOTIFICATION_PROVIDER=twilio but TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, or TWILIO_FROM is unset, defaulting to log")
			return &logNotificationProvider{}
		}
		log.Printf("[notify] using Twilio SMS provider (from=%s)", from)
		return newTwilioProvider(sid, token, from)
	default:
		return &logNotificationProvider{}
	}
}
