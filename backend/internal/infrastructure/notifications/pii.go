package notifications

import "strings"

// maskEmail returns a redacted email safe for logging: "us**@example.com".
func maskEmail(email string) string {
	if email == "" {
		return "<none>"
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 2 {
		return strings.Repeat("*", len(local)) + domain
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + domain
}

// maskPhone returns a redacted phone safe for logging: "***-***-1234".
func maskPhone(phone string) string {
	if phone == "" {
		return "<none>"
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if len(digits) < 4 {
		return "***"
	}
	return "***-***-" + digits[len(digits)-4:]
}
