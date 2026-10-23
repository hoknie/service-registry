package types

import (
	"net/url"
	"regexp"
	"unicode/utf8"
)

const (
	MaxContacts       = 10
	MaxContactURLLen  = 2048
	ContactKindEmail  = "email"
	ContactKindChat   = "chat"
	ContactKindURL    = "url"
	ContactKindPhone  = "phone"
	contactPhoneMatch = `^\+[0-9 -]{5,20}$`
)

var phonePattern = regexp.MustCompile(contactPhoneMatch)

type Contact struct {
	Kind  string
	Value string
	Label *string
}

type GroupContacts struct {
	Contacts  []Contact
	OnCallURL *string
}

func ValidateGroupContacts(in GroupContacts) (GroupContacts, error) {
	if len(in.Contacts) > MaxContacts {
		return GroupContacts{}, InvalidContacts
	}
	out := GroupContacts{Contacts: make([]Contact, 0, len(in.Contacts))}
	for _, c := range in.Contacts {
		v, err := validateContact(c)
		if err != nil {
			return GroupContacts{}, err
		}
		out.Contacts = append(out.Contacts, v)
	}
	if in.OnCallURL != nil {
		u := trim(*in.OnCallURL)
		if !webAddress(u, true) {
			return GroupContacts{}, InvalidOnCallURL
		}
		out.OnCallURL = &u
	}
	return out, nil
}

func validateContact(c Contact) (Contact, error) {
	value := trim(c.Value)
	var ok bool
	switch c.Kind {
	case ContactKindEmail:
		var err error
		value, err = ValidateEmail(value)
		ok = err == nil
	case ContactKindChat, ContactKindURL:
		ok = webAddress(value, false)
	case ContactKindPhone:
		ok = phonePattern.MatchString(value)
	}
	if !ok {
		return Contact{}, InvalidContacts
	}
	out := Contact{Kind: c.Kind, Value: value}
	if c.Label != nil {
		label, valid := validName(*c.Label)
		if !valid {
			return Contact{}, InvalidContacts
		}
		out.Label = &label
	}
	return out, nil
}

func webAddress(raw string, httpsOnly bool) bool {
	if raw == "" || utf8.RuneCountInString(raw) > MaxContactURLLen {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		return !httpsOnly
	}
	return false
}
