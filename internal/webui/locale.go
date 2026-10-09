package webui

import (
	"strings"

	"golang.org/x/text/language"
)

var Locales = [4]string{"en", "es", "ru", "zh"}

const LocaleCookie = "NEXT_LOCALE"

func PickLocale(acceptLanguage string, ok bool) string {
	if !ok {
		return Locales[0]
	}
	tags, weights, _ := language.ParseAcceptLanguage(acceptLanguage)
	for i, tag := range tags {
		if weights[i] <= 0 {
			continue
		}
		base, _ := tag.Base()
		if l, ok := asLocale(base.String()); ok {
			return l
		}
	}
	return Locales[0]
}

func pathLocale(first string) (string, bool) {
	stem := strings.TrimSuffix(strings.TrimSuffix(first, ".html"), ".txt")
	return asLocale(stem)
}

func asLocale(v string) (string, bool) {
	for _, l := range Locales {
		if l == v {
			return l, true
		}
	}
	return "", false
}
