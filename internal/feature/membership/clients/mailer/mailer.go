package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	membershiptypes "svc-registry/internal/feature/membership/types"
)

type Options struct {
	Server   *url.URL
	From     *mail.Address
	Username string
	Password string
	Timeout  time.Duration
	RootCAs  *x509.CertPool
}

type MailerClient struct{ o Options }

func New(o Options) *MailerClient { return &MailerClient{o: o} }

func (c *MailerClient) Enabled() bool { return c != nil && c.o.Server != nil && c.o.From != nil }

func (c *MailerClient) Send(ctx context.Context, l membershiptypes.Letter) error {
	if !c.Enabled() {
		return errors.New("mail is not configured")
	}
	host := c.o.Server.Hostname()
	port := c.o.Server.Port()
	implicit := c.o.Server.Scheme == "smtps"
	if port == "" {
		port = "587"
		if implicit {
			port = "465"
		}
	}
	deadline := time.Now().Add(c.o.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := &net.Dialer{Deadline: deadline}
	tlsConfig := &tls.Config{ServerName: host, RootCAs: c.o.RootCAs, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	if implicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	}
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	_ = conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("greeting: %w", err)
	}
	defer func() { _ = client.Close() }()
	if !implicit {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	if c.o.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", c.o.Username, c.o.Password, host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(c.o.From.Address); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(l.To); err != nil {
		return fmt.Errorf("rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(c.compose(l)); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	return client.Quit()
}

func (c *MailerClient) compose(l membershiptypes.Letter) []byte {
	var b bytes.Buffer
	domain := c.o.From.Address[strings.LastIndex(c.o.From.Address, "@")+1:]
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", c.o.From.String())
	header("To", l.To)
	header("Subject", mime.QEncoding.Encode("utf-8", l.Subject))
	header("Date", time.Now().UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(strings.ReplaceAll(l.Body, "\n", "\r\n")))
	_ = qp.Close()
	return b.Bytes()
}
