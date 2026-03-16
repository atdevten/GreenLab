package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSender implements application.EmailSender using net/smtp.
type SMTPSender struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// Config holds SMTP configuration.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// NewSMTPSender creates a new SMTPSender.
func NewSMTPSender(cfg Config) *SMTPSender {
	return &SMTPSender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}
}

// Send sends an email via SMTP.
func (s *SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	addr := s.host + ":" + s.port

	msg := buildMessage(s.from, to, subject, body)

	tlsConfig := &tls.Config{
		ServerName: s.host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	defer w.Close()

	if _, err := fmt.Fprint(w, msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}

	return nil
}

func buildMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}
