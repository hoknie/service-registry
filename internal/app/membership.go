package app

import (
	"svc-registry/internal/feature/membership/clients/mailer"
	membershipports "svc-registry/internal/feature/membership/ports"
	membershipservices "svc-registry/internal/feature/membership/services"
	"svc-registry/internal/platform/auth"
)

func (a *App) buildMembership(o overrides) {
	a.Membership = membershipservices.New(membershipservices.Deps{DB: a.DB, Config: a.Config, Catalog: a.Catalog, Access: a.Access,
		Mailer: mailerOf(o, a), Limiter: auth.NewPublicDocsLimiter(30)})
}

func mailerOf(o overrides, a *App) membershipports.MailerPort {
	if o.mailer != nil {
		return o.mailer
	}
	m := a.Config.Mail
	return mailer.New(mailer.Options{Server: m.Server, From: m.From, Username: m.Username, Password: m.Password, Timeout: m.Timeout(),
		RootCAs: a.Config.Outbound.Options().RootCAs()})
}
