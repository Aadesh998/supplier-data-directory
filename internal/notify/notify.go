// Package notify sends transactional emails. It never sends marketing or bulk mail.
package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/*.txt
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.txt"))

// Message is one email.
type Message struct {
	To       string
	Template string // file name without .txt
	Data     map[string]any
}

// Mailer delivers messages.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// Render produces subject and body. Templates start with "Subject: ..." on the first line.
func Render(m Message) (subject, body string, err error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, m.Template+".txt", m.Data); err != nil {
		return "", "", err
	}
	text := buf.String()
	first, rest, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "Subject: ") {
		return "", "", fmt.Errorf("template %s lacks a Subject line", m.Template)
	}
	return strings.TrimPrefix(first, "Subject: "), strings.TrimLeft(rest, "\n"), nil
}

// LogMailer logs that an email would be sent, without recipient address or body (development).
type LogMailer struct{ Log *slog.Logger }

// Send implements Mailer.
func (l LogMailer) Send(ctx context.Context, m Message) error {
	subject, _, err := Render(m)
	if err != nil {
		return err
	}
	domain := ""
	if at := strings.LastIndex(m.To, "@"); at >= 0 {
		domain = m.To[at+1:]
	}
	l.Log.InfoContext(ctx, "email (not sent: SMTP not configured)", "template", m.Template, "subject", subject, "recipient_domain", domain)
	return nil
}

// SMTPMailer sends via SMTP with STARTTLS when offered.
type SMTPMailer struct {
	Host, Username, Password, From string
	Port                           int
}

// Send implements Mailer.
func (s SMTPMailer) Send(ctx context.Context, m Message) error {
	subject, body, err := Render(m)
	if err != nil {
		return err
	}
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("invalid MAIL_FROM: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n", from.String(), to.String(),
		mime.QEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z))
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\nAuto-Submitted: auto-generated\r\n\r\n")
	msg.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg.Bytes()); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// Async sends in the background with a timeout; failures are logged, never surfaced to users.
func Async(log *slog.Logger, mailer Mailer, m Message) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := mailer.Send(ctx, m); err != nil {
			log.Error("email send failed", "template", m.Template, "err", err)
		}
	}()
}
