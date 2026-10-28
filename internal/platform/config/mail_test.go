package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"svc-registry/internal/platform/config"
)

func TestMailDefaultsSendNothing(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.MailConfig](lookup())
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() || c.TimeoutSecs != 10 || c.InviteTTLHours != 168 {
		t.Fatalf("defaults %+v", c)
	}
}

func TestMailReadsServerSenderAndPassword(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.MailConfig](lookup("SMTP_URL", "smtp://robot:s3cret@mail.example:587", "SMTP_FROM", "Registry <robot@example.com>"))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled() || c.Server.Host != "mail.example:587" || c.Username != "robot" || c.Password != "s3cret" || c.From.Address != "robot@example.com" {
		t.Fatalf("got %+v", c)
	}
	file := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = config.Load[config.MailConfig](lookup("SMTP_URL", "smtps://robot@mail.example", "SMTP_FROM", "robot@example.com", "SMTP_PASSWORD", "file:"+file))
	if err != nil {
		t.Fatal(err)
	}
	if c.Password != "from-file" {
		t.Fatalf("password %q", c.Password)
	}
}

func TestMailRefusesBadSettingsWithoutLeaking(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pairs []string
		want  []string
	}{
		{[]string{"SMTP_URL", "smtp://mail.example:587"}, []string{"SMTP_FROM"}},
		{[]string{"SMTP_URL", "http://mail.example", "SMTP_FROM", "a@b.c"}, []string{"SMTP_URL"}},
		{[]string{"SMTP_URL", "smtp://u:inline-secret@mail.example", "SMTP_FROM", "a@b.c", "SMTP_PASSWORD", "other"}, []string{"SMTP_PASSWORD"}},
		{[]string{"SMTP_URL", "smtp://mail.example", "SMTP_FROM", "not an address"}, []string{"SMTP_FROM"}},
		{[]string{"SMTP_TIMEOUT_SECS", "90"}, []string{"SMTP_TIMEOUT_SECS"}},
		{[]string{"INVITE_TTL_HOURS", "0"}, []string{"INVITE_TTL_HOURS"}},
	} {
		_, err := config.Load[config.Config](lookup(append([]string{"DATABASE_URL", dbURL}, c.pairs...)...))
		if got := vars(t, err); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%v: %v", c.pairs, got)
		}
		if err != nil && strings.Contains(err.Error(), "inline-secret") {
			t.Fatalf("password leaked: %v", err)
		}
	}
}
