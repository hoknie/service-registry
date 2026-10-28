package types

import (
	"strings"
	"time"
)

type Letter struct {
	To      string
	Subject string
	Body    string
}

type invitationText struct {
	subject string
	body    string
	roles   map[Role]string
	layout  string
}

var invitationTexts = map[string]invitationText{
	"en": {
		subject: "{inviter} invites you to {org}",
		body: "Hello!\n\n{inviter} invites you to the organization \"{org}\" in the service registry with the role \"{role}\".\n\n" +
			"Accept the invitation: {link}\n\nThe link works until {expires} and only once. If you did not expect this letter, ignore it.\n",
		roles:  map[Role]string{Viewer: "viewer", Editor: "editor", Admin: "admin"},
		layout: "January 2, 2006 15:04 MST",
	},
	"es": {
		subject: "{inviter} te invita a {org}",
		body: "¡Hola!\n\n{inviter} te invita a la organización «{org}» del registro de servicios con el rol «{role}».\n\n" +
			"Acepta la invitación: {link}\n\nEl enlace funciona hasta {expires} y una sola vez. Si no esperabas esta carta, ignórala.\n",
		roles:  map[Role]string{Viewer: "lector", Editor: "editor", Admin: "administrador"},
		layout: "02.01.2006 15:04 MST",
	},
	"ru": {
		subject: "{inviter} приглашает вас в {org}",
		body: "Здравствуйте!\n\n{inviter} приглашает вас в организацию «{org}» в реестре сервисов с ролью «{role}».\n\n" +
			"Принять приглашение: {link}\n\nСсылка действует до {expires} и только один раз. Если вы не ждали этого письма, просто проигнорируйте его.\n",
		roles:  map[Role]string{Viewer: "Просмотр", Editor: "Редактор", Admin: "Администратор"},
		layout: "02.01.2006 15:04 MST",
	},
	"zh": {
		subject: "{inviter} 邀请你加入 {org}",
		body: "你好！\n\n{inviter} 邀请你以「{role}」角色加入服务注册中心的组织「{org}」。\n\n" +
			"接受邀请：{link}\n\n该链接有效期至 {expires}，且只能使用一次。如果你没有预期收到这封信，请忽略它。\n",
		roles:  map[Role]string{Viewer: "查看者", Editor: "编辑者", Admin: "管理员"},
		layout: "2006-01-02 15:04 MST",
	},
}

func InvitationLetter(locale, to, org, inviter string, role Role, link string, expires time.Time) Letter {
	text, ok := invitationTexts[locale]
	if !ok {
		text = invitationTexts["en"]
	}
	r := strings.NewReplacer("{inviter}", inviter, "{org}", org, "{role}", text.roles[role], "{link}", link,
		"{expires}", expires.UTC().Format(text.layout))
	return Letter{To: to, Subject: r.Replace(text.subject), Body: r.Replace(text.body)}
}
