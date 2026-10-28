package tests

import (
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/feature/membership/clients/mailer"
	"svc-registry/pkg/testing/smtpfake"
)

func invitationsPath(org string) string { return nodePath(org) + "/invitations" }

func startWithMail(t *testing.T, smtp *smtpfake.Server, extra ...string) *testApp {
	t.Helper()
	m := mailer.New(mailer.Options{Server: &url.URL{Scheme: "smtp", Host: smtp.Addr}, From: &mail.Address{Address: "robot@example.com"},
		Timeout: 2 * time.Second, RootCAs: smtp.RootCAs()})
	return startAppWith(t, []svcapp.Option{svcapp.WithMailer(m)}, append([]string{"PUBLIC_URL", "https://registry.example"}, extra...)...)
}

var tokenInLink = regexp.MustCompile(`svci_[A-Za-z0-9]+`)

func letterText(t *testing.T, l smtpfake.Letter) (string, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(l.Data))
	must(t, err)
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	must(t, err)
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	must(t, err)
	return subject, string(body)
}

func TestInvitationsAreSentListedResentAndRevoked(t *testing.T) {
	t.Parallel()
	smtp := smtpfake.Start(t)
	a := startWithMail(t, smtp)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("bob@example.com", false)
	a.grant(admin, acme, "user", "bob@example.com", "viewer")
	bob := a.session("bob@example.com", password)

	r := a.send("POST", invitationsPath(acme), admin, obj{"email": "New@Example.com", "role": "editor", "locale": "ru"})
	eq(t, r.status, 201, r.text())
	issued := r.json(t)
	eq(t, issued["mail"], any("sent"))
	link := issued["link"].(string)
	eq(t, strings.HasPrefix(link, "https://registry.example/ru/invite?token=svci_"), true, link)
	eq(t, at(issued, "invitation", "email"), any("new@example.com"))
	eq(t, at(issued, "invitation", "state"), any("pending"))
	letters := smtp.Letters()
	eq(t, len(letters), 1)
	eq(t, letters[0].To[0], "new@example.com")
	subject, body := letterText(t, letters[0])
	contains(t, subject, "acme")
	contains(t, body, link)
	contains(t, body, "Редактор")

	eq(t, code(t, a.send("POST", invitationsPath(acme), admin, obj{"email": "new@example.com", "role": "viewer", "locale": "en"})),
		any("conflict.invitation_pending"))
	eq(t, code(t, a.send("POST", invitationsPath(acme), admin, obj{"email": "bob@example.com", "locale": "en"})), any("conflict.already_member"))
	eq(t, code(t, a.send("POST", invitationsPath(acme), admin, obj{"email": "x@example.com", "locale": "de"})), any("validation.invalid_invitation"))
	eq(t, a.send("POST", invitationsPath(acme), bob, obj{"email": "y@example.com", "locale": "en"}).status, 403)
	eq(t, fieldsOf(t, a.get(invitationsPath(acme), admin).json(t), "email"), "new@example.com")

	old := tokenInLink.FindString(link)
	iid := at(issued, "invitation", "id").(string)
	r = a.send("POST", invitationsPath(acme)+"/"+iid+"/resend", admin, obj{"locale": "en"})
	eq(t, r.status, 201, r.text())
	fresh := tokenInLink.FindString(r.json(t)["link"].(string))
	eq(t, fresh != old, true)
	eq(t, a.get("/api/v1/invitations/"+old, "").status, 404)
	view := a.get("/api/v1/invitations/"+fresh, "").json(t)
	eq(t, view["account"], any("new"))
	eq(t, at(view, "organization", "name"), any("acme"))
	eq(t, view["state"], any("pending"))
	eq(t, len(smtp.Letters()), 2)

	eq(t, a.call("DELETE", invitationsPath(acme)+"/"+iid, admin).status, 204)
	eq(t, a.get("/api/v1/invitations/"+fresh, "").json(t)["state"], any("revoked"))
	eq(t, len(list(t, a.get(invitationsPath(acme), admin).json(t), "items")), 0)
	eq(t, code(t, a.send("POST", invitationsPath(acme)+"/"+iid+"/resend", admin, obj{})), any("conflict.invitation_closed"))
}

func TestInvitationsWithoutMailAndWithAFailingServer(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	r := a.send("POST", invitationsPath(acme), admin, obj{"email": "new@example.com", "role": "editor", "locale": "en"})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["mail"], any("disabled"))
	eq(t, strings.HasPrefix(r.json(t)["link"].(string), "/en/invite?token=svci_"), true)

	smtp := smtpfake.Start(t)
	smtp.RefuseData(true)
	b := startWithMail(t, smtp)
	root := b.admin()
	org := b.nodeID(root, "organization", "", "globex")
	r = b.send("POST", invitationsPath(org), root, obj{"email": "new@example.com", "locale": "en"})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["mail"], any("failed"))
	contains(t, r.json(t)["link"].(string), "https://registry.example/en/invite?token=svci_")
}

func TestInvitationsAreAcceptedOrRegistered(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	invite := func(email, role string) string {
		r := a.send("POST", invitationsPath(acme), admin, obj{"email": email, "role": role, "locale": "en"})
		eq(t, r.status, 201, r.text())
		return tokenInLink.FindString(r.json(t)["link"].(string))
	}

	token := invite("new@example.com", "editor")
	r := a.send("POST", "/api/v1/invitations/"+token+"/register", "", obj{"name": "New Person", "password": "a-long-password-1"})
	eq(t, r.status, 201, r.text())
	session := cookieOf(t, r)
	eq(t, a.get("/api/v1/auth/me", session).json(t)["email"], any("new@example.com"))
	members := a.get(membersPath(acme), admin).json(t)
	var found obj
	for _, m := range list(t, members, "items") {
		if at(m, "user", "email") == any("new@example.com") {
			found = m.(map[string]any)
		}
	}
	eq(t, found["source"], any("invitation"))
	eq(t, found["role"], any("editor"))
	eq(t, code(t, a.send("POST", "/api/v1/invitations/"+token+"/register", "", obj{"name": "Again", "password": "a-long-password-1"})),
		any("invitation.not_pending"))

	a.user("ann@example.com", false)
	ann := a.session("ann@example.com", password)
	token = invite("bob@example.com", "viewer")
	eq(t, code(t, a.send("POST", "/api/v1/invitations/"+token+"/accept", ann, obj{})), any("invitation.email_mismatch"))
	eq(t, a.get("/api/v1/invitations/"+token, "").json(t)["state"], any("pending"))
	token = invite("ann@example.com", "editor")
	eq(t, a.get("/api/v1/invitations/"+token, "").json(t)["account"], any("exists"))
	eq(t, code(t, a.send("POST", "/api/v1/invitations/"+token+"/register", "", obj{"name": "Ann", "password": "a-long-password-1"})),
		any("invitation.sign_in_required"))
	r = a.send("POST", "/api/v1/invitations/"+token+"/accept", ann, obj{})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "organization", "name"), any("acme"))
	eq(t, a.get(nodePath(acme), ann).status, 200)
}

func TestInvitationLinksAreRateLimited(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	status := 0
	for range 40 {
		status = a.get("/api/v1/invitations/svci_nothing", "").status
	}
	eq(t, status, 429)
}
