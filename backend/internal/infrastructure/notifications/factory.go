// infrastructure/notifications/factory.go
package notifications

import (
	"log"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewNotificationProvider builds a composite provider that fans every event
// to all active channels. The log provider is always included (stdout).
//
// Channel activation:
//
//	EMAIL_PROVIDER=resend  →  requires RESEND_API_KEY + RESEND_FROM
//	SMS_PROVIDER=twilio    →  requires TWILIO_ACCOUNT_SID + TWILIO_AUTH_TOKEN + TWILIO_FROM
//
// Unset or unrecognised values fall back to log-only for that channel.
// Both channels can be active simultaneously.
func NewNotificationProvider() ports.NotificationProvider {
	providers := []ports.NotificationProvider{&logNotificationProvider{}}

	switch os.Getenv("EMAIL_PROVIDER") {
	case "resend":
		apiKey := os.Getenv("RESEND_API_KEY")
		from := os.Getenv("RESEND_FROM")
		if apiKey == "" || from == "" {
			log.Printf("[notify] EMAIL_PROVIDER=resend but RESEND_API_KEY or RESEND_FROM is unset — email notifications disabled")
		} else {
			log.Printf("[notify] email: Resend (from=%s)", from)
			providers = append(providers, newResendProvider(apiKey, from))
		}
	}

	switch os.Getenv("SMS_PROVIDER") {
	case "twilio":
		sid := os.Getenv("TWILIO_ACCOUNT_SID")
		token := os.Getenv("TWILIO_AUTH_TOKEN")
		from := os.Getenv("TWILIO_FROM")
		if sid == "" || token == "" || from == "" {
			log.Printf("[notify] SMS_PROVIDER=twilio but TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, or TWILIO_FROM is unset — SMS notifications disabled")
		} else {
			log.Printf("[notify] SMS: Twilio (from=%s)", from)
			providers = append(providers, newTwilioProvider(sid, token, from))
		}
	}

	return &compositeNotificationProvider{providers: providers}
}
