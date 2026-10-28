package types

import (
	"bytes"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"
)

const MaxPathBytes = 512

var ignoredNames = map[string]bool{".git": true, ".DS_Store": true, "__MACOSX": true}

func CleanPath(raw string) (string, error) {
	if raw == "" || len(raw) > MaxPathBytes || !utf8.ValidString(raw) || strings.ContainsAny(raw, "\\\x00") ||
		strings.HasPrefix(raw, "/") {
		return "", ArchiveInvalid
	}
	for _, part := range strings.Split(raw, "/") {
		if part == "" || part == "." || part == ".." {
			return "", ArchiveInvalid
		}
	}
	return raw, nil
}

func Ignored(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if ignoredNames[part] {
			return true
		}
	}
	return false
}

func KindOf(p string, data []byte) (FileKind, string) {
	ext := strings.ToLower(path.Ext(p))
	if ext == ".svg" && bytes.Contains(data, []byte("<svg")) {
		return KindImage, "image/svg+xml"
	}
	switch ct := http.DetectContentType(data); ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return KindImage, ct
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return KindBinary, "application/octet-stream"
	}
	if ext == ".md" || ext == ".markdown" {
		return KindMarkdown, "text/markdown; charset=utf-8"
	}
	return KindText, "text/plain; charset=utf-8"
}

func ImageType(data []byte) (string, bool) {
	switch ct := http.DetectContentType(data); ct {
	case "image/png", "image/jpeg", "image/webp":
		return ct, true
	}
	head := data[:min(len(data), 1024)]
	if utf8.Valid(head) && bytes.Contains(bytes.ToLower(head), []byte("<svg")) {
		return "image/svg+xml", true
	}
	return "", false
}

func RelativeTo(dir, target string) string {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "/") {
		return ""
	}
	joined := path.Clean(path.Join(dir, target))
	if joined == "." || strings.HasPrefix(joined, "../") || joined == ".." {
		return ""
	}
	return joined
}
