package types

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateGroupContacts(t *testing.T) {
	t.Parallel()
	label := "#platform"
	long := "https://chat.example/" + strings.Repeat("a", MaxContactURLLen)
	for name, tc := range map[string]struct {
		in   GroupContacts
		want error
	}{
		"empty":           {GroupContacts{}, nil},
		"chat with label": {GroupContacts{Contacts: []Contact{{Kind: "chat", Value: "https://chat.example/p", Label: &label}}}, nil},
		"email":           {GroupContacts{Contacts: []Contact{{Kind: "email", Value: " Team@Example.com "}}}, nil},
		"http url":        {GroupContacts{Contacts: []Contact{{Kind: "url", Value: "http://wiki.example/team"}}}, nil},
		"phone":           {GroupContacts{Contacts: []Contact{{Kind: "phone", Value: "+7 999 123-45-67"}}}, nil},
		"on-call":         {GroupContacts{OnCallURL: new("https://oncall.example/p")}, nil},
		"unknown kind":    {GroupContacts{Contacts: []Contact{{Kind: "fax", Value: "x"}}}, InvalidContacts},
		"bad email":       {GroupContacts{Contacts: []Contact{{Kind: "email", Value: "nobody"}}}, InvalidContacts},
		"javascript chat": {GroupContacts{Contacts: []Contact{{Kind: "chat", Value: "javascript:alert(1)"}}}, InvalidContacts},
		"relative url":    {GroupContacts{Contacts: []Contact{{Kind: "url", Value: "/team"}}}, InvalidContacts},
		"long url":        {GroupContacts{Contacts: []Contact{{Kind: "url", Value: long}}}, InvalidContacts},
		"bad phone":       {GroupContacts{Contacts: []Contact{{Kind: "phone", Value: "123"}}}, InvalidContacts},
		"empty label":     {GroupContacts{Contacts: []Contact{{Kind: "url", Value: "https://a.example", Label: new("")}}}, InvalidContacts},
		"too many":        {GroupContacts{Contacts: make([]Contact, MaxContacts+1)}, InvalidContacts},
		"http on-call":    {GroupContacts{OnCallURL: new("http://oncall.example")}, InvalidOnCallURL},
		"script on-call":  {GroupContacts{OnCallURL: new("javascript:alert(1)")}, InvalidOnCallURL},
		"word on-call":    {GroupContacts{OnCallURL: new("oncall")}, InvalidOnCallURL},
	} {
		_, err := ValidateGroupContacts(tc.in)
		if !errors.Is(err, tc.want) && !(err == nil && tc.want == nil) {
			t.Errorf("%s: err %v, want %v", name, err, tc.want)
		}
	}
	got, err := ValidateGroupContacts(GroupContacts{Contacts: []Contact{{Kind: "email", Value: " Team@Example.com "}}})
	if err != nil || got.Contacts[0].Value != "team@example.com" {
		t.Errorf("email normalised to %q, %v", got.Contacts[0].Value, err)
	}
}
