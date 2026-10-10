package knowledge

import (
	"encoding/base64"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Search struct {
	Q       string
	Project *uuid.UUID
	Branch  *string
	Path    string
	Limit   int
	Offset  int
}

type SearchInput struct {
	Q       string
	Project *uuid.UUID
	Branch  *string
	Path    string
	Limit   *int
	Cursor  string
	Mode    string
}

func ValidateSearch(in SearchInput) (Search, error) {
	q := strings.TrimSpace(in.Q)
	if q == "" || utf8.RuneCountInString(q) > 200 || !utf8.ValidString(q) || strings.ContainsRune(q, 0) ||
		strings.ContainsRune(in.Path, 0) || len(in.Path) > 1000 {
		return Search{}, InvalidSearch
	}
	s := Search{Q: q, Project: in.Project, Branch: in.Branch, Path: in.Path, Limit: 20}
	if in.Limit != nil {
		if *in.Limit < 1 || *in.Limit > 50 {
			return Search{}, InvalidSearch
		}
		s.Limit = *in.Limit
	}
	if in.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		n, convErr := strconv.Atoi(strings.TrimPrefix(string(raw), "o:"))
		if err != nil || convErr != nil || !strings.HasPrefix(string(raw), "o:") || n < 0 || n > 100000 {
			return Search{}, InvalidSearch
		}
		s.Offset = n
	}
	return s, nil
}

func Cursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(offset)))
}

func ExcludedWords(q string) string {
	var out []string
	for _, field := range strings.Fields(q) {
		if w, ok := strings.CutPrefix(field, "-"); ok && strings.Trim(w, `"`) != "" {
			out = append(out, strings.Trim(w, `"`))
		}
	}
	return strings.Join(out, " or ")
}

func PrefixQuery(q string) (pos, neg string, ok bool) {
	if strings.Contains(q, `"`) {
		return "", "", false
	}
	var plus, minus []string
	for _, field := range strings.Fields(strings.ToLower(q)) {
		if field == "or" {
			return "", "", false
		}
		negated := strings.HasPrefix(field, "-")
		for _, w := range strings.FieldsFunc(strings.TrimPrefix(field, "-"), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			if utf8.RuneCountInString(w) < 2 {
				continue
			}
			if negated {
				minus = append(minus, "'"+w+"':*")
			} else {
				plus = append(plus, "'"+w+"':*")
			}
		}
	}
	return strings.Join(plus, " & "), strings.Join(minus, " | "), true
}

func PathWords(q string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(q)) {
		w = strings.Trim(w, `"`)
		if w == "" || w == "or" || strings.HasPrefix(w, "-") {
			continue
		}
		out = append(out, w)
	}
	return out
}

const (
	MarkStart = ""
	MarkStop  = ""
)

type Segment struct {
	Text  string `json:"text"`
	Match bool   `json:"match"`
}

func Segments(fragment string) []Segment {
	out := []Segment{}
	for fragment != "" {
		i := strings.Index(fragment, MarkStart)
		if i < 0 {
			out = append(out, Segment{Text: fragment})
			break
		}
		if i > 0 {
			out = append(out, Segment{Text: fragment[:i]})
		}
		rest := fragment[i+len(MarkStart):]
		j := strings.Index(rest, MarkStop)
		if j < 0 {
			out = append(out, Segment{Text: rest, Match: true})
			break
		}
		if j > 0 {
			out = append(out, Segment{Text: rest[:j], Match: true})
		}
		fragment = rest[j+len(MarkStop):]
	}
	return out
}
