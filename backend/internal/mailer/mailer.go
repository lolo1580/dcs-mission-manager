// Package mailer sends the small set of transactional emails the server needs.
// Password recovery is the only message today, so the package stays deliberately
// small and uses net/smtp with STARTTLS.
package mailer

import (
	"errors"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Sender is one SMTP configuration. An empty Host disables sending.
type Sender struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

// Configured reports whether a host is set.
func (s Sender) Configured() bool { return strings.TrimSpace(s.Host) != "" }

// SendPasswordReset sends the reset link to one account.
func (s Sender) SendPasswordReset(to, username, link string, ttl time.Duration) error {
	if !s.Configured() {
		return errors.New("email recovery is not configured")
	}
	if !validAddress(to) {
		return errors.New("invalid recipient address")
	}
	from := strings.TrimSpace(s.From)
	envelope := from
	if address, err := mail.ParseAddress(from); err == nil {
		envelope = address.Address
	}
	if !validAddress(envelope) {
		return errors.New("invalid sender address")
	}

	minutes := int(ttl.Minutes())
	if minutes <= 0 {
		minutes = 30
	}
	subject := "DCS Manager password reset"
	body := fmt.Sprintf("Hello %s,\r\n\r\n"+
		"A password reset was requested for your DCS Manager account.\r\n"+
		"Open this link within %d minutes:\r\n\r\n%s\r\n\r\n"+
		"If you did not request it, you can ignore this message.\r\n",
		username, minutes, link)
	message := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" + body

	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Password, s.Host)
	}
	address := fmt.Sprintf("%s:%d", s.Host, s.Port)
	return smtp.SendMail(address, auth, envelope, []string{to}, []byte(message))
}

func validAddress(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}
