package webui

import (
	"mime"
	"path/filepath"
)

func init() {
	for ext, typ := range map[string]string{
		".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
		".txt": "text/plain; charset=utf-8", ".js": "text/javascript; charset=utf-8",
		".mjs": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8",
		".json": "application/json", ".map": "application/json", ".svg": "image/svg+xml",
		".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
		".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon",
		".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
		".xml": "application/xml", ".webmanifest": "application/manifest+json",
		".wasm": "application/wasm", ".pdf": "application/pdf",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			panic(err)
		}
	}
}

func ContentType(fileName string) string {
	if t := mime.TypeByExtension(filepath.Ext(fileName)); t != "" {
		return t
	}
	return "application/octet-stream"
}
