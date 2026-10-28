package mailer

import (
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"net/url"
	"strings"
	"testing"
	"time"

	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/pkg/testing/smtpfake"
)

func client(t *testing.T, s *smtpfake.Server, user string) *MailerClient {
	t.Helper()
	u := &url.URL{Scheme: "smtp", Host: s.Addr}
	return New(Options{Server: u, From: &mail.Address{Name: "Registry", Address: "robot@example.com"}, Username: user,
		Password: "s3cret", Timeout: 2 * time.Second, RootCAs: s.RootCAs()})
}

func body(t *testing.T, data string) (string, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	return subject, string(text)
}

func TestLettersGoOverStartTLSWithAuthInEveryLanguage(t *testing.T) {
	t.Parallel()
	s := smtpfake.Start(t)
	c := client(t, s, "robot")
	expires := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	for _, locale := range membershiptypes.Locales {
		l := membershiptypes.InvitationLetter(locale, "new@example.com", "Acme", "Ann", membershiptypes.Editor,
			"https://registry.example/"+locale+"/invite?token=svci_x", expires)
		if err := c.Send(context.Background(), l); err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
	}
	letters := s.Letters()
	if len(letters) != 4 {
		t.Fatalf("letters %d", len(letters))
	}
	for i, want := range map[int][2]string{0: {"Ann invites you to Acme", "editor"}, 2: {"Ann приглашает вас в Acme", "Редактор"},
		3: {"Ann 邀请你加入 Acme", "编辑者"}} {
		l := letters[i]
		if !l.TLS || l.Auth != ":robot:s3cret" || l.From != "robot@example.com" || l.To[0] != "new@example.com" {
			t.Fatalf("envelope %+v", l)
		}
		subject, text := body(t, l.Data)
		if subject != want[0] || !strings.Contains(text, want[1]) || !strings.Contains(text, "/invite?token=svci_x") {
			t.Fatalf("%d: %q %q", i, subject, text)
		}
	}
}

func TestRefusalsAndTimeoutsAreErrors(t *testing.T) {
	t.Parallel()
	s := smtpfake.Start(t)
	s.RefuseData(true)
	l := membershiptypes.Letter{To: "a@example.com", Subject: "x", Body: "y"}
	if err := client(t, s, "").Send(context.Background(), l); err == nil {
		t.Fatal("refused data must fail")
	}
	slow := smtpfake.Start(t)
	slow.SetDelay(3 * time.Second)
	c := client(t, slow, "")
	c.o.Timeout = 300 * time.Millisecond
	start := time.Now()
	if err := c.Send(context.Background(), l); err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("timeout: %v after %v", err, time.Since(start))
	}
	if (&MailerClient{}).Enabled() {
		t.Fatal("an empty client sends nothing")
	}
}
