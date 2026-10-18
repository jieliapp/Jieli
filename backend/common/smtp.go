package common

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig describes a TLS-protected SMTP connection owned by the administrator.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	TLSMode  string `json:"tls_mode"`
}

func (cfg SMTPConfig) Validate() error {
	sender, err := mail.ParseAddress(cfg.From)
	if err != nil || sender.Address != cfg.From || strings.ContainsAny(cfg.From, "\r\n") {
		return errors.New("valid sender email required")
	}
	if strings.TrimSpace(cfg.Host) == "" || strings.ContainsAny(cfg.Host, "/\r\n") || cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("valid SMTP host and port required")
	}
	if cfg.TLSMode != "tls" && cfg.TLSMode != "starttls" {
		return errors.New("SMTP TLS mode must be tls or starttls")
	}
	return nil
}

// SendSMTP sends a verification message without permitting plaintext credentials.
func SendSMTP(ctx context.Context, cfg SMTPConfig, to, body string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil || recipient.Address != to || strings.ContainsAny(to, "\r\n") {
		return errors.New("invalid recipient")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("SMTP connection failed")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	if cfg.TLSMode == "tls" {
		secure := tls.Client(conn, tlsConfig)
		if err = secure.HandshakeContext(ctx); err != nil {
			return errors.New("SMTP TLS failed")
		}
		conn = secure
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return errors.New("SMTP greeting failed")
	}
	defer client.Close()
	if cfg.TLSMode == "starttls" {
		if err = client.StartTLS(tlsConfig); err != nil {
			return errors.New("SMTP STARTTLS failed")
		}
	}
	if cfg.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return errors.New("SMTP authentication failed")
		}
	}
	if err = client.Mail(cfg.From); err != nil {
		return errors.New("SMTP sender rejected")
	}
	if err = client.Rcpt(to); err != nil {
		return errors.New("SMTP recipient rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return errors.New("SMTP message rejected")
	}
	_, err = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: Jieli email verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", cfg.From, to, body)
	if err != nil {
		return errors.New("SMTP message write failed")
	}
	if err = writer.Close(); err != nil {
		return errors.New("SMTP delivery failed")
	}
	return client.Quit()
}
