package identity

import (
	"net/mail"
	"strings"
)

// NormalizeLoginEmail accepts only a mailbox address, never a display-name
// wrapper. Historical profile email is the current read-only login source.
func NormalizeLoginEmail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 254 {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || !strings.Contains(value, "@") {
		return ""
	}
	return value
}

// EmailBinding intentionally advertises no mutation capability. A future SMTP
// verification flow must explicitly replace this historical-email policy.
type EmailBinding struct {
	Email     string `json:"email"`
	Verified  bool   `json:"verified"`
	Available bool   `json:"available"`
}

func HistoricalEmailBinding(email string) EmailBinding {
	email = strings.TrimSpace(email)
	return EmailBinding{Email: email, Verified: email != "", Available: false}
}
