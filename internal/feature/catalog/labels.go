package catalog

import (
	"strings"
	"unicode/utf8"
)

type LabelQuery struct {
	Q   string
	Key *string
}

type LabelSuggestion struct {
	Value string
	Count int64
}

func ValidateLabelQuery(q LabelQuery) (LabelQuery, error) {
	out := LabelQuery{Q: strings.TrimSpace(q.Q)}
	if utf8.RuneCountInString(out.Q) > 100 {
		return LabelQuery{}, InvalidFilter
	}
	if q.Key != nil {
		k := strings.TrimSpace(*q.Key)
		if k == "" || utf8.RuneCountInString(k) > 63 {
			return LabelQuery{}, InvalidFilter
		}
		out.Key = &k
	}
	return out, nil
}

func LikePrefix(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(q) + "%"
}
