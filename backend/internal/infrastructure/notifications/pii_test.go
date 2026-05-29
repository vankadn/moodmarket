// infrastructure/notifications/pii_test.go
//
// These maskers are the last line of defence against leaking user emails and
// phone numbers into logs and notification payloads, so every branch is covered.
package notifications

import "testing"

func TestMaskEmail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		email string
		want  string
	}{
		{"empty_is_placeholder", "", "<none>"},
		{"normal_address", "user@example.com", "us**@example.com"},
		{"two_char_local_fully_masked", "ab@example.com", "**@example.com"},
		{"single_char_local_fully_masked", "a@example.com", "*@example.com"},
		{"long_local_keeps_first_two", "alexander@example.com", "al*******@example.com"},
		{"no_at_sign_is_fully_redacted", "not-an-email", "***"},
		{"leading_at_is_fully_redacted", "@example.com", "***"},
		{"subdomain_preserved", "jane@mail.corp.example.com", "ja**@mail.corp.example.com"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := maskEmail(tc.email); got != tc.want {
				t.Errorf("maskEmail(%q) = %q, want %q", tc.email, got, tc.want)
			}
		})
	}
}

func TestMaskPhone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		phone string
		want  string
	}{
		{"empty_is_placeholder", "", "<none>"},
		{"plain_ten_digits", "5551234567", "***-***-4567"},
		{"formatted_us_number", "(555) 123-4567", "***-***-4567"},
		{"e164_number", "+1-555-123-4567", "***-***-4567"},
		{"exactly_four_digits", "4567", "***-***-4567"},
		{"three_digits_too_short", "123", "***"},
		{"no_digits_too_short", "phone", "***"},
		{"non_digits_stripped_before_masking", "ext.4567x", "***-***-4567"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := maskPhone(tc.phone); got != tc.want {
				t.Errorf("maskPhone(%q) = %q, want %q", tc.phone, got, tc.want)
			}
		})
	}
}
